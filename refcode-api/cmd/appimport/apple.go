package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Apple 的公開介面，兩支都不需要金鑰：
//   - RSS 排行榜：只給得到 app id 與名稱
//   - Lookup API：拿完整欄位（開發商官網、圖示、分類）
//
// 刻意不碰 Google Play —— 它沒有官方 API，只能爬 HTML，那違反它的服務條款，
// 而我們正要把 app 送去 Play 上架。
const (
	rssURL    = "https://itunes.apple.com/%s/rss/%s/limit=%d/json"
	lookupURL = "https://itunes.apple.com/lookup"

	// Lookup 一次能查多少個 id。Apple 沒有明講上限，200 是社群共識的安全值，
	// 這裡取一半，被限流的機率低一些。
	lookupBatch = 100
)

type app struct {
	TrackID          int64    `json:"trackId"`
	TrackName        string   `json:"trackName"`
	SellerName       string   `json:"sellerName"`
	SellerURL        string   `json:"sellerUrl"`
	TrackViewURL     string   `json:"trackViewUrl"`
	ArtworkURL512    string   `json:"artworkUrl512"`
	ArtworkURL100    string   `json:"artworkUrl100"`
	PrimaryGenreName string   `json:"primaryGenreName"`
	Genres           []string `json:"genres"`
	BundleID         string   `json:"bundleId"`
}

// logoURL 優先用 512px：目錄的 logo 在 retina 上會放到 80px 以上，100px 那張會糊。
func (a app) logoURL() string {
	if a.ArtworkURL512 != "" {
		return a.ArtworkURL512
	}
	return a.ArtworkURL100
}

// signupURL 用開發商自己的網站，沒有才退回 App Store 頁面 ——
// 「前往註冊」要把人帶到能填推薦碼的地方，App Store 只能下載。
func (a app) signupURL() string {
	if strings.HasPrefix(a.SellerURL, "http") {
		return a.SellerURL
	}
	return a.TrackViewURL
}

type client struct {
	http    *http.Client
	country string
}

func newClient(country string) *client {
	return &client{http: &http.Client{Timeout: 30 * time.Second}, country: country}
}

// chartIDs 抓一張排行榜，回傳上面的 app id。
func (c *client) chartIDs(ctx context.Context, chart string, limit int) ([]string, error) {
	u := fmt.Sprintf(rssURL, c.country, chart, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("抓 %s 排行榜: %w", chart, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("抓 %s 排行榜: HTTP %d", chart, resp.StatusCode)
	}

	// RSS 的 JSON 形狀很囉唆（每個值都包一層 label），只取 id 就好，
	// 其餘欄位一律等 Lookup 給乾淨的版本。
	var out struct {
		Feed struct {
			Entry []struct {
				ID struct {
					Attributes struct {
						IMID string `json:"im:id"`
					} `json:"attributes"`
				} `json:"id"`
			} `json:"entry"`
		} `json:"feed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("解析 %s 排行榜: %w", chart, err)
	}

	ids := make([]string, 0, len(out.Feed.Entry))
	for _, e := range out.Feed.Entry {
		if id := e.ID.Attributes.IMID; id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// lookup 依 id 取完整資料。回傳的順序不保證跟輸入一致，呼叫端不要依賴它。
func (c *client) lookup(ctx context.Context, ids []string) ([]app, error) {
	var all []app

	for start := 0; start < len(ids); start += lookupBatch {
		end := min(start+lookupBatch, len(ids))

		q := url.Values{}
		q.Set("id", strings.Join(ids[start:end], ","))
		q.Set("country", c.country)
		q.Set("entity", "software")

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("lookup: %w", err)
		}

		var out struct {
			Results []app `json:"results"`
		}
		err = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("解析 lookup 回應: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("lookup: HTTP %d", resp.StatusCode)
		}
		all = append(all, out.Results...)

		// 公開介面沒有明訂速率，但短時間連打會被擋。兩批之間停一下。
		if end < len(ids) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
	return all, nil
}
