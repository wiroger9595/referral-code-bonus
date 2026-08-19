package httpapi

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"refcode-api/internal/auth"
	"refcode-api/internal/geo"
	"refcode-api/internal/ranking"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

type merchantSummary struct {
	ID         uuid.UUID `json:"id"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	LogoURL    *string   `json:"logo_url"`
	SignupURL  string    `json:"signup_url"`
	RewardDesc string    `json:"reward_desc"`
	// 分類一律用 id 認：分類頁的網址與 ?category= 篩選都是它，沒有 slug 這個東西。
	CategoryID      uuid.UUID  `json:"category_id"`
	CategoryName    string     `json:"category_name"`
	ActiveCodeCount int64      `json:"active_code_count"`
	SoonestExpires  *time.Time `json:"soonest_expires_at"`
	// 這家在哪些國家能用。空的代表不分地區（串流、雲端這種跨國服務）。
	Countries []string `json:"countries"`
	// 這家收哪幾種碼（referral／discount）。上架表單靠它決定要顯示哪些選項——
	// 沒有推薦計畫的服務商只會有 discount。
	AllowedCodeTypes []string `json:"allowed_code_types"`
}

type codeItem struct {
	ID uuid.UUID `json:"id"`
	// 沒登入看不到碼本身——要註冊才能拿到推薦碼。Code 是 nil、Masked 是 true 的時候，
	// 前端要顯示「登入查看」而不是複製按鈕；其餘欄位（家數、評價、備註）照常公開，
	// 服務商頁本身仍然值得被搜尋引擎收錄。
	Code   *string `json:"code"`
	Masked bool    `json:"masked"`
	Note   string  `json:"note"`
	// referral 或 discount。清單是兩種混在一起的，卡片靠這個標 badge。
	CodeType    string  `json:"code_type"`
	OwnerName   string  `json:"owner_name"`
	OwnerAvatar *string `json:"owner_avatar_url"`
	Quality     int32   `json:"quality_score"`
	WorkedCount int64   `json:"worked_count"`
	FailedCount int64   `json:"failed_count"`
	// nil 代表永久有效，前端不顯示倒數（見 00013_codes_nullable_expiry.sql）。
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
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

// pickLang 從 ?lang= 決定這次要回哪個語言的內容。認不出來就當中文 ——
// 語言錯了顯示中文，比回 400 讓整個頁面掛掉好。
func pickLang(r *http.Request) string {
	switch r.URL.Query().Get("lang") {
	case "en":
		return "en"
	case "ja":
		return "ja"
	default:
		return "zh"
	}
}

// localized 挑出這個語言的字。譯文是 NULL 或空字串（後台還沒填）就退回中文，
// 而不是給出一格空白 —— 空白的分類磁磚比顯示中文更難懂。
func localized(zh string, en, ja *string, lang string) string {
	var v *string
	switch lang {
	case "en":
		v = en
	case "ja":
		v = ja
	}
	if v == nil || *v == "" {
		return zh
	}
	return *v
}

// 分類的對外形狀。name 已經照 ?lang= 挑好，name_en / name_ja 原樣附著 ——
// 後台的分類編輯頁就是打這支拿三語的值，沒有另外的 admin 列表 API。
type categoryDTO struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	NameEn    *string   `json:"name_en"`
	NameJa    *string   `json:"name_ja"`
	SortOrder int32     `json:"sort_order"`
	ImageURL  *string   `json:"image_url"`
}

func toCategoryDTO(c dbgen.MerchantCategory, lang string) categoryDTO {
	return categoryDTO{
		ID:        c.ID,
		Name:      localized(c.Name, c.NameEn, c.NameJa, lang),
		NameEn:    c.NameEn,
		NameJa:    c.NameJa,
		SortOrder: c.SortOrder,
		ImageURL:  c.ImageUrl,
	}
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListCategories(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}

	lang := pickLang(r)
	out := make([]categoryDTO, len(rows))
	for i, c := range rows {
		out[i] = toCategoryDTO(c, lang)
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

// handleGetCategory 給分類頁用：網址上只有 id，但頁面要顯示分類名稱。
func (s *Server) handleGetCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	cat, err := s.store.GetCategoryByID(r.Context(), id)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCategoryNotFound, "找不到這個分類")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryDTO(cat, pickLang(r)))
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

// resolveRegion 決定這次要不要把目錄限縮在某個地區。
//
//	?region=all  → 不篩（使用者自己選了「顯示所有地區」）
//	?region=US   → 篩 US（app 從裝置地區推出來的預設，或使用者自己挑的）
//	沒帶 region  → 退回登入者填的所在地；匿名或沒填就不篩
//
// 匿名不篩是刻意的：官網匿名的 SSR 內容不能因人而異，否則 Googlebot 從不同機房
// 爬到的頁面會長得不一樣。要地區行為的用戶端自己帶 ?region=。
//
// 認不出來的地區碼當成沒帶，而不是回 400 —— 地區只是縮小範圍，
// 值壞掉時給全部比讓整份目錄失敗有用。
func resolveRegion(r *http.Request, userCountry *string) *string {
	raw := strings.TrimSpace(r.URL.Query().Get("region"))
	if strings.EqualFold(raw, "all") {
		return nil
	}
	if raw == "" {
		return userCountry
	}
	code, err := geo.Normalize(raw)
	if err != nil || code == "" {
		return userCountry
	}
	return &code
}

func (s *Server) handleListMerchants(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 30, 100)

	// 讀一次就好：排序與過濾都要用，分開讀會變成兩次 GetUserByID。
	country := s.viewerCountry(r)

	params := dbgen.ListMerchantsParams{
		Limit:  limit,
		Offset: offset,
		// 沒登入時是 nil，排序退回原本的（active_code_count DESC, name），
		// 匿名訪客拿到的 SSR 內容因此不會因人而異。
		ViewerCountry: country,
		Region:        resolveRegion(r, country),
	}
	// 分類一律用 id。認不出來就是不篩，不要靜靜地回全部又讓人以為篩過了 ——
	// 這裡直接擋掉比較好抓錯。
	if v := r.URL.Query().Get("category"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			badRequest(w, codeInvalidID, "category 要是分類的 id")
			return
		}
		params.CategoryID = &id
	}
	// 原字串留著給 similarity() 與熱門榜用：那兩條路不吃 LIKE 的萬用字元，
	// escape 過的反斜線反而會變成要比對的內容。
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q != "" {
		escaped := escapeLike(q)
		params.Search = &escaped
	}

	rows, err := s.store.ListMerchants(r.Context(), params)
	if err != nil {
		internalError(w, r, err)
		return
	}

	lang := pickLang(r)
	out := make([]merchantSummary, len(rows))
	for i, m := range rows {
		out[i] = merchantSummary{
			ID: m.ID, Slug: m.Slug, Name: m.Name,
			LogoURL: m.LogoUrl, SignupURL: m.SignupUrl,
			// 服務商名不翻（品牌名），獎勵說明與分類名跟著語言走。
			RewardDesc:       localized(m.RewardDesc, m.RewardDescEn, m.RewardDescJa, lang),
			CategoryID:       m.CategoryID,
			CategoryName:     localized(m.CategoryName, m.CategoryNameEn, m.CategoryNameJa, lang),
			ActiveCodeCount:  m.ActiveCodeCount,
			SoonestExpires:   soonestExpiry(m.SoonestExpiresAt),
			Countries:        m.Countries,
			AllowedCodeTypes: m.AllowedCodeTypes,
		}
	}

	// 一筆都沒有時 window function 連一列都不會回，總數就是 0。
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}

	body := map[string]any{"merchants": out, "total": total}

	if q != "" {
		body["suggestions"] = s.searchSuggestions(r, q, len(rows))

		// commit 代表這是使用者「確定要搜的」（按下 Enter、點了建議或歷史、
		// 直接開 /search?q=），逐字輸入不帶。少了這個條件，打一次「台新銀行」
		// 會在榜上留下「台」「台新」「台新銀」四筆垃圾。
		//
		// 搜不到東西的詞也不記：熱門榜是拿來讓人點的，點進去是空頁就不是熱門，
		// 是待補的服務商 —— 那是另一件事，不該混進同一份榜。
		if len(rows) > 0 && r.URL.Query().Get("commit") == "1" {
			if term := normalizeTerm(q); term != "" {
				s.recordSearchTerm(r, term, lang)
			}
		}
	}

	writeJSON(w, http.StatusOK, body)
}

type searchSuggestion struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// searchSuggestions 只在一筆都搜不到時才去查 —— 有結果的時候使用者要的是結果，
// 多打一次 similarity 查詢只是白花錢。查失敗也不讓整頁失敗：建議是加分，
// 沒有建議的空結果頁仍然是完整的頁面。
func (s *Server) searchSuggestions(r *http.Request, q string, found int) []searchSuggestion {
	if found > 0 {
		return []searchSuggestion{}
	}

	rows, err := s.store.SuggestMerchants(r.Context(), dbgen.SuggestMerchantsParams{
		Search:     q,
		MaxResults: suggestLimit,
	})
	if err != nil {
		slog.Warn("查詢搜尋建議失敗，這次不給建議", "q", q, "err", err)
		return []searchSuggestion{}
	}

	out := make([]searchSuggestion, len(rows))
	for i, m := range rows {
		out[i] = searchSuggestion{Slug: m.Slug, Name: m.Name}
	}
	return out
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
		// 新鮮度從審核通過那一刻起算。activated_at 理論上 active 的碼都有值，
		// 但這個欄位是後來才加的，舊資料可能還是 NULL，退回建立時間。
		listedAt := row.CreatedAt
		if row.ActivatedAt != nil {
			listedAt = *row.ActivatedAt
		}

		candidates[i] = ranking.Candidate{
			ID:           row.ID,
			QualityScore: row.QualityScore,
			Impressions:  row.Impressions,
			ListedAt:     listedAt,
			IsPro:        row.OwnerIsPro,
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
			CodeType:  row.CodeType,
			OwnerName: row.OwnerName, OwnerAvatar: row.OwnerAvatarUrl,
			Quality: row.QualityScore, WorkedCount: row.WorkedCount, FailedCount: row.FailedCount,
			ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		}
	}

	s.recordImpressions(r, merchant.ID, ranked)

	lang := pickLang(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"merchant": merchantSummary{
			ID: merchant.ID, Slug: merchant.Slug, Name: merchant.Name,
			LogoURL: merchant.LogoUrl, SignupURL: merchant.SignupUrl,
			RewardDesc:       localized(merchant.RewardDesc, merchant.RewardDescEn, merchant.RewardDescJa, lang),
			CategoryID:       merchant.CategoryID,
			CategoryName:     localized(merchant.CategoryName, merchant.CategoryNameEn, merchant.CategoryNameJa, lang),
			ActiveCodeCount:  int64(len(rows)),
			Countries:        merchant.Countries,
			AllowedCodeTypes: merchant.AllowedCodeTypes,
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
