// Package merchantaudit 每週去各家平台的頁面上確認「這家還有沒有在發推薦碼」。
//
// 目錄裡的服務商是 cmd/appimport 匯進來或後台建的，沒有人回頭看它們是不是還在
// 跑推薦計畫。平台把計畫收掉之後，那一頁在目錄上就變成使用者點進去才發現
// 「根本沒有這回事」的死頁面。
//
// 這支排程只回報，不下架。判斷方式是抓公開頁面比對關鍵字，準確度撐不起自動下架：
// 拿 30 家人工整理過、確定有推薦計畫的服務商實測，只有 4 家判 found，13 家判
// missing、13 家被 403 擋掉判 unreachable —— 誤判率 43%（Airbnb、Notion、Todoist、
// AT&T、Instacart 都在誤判那一堆裡）。原本的設計是連續兩次 missing 就自動下架，
// 靠「架上還有可用碼」當擋牆，但全庫只有 12 個碼散在 6 家，啟用中的服務商有
// 259 家 —— 253 家完全不受保護，直接開下去兩週內約 110 家會被誤殺。
//
// 所以判定只寫進軌跡表，由後台的人看 ListMerchantsFlaggedByCodeAudit 複檢，
// 沒有任何路徑會自動改 is_active。各項取捨仍然一律往「不要誤判」倒：
//
//   - 命中任何一個相關字樣就算 found，關鍵字表刻意收得寬。
//   - 抓不到頁面（403、timeout、SPA 空殼）記 unreachable，不當成證據。
//   - 連續兩次 missing 才進複檢清單。單次誤判（改版、暫時擋爬蟲）不會上榜。
//   - signup_url 指向 App Store／Google Play 的不抓，理由見 check。
//
// 每次檢查都留一列軌跡（見 00018 migration），複檢的依據事後查得到。
package merchantaudit

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// merchant_code_audits.result 的三個值，跟 migration 的 CHECK 對齊。
const (
	resultFound       = "found"
	resultMissing     = "missing"
	resultUnreachable = "unreachable"
)

// 用一般瀏覽器的 UA。理由同 cmd/logobackfill：不少站對沒有 UA 或帶 bot 字樣的
// 請求直接回 403，那會讓「被擋」看起來像「這家沒在發碼」，正是最不能誤判的方向。
const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

const (
	// 每頁最多讀多少 HTML。關鍵字幾乎都在導覽列或頁尾，512KB 綽綽有餘。
	maxPageBytes = 512 << 10
	// 一家抓回來的內容總長短於這個值就當成沒抓到。正常頁面再簡單也有好幾 KB，
	// 這種多半是 SPA 的空殼或擋爬蟲的錯誤頁 —— 內容不在 HTML 裡，比對關鍵字
	// 一定是 missing，那是假的證據。
	minPageBytes = 1 << 10
	// 兩次請求之間的間隔。一輪要打幾百個外站，序列跑加間隔比較不會被當成攻擊，
	// 也不會為了省幾分鐘把自己弄進對方的黑名單。
	requestGap = 300 * time.Millisecond
	// 每家隔幾天重新檢查一次。
	staleDays = 7
	// 第二階段最多再跟幾條首頁上的連結。壓低是刻意的：這是「多看兩眼」，
	// 不是爬整站 —— 一家多打十幾個請求，一輪幾千家就成了對每個網域的小型掃描。
	maxHrefFollow = 2
)

// referralPaths 是推薦計畫最常見的落點。只有第一階段（註冊頁＋首頁）沒命中時
// 才會去抓，首頁就找得到的那些不必多打這幾個請求。
var referralPaths = []string{
	"/referral", "/referrals", "/refer", "/refer-a-friend", "/invite", "/rewards",
}

// hrefRe 從 HTML 撈出可能是推薦頁的連結。只看 href 的值，不解析整份 DOM ——
// 這裡要的只是「有沒有一條路徑長得像推薦頁」，正則就夠，而且不會被壞掉的
// HTML 卡住（目錄裡的站什麼寫法都有）。
var hrefRe = regexp.MustCompile(`(?i)href="([^"]*(?:referral|refer-a-friend|invite|rewards)[^"]*)"`)

// challengeSignatures 認反爬蟲的挑戰頁。
//
// 被擋有兩種形狀：直接回 403／429，狀態碼就分得出來；或者回 200 帶一頁「請稍候，
// 正在驗證您的瀏覽器」。後者是危險的那種 —— 狀態碼正常、長度也夠，整頁比對不到
// 推薦碼字樣，於是被判成 missing，也就是「這家沒在發碼」。那是最不該誤判的方向。
//
// 只收各家 WAF 的專有標記，不收 "access denied" 這種一般網頁也寫得出來的字串：
// 誤認成被擋只會讓結論落在 unreachable（不累積、不進複檢清單），代價比誤判成
// missing 小，但白白丟掉一次判定仍然是損失。
var challengeSignatures = []string{
	"just a moment", "checking your browser", "cf-browser-verification",
	"__cf_chl", "cf_chl_opt", "enable javascript and cookies",
	"px-captcha", "perimeterx", "_incapsula_", "captcha-delivery",
}

// looksBlocked 判斷這次請求是不是被反爬蟲擋下來的。
//
// 只掃開頭 8KB：挑戰頁本來就短，特徵一定在最前面；掃整份 512KB 的正常頁面
// 反而會提高認錯的機率。
func looksBlocked(status int, body []byte) bool {
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return true
	}
	if len(body) == 0 {
		return false
	}
	page := strings.ToLower(string(body[:min(len(body), 8<<10)]))
	for _, sig := range challengeSignatures {
		if strings.Contains(page, sig) {
			return true
		}
	}
	return false
}

// 命中任一個就算這家還在發碼。三種語言都要收 —— 目錄涵蓋 tw / jp / us 的平台，
// 只比對中文會把所有海外平台都判成 missing。
//
// 比對的是原始 HTML 不是純文字，所以連結路徑（href="/referral"）也會命中，
// 那通常比內文更可靠：推薦計畫收掉時，導覽列的連結會跟著消失。
var keywords = []string{
	// 中文（繁簡都收，同一個平台的繁體站與簡體站常常是目錄裡的兩家）
	"推薦碼", "推荐码", "推薦代碼", "推荐代码",
	"邀請碼", "邀请码", "邀請代碼",
	"優惠碼", "优惠码", "折扣碼", "折扣码",
	"推薦獎勵", "推荐奖励", "推薦計畫", "推薦計劃", "推荐计划",
	"好友推薦", "好友推荐", "邀請好友", "邀请好友", "推薦連結", "推荐链接",
	// 日文
	"紹介コード", "招待コード", "クーポンコード", "プロモーションコード",
	"友達紹介", "紹介プログラム", "紹介キャンペーン",
	// 英文
	"referral code", "referral program", "referral bonus", "referral link",
	"refer a friend", "refer-a-friend", "refer and earn",
	"invite code", "invitation code", "invite friends", "invite a friend",
	"promo code", "promotion code", "discount code", "coupon code",
	// 連結路徑。這幾個是上面那些字樣最常見的落點，
	// 頁面文案改寫過（"Share & Earn" 之類）時往往只剩它們還在。
	"/referral", "/refer-a-friend", "/invite",
}

type Auditor struct {
	store *store.Store
	http  *http.Client
}

func New(st *store.Store) *Auditor {
	return &Auditor{
		store: st,
		// 逾時給寬一點 —— 外站慢不代表它沒在發碼，逾時太短會把一堆正常的平台
		// 記成 unreachable，稽核就永遠得不出結論。
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Sweep 檢查這一輪到期的服務商，回傳檢查了幾家、其中幾家進了複檢清單。
//
// 停用的也掃，包含 appimport 匯進來的草稿 —— 理由見 ListMerchantsDueForCodeAudit。
// 只有已上架的會進複檢清單（ListMerchantsFlaggedByCodeAudit），草稿的判定結果
// 留在軌跡上，給後台補完資料要上架時當參考。
func (a *Auditor) Sweep(ctx context.Context) (checked, flagged, blocked int, err error) {
	due, err := a.store.ListMerchantsDueForCodeAudit(ctx, staleDays)
	if err != nil {
		return 0, 0, 0, err
	}

	for _, m := range due {
		// 一輪要跑好幾分鐘，部署或關機會在中途取消 ctx。中斷不算失敗：
		// 下次醒來時剩下那些仍然「距離上次檢查超過一週」，會被接著撿起來。
		if ctx.Err() != nil {
			return checked, flagged, blocked, nil
		}

		res := a.check(ctx, m.SignupUrl)
		checked++
		if res.blocked {
			blocked++
		}

		// 連續兩次都沒找到才算數。上一次是 unreachable 的不算第一次 ——
		// 那次根本沒看到頁面，拿它當證據等於把「被擋」當成「沒在發碼」。
		//
		// 這裡只記 log 與軌跡，不動 is_active。哪些家上榜由後台查
		// ListMerchantsFlaggedByCodeAudit，判定條件寫在那支查詢裡 ——
		// 兩邊都算一次是刻意的：這裡是給值班的人看的即時訊號，那邊是複檢清單。
		if res.result == resultMissing && m.LastResult == resultMissing {
			flagged++
			slog.Info("稽核連續兩次沒找到字樣，待複檢",
				"merchant", m.Name, "signup_url", m.SignupUrl,
				"active_codes", m.ActiveCodeCount, "note", res.note)
		}

		if _, err := a.store.CreateMerchantCodeAudit(ctx, dbgen.CreateMerchantCodeAuditParams{
			MerchantID:  m.ID,
			Result:      res.result,
			CheckedUrls: res.urls,
			Matched:     res.matched,
			Note:        res.note,
			// 永遠 false：這支排程不下架。欄位留著是給將來真的接回自動下架時用的。
			Deactivated: false,
		}); err != nil {
			return checked, flagged, blocked, err
		}
	}

	// 被擋只在一輪結束時記一行總數，不是每家記一次 —— 一輪有幾十家被擋是常態，
	// 逐家寫會把 log 洗掉。哪幾家被擋、擋在哪個網址，都在 merchant_code_audits
	// 的 checked_urls 與 note 上，那份不會因為 log 保留期限到了就消失。
	if blocked > 0 {
		slog.Warn("稽核有服務商被反爬蟲擋下", "blocked", blocked, "checked", checked)
	}

	return checked, flagged, blocked, nil
}

type outcome struct {
	result  string
	urls    []string
	matched []string
	note    string
	// blocked 記「這次有沒有撞到反爬蟲」。它不影響 result —— 被擋的頁面根本沒被
	// 讀進來，結論自然會落在 unreachable 或維持原本的判定。存在的意義是讓一輪
	// 結束時數得出被擋幾家：被對方擋掉是慢慢發生的，要看得到趨勢才發現得了。
	blocked bool
}

// check 抓這家的頁面並判斷有沒有在發碼。抓頁面失敗不回 error：抓不到本身就是
// 一種結論（unreachable），要跟著軌跡記下來，不是中斷整輪的理由。
func (a *Auditor) check(ctx context.Context, signupURL string) outcome {
	out := outcome{result: resultUnreachable, urls: []string{}, matched: []string{}}

	// 商店頁不抓。App Store 的商品頁是商店寫的介紹，不是平台自己的網站，上面不會
	// 有推薦計畫的字樣 —— 照常檢查的話這些一定判 missing，兩輪之後就把還在發碼的
	// 平台下架了。目錄裡實際有 15 家只有商店連結，其中至少 10 家查得到現行的
	// 推薦碼（55688 的 TW6000、Richart 的 TSB50、K Cash 的薦友賞…），而且它們
	// 架上都沒有使用者上傳的碼，擋牆一家都救不到。
	//
	// 記成 unreachable 而不是直接跳過不留紀錄：unreachable 不會累積成下架的理由，
	// 又更新了檢查時間 —— 不留紀錄的話這些會永遠卡在 due 清單最前面每天重撿一次。
	// 之後有人把 signup_url 換成官網，下一輪就會自動開始真的檢查。
	if isStorePage(signupURL) {
		out.note = "signup_url 指向 App Store／Google Play，商店頁上不會有推薦計畫的字樣，不判定"
		return out
	}

	var body strings.Builder

	// 第一階段：註冊頁與同網域首頁。這一段的失敗要留在 note 上 ——
	// 首頁都抓不到就是「這站連不上」的證據，複檢時最需要看到的就是它。
	failures := a.fetchInto(ctx, &out, &body, pagesFor(signupURL))
	out.note = strings.Join(failures, "; ")

	if ctx.Err() != nil {
		out.note = "檢查中斷"
		return out
	}

	if body.Len() < minPageBytes {
		if out.note == "" {
			out.note = "抓回來的內容過短，多半是 SPA 空殼或被擋"
		}
		return out
	}

	if out.matched = matchKeywords(body.String()); len(out.matched) > 0 {
		out.result = resultFound
		return out
	}

	// 第二階段：首頁上找不到才往下追。推薦計畫多半不掛在首頁，而是在 /referral
	// 這種固定路徑上，或首頁某個不起眼的連結後面 —— 只看首頁的話這些會全部被
	// 判成沒在發碼（實測已上架的 259 家有 150 家判 missing，裡面包括 1Password
	// 與 Airbnb）。
	//
	// 這一段的失敗不寫進 note：探測不存在的路徑回 404 是預期中的事，記下來只會
	// 把第一階段真正有用的錯誤淹掉。
	a.fetchInto(ctx, &out, &body, deepPages(signupURL, body.String()))

	if ctx.Err() != nil {
		out.note = "檢查中斷"
		return out
	}

	if out.matched = matchKeywords(body.String()); len(out.matched) > 0 {
		out.result = resultFound
	} else {
		out.result = resultMissing
	}
	// 判成 missing 而第一階段有頁面抓失敗時，failures 是複檢的重要旁證，留著不清掉。
	return out
}

// fetchInto 依序抓 targets，把內容接進 body，回傳失敗的那幾個。
//
// ctx 取消時提早返回而不是回報錯誤 —— 呼叫端會自己檢查 ctx，中斷跟「這站抓不到」
// 是兩件事，混在一起會讓部署時剛好被切斷的那幾家留下假的 missing 證據。
func (a *Auditor) fetchInto(ctx context.Context, out *outcome, body *strings.Builder, targets []string) []string {
	var failures []string

	for _, target := range targets {
		select {
		case <-ctx.Done():
			return failures
		case <-time.After(requestGap):
		}

		out.urls = append(out.urls, target)
		b, status, err := a.get(ctx, target)
		if looksBlocked(status, b) {
			out.blocked = true
			// 挑戰頁不接進 body。它的狀態碼與長度都像正常頁面，接進去只會讓整份
			// 內容比對不到字樣而被判成 missing —— 那是把「被擋」當成「沒在發碼」。
			reason := "疑似反爬蟲攔截"
			if status != 0 && status != http.StatusOK {
				reason = fmt.Sprintf("HTTP %d（疑似反爬蟲攔截）", status)
			}
			failures = append(failures, target+": "+reason)
			continue
		}
		if err != nil {
			failures = append(failures, target+": "+err.Error())
			continue
		}
		// 兩頁之間插分隔，免得接縫處拼出原本不存在的字樣。
		body.Write(b)
		body.WriteByte('\n')
	}
	return failures
}

// deepPages 是第二階段要抓的網址：同網域下推薦計畫最常見的固定路徑，加上首頁
// HTML 裡長得像推薦頁的連結。
//
// 只追同網域 —— 站外連結多半是社群或聯盟行銷平台，抓到那邊的字樣不能證明
// 這家自己在發碼。
func deepPages(signupURL, page string) []string {
	u, err := url.Parse(signupURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	root := u.Scheme + "://" + u.Host

	// 第一階段抓過的不再抓一次。
	seen := map[string]bool{signupURL: true, root + "/": true}
	var pages []string
	add := func(target string) bool {
		if seen[target] {
			return false
		}
		seen[target] = true
		pages = append(pages, target)
		return true
	}

	for _, p := range referralPaths {
		add(root + p)
	}

	// 再從首頁的連結裡撈幾條。文案改寫過（"Share & Earn" 那類）而路徑又不在
	// referralPaths 上時，這是唯一找得到的方式。
	followed := 0
	for _, m := range hrefRe.FindAllStringSubmatch(page, -1) {
		if followed >= maxHrefFollow {
			break
		}
		ref, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			continue
		}
		abs := u.ResolveReference(ref)
		if abs.Host != u.Host {
			continue
		}
		abs.Fragment = ""
		if add(abs.String()) {
			followed++
		}
	}
	return pages
}

// pagesFor 回傳要抓的頁面：註冊頁本身，加上同網域的首頁。
//
// 推薦計畫的說明常常不在註冊頁上 —— 註冊頁只有表單，「邀請好友」掛在首頁的
// 導覽列或頁尾。只抓註冊頁會把一大票還在發碼的平台判成 missing。
func pagesFor(signupURL string) []string {
	pages := []string{signupURL}

	u, err := url.Parse(signupURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return pages
	}
	if root := u.Scheme + "://" + u.Host + "/"; root != signupURL {
		pages = append(pages, root)
	}
	return pages
}

// isStorePage 判斷 signup_url 指的是不是 App Store／Google Play 的商品頁。
// cmd/appimport 匯進來的服務商有一部分只有商店連結，沒有填官網。
func isStorePage(signupURL string) bool {
	u, err := url.Parse(signupURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "apps.apple.com", "itunes.apple.com", "play.google.com":
		return true
	}
	return false
}

func (a *Auditor) get(ctx context.Context, target string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", browserUA)
	// 有些站會照 Accept-Language 回不同語言的文案。關鍵字表三語都收，
	// 但把中文排前面能讓台灣的平台回中文版，命中率比預設的英文版高。
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,ja;q=0.8,en;q=0.7")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	return body, resp.StatusCode, err
}

func matchKeywords(page string) []string {
	page = strings.ToLower(page)

	hits := []string{}
	for _, kw := range keywords {
		if strings.Contains(page, kw) {
			hits = append(hits, kw)
		}
	}
	return hits
}
