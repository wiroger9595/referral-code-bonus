package httpapi

import (
	"net/http"
	"regexp"
	"strings"

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
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		SortOrder int32  `json:"sort_order"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if !slugPattern.MatchString(req.Slug) {
		badRequest(w, codeSlugInvalid, "slug 只能是小寫英數字與連字號")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		badRequest(w, codeNameRequired, "名稱不能空白")
		return
	}

	ctx := r.Context()
	var cat dbgen.MerchantCategory
	err := s.store.InTx(ctx, func(q *dbgen.Queries) error {
		// 新建的分類可能剛好用了某個被改掉的舊 slug —— 那筆轉址要讓位給現用的。
		if err := q.ClaimCategorySlug(ctx, req.Slug); err != nil {
			return err
		}
		var err error
		cat, err = q.CreateCategory(ctx, dbgen.CreateCategoryParams{
			Slug:      req.Slug,
			Name:      strings.TrimSpace(req.Name),
			SortOrder: req.SortOrder,
		})
		return err
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
			return
		}
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
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		SortOrder int32  `json:"sort_order"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if !slugPattern.MatchString(req.Slug) {
		badRequest(w, codeSlugInvalid, "slug 只能是小寫英數字與連字號")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		badRequest(w, codeNameRequired, "名稱不能空白")
		return
	}

	ctx := r.Context()
	current, err := s.store.GetCategoryByID(ctx, id)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCategoryNotFound, "找不到這個分類")
			return
		}
		internalError(w, r, err)
		return
	}

	// 跟服務商同一套：改掉的 slug 進歷史表，舊網址才轉得回來。
	var cat dbgen.MerchantCategory
	err = s.store.InTx(ctx, func(q *dbgen.Queries) error {
		if err := q.ClaimCategorySlug(ctx, req.Slug); err != nil {
			return err
		}
		cat, err = q.UpdateCategory(ctx, dbgen.UpdateCategoryParams{
			ID:        id,
			Slug:      req.Slug,
			Name:      strings.TrimSpace(req.Name),
			SortOrder: req.SortOrder,
		})
		if err != nil {
			return err
		}
		if current.Slug == req.Slug {
			return nil
		}
		return q.RecordCategorySlugChange(ctx, dbgen.RecordCategorySlugChangeParams{
			Slug:       current.Slug,
			CategoryID: id,
		})
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
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

	ctx := r.Context()
	var m dbgen.Merchant
	err = s.store.InTx(ctx, func(q *dbgen.Queries) error {
		// 新建的服務商可能剛好用了某個被改掉的舊 slug —— 那筆轉址要讓位給現用的。
		if err := q.ClaimMerchantSlug(ctx, in.Slug); err != nil {
			return err
		}
		var err error
		m, err = q.CreateMerchant(ctx, dbgen.CreateMerchantParams{
			Slug:            in.Slug,
			Name:            strings.TrimSpace(in.Name),
			CategoryID:      categoryID,
			LogoUrl:         in.LogoURL,
			SignupUrl:       in.SignupURL,
			RewardDesc:      in.RewardDesc,
			CodeFormatRegex: in.CodeFormatRegex,
			Countries:       countries,
		})
		return err
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
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

	ctx := r.Context()
	current, err := s.store.GetMerchantByID(ctx, id)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeMerchantNotFound, "找不到這個服務商")
			return
		}
		internalError(w, r, err)
		return
	}

	// slug 可以改，但舊網址還在外面流傳，所以改名的同時要把舊的記進歷史表，
	// 公開端點才找得回來並 301（見 00007_slug_history.sql）。
	// 三步要嘛全成功要嘛全不做：只寫了一半會留下指向錯誤服務商的轉址。
	var m dbgen.Merchant
	err = s.store.InTx(ctx, func(q *dbgen.Queries) error {
		// 這個 slug 從現在起是現用的，不該再同時是誰的舊網址。
		if err := q.ClaimMerchantSlug(ctx, in.Slug); err != nil {
			return err
		}
		m, err = q.UpdateMerchant(ctx, dbgen.UpdateMerchantParams{
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
			return err
		}
		if current.Slug == in.Slug {
			return nil
		}
		return q.RecordMerchantSlugChange(ctx, dbgen.RecordMerchantSlugChangeParams{
			Slug:       current.Slug,
			MerchantID: id,
		})
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeSlugTaken, "這個 slug 已經有人用了")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
