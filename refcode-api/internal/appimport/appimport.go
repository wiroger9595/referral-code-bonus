// Package appimport 從 App Store 的公開排行榜把知名 app 匯進服務商目錄。
//
// 匯進來的一律是**停用的草稿**：推薦獎勵（reward_desc）在 App Store 上沒有這個
// 欄位，爬不到也不該亂編，要由後台的人補完再上架。這裡只負責把名稱、圖示、官網、
// 分類這些查得到的欄位先備好，省掉一筆一筆手 key。
//
// 兩個入口：cmd/appimport 是手動跑的（可以先預演再決定要不要寫），Sweep 是排程
// 用的（見 internal/worker）。兩邊共用同一段 Plan/Write，所以預演看到的就是排程
// 會做的事。
package appimport

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// Row 是「這支 app 會被寫成什麼」。Plan 產生，Write 消化 ——
// 中間隔一層是為了讓預演跟實寫走同一段邏輯。
type Row struct {
	Slug      string
	Name      string
	LogoURL   string
	SignupURL string
	Country   string
	Category  Category
}

// Result 是一輪寫入的結果。三個數字分開記，因為它們的意義差很多：
// Created 是真的多了一家，CountryAdded 只是既有的那家多適用一個國別。
type Result struct {
	Created      int // 新建的服務商
	CountryAdded int // 已經有了，這次把國別加進 countries
	Untouched    int // 已經有了，而且已經標了這個國別
}

// Fetch 取免費榜與暢銷榜的聯集。只看免費榜的話會漏掉訂閱制的服務
// （Netflix、Spotify 那類常年在暢銷榜而不在免費榜）。
func Fetch(ctx context.Context, country string, limit int) ([]App, error) {
	c := newClient(country)

	seen := map[string]bool{}
	var ids []string
	for _, chart := range []string{"topfreeapplications", "topgrossingapplications"} {
		got, err := c.chartIDs(ctx, chart, limit)
		if err != nil {
			return nil, err
		}
		for _, id := range got {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return c.lookup(ctx, ids)
}

// Plan 決定要寫什麼，不碰資料庫 —— 這樣預演跟實寫走的是同一段邏輯。
func Plan(apps []App, country string) []Row {
	seenSlug := map[string]bool{}
	var rows []Row

	for _, a := range apps {
		cat, ok := genreToCategory[a.PrimaryGenreName]
		if !ok {
			continue
		}
		slug := slugFor(a, seenSlug)
		if slug == "" {
			continue // 同一支 app 同時出現在兩張榜上
		}
		seenSlug[slug] = true

		rows = append(rows, Row{
			Slug:      slug,
			Name:      a.TrackName,
			LogoURL:   a.logoURL(),
			SignupURL: a.signupURL(),
			Country:   strings.ToUpper(country),
			Category:  cat,
		})
	}
	return rows
}

// Write 把 Plan 的結果寫進資料庫。onCategory 在建立新分類時被呼叫，給 CLI 印訊息用
// （排程那邊傳 nil）—— package 裡不直接 fmt.Print，那會讓排程的輸出跑到 stdout
// 而不是 slog。
func Write(ctx context.Context, st *store.Store, rows []Row, onCategory func(name string)) (Result, error) {
	var res Result

	cats, err := ensureCategories(ctx, st, rows, onCategory)
	if err != nil {
		return res, err
	}

	for _, r := range rows {
		logo := r.LogoURL
		_, err := st.CreateImportedMerchant(ctx, dbgen.CreateImportedMerchantParams{
			Slug:       r.Slug,
			Name:       r.Name,
			CategoryID: cats[r.Category.name].ID,
			LogoUrl:    &logo,
			SignupUrl:  r.SignupURL,
			// 排行榜是分國別的，直接把來源國當成適用國家。countries 是 NOT NULL，
			// nil slice 會被 pgx 編成 SQL NULL 而不是空陣列，一定要給非 nil 的值。
			Countries: []string{r.Country},
		})
		if err == nil {
			res.Created++
			continue
		}
		if !store.IsUniqueViolation(err) {
			return res, fmt.Errorf("寫入 %s: %w", r.Slug, err)
		}

		// slug 撞到＝這家已經匯過了（多半是上次跑別的國別）。同一家不建第二列，
		// 只把這次的國別補進 countries —— 這也讓這支可以重複跑。
		//
		// 已經有人工改過的欄位（名稱、獎勵說明、logo）一律不覆蓋：
		// 排行榜上的名稱常常帶一長串促銷副標，回頭蓋掉後台整理好的資料是幫倒忙。
		n, err := st.AddMerchantCountry(ctx, dbgen.AddMerchantCountryParams{
			Slug:    r.Slug,
			Country: r.Country,
		})
		if err != nil {
			return res, fmt.Errorf("補國別 %s: %w", r.Slug, err)
		}
		if n > 0 {
			res.CountryAdded++
		} else {
			res.Untouched++
		}
	}
	return res, nil
}

// DefaultLimit 是每張榜取幾名。跟 CLI 的預設一致。
//
// 200 是 Apple RSS 的 limit 上限，再大也只會回 200 筆。實際拿到的比這個多：
// Fetch 取的是免費榜與暢銷榜的聯集，所以一個國別最多會有 400 個 id，
// 去重、再濾掉 genreToCategory 對不到的分類之後才落地。
const DefaultLimit = 200

// Importer 是排程用的入口。
type Importer struct {
	store     *store.Store
	countries []string
	limit     int
}

func New(st *store.Store, countries []string) *Importer {
	return &Importer{store: st, countries: countries, limit: DefaultLimit}
}

// Sweep 依序跑過設定的國別，回一句給後台看的結果。
//
// 一個國別失敗不中斷其餘的：Apple 的公開介面偶爾會對單一國別回 5xx，
// 為了那個把整輪（含已經寫進去的部分）算成失敗，只會讓後台看到一片紅而
// 查不出真正的問題。錯誤累積起來一起回。
func (i *Importer) Sweep(ctx context.Context) (string, error) {
	var total Result
	var failed []string

	for _, country := range i.countries {
		apps, err := Fetch(ctx, country, i.limit)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", country, err))
			continue
		}

		res, err := Write(ctx, i.store, Plan(apps, country), nil)
		// Write 中途失敗時前面幾筆已經寫進去了，那些照樣計入 —— 回報要跟
		// 資料庫的實際狀態一致。
		total.Created += res.Created
		total.CountryAdded += res.CountryAdded
		total.Untouched += res.Untouched
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", country, err))
		}
	}

	summary := fmt.Sprintf("新增 %d 家（停用草稿）、補國別 %d 家、已存在 %d 家",
		total.Created, total.CountryAdded, total.Untouched)
	if len(failed) > 0 {
		return summary, fmt.Errorf("部分國別失敗 —— %s", strings.Join(failed, "；"))
	}
	return summary, nil
}

// ensureCategories 比對名稱找既有分類，沒有的才建 —— 分類沒有唯一鍵
// （slug 在 00007 拿掉了），重跑很容易長出一堆同名分類。
func ensureCategories(ctx context.Context, st *store.Store, rows []Row, onCategory func(string)) (map[string]dbgen.MerchantCategory, error) {
	existing, err := st.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	byName := map[string]dbgen.MerchantCategory{}
	for _, c := range existing {
		byName[c.Name] = c
	}

	for _, r := range rows {
		if _, ok := byName[r.Category.name]; ok {
			continue
		}
		en, ja := r.Category.nameEn, r.Category.nameJa
		created, err := st.CreateCategory(ctx, dbgen.CreateCategoryParams{
			Name:      r.Category.name,
			SortOrder: r.Category.sort,
			NameEn:    &en,
			NameJa:    &ja,
		})
		if err != nil {
			return nil, fmt.Errorf("建立分類 %s: %w", r.Category.name, err)
		}
		if onCategory != nil {
			onCategory(created.Name)
		}
		byName[created.Name] = created
	}
	return byName, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug 是網址的一部分（/merchant/{slug}），也是重跑時判斷「這家已經匯過」的唯一鍵，
// 所以寧可醜也不能空、不能撞。順序：app 名稱 → bundle id → App Store 的數字 id。
// 匯進來之後後台可以改，改了舊網址不轉址（見 refcode-api README）。
func slugFor(a App, taken map[string]bool) string {
	for _, c := range []string{slugify(a.TrackName), bundleSlug(a.BundleID), fmt.Sprintf("app-%d", a.TrackID)} {
		if usableSlug(c) && !taken[c] {
			return c
		}
	}
	return ""
}

// 中日文的名稱被 slugify 清完之後常常只剩一兩個字母（「麥當勞APP」→ app、
// 「台鐵e訂通」→ e），那種當網址沒有意義，而且很容易兩家撞在一起。
var genericSlug = map[string]bool{"app": true, "tv": true, "pro": true, "lite": true, "plus": true, "mobile": true, "tw": true}

func usableSlug(s string) bool {
	return len(s) >= 4 && !genericSlug[s]
}

// bundleSlug 把 bundle id 去掉沒有辨識度的段（com/tw/net/gov…）之後接起來：
// com.beeasy.shopee.tw → beeasy-shopee、com.ubercab.UberClient → ubercab-uberclient。
func bundleSlug(bundleID string) string {
	skip := map[string]bool{"com": true, "tw": true, "net": true, "org": true, "io": true, "gov": true, "co": true, "app": true, "apps": true}

	var parts []string
	for _, seg := range strings.Split(strings.ToLower(bundleID), ".") {
		if seg = slugify(seg); seg != "" && !skip[seg] {
			parts = append(parts, seg)
		}
	}
	return truncSlug(strings.Join(parts, "-"))
}

// slugify 只留 ASCII。中日文名稱會被清成空字串，由 slugFor 決定退路。
func slugify(s string) string {
	return truncSlug(strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-"))
}

// 名稱裡常有一長串副標（「Coupang 酷澎購物—隔日到貨…」），切在連字號上，
// 免得網址出現半截單字。
func truncSlug(s string) string {
	const max = 40
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, "-"); i > 0 {
		cut = cut[:i]
	}
	return strings.Trim(cut, "-")
}
