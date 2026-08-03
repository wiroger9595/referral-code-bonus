// Package geo 只做一件事：把使用者或後台填進來的國家代碼正規化成 ISO 3166-1 alpha-2。
//
// 這裡刻意不維護國家白名單。白名單會擋掉沒想到的國家（使用者填不了、服務商建不了），
// 而且清單會同時散在後端與三個前端。前端的選單只是「常用選項」，後端只認格式。
package geo

import (
	"errors"
	"regexp"
	"strings"
)

var ErrInvalid = errors.New("國家代碼要 ISO 3166-1 alpha-2（兩個英文字母，例如 TW）")

var pattern = regexp.MustCompile(`^[A-Z]{2}$`)

// Normalize 去空白轉大寫後驗格式。空字串代表「沒填」，不是錯誤 ——
// 使用者的所在地是選填，社群登入建立的帳號一開始也不會有。
func Normalize(raw string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return "", nil
	}
	if !pattern.MatchString(s) {
		return "", ErrInvalid
	}
	return s, nil
}

// NormalizePtr 給資料庫的 nullable 欄位用：沒填就是 NULL。
func NormalizePtr(raw string) (*string, error) {
	s, err := Normalize(raw)
	if err != nil || s == "" {
		return nil, err
	}
	return &s, nil
}

// NormalizeList 給服務商的適用國家用。順手去重並丟掉空值，
// 空的結果代表「不分地區」（跨國服務），不是「哪裡都不能用」。
func NormalizeList(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))

	for _, v := range raw {
		s, err := Normalize(v)
		if err != nil {
			return nil, err
		}
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, nil
}
