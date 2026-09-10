// Command appimport 從 App Store 的公開排行榜把知名 app 匯進服務商目錄。
//
// 實際的抓取與寫入邏輯在 internal/appimport，排程（internal/worker 的 app-import）
// 走的是同一段。這支存在的意義是**先預演再決定**：排程沒有預演模式，加一個新國別
// 或改了分類對應表之後，先在這裡看清單再打開排程比較安全。
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
	"strings"

	"refcode-api/internal/appimport"
	"refcode-api/internal/config"
	"refcode-api/internal/store"
)

func main() {
	var (
		country = flag.String("country", "tw", "App Store 國別（tw / jp / us …）")
		limit   = flag.Int("limit", appimport.DefaultLimit, "每張排行榜取幾名")
		apply   = flag.Bool("apply", false, "真的寫進資料庫。預設只印出來看")
	)
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	apps, err := appimport.Fetch(ctx, *country, *limit)
	if err != nil {
		log.Fatal(err)
	}

	rows := appimport.Plan(apps, *country)
	fmt.Printf("排行榜共 %d 支 app，其中 %d 支落在有推薦計畫的分類裡\n\n", len(apps), len(rows))
	for _, r := range rows {
		fmt.Printf("  %-26s %-24s %s\n", trunc(r.Name, 26), r.Slug, trunc(r.SignupURL, 44))
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

	res, err := appimport.Write(ctx, st, rows, func(name string) {
		fmt.Printf("新增分類：%s\n", name)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n新增 %d 筆（全部是停用狀態，到後台補上推薦獎勵說明再逐一啟用）\n", res.Created)
	fmt.Printf("既有的 %d 筆補上 %s、%d 筆本來就有了\n", res.CountryAdded, strings.ToUpper(*country), res.Untouched)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
