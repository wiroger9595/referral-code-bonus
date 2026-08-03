package httpapi

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"refcode-api/internal/auth"
	"refcode-api/internal/ranking"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

type merchantSummary struct {
	ID              uuid.UUID  `json:"id"`
	Slug            string     `json:"slug"`
	Name            string     `json:"name"`
	LogoURL         *string    `json:"logo_url"`
	SignupURL       string     `json:"signup_url"`
	RewardDesc      string     `json:"reward_desc"`
	CategorySlug    string     `json:"category_slug"`
	CategoryName    string     `json:"category_name"`
	ActiveCodeCount int64      `json:"active_code_count"`
	SoonestExpires  *time.Time `json:"soonest_expires_at"`
	// 這家在哪些國家能用。空的代表不分地區（串流、雲端這種跨國服務）。
	Countries []string `json:"countries"`
}

type codeItem struct {
	ID uuid.UUID `json:"id"`
	// 沒登入看不到碼本身——要註冊才能拿到推薦碼。Code 是 nil、Masked 是 true 的時候，
	// 前端要顯示「登入查看」而不是複製按鈕；其餘欄位（家數、評價、備註）照常公開，
	// 服務商頁本身仍然值得被搜尋引擎收錄。
	Code        *string   `json:"code"`
	Masked      bool      `json:"masked"`
	Note        string    `json:"note"`
	OwnerName   string    `json:"owner_name"`
	OwnerAvatar *string   `json:"owner_avatar_url"`
	Quality     int32     `json:"quality_score"`
	WorkedCount int64     `json:"worked_count"`
	FailedCount int64     `json:"failed_count"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func revealCode(code string, loggedIn bool) *string {
	if !loggedIn {
		return nil
	}
	return &code
}

// ListMerchants 的 soonest_expires_at 是 min() 的結果，sqlc 推不出 nullability 只給
// interface{}（見 db/queries/merchants.sql）。這家沒有可用的碼時是 nil。
func soonestExpiry(v any) *time.Time {
	t, ok := v.(time.Time)
	if !ok {
		return nil
	}
	return &t
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListCategories(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": rows})
}

// handleGetCategory 存在的理由只有一個：slug 可以改，官網拿到舊 slug 時要問得出
// 現在的 slug 是什麼才能 301。回傳的 slug 一定是現用的，呼叫端拿它跟網址上的比對。
func (s *Server) handleGetCategory(w http.ResponseWriter, r *http.Request) {
	cat, err := s.store.GetCategoryBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCategoryNotFound, "找不到這個分類")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// viewerCountry 是目錄排序要用的所在地：登入的人才有，沒填就是 nil。
// 查不到不算錯誤 —— 地區只是排序偏好，為了它讓整個列表失敗不划算。
func (s *Server) viewerCountry(r *http.Request) *string {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		return nil
	}
	user, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		slog.Warn("讀取使用者所在地失敗，這次用預設排序", "user_id", userID, "err", err)
		return nil
	}
	return user.Country
}

func (s *Server) handleListMerchants(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 30, 100)

	params := dbgen.ListMerchantsParams{
		Limit:  limit,
		Offset: offset,
		// 沒登入時是 nil，排序退回原本的（active_code_count DESC, name），
		// 匿名訪客拿到的 SSR 內容因此不會因人而異。
		ViewerCountry: s.viewerCountry(r),
	}
	if v := r.URL.Query().Get("category"); v != "" {
		params.CategorySlug = &v
	}
	if v := r.URL.Query().Get("q"); v != "" {
		params.Search = &v
	}

	rows, err := s.store.ListMerchants(r.Context(), params)
	if err != nil {
		internalError(w, r, err)
		return
	}

	out := make([]merchantSummary, len(rows))
	for i, m := range rows {
		out[i] = merchantSummary{
			ID: m.ID, Slug: m.Slug, Name: m.Name,
			LogoURL: m.LogoUrl, SignupURL: m.SignupUrl, RewardDesc: m.RewardDesc,
			CategorySlug: m.CategorySlug, CategoryName: m.CategoryName,
			ActiveCodeCount: m.ActiveCodeCount,
			SoonestExpires:  soonestExpiry(m.SoonestExpiresAt),
			Countries:       m.Countries,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"merchants": out})
}

func (s *Server) handleMerchantSitemap(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListMerchantSlugs(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": rows})
}

// handleGetMerchant 是整個平台流量最集中的一頁：服務商詳情 + 推薦碼列表。
// 排序在這裡發生，曝光也在這裡記錄。
func (s *Server) handleGetMerchant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	merchant, err := s.store.GetMerchantBySlug(ctx, chi.URLParam(r, "slug"))
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeMerchantNotFound, "找不到這個服務商")
			return
		}
		internalError(w, r, err)
		return
	}

	rows, err := s.store.ListActiveCodesForMerchant(ctx, merchant.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}

	candidates := make([]ranking.Candidate, len(rows))
	byID := make(map[uuid.UUID]dbgen.ListActiveCodesForMerchantRow, len(rows))
	for i, row := range rows {
		candidates[i] = ranking.Candidate{
			ID:           row.ID,
			QualityScore: row.QualityScore,
			Impressions:  row.Impressions,
			CreatedAt:    row.CreatedAt,
		}
		byID[row.ID] = row
	}

	// 每個請求用自己的 rng，不共用狀態；種子取自全域（v2 的全域函式本身是併發安全的）。
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	ranked := ranking.Rank(candidates, s.rankOpts, time.Now(), rng)

	limit, _ := paginate(r, 20, 50)
	if int(limit) < len(ranked) {
		ranked = ranked[:limit]
	}

	// 要註冊才能拿到推薦碼：碼本身只給登入的人看，其餘欄位（家數、評價、備註）
	// 照常公開——服務商頁的可看性不能因此受影響，這頁還要值得被搜尋引擎收錄。
	_, loggedIn := auth.UserID(ctx)

	items := make([]codeItem, len(ranked))
	for i, c := range ranked {
		row := byID[c.ID]
		items[i] = codeItem{
			ID: row.ID, Code: revealCode(row.Code, loggedIn), Masked: !loggedIn, Note: row.Note,
			OwnerName: row.OwnerName, OwnerAvatar: row.OwnerAvatarUrl,
			Quality: row.QualityScore, WorkedCount: row.WorkedCount, FailedCount: row.FailedCount,
			ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		}
	}

	s.recordImpressions(r, merchant.ID, ranked)

	writeJSON(w, http.StatusOK, map[string]any{
		"merchant": merchantSummary{
			ID: merchant.ID, Slug: merchant.Slug, Name: merchant.Name,
			LogoURL: merchant.LogoUrl, SignupURL: merchant.SignupUrl,
			RewardDesc:   merchant.RewardDesc,
			CategorySlug: merchant.CategorySlug, CategoryName: merchant.CategoryName,
			ActiveCodeCount: int64(len(rows)),
			Countries:       merchant.Countries,
		},
		"codes": items,
		"total": len(rows),
	})
}

// recordImpressions 不擋 response：曝光統計晚幾毫秒沒關係，
// 但使用者不該為了寫統計而多等。用 WithoutCancel 讓 request 結束後仍能寫完。
func (s *Server) recordImpressions(r *http.Request, merchantID uuid.UUID, shown []ranking.Candidate) {
	if len(shown) == 0 || isBot(r) {
		return
	}

	var userID *uuid.UUID
	if id, ok := auth.UserID(r.Context()); ok {
		userID = &id
	}
	device, ip := deviceHash(r), ipHash(r)

	rows := make([]store.EventRow, len(shown))
	ids := make([]uuid.UUID, len(shown))
	for i, c := range shown {
		rows[i] = store.EventRow{
			CodeID: c.ID, MerchantID: merchantID, EventType: "impression",
			UserID: userID, DeviceHash: device, IPHash: ip,
		}
		ids[i] = c.ID
	}

	ctx := context.WithoutCancel(r.Context())
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := s.store.InsertEvents(ctx, rows); err != nil {
			slog.Error("寫入曝光事件失敗", "err", err)
			return
		}
		// impressions 是排序權重的輸入，跟事件一起更新才不會讓權重落後太多。
		if err := s.store.AddCodeImpressions(ctx, dbgen.AddCodeImpressionsParams{
			Delta:   1,
			CodeIds: ids,
		}); err != nil {
			slog.Error("累加曝光數失敗", "err", err)
		}
	}()
}

func paginate(r *http.Request, def, max int32) (limit, offset int32) {
	limit, offset = def, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > max {
			limit = max
		}
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = int32(v)
	}
	return limit, offset
}
