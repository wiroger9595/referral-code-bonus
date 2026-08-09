// Command appimport 從 App Store 的公開排行榜把知名 app 匯進服務商目錄。
//
// 匯進來的一律是**停用的草稿**：推薦獎勵（reward_desc）在 App Store 上沒有這個欄位，
// 爬不到也不該亂編，要由後台的人補完再上架。這支只負責把名稱、圖示、官網、分類
// 這些查得到的欄位先備好，省掉一筆一筆手key。
//
//	go run ./cmd/appimport                 # 只印出會寫什麼，不碰資料庫
//	go run ./cmd/appimport --apply         # 真的寫進去
//	go run ./cmd/appimport --country jp --limit 100 --apply
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"

	"refcode-api/internal/config"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

func main() {
	var (
		country = flag.String("country", "tw", "App Store 國別（tw / jp / us …）")
		limit   = flag.Int("limit", 100, "每張排行榜取幾名")
		apply   = flag.Bool("apply", false, "真的寫進資料庫。預設只印出來看")
	)
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	apps, err := fetch(ctx, *country, *limit)
	if err != nil {
		log.Fatal(err)
	}

	rows := plan(apps, *country)
	fmt.Printf("排行榜共 %d 支 app，其中 %d 支落在有推薦計畫的分類裡\n\n", len(apps), len(rows))
	for _, r := range rows {
		fmt.Printf("  %-26s %-24s %-12s %s\n", trunc(r.name, 26), r.slug, r.category.name, trunc(r.signupURL, 44))
	}

	if !*apply {
		fmt.Printf("\n這是預演，沒有寫入任何東西。要寫的話加 --apply\n")
		return
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	res, err := write(ctx, st, rows)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n新增 %d 筆（全部是停用狀態，到後台補上推薦獎勵說明再逐一啟用）\n", res.created)
	fmt.Printf("既有的 %d 筆補上 %s、%d 筆本來就有了\n", res.countryAdded, strings.ToUpper(*country), res.untouched)
}

// fetch 取免費榜與暢銷榜的聯集。只看免費榜的話會漏掉訂閱制的服務
// （Netflix、Spotify 那類常年在暢銷榜而不在免費榜）。
func fetch(ctx context.Context, country string, limit int) ([]app, error) {
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

type row struct {
	slug      string
	name      string
	logoURL   string
	signupURL string
	country   string
	category  category
}

// plan 決定要寫什麼，不碰資料庫 —— 這樣預演跟實寫走的是同一段邏輯。
func plan(apps []app, country string) []row {
	seenSlug := map[string]bool{}
	var rows []row

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

		rows = append(rows, row{
			slug:      slug,
			name:      a.TrackName,
			logoURL:   a.logoURL(),
			signupURL: a.signupURL(),
			country:   strings.ToUpper(country),
			category:  cat,
		})
	}
	return rows
}

type result struct {
	created      int // 新建的服務商
	countryAdded int // 已經有了，這次把國別加進 countries
	untouched    int // 已經有了，而且已經標了這個國別
}

func write(ctx context.Context, st *store.Store, rows []row) (result, error) {
	var res result

	cats, err := ensureCategories(ctx, st, rows)
	if err != nil {
		return res, err
	}

	for _, r := range rows {
		logo := r.logoURL
		_, err := st.CreateImportedMerchant(ctx, dbgen.CreateImportedMerchantParams{
			Slug:       r.slug,
			Name:       r.name,
			CategoryID: cats[r.category.name].ID,
			LogoUrl:    &logo,
			SignupUrl:  r.signupURL,
			// 排行榜是分國別的，直接把來源國當成適用國家。countries 是 NOT NULL，
			// nil slice 會被 pgx 編成 SQL NULL 而不是空陣列，一定要給非 nil 的值。
			Countries: []string{r.country},
		})
		if err == nil {
			res.created++
			continue
		}
		if !store.IsUniqueViolation(err) {
			return res, fmt.Errorf("寫入 %s: %w", r.slug, err)
		}

		// slug 撞到＝這家已經匯過了（多半是上次跑別的國別）。同一家不建第二列，
		// 只把這次的國別補進 countries —— 這也讓這支腳本可以重複跑。
		//
		// 已經有人工改過的欄位（名稱、獎勵說明、logo）一律不覆蓋：
		// 排行榜上的名稱常常帶一長串促銷副標，回頭蓋掉後台整理好的資料是幫倒忙。
		n, err := st.AddMerchantCountry(ctx, dbgen.AddMerchantCountryParams{
			Slug:    r.slug,
			Country: r.country,
		})
		if err != nil {
			return res, fmt.Errorf("補國別 %s: %w", r.slug, err)
		}
		if n > 0 {
			res.countryAdded++
		} else {
			res.untouched++
		}
	}
	return res, nil
}

// ensureCategories 比對名稱找既有分類，沒有的才建 —— 分類沒有唯一鍵
// （slug 在 00007 拿掉了），重跑這支腳本很容易長出一堆同名分類。
func ensureCategories(ctx context.Context, st *store.Store, rows []row) (map[string]dbgen.MerchantCategory, error) {
	existing, err := st.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	byName := map[string]dbgen.MerchantCategory{}
	for _, c := range existing {
		byName[c.Name] = c
	}

	for _, r := range rows {
		if _, ok := byName[r.category.name]; ok {
			continue
		}
		en, ja := r.category.nameEn, r.category.nameJa
		created, err := st.CreateCategory(ctx, dbgen.CreateCategoryParams{
			Name:      r.category.name,
			SortOrder: r.category.sort,
			NameEn:    &en,
			NameJa:    &ja,
		})
		if err != nil {
			return nil, fmt.Errorf("建立分類 %s: %w", r.category.name, err)
		}
		fmt.Printf("新增分類：%s\n", created.Name)
		byName[created.Name] = created
	}
	return byName, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug 是網址的一部分（/merchant/{slug}），也是重跑時判斷「這家已經匯過」的唯一鍵，
// 所以寧可醜也不能空、不能撞。順序：app 名稱 → bundle id → App Store 的數字 id。
// 匯進來之後後台可以改，改了舊網址不轉址（見 refcode-api README）。
func slugFor(a app, taken map[string]bool) string {
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

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
