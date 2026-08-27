// Package cloudinary 把圖片丟上 Cloudinary。只用得到「簽名上傳一張圖」這一種呼叫，
// 犯不著拉官方 SDK 進來，直接用 net/http 打 signed upload API。
package cloudinary

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cloudinary 那個帳號跟其他 side project 共用，所以這個專案的圖全部掛在這個
// 資料夾底下 —— 跟 Postgres 用 schema 隔開是同一個道理，散在 root 之後就分不出
// 哪張圖是誰的。實際路徑會長成 referral_code_bonus/avatars/xxx。
//
// 早於這個前綴上傳的圖還在舊路徑（avatars/xxx），但刪除是拿資料庫存的
// public_id 去打 destroy，不受影響，不用搬。
const rootFolder = "referral_code_bonus"

type Client struct {
	cloudName string
	apiKey    string
	apiSecret string
	http      *http.Client
}

func New(cloudName, apiKey, apiSecret string) *Client {
	return &Client{
		cloudName: cloudName,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http:      &http.Client{Timeout: 20 * time.Second},
	}
}

// Enabled 讓呼叫端在還沒設定 CLOUDINARY_* 時給出清楚的錯誤，
// 而不是打一半 HTTP request 才發現 cloud name 是空的。
func (c *Client) Enabled() bool {
	return c.cloudName != "" && c.apiKey != "" && c.apiSecret != ""
}

// Upload 簽名上傳一張圖到 rootFolder/folder 底下，回傳 Cloudinary 的 https URL
// 與 public_id。folder 只給用途（avatars / merchants / categories），專案前綴
// 一律在這裡補上，不要讓呼叫端自己拼 —— 漏掉一處就有圖跑到帳號 root 去。
//
// public_id 是之後要刪這張圖的唯一依據 —— 從 URL 反推得出來，但 Cloudinary 的
// URL 中間可能插入 transformation 或版本號，反推容易錯，寧可存下來。
func (c *Client) Upload(ctx context.Context, file io.Reader, folder string) (string, string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	folder = path.Join(rootFolder, folder)

	// 只有這兩個參數要簽名。folder 底下再依用途分 merchants / categories /
	// avatars，方便之後去後台找特定用途的圖。
	params := map[string]string{
		"folder":    folder,
		"timestamp": timestamp,
	}
	signature := sign(params, c.apiSecret)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range params {
		mw.WriteField(k, v)
	}
	mw.WriteField("api_key", c.apiKey)
	mw.WriteField("signature", signature)
	part, err := mw.CreateFormFile("file", "upload")
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", "", err
	}
	if err := mw.Close(); err != nil {
		return "", "", err
	}

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.cloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("呼叫 Cloudinary: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		SecureURL string `json:"secure_url"`
		PublicID  string `json:"public_id"`
		Error     struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("解析 Cloudinary 回應: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Cloudinary 上傳失敗（%d）: %s", resp.StatusCode, out.Error.Message)
	}
	return out.SecureURL, out.PublicID, nil
}

// Destroy 刪掉一張已上傳的圖。Apple 要求刪帳號時連使用者產生的內容一起刪，
// 大頭照的圖檔本身也算在內 —— 只把資料庫欄位清掉的話，圖還躺在公開網址上。
//
// 找不到那張圖（已經刪過、或 public_id 是舊資料）不算錯誤：Cloudinary 會回
// {"result":"not found"}，對呼叫端來說結果一樣是「這張圖不在了」。
func (c *Client) Destroy(ctx context.Context, publicID string) error {
	if publicID == "" {
		return nil
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{
		// 刪檔案不等於 CDN 上就看不到了 —— 實測過：destroy 之後 storage 立刻 404，
		// 但 res.cloudinary.com 的邊緣節點還是繼續送舊的副本。invalidate 才會去清。
		// 它要一起進簽名，漏了會被 Cloudinary 當成簽名不符。
		"invalidate": "true",
		"public_id":  publicID,
		"timestamp":  timestamp,
	}

	form := neturl.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("api_key", c.apiKey)
	form.Set("signature", sign(params, c.apiSecret))

	endpoint := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/destroy", c.cloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("呼叫 Cloudinary: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("解析 Cloudinary 回應: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Cloudinary 刪除失敗（%d）: %s", resp.StatusCode, out.Error.Message)
	}
	if out.Result != "ok" && out.Result != "not found" {
		return fmt.Errorf("Cloudinary 刪除失敗: %s", out.Result)
	}
	return nil
}

// sign 是 Cloudinary 簽名上傳的固定算法：參數依 key 字母排序組成
// key1=val1&key2=val2 的字串，接上 api secret 後取 SHA1。
func sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		fmt.Fprintf(&b, "%s=%s", k, params[k])
	}
	b.WriteString(secret)

	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
