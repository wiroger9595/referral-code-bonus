package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"refcode-api/internal/store/dbgen"
)

// 搜不到時最多給幾個「你是不是要找」。給太多就不像建議，像是另一份結果列表。
const suggestLimit = 5

// 熱門關鍵字的時間窗與預設筆數。不設時間窗的話，早期洗上去的詞會永遠卡在前面，
// 新的熱門服務商永遠擠不進來；30 天短到跟得上檔期，長到冷門語言也累積得出榜。
const (
	popularWindowDays = 30
	popularDefault    = 8
	popularMax        = 20
)

// 關鍵字最長幾個字才收進熱門榜。超過的多半是整段貼上來的東西，
// 當成熱門詞顯示只會撐爆版面。
const maxTermRunes = 50

// escapeLike 把使用者輸入裡的萬用字元變回普通字元。
// ILIKE 的 % 是「任何東西」，不處理的話搜「50%」會命中整個目錄；_ 同理。
// 反斜線是 LIKE 的預設 escape 字元，所以它自己要先跳脫，順序不能顛倒。
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// normalizeTerm 把查詢字串收斂成榜單上的一筆。轉小寫是為了讓 Rakuten 與 rakuten
// 算同一個詞（中日文沒有大小寫，這步對它們是 no-op）。
// 回傳空字串代表這個詞不該進榜。
func normalizeTerm(q string) string {
	t := strings.ToLower(strings.Join(strings.Fields(q), " "))
	if t == "" || len([]rune(t)) > maxTermRunes {
		return ""
	}
	return t
}

// recordSearchTerm 不擋 response —— 榜單晚幾毫秒沒關係，但搜尋的人不該為了
// 寫統計多等。跟 recordImpressions 一樣用 WithoutCancel，request 結束後仍寫得完。
func (s *Server) recordSearchTerm(r *http.Request, term, lang string) {
	ctx := context.WithoutCancel(r.Context())
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := s.store.UpsertSearchTerm(ctx, dbgen.UpsertSearchTermParams{
			Term: term,
			Lang: lang,
		}); err != nil {
			slog.Error("寫入熱門關鍵字失敗", "term", term, "err", err)
		}
	}()
}

type popularTerm struct {
	Term string `json:"term"`
	Hits int64  `json:"hits"`
}

// handleSearchPopular 給搜尋框在空白狀態時顯示的熱門關鍵字。
// 榜單依語言分開：中文站的「銀行」跟英文站的「bank」是兩份榜，混在一起會讓
// 英文使用者看到一排看不懂的中文詞。
func (s *Server) handleSearchPopular(w http.ResponseWriter, r *http.Request) {
	limit := int32(popularDefault)
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > popularMax {
			limit = popularMax
		}
	}

	rows, err := s.store.ListPopularSearchTerms(r.Context(), dbgen.ListPopularSearchTermsParams{
		Lang:       pickLang(r),
		WindowDays: popularWindowDays,
		MaxResults: limit,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	out := make([]popularTerm, len(rows))
	for i, t := range rows {
		out[i] = popularTerm{Term: t.Term, Hits: t.Hits}
	}
	writeJSON(w, http.StatusOK, map[string]any{"terms": out})
}
