// Command logobackfill 幫沒有 logo 的服務商補圖。
//
// 目錄裡早期由 cmd/appimport 匯進來的都有圖（App Store API 會回 artworkUrl512），
// 但後來手動或批次建的那批沒有圖片來源，前端只能顯示品牌首字母。
//
// 補圖來源依序是官網的 apple-touch-icon、官網根目錄、iTunes 的 app 圖示；
// 抓到的圖上傳 Cloudinary 再把網址寫回 logo_url。上傳而不是直連對方網站：
// 對方改版就破圖，而 logo 破圖比沒有圖更糟。
//
//	go run ./cmd/logobackfill                 # 只印出會補什麼，不上傳也不寫入
//	go run ./cmd/logobackfill --apply         # 真的上傳並寫入
//	go run ./cmd/logobackfill --limit 5 --apply
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"refcode-api/internal/cloudinary"
	"refcode-api/internal/config"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

func main() {
	var (
		apply = flag.Bool("apply", false, "真的上傳並寫入資料庫。預設只印出來看")
		minPx = flag.Int("min-px", 128, "最小寬度。app 裡 logo 是 44pt，3x 螢幕要 132px 才不糊")
		limit = flag.Int("limit", 0, "只處理前幾家，0 是全部")
		gap   = flag.Duration("itunes-gap", 1200*time.Millisecond, "兩次 iTunes 查詢之間至少等多久（連打會被回 429）")
	)
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	images := cloudinary.New(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret)
	// 預演不需要 Cloudinary，但真的要寫的時候沒設定會在傳到一半才失敗，先擋住。
	if *apply && !images.Enabled() {
		log.Fatal("要 --apply 就得先設好 CLOUDINARY_CLOUD_NAME / API_KEY / API_SECRET")
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	rows, err := st.ListMerchantsWithoutLogo(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if *limit > 0 && *limit < len(rows) {
		rows = rows[:*limit]
	}

	fmt.Printf("沒有 logo 的服務商：%d 家（最小寬度 %dpx）\n\n", len(rows), *minPx)

	f := newFetcher(*gap)
	var done, skipped int
	for i, m := range rows {
		ic, err := f.find(ctx, m.Name, m.SignupUrl, *minPx)
		if err != nil {
			skipped++
			fmt.Printf("  [%3d/%d] %-30s 跳過：%v\n", i+1, len(rows), trunc(m.Name, 30), err)
			continue
		}

		if !*apply {
			done++
			fmt.Printf("  [%3d/%d] %-30s %-7s %dpx  %s\n",
				i+1, len(rows), trunc(m.Name, 30), ic.source, ic.width, trunc(ic.url, 52))
			continue
		}

		url, _, err := images.Upload(ctx, bytes.NewReader(ic.data), "merchants")
		if err != nil {
			skipped++
			fmt.Printf("  [%3d/%d] %-30s 上傳失敗：%v\n", i+1, len(rows), trunc(m.Name, 30), err)
			continue
		}

		// SetMerchantLogo 帶 logo_url IS NULL 條件，所以中途有人在後台補了圖
		// 就會回 0 列，這裡不當成錯誤 —— 人工填的優先。
		n, err := st.SetMerchantLogo(ctx, dbgen.SetMerchantLogoParams{ID: m.ID, LogoUrl: &url})
		if err != nil {
			log.Fatalf("寫入 %s: %v", m.Slug, err)
		}
		if n == 0 {
			skipped++
			fmt.Printf("  [%3d/%d] %-30s 已經有圖了，跳過\n", i+1, len(rows), trunc(m.Name, 30))
			continue
		}
		done++
		fmt.Printf("  [%3d/%d] %-30s %-7s %dpx  ✓\n", i+1, len(rows), trunc(m.Name, 30), ic.source, ic.width)
	}

	fmt.Printf("\n補到 %d 家、跳過 %d 家\n", done, skipped)
	if !*apply {
		fmt.Println("這是預演，沒有上傳也沒有寫入。要寫的話加 --apply")
	}
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
