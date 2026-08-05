package httpapi

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"refcode-api/internal/auth"
	"refcode-api/internal/geo"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// slug 會直接出現在網址上（/factories/ooo-bank），限制字元避免產生怪 URL。
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (s *Server) handleListPendingCodes(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 50, 200)

	rows, err := s.store.ListPendingCodes(r.Context(), dbgen.ListPendingCodesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	total, err := s.store.CountPendingCodes(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"codes": rows, "total": total})
}

// 審核動作與碼的狀態是一對一的，集中在這裡對應，呼叫端不各自判斷。
var reviewActionStatus = map[string]string{
	"approve": "active",
	"reject":  "rejected",
	"disable": "disabled",
	"restore": "active",
}

func (s *Server) handleReviewCode(w http.ResponseWriter, r *http.Request) {
	codeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	status, ok := reviewActionStatus[req.Action]
	if !ok {
		badRequest(w, codeReviewActionInvalid, "action 只能是 approve / reject / disable / restore")
		return
	}
	if req.Action != "approve" && strings.TrimSpace(req.Reason) == "" {
		// 拒絕與下架一定要有理由：使用者申訴時要拿得出當初的判斷依據。
		badRequest(w, codeReviewReasonRequired, "這個動作必須填寫原因")
		return
	}

	ctx := r.Context()
	if _, err := s.store.GetCodeByID(ctx, codeID); err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCodeNotFound, "找不到這個推薦碼")
			return
		}
		internalError(w, r, err)
		return
	}

	admin, _ := auth.Admin(ctx)
	var updated dbgen.ReferralCode
	err = s.store.InTx(ctx, func(q *dbgen.Queries) error {
		var err error
		updated, err = q.SetCodeStatus(ctx, dbgen.SetCodeStatusParams{ID: codeID, Status: status})
		if err != nil {
			return err
		}
		_, err = q.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID:  codeID,
			AdminID: &admin.ID,
			Action:  req.Action,
			Reason:  strings.TrimSpace(req.Reason),
		})
		return err
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleListMerchantsForAdmin(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListMerchantsForAdmin(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"merchants": rows})
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		SortOrder int32   `json:"sort_order"`
		ImageURL  *string `json:"image_url"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		badRequest(w, codeNameRequired, "名稱不能空白")
		return
	}

	cat, err := s.store.CreateCategory(r.Context(), dbgen.CreateCategoryParams{
		Name:      strings.TrimSpace(req.Name),
		SortOrder: req.SortOrder,
		ImageUrl:  req.ImageURL,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	var req struct {
		Name      string  `json:"name"`
		SortOrder int32   `json:"sort_order"`
		ImageURL  *string `json:"image_url"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		badRequest(w, codeNameRequired, "名稱不能空白")
		return
	}

	cat, err := s.store.UpdateCategory(r.Context(), dbgen.UpdateCategoryParams{
		ID:        id,
		Name:      strings.TrimSpace(req.Name),
		SortOrder: req.SortOrder,
		ImageUrl:  req.ImageURL,
	})
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

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	if err := s.store.DeleteCategory(r.Context(), id); err != nil {
		if store.IsForeignKeyViolation(err) {
			conflict(w, codeCategoryInUse, "還有服務商屬於這個分類，請先改到別的分類再刪除")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type merchantInput struct {
	Slug            string  `json:"slug"`
	Name            string  `json:"name"`
	CategoryID      string  `json:"category_id"`
	LogoURL         *string `json:"logo_url"`
	SignupURL       string  `json:"signup_url"`
	RewardDesc      string  `json:"reward_desc"`
	CodeFormatRegex *string `json:"code_format_regex"`
	IsActive        *bool   `json:"is_active"`
	// 適用國家（ISO 3166-1 alpha-2）。留空代表不分地區，目錄排序時當中性處理。
	Countries []string `json:"countries"`
}

// validate 回傳正規化後的 category id 與適用國家。
func (in merchantInput) validate(requireSlug bool) (uuid.UUID, []string, error) {
	var categoryID uuid.UUID

	if requireSlug && !slugPattern.MatchString(in.Slug) {
		return categoryID, nil, errValidation("slug 只能是小寫英數字與連字號")
	}
	if strings.TrimSpace(in.Name) == "" {
		return categoryID, nil, errValidation("名稱不能空白")
	}
	if !strings.HasPrefix(in.SignupURL, "https://") {
		return categoryID, nil, errValidation("註冊連結必須是 https")
	}

	categoryID, err := uuid.Parse(in.CategoryID)
	if err != nil {
		return categoryID, nil, errValidation("category_id 格式錯誤")
	}

	countries, err := geo.NormalizeList(in.Countries)
	if err != nil {
		return categoryID, nil, errValidation(err.Error())
	}

	// 格式規則存進去之前先確認編得起來，否則上架流程會整個壞掉。
	if in.CodeFormatRegex != nil && *in.CodeFormatRegex != "" {
		if _, err := regexp.Compile(*in.CodeFormatRegex); err != nil {
			return categoryID, nil, errValidation("code_format_regex 不是合法的正規表達式")
		}
	}
	return categoryID, countries, nil
}

type validationError string

func (e validationError) Error() string { return string(e) }
func errValidation(msg string) error    { return validationError(msg) }

func (s *Server) handleCreateMerchant(w http.ResponseWriter, r *http.Request) {
	var in merchantInput
	if err := decodeJSON(w, r, &in); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	categoryID, countries, err := in.validate(true)
	if err != nil {
		badRequest(w, codeInvalidRequest, err.Error())
		return
	}

	m, err := s.store.CreateMerchant(r.Context(), dbgen.CreateMerchantParams{
		Slug:            in.Slug,
		Name:            strings.TrimSpace(in.Name),
		CategoryID:      categoryID,
		LogoUrl:         in.LogoURL,
		SignupUrl:       in.SignupURL,
		RewardDesc:      in.RewardDesc,
		CodeFormatRegex: in.CodeFormatRegex,
		Countries:       countries,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
			return
		}
		// category_id 通過了 uuid.Parse 但指到一個不存在的分類——多半是表單資料過期
		// （分類被刪掉了），是使用者輸入錯誤，不是伺服器故障，不該回 500。
		if store.IsForeignKeyViolation(err) {
			badRequest(w, codeCategoryNotFound, "找不到這個分類，請重新整理後再試")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleUpdateMerchant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	var in merchantInput
	if err := decodeJSON(w, r, &in); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	categoryID, countries, err := in.validate(true)
	if err != nil {
		badRequest(w, codeInvalidRequest, err.Error())
		return
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	m, err := s.store.UpdateMerchant(r.Context(), dbgen.UpdateMerchantParams{
		ID:              id,
		Slug:            in.Slug,
		Name:            strings.TrimSpace(in.Name),
		CategoryID:      categoryID,
		LogoUrl:         in.LogoURL,
		SignupUrl:       in.SignupURL,
		RewardDesc:      in.RewardDesc,
		CodeFormatRegex: in.CodeFormatRegex,
		IsActive:        isActive,
		Countries:       countries,
	})
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeMerchantNotFound, "找不到這個服務商")
			return
		}
		if store.IsUniqueViolation(err) {
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
			return
		}
		if store.IsForeignKeyViolation(err) {
			badRequest(w, codeCategoryNotFound, "找不到這個分類，請重新整理後再試")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type adminUserItem struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	IsPro        bool       `json:"is_pro"`
	ProExpiresAt *time.Time `json:"pro_expires_at"`
	ProStore     *string    `json:"pro_store"`
	ProProductID *string    `json:"pro_product_id"`
}

// handleAdminListUsers 是客服查帳號、查訂閱狀態的入口——退款爭議或要手動
// 補發/撤銷 Pro 時，得先在這裡找到人。
func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 50, 200)
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, err := s.store.ListUsersAdmin(r.Context(), dbgen.ListUsersAdminParams{
		Limit:  limit,
		Offset: offset,
		Q:      q,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	total, err := s.store.CountUsersAdmin(r.Context(), q)
	if err != nil {
		internalError(w, r, err)
		return
	}

	items := make([]adminUserItem, len(rows))
	for i, row := range rows {
		items[i] = adminUserItem{
			ID: row.ID, Email: row.Email, DisplayName: row.DisplayName,
			Status: row.Status, CreatedAt: row.CreatedAt,
			IsPro:        row.IsPro != nil && *row.IsPro,
			ProExpiresAt: row.ProExpiresAt, ProStore: row.ProStore, ProProductID: row.ProProductID,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": items, "total": total})
}

// handleAdminGrantPro 是客服手動補發 Pro 的入口（例如客訴補償、行銷贈送）。
// 走 store='promotional'，跟 RevenueCat webhook 進來的訂閱共用同一張表、
// 同一套 isPro() 判斷——之後真的商店訂閱進來一樣會 upsert 蓋過去，不會衝突。
func (s *Server) handleAdminGrantPro(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	var req struct {
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	ctx := r.Context()
	if _, err := s.store.GetUserByID(ctx, userID); err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeUserNotFound, "找不到這個使用者")
			return
		}
		internalError(w, r, err)
		return
	}

	sub, err := s.store.UpsertSubscription(ctx, dbgen.UpsertSubscriptionParams{
		UserID:      userID,
		Entitlement: s.cfg.ProEntitlement,
		ProductID:   "admin_grant",
		Store:       "promotional",
		IsActive:    true,
		WillRenew:   false,
		ExpiresAt:   req.ExpiresAt,
		RcAppUserID: userID.String(),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

// handleAdminRevokePro 撤銷手動或商店授權的 Pro（退款爭議、誤發）。
// 跟刪分類一樣是冪等操作：本來就沒有生效中的訂閱也回 204，不當錯誤處理。
func (s *Server) handleAdminRevokePro(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	ctx := r.Context()
	if _, err := s.store.GetUserByID(ctx, userID); err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeUserNotFound, "找不到這個使用者")
			return
		}
		internalError(w, r, err)
		return
	}

	if _, err := s.store.RevokeSubscription(ctx, userID); err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
