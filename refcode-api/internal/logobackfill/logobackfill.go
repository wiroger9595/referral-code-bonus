// Package logobackfill 幫沒有 logo 的服務商補圖。
//
// 目錄裡早期由 appimport 匯進來的都有圖（App Store API 會回 artworkUrl512），
// 但後來手動或批次建的那批沒有圖片來源，前端只能顯示品牌首字母。
//
// 補圖來源依序是官網的 apple-touch-icon、官網根目錄、iTunes 的 app 圖示；
// 抓到的圖上傳 Cloudinary 再把網址寫回 logo_url。上傳而不是直連對方網站：
// 對方改版就破圖，而 logo 破圖比沒有圖更糟。
//
// 兩個入口：cmd/logobackfill 手動跑（可以先預演），Sweep 給排程用
// （見 internal/worker）。
package logobackfill

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

const (
	// DefaultMinPx 是最小寬度。app 裡 logo 是 44pt，3x 螢幕要 132px 才不糊。
	DefaultMinPx = 128
	// DefaultGap 是兩次 iTunes 查詢之間至少等多久（連打會被回 429）。
	DefaultGap = 1200 * time.Millisecond
	// defaultBatch 是排程一輪最多補幾家。整批可能有幾百家，每家都要打好幾個
	// 外站，一次跑完會是幾十分鐘的連續請求。分批跑，下一輪接著補剩下的
	// —— 沒有 logo 的那些每輪都會重新被 ListMerchantsWithoutLogo 撈出來。
	defaultBatch = 25
)

// Uploader 是這個 package 需要的上傳能力，由 cloudinary.Client 滿足。
// 用 interface 而不是直接吃 *cloudinary.Client，是為了讓 worker 的 Deps
// 不必為了型別去 import cloudinary。
type Uploader interface {
	Enabled() bool
	Upload(ctx context.Context, file io.Reader, folder string) (string, string, error)
}

// Candidate 是找到的一張圖。給 CLI 預演時印用。
type Candidate struct {
	URL    string
	Source string // html / root / itunes
	Width  int
	Data   []byte
}

// Finder 找圖但不寫入。CLI 的預演模式用它。
type Finder struct{ f *fetcher }

func NewFinder(gap time.Duration) *Finder { return &Finder{f: newFetcher(gap)} }

func (fd *Finder) Find(ctx context.Context, name, site string, minPx int) (*Candidate, error) {
	ic, err := fd.f.find(ctx, name, site, minPx)
	if err != nil {
		return nil, err
	}
	return &Candidate{URL: ic.url, Source: ic.source, Width: ic.width, Data: ic.data}, nil
}

// Backfiller 是排程用的入口。
type Backfiller struct {
	store  *store.Store
	images Uploader
	minPx  int
	batch  int
	gap    time.Duration
}

func New(st *store.Store, images Uploader) *Backfiller {
	return &Backfiller{
		store:  st,
		images: images,
		minPx:  DefaultMinPx,
		batch:  defaultBatch,
		gap:    DefaultGap,
	}
}

// Sweep 補一批圖，回一句給後台看的結果。
//
// 單一家失敗（抓不到圖、圖太小、上傳失敗）不中斷整輪：這種失敗是常態，
// 有些服務商就是沒有公開的圖可抓，為了那些把整輪算成失敗會讓後台一直紅著。
// 只有資料庫寫入失敗才中止 —— 那代表環境有問題，繼續跑沒有意義。
func (b *Backfiller) Sweep(ctx context.Context) (string, error) {
	if !b.images.Enabled() {
		return "", fmt.Errorf("CLOUDINARY_* 未設定，無法上傳")
	}

	rows, err := b.store.ListMerchantsWithoutLogo(ctx)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil // 沒有要補的，不值得在後台留一行
	}

	remaining := len(rows)
	if b.batch > 0 && b.batch < len(rows) {
		rows = rows[:b.batch]
	}

	f := newFetcher(b.gap)
	var done, skipped int

	for _, m := range rows {
		// 一輪要打幾十個外站，服務關閉時要停得下來。
		if err := ctx.Err(); err != nil {
			return fmt.Sprintf("補到 %d 家、跳過 %d 家（中途停止）", done, skipped), err
		}

		ic, err := f.find(ctx, m.Name, m.SignupUrl, b.minPx)
		if err != nil {
			skipped++
			continue
		}

		url, _, err := b.images.Upload(ctx, bytes.NewReader(ic.data), "merchants")
		if err != nil {
			skipped++
			continue
		}

		// SetMerchantLogo 帶 logo_url IS NULL 條件，所以中途有人在後台補了圖
		// 就會回 0 列，這裡不當成錯誤 —— 人工填的優先。
		n, err := b.store.SetMerchantLogo(ctx, dbgen.SetMerchantLogoParams{ID: m.ID, LogoUrl: &url})
		if err != nil {
			return "", fmt.Errorf("寫入 %s: %w", m.Slug, err)
		}
		if n == 0 {
			skipped++
			continue
		}
		done++
	}

	summary := fmt.Sprintf("補到 %d 家、跳過 %d 家", done, skipped)
	if left := remaining - len(rows); left > 0 {
		summary += fmt.Sprintf("，還有 %d 家等下一輪", left)
	}
	return summary, nil
}
