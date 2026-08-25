package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 用一般瀏覽器的 UA。不少站對沒有 UA 或帶 bot 字樣的請求直接回 403，
// 那會讓抓圖看起來像「這家沒有圖」，其實只是被擋。
const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

// 圖片下載的上限。apple-touch-icon 正常在 100KB 以內，
// 抓到超過這個大小多半是拿錯東西（整張首頁背景圖之類）。
const maxIconBytes = 2 << 20

type icon struct {
	url    string
	source string // html / root / itunes
	data   []byte
	width  int
}

type fetcher struct {
	http *http.Client
	// iTunes 的 search 端點會擋連續請求（實測連打會回 429），每次呼叫前等一下。
	// lookup 可以一次查 100 筆，但那要 app id，而我們手上只有品牌名，只能先 search。
	itunesGap  time.Duration
	lastItunes time.Time
}

func newFetcher(itunesGap time.Duration) *fetcher {
	return &fetcher{
		http:      &http.Client{Timeout: 20 * time.Second},
		itunesGap: itunesGap,
	}
}

func (f *fetcher) get(ctx context.Context, target string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", browserUA)
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return b, resp.Header.Get("Content-Type"), err
}

var appleIconTag = regexp.MustCompile(`(?i)<link[^>]+apple-touch-icon[^>]*>`)
var hrefAttr = regexp.MustCompile(`(?i)href=["']([^"']+)`)
var sizesAttr = regexp.MustCompile(`(?i)sizes=["'](\d+)`)

// fromHTML 讀官網首頁，挑 sizes 最大的那個 apple-touch-icon。
// 這是品質最好的一條路 —— 那是給 iOS 加到主畫面用的圖，本來就要求方形且夠大。
func (f *fetcher) fromHTML(ctx context.Context, site string) (string, bool) {
	body, _, err := f.get(ctx, site, 512<<10)
	if err != nil {
		return "", false
	}
	base, err := url.Parse(site)
	if err != nil {
		return "", false
	}

	best, bestPx := "", -1
	for _, tag := range appleIconTag.FindAllString(string(body), -1) {
		h := hrefAttr.FindStringSubmatch(tag)
		if h == nil {
			continue
		}
		px := 0
		if s := sizesAttr.FindStringSubmatch(tag); s != nil {
			px, _ = strconv.Atoi(s[1])
		}
		// 沒寫 sizes 的當 0，但仍比「一個都沒有」好，所以用 > 而不是 >=。
		if px > bestPx {
			if ref, err := url.Parse(h[1]); err == nil {
				best, bestPx = base.ResolveReference(ref).String(), px
			}
		}
	}
	return best, best != ""
}

type itunesResult struct {
	Results []struct {
		TrackName     string `json:"trackName"`
		SellerName    string `json:"sellerName"`
		SellerURL     string `json:"sellerUrl"`
		ArtworkURL512 string `json:"artworkUrl512"`
		ArtworkURL100 string `json:"artworkUrl100"`
	} `json:"results"`
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]`)

func normalize(s string) string { return nonAlnum.ReplaceAllString(strings.ToLower(s), "") }

// brandOf 砍掉 App Store 名稱常見的一長串促銷副標，只留品牌本身。
// 「Coupang - 酷澎購物—隔日到貨…」拿整串去搜是搜不到的。
func brandOf(name string) string {
	return strings.TrimSpace(strings.SplitN(
		strings.NewReplacer("–", "-", "—", "-", "：", ":", "|", ":").Replace(name), "-", 2)[0])
}

// fromItunes 依品牌名找 app，取 512px 圖示 —— 跟目錄裡既有那 230 家同一個來源，
// 補出來的圖視覺上才會一致。
//
// 用評分而不是「第一個命中就要」：搜 Discover、Current 這種常見字，
// 排第一的常常是完全不相干的 app。開發商官網跟 signup_url 同網域是最強的證據。
func (f *fetcher) fromItunes(ctx context.Context, name, site string) (string, bool) {
	if gap := f.itunesGap - time.Since(f.lastItunes); gap > 0 {
		select {
		case <-time.After(gap):
		case <-ctx.Done():
			return "", false
		}
	}
	f.lastItunes = time.Now()

	brand := brandOf(name)
	host := strings.TrimPrefix(hostOf(site), "www.")
	q := url.Values{
		"term": {brand}, "country": {"us"}, "entity": {"software"}, "limit": {"10"},
	}
	body, _, err := f.get(ctx, "https://itunes.apple.com/search?"+q.Encode(), 1<<20)
	if err != nil {
		return "", false
	}
	var res itunesResult
	if err := json.Unmarshal(body, &res); err != nil {
		return "", false
	}

	nb := normalize(brand)
	root, _, _ := strings.Cut(host, ".")
	bestURL, bestScore := "", 0
	for _, it := range res.Results {
		art := it.ArtworkURL512
		if art == "" {
			art = it.ArtworkURL100
		}
		if art == "" {
			continue
		}
		tn, sn := normalize(it.TrackName), normalize(it.SellerName)
		score := 0
		if sh := strings.TrimPrefix(hostOf(it.SellerURL), "www."); sh != "" &&
			(sh == host || strings.HasSuffix(sh, "."+host) || strings.HasSuffix(host, "."+sh)) {
			score += 100
		}
		switch {
		case tn == nb:
			score += 50
		case strings.HasPrefix(tn, nb):
			score += 30
		case nb != "" && strings.Contains(tn, nb):
			score += 12
		}
		if nb != "" && strings.Contains(sn, nb) {
			score += 20
		}
		if root != "" && strings.Contains(tn, root) {
			score += 8
		}
		if score > bestScore {
			bestURL, bestScore = art, score
		}
	}
	// 30 分＝至少名字開頭吻合。低於這個的寧可不補，錯的圖比沒有圖更糟。
	if bestScore < 30 {
		return "", false
	}
	return bestURL, true
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// download 抓圖並量出寬度。量得到寬度才收 —— .ico 常常只有 16 或 32 像素，
// 放進目錄會比沒有圖還難看（app 裡 logo 是 44pt，在 3x 螢幕上要 132px 才不糊）。
func (f *fetcher) download(ctx context.Context, target string) ([]byte, int, error) {
	b, ct, err := f.get(ctx, target, maxIconBytes)
	if err != nil {
		return nil, 0, err
	}
	if len(b) < 200 {
		return nil, 0, fmt.Errorf("檔案太小（%d bytes）", len(b))
	}
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, 0, fmt.Errorf("不是圖片（Content-Type: %s）", ct)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		// SVG 與 ICO 標準函式庫解不開。SVG 其實是好東西但 Cloudinary 那邊要另外處理，
		// ICO 幾乎都太小 —— 兩者都跳過，讓它退到下一個來源。
		return nil, 0, fmt.Errorf("讀不出尺寸：%w", err)
	}
	return b, cfg.Width, nil
}

// find 依序試三個來源，回傳第一個「夠大」的。
func (f *fetcher) find(ctx context.Context, name, site string, minPx int) (*icon, error) {
	type candidate struct {
		url    string
		source string
	}
	var cands []candidate

	if u, ok := f.fromHTML(ctx, site); ok {
		cands = append(cands, candidate{u, "html"})
	}
	if base, err := url.Parse(site); err == nil {
		cands = append(cands, candidate{base.ResolveReference(&url.URL{Path: "/apple-touch-icon.png"}).String(), "root"})
	}
	if u, ok := f.fromItunes(ctx, name, site); ok {
		cands = append(cands, candidate{u, "itunes"})
	}

	var lastErr error
	for _, c := range cands {
		data, w, err := f.download(ctx, c.url)
		if err != nil {
			lastErr = err
			continue
		}
		if w < minPx {
			lastErr = fmt.Errorf("只有 %dpx", w)
			continue
		}
		return &icon{url: c.url, source: c.source, data: data, width: w}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("找不到任何圖")
	}
	return nil, lastErr
}
