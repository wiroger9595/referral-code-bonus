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

// 後台碼列表能篩的狀態。pending 不在裡面 —— 那批屬於審核佇列，
// 在這裡再列一次會讓同一件工作在兩個頁面各做一半。
var adminCodeStatuses = map[string]bool{
	"active":   true,
	"disabled": true,
	"expired":  true,
	"rejected": true,
}

// handleAdminListCodes 是已上架推薦碼的列表，帶使用者回報統計。
// 排序把有負面回報的放最前面（見 SQL），所以不分頁翻到底也看得到該處理的碼。
func (s *Server) handleAdminListCodes(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 50, 200)

	var status *string
	if v := r.URL.Query().Get("status"); v != "" {
		if !adminCodeStatuses[v] {
			badRequest(w, codeCodeStatusInvalid, "status 只能是 active / disabled / expired / rejected")
			return
		}
		status = &v
	}

	var search *string
	// 搜尋字串會進 ILIKE，這裡跟公開的搜尋一樣要先把萬用字元跳脫掉。
	if v := strings.TrimSpace(r.URL.Query().Get("q")); v != "" {
		escaped := escapeLike(v)
		search = &escaped
	}

	rows, err := s.store.ListCodesForAdmin(r.Context(), dbgen.ListCodesForAdminParams{
		Limit:  limit,
		Offset: offset,
		Status: status,
		Search: search,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	// total_count 是 window function 的結果，每一列都一樣；沒有列就是 0 筆。
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}
	writeJSON(w, http.StatusOK, map[string]any{"codes": rows, "total": total})
}

// handleListAutoDisabledCodes 是被系統自動下架、還沒有人複核的碼。
// 這份清單要跟一般列表分開：自動下架有誤判的可能（少數幾筆惡意回報就湊得出門檻），
// 混在幾百筆已上架的碼裡沒有人會主動去找。
func (s *Server) handleListAutoDisabledCodes(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 50, 200)

	rows, err := s.store.ListAutoDisabledCodes(r.Context(), dbgen.ListAutoDisabledCodesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
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
			notFound(w, codeCodeNotFound, "找不到這個碼")
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

// handleListMerchantSuggestions 是使用者提報的待審平台。
// 審過的不再回來——建議單只會被審一次，通過的那些已經變成服務商了。
func (s *Server) handleListMerchantSuggestions(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 50, 200)

	rows, err := s.store.ListPendingMerchantSuggestions(r.Context(), dbgen.ListPendingMerchantSuggestionsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	// total_count 是 window function 的結果，每一列都一樣；沒有列就是 0 筆。
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": rows, "total": total})
}

// handleReviewMerchantSuggestion 審一筆平台建議。
//
// 通過會直接建出一家停用的服務商——使用者填得出來的只有名稱與官網，slug 與分類
// 得由 admin 在這裡補（兩個都是 merchants 的必要欄位），獎勵說明、logo 與適用國家
// 留給後續在服務商頁編輯。跟 cmd/appimport 匯進來的那批一樣是 is_active=false 的
// 草稿：沒有獎勵說明就上架，使用者點進去只會看到空白。
func (s *Server) handleReviewMerchantSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
		// 只有 approve 用得到：要建出來的那家服務商缺的兩個必填欄位。
		Slug       string `json:"slug"`
		CategoryID string `json:"category_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if req.Action != "approve" && req.Action != "reject" {
		badRequest(w, codeReviewActionInvalid, "action 只能是 approve / reject")
		return
	}

	var categoryID uuid.UUID
	if req.Action == "approve" {
		if !slugPattern.MatchString(req.Slug) {
			badRequest(w, codeSlugInvalid, "slug 只能是小寫英數字與連字號")
			return
		}
		if categoryID, err = uuid.Parse(req.CategoryID); err != nil {
			badRequest(w, codeInvalidRequest, "category_id 格式錯誤")
			return
		}
	} else if strings.TrimSpace(req.Reason) == "" {
		// 跟拒絕推薦碼一樣要留下理由：使用者問「為什麼沒上」時要拿得出當初的判斷。
		badRequest(w, codeReviewReasonRequired, "拒絕必須填寫原因")
		return
	}

	ctx := r.Context()
	suggestion, err := s.store.GetMerchantSuggestionByID(ctx, id)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeSuggestionNotFound, "找不到這筆平台建議")
			return
		}
		internalError(w, r, err)
		return
	}
	if suggestion.Status != "pending" {
		conflict(w, codeSuggestionReviewed, "這筆建議已經審過了")
		return
	}

	admin, _ := auth.Admin(ctx)
	params := dbgen.ReviewMerchantSuggestionParams{
		ID:           id,
		Status:       "rejected",
		ReviewedBy:   &admin.ID,
		ReviewReason: strings.TrimSpace(req.Reason),
	}

	var created dbgen.Merchant
	err = s.store.InTx(ctx, func(q *dbgen.Queries) error {
		if req.Action == "approve" {
			// 使用者提報的平台不分地區（他填不出適用國家），countries 留空，
			// 由後台之後在服務商頁補——空陣列在目錄裡是「哪裡都看得到」。
			var cerr error
			created, cerr = q.CreateImportedMerchant(ctx, dbgen.CreateImportedMerchantParams{
				Slug:       req.Slug,
				Name:       suggestion.Name,
				CategoryID: categoryID,
				SignupUrl:  suggestion.SignupUrl,
				Countries:  []string{},
			})
			if cerr != nil {
				return cerr
			}
			params.Status = "approved"
			params.MerchantID = &created.ID
		}

		// 撈不到列代表狀態在剛才那次讀取之後被別人改掉了（兩個 admin 同時審）。
		// 回傳的錯誤會讓整個交易回滾，上面建出來的服務商也跟著不算數。
		_, err := q.ReviewMerchantSuggestion(ctx, params)
		return err
	})
	if err != nil {
		switch {
		case store.IsNotFound(err):
			conflict(w, codeSuggestionReviewed, "這筆建議已經審過了")
		case store.IsUniqueViolation(err):
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
		case store.IsForeignKeyViolation(err):
			badRequest(w, codeCategoryNotFound, "找不到這個分類，請重新整理後再試")
		default:
			internalError(w, r, err)
		}
		return
	}

	// 拒絕時沒有服務商，merchant 是 null——後台靠它決定要不要顯示「去補資料」的連結。
	if req.Action == "approve" {
		writeJSON(w, http.StatusOK, map[string]any{"merchant": created})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"merchant": nil})
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
		// 譯文留空代表還沒翻，公開 API 會退回中文那份（見 handlers_merchants 的 localized）。
		NameEn *string `json:"name_en"`
		NameJa *string `json:"name_ja"`
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
		NameEn:    trimmedOrNil(req.NameEn),
		NameJa:    trimmedOrNil(req.NameJa),
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
		// 譯文留空代表還沒翻，公開 API 會退回中文那份（見 handlers_merchants 的 localized）。
		NameEn *string `json:"name_en"`
		NameJa *string `json:"name_ja"`
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
		NameEn:    trimmedOrNil(req.NameEn),
		NameJa:    trimmedOrNil(req.NameJa),
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
	// 折扣碼另一條規則。兩種碼的格式差很多（推薦碼是系統發的固定格式，
	// 折扣碼是行銷活動字串），共用一條會把其中一種全部誤擋。
	DiscountCodeFormatRegex *string `json:"discount_code_format_regex"`
	// 這家收哪幾種碼。留空當成只收推薦碼，維持這個欄位存在之前的行為。
	AllowedCodeTypes []string `json:"allowed_code_types"`
	IsActive         *bool    `json:"is_active"`
	// 獎勵說明的譯文。留空代表還沒翻，公開 API 會退回中文那份。
	// 服務商名（Name）刻意沒有譯文欄位 —— 那是品牌名。
	RewardDescEn *string `json:"reward_desc_en"`
	RewardDescJa *string `json:"reward_desc_ja"`
	// 適用國家（ISO 3166-1 alpha-2）。留空代表不分地區，目錄排序時當中性處理。
	Countries []string `json:"countries"`
}

// validate 回傳正規化後的 category id、適用國家與開放的碼類型。
func (in merchantInput) validate(requireSlug bool) (uuid.UUID, []string, []string, error) {
	var categoryID uuid.UUID

	if requireSlug && !slugPattern.MatchString(in.Slug) {
		return categoryID, nil, nil, errValidation("slug 只能是小寫英數字與連字號")
	}
	if strings.TrimSpace(in.Name) == "" {
		return categoryID, nil, nil, errValidation("名稱不能空白")
	}
	if !strings.HasPrefix(in.SignupURL, "https://") {
		return categoryID, nil, nil, errValidation("註冊連結必須是 https")
	}

	categoryID, err := uuid.Parse(in.CategoryID)
	if err != nil {
		return categoryID, nil, nil, errValidation("category_id 格式錯誤")
	}

	countries, err := geo.NormalizeList(in.Countries)
	if err != nil {
		return categoryID, nil, nil, errValidation(err.Error())
	}

	codeTypes, err := normalizeCodeTypes(in.AllowedCodeTypes)
	if err != nil {
		return categoryID, nil, nil, err
	}

	// 格式規則存進去之前先確認編得起來，否則上架流程會整個壞掉。兩條都要驗。
	for label, re := range map[string]*string{
		"code_format_regex":          in.CodeFormatRegex,
		"discount_code_format_regex": in.DiscountCodeFormatRegex,
	} {
		if re != nil && *re != "" {
			if _, err := regexp.Compile(*re); err != nil {
				return categoryID, nil, nil, errValidation(label + " 不是合法的正規表達式")
			}
		}
	}
	return categoryID, countries, codeTypes, nil
}

// normalizeCodeTypes 去重並固定順序（referral 在前），空的當成只收推薦碼。
// 順序固定是為了讓後台表單每次讀回來的勾選狀態一致，不受送出時的順序影響。
func normalizeCodeTypes(in []string) ([]string, error) {
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !isCodeType(v) {
			return nil, errValidation("allowed_code_types 只能是 referral 或 discount")
		}
		seen[v] = true
	}
	if len(seen) == 0 {
		return []string{codeTypeReferral}, nil
	}
	out := make([]string, 0, 2)
	for _, v := range []string{codeTypeReferral, codeTypeDiscount} {
		if seen[v] {
			out = append(out, v)
		}
	}
	return out, nil
}

type validationError string

func (e validationError) Error() string { return string(e) }
func errValidation(msg string) error    { return validationError(msg) }

// trimmedOrNil 把譯文欄位正規化成「有值」或 NULL。後台的輸入框清空之後送過來的是
// 空字串，直接存進去會讓公開 API 把那個語言顯示成一片空白 —— 存 NULL 才會退回中文。
func trimmedOrNil(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *Server) handleCreateMerchant(w http.ResponseWriter, r *http.Request) {
	var in merchantInput
	if err := decodeJSON(w, r, &in); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	categoryID, countries, codeTypes, err := in.validate(true)
	if err != nil {
		badRequest(w, codeInvalidRequest, err.Error())
		return
	}

	m, err := s.store.CreateMerchant(r.Context(), dbgen.CreateMerchantParams{
		Slug:                    in.Slug,
		Name:                    strings.TrimSpace(in.Name),
		CategoryID:              categoryID,
		LogoUrl:                 in.LogoURL,
		SignupUrl:               in.SignupURL,
		RewardDesc:              in.RewardDesc,
		RewardDescEn:            trimmedOrNil(in.RewardDescEn),
		RewardDescJa:            trimmedOrNil(in.RewardDescJa),
		CodeFormatRegex:         in.CodeFormatRegex,
		DiscountCodeFormatRegex: in.DiscountCodeFormatRegex,
		AllowedCodeTypes:        codeTypes,
		Countries:               countries,
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
	categoryID, countries, codeTypes, err := in.validate(true)
	if err != nil {
		badRequest(w, codeInvalidRequest, err.Error())
		return
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	m, err := s.store.UpdateMerchant(r.Context(), dbgen.UpdateMerchantParams{
		ID:                      id,
		Slug:                    in.Slug,
		Name:                    strings.TrimSpace(in.Name),
		CategoryID:              categoryID,
		LogoUrl:                 in.LogoURL,
		SignupUrl:               in.SignupURL,
		RewardDesc:              in.RewardDesc,
		RewardDescEn:            trimmedOrNil(in.RewardDescEn),
		RewardDescJa:            trimmedOrNil(in.RewardDescJa),
		CodeFormatRegex:         in.CodeFormatRegex,
		DiscountCodeFormatRegex: in.DiscountCodeFormatRegex,
		AllowedCodeTypes:        codeTypes,
		IsActive:                isActive,
		Countries:               countries,
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
