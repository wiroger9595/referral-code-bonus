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
	"sort"
	"strconv"
	"strings"
	"time"
)

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

// Upload 簽名上傳一張圖到 folder 底下，回傳 Cloudinary 的 https URL。
func (c *Client) Upload(ctx context.Context, file io.Reader, folder string) (string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// 只有這兩個參數要簽名。folder 是我們自己分的 merchants / categories，
	// 用來在 Cloudinary 那邊把圖分開放，方便之後找。
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
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.cloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("呼叫 Cloudinary: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		SecureURL string `json:"secure_url"`
		Error     struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("解析 Cloudinary 回應: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Cloudinary 上傳失敗（%d）: %s", resp.StatusCode, out.Error.Message)
	}
	return out.SecureURL, nil
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
