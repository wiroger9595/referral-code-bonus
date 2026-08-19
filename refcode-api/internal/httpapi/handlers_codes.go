package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
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

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	userID := user.ID

	resp := toUserResponse(user)
	resp.IsPro, resp.ProExpiresAt = s.isPro(r, userID)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string  `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
		Country     string  `json:"country"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		badRequest(w, codeDisplayNameRequired, "顯示名稱不能空白")
		return
	}

	// 空字串代表清掉所在地，等於退回沒有地區偏好的排序。
	country, err := geo.NormalizePtr(req.Country)
	if err != nil {
		badRequest(w, codeCountryInvalid, err.Error())
		return
	}

	userID, _ := auth.UserID(r.Context())
	user, err := s.store.UpdateUserProfile(r.Context(), dbgen.UpdateUserProfileParams{
		ID:          userID,
		DisplayName: name,
		AvatarUrl:   req.AvatarURL,
		Country:     country,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// handleDeleteMe 刪除帳號。Apple 5.1.1(v) 與 Play 的使用者資料政策都要求
// 「app 內能自己刪」，而且不能只給一個「寄信給我們」的連結。
//
// 刪除是永久的，沒有寬限期也沒有還原 —— 加寬限期就等於沒刪，兩家審核都不吃這套。
func (s *Server) handleDeleteMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// 要求前端把帳號的 email 原樣送回來，確認不是誤觸。
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	ctx := r.Context()
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	userID := user.ID

	if !strings.EqualFold(strings.TrimSpace(req.Confirm), user.Email) {
		badRequest(w, codeDeleteConfirmation, "請輸入你的 email 以確認刪除")
		return
	}

	// 順序有意義：先抹事件（那張表沒有外鍵，不會被 cascade 帶走），
	// 再刪 user 讓其餘子表照外鍵處理。一個交易，中途失敗全部回滾。
	if err := s.store.InTx(ctx, func(q *dbgen.Queries) error {
		if err := q.AnonymizeUserEvents(ctx, &userID); err != nil {
			return err
		}
		return q.DeleteUser(ctx, userID)
	}); err != nil {
		internalError(w, r, err)
		return
	}

	// 大頭照的圖檔要一起刪 —— Apple 5.1.1(v) 要求刪帳號時連使用者產生的內容一起刪，
	// 只清掉資料庫欄位的話，圖還留在 Cloudinary 的公開網址上。
	// 放在交易外面：Cloudinary 掛掉不該讓已經刪掉的帳號回滾回來。
	if user.AvatarPublicID != nil {
		if err := s.images.Destroy(ctx, *user.AvatarPublicID); err != nil {
			slog.Error("帳號已刪除但大頭照沒刪掉，要手動清",
				"user_id", userID, "public_id", *user.AvatarPublicID, "err", err)
		}
	}

	slog.Info("帳號已刪除", "user_id", userID)
	writeJSON(w, http.StatusNoContent, nil)
}

// 碼的兩種來源。referral 是使用者自己的推薦碼（推薦人與被推薦人各拿獎勵），
// discount 是使用者手上的折扣碼（只有使用的人拿到折扣）。
// 值域與下面的折扣欄位規則都跟 00014_code_types.sql 的 CHECK 對齊 ——
// 資料庫那層是最後防線，這裡先擋是為了回得出前端能對照語系檔的錯誤碼。
const (
	codeTypeReferral = "referral"
	codeTypeDiscount = "discount"
)

func isCodeType(v string) bool {
	return v == codeTypeReferral || v == codeTypeDiscount
}

func (s *Server) handleCreateCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MerchantID uuid.UUID `json:"merchant_id"`
		Code       string    `json:"code"`
		Note       string    `json:"note"`
		// null 或缺欄位代表永久有效，不是漏填。
		ExpiresAt *time.Time `json:"expires_at"`
		// 缺欄位當推薦碼：舊版 app 不會送這個欄位，而在這次改動之前
		// 送上來的碼本來就全部是推薦碼。
		CodeType string `json:"code_type"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	codeType := req.CodeType
	if codeType == "" {
		codeType = codeTypeReferral
	}
	if !isCodeType(codeType) {
		badRequest(w, codeCodeTypeInvalid, "碼的類型不在允許的值內")
		return
	}

	ctx := r.Context()
	merchant, err := s.store.GetMerchantByID(ctx, req.MerchantID)
	if err != nil {
		if store.IsNotFound(err) {
			badRequest(w, codeMerchantNotFound, "找不到這個服務商")
			return
		}
		internalError(w, r, err)
		return
	}
	if !merchant.IsActive {
		badRequest(w, codeMerchantClosed, "這個服務商目前沒有開放上架")
		return
	}
	// 有沒有推薦計畫是服務商的事實，不是上架者能選的。沒開放的類型擋在這裡，
	// 否則沒有推薦計畫的服務商底下會塞滿只能靠人工審核擋的假推薦碼。
	if !slices.Contains(merchant.AllowedCodeTypes, codeType) {
		badRequest(w, codeCodeTypeNotAllowed, "這個服務商沒有開放上架這種碼")
		return
	}

	// 推薦碼大小寫敏感，只去頭尾空白，中間原樣保留。折扣碼同樣處理：
	// 活動碼常常刻意用大小寫混排。
	code := strings.TrimSpace(req.Code)
	if code == "" {
		badRequest(w, codeCodeRequired, "碼不能空白")
		return
	}
	// 兩種碼的格式規則不同（推薦碼多半是系統發的固定格式，折扣碼是行銷活動字串），
	// 各驗各的那一條。
	formatRegex := merchant.CodeFormatRegex
	if codeType == codeTypeDiscount {
		formatRegex = merchant.DiscountCodeFormatRegex
	}
	if formatRegex != nil && *formatRegex != "" {
		re, err := regexp.Compile(*formatRegex)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if !re.MatchString(code) {
			badRequest(w, codeCodeFormatMismatch, "格式不符合這家服務商的規則")
			return
		}
	}

	// 折扣碼沒有結構化的優惠欄位，那句「這是什麼優惠」只能寫在備註裡，
	// 空著的話撿到碼的人只看得到一組字串，不知道能換到什麼。
	// 推薦碼不強制：獎勵內容是服務商的推薦計畫本身，服務商頁上就寫著。
	note := strings.TrimSpace(req.Note)
	if codeType == codeTypeDiscount && note == "" {
		badRequest(w, codeDiscountNoteRequired, "折扣碼請在備註說明這是什麼優惠")
		return
	}

	// 有填到期日的話最早只能是明天 —— 今天到期的碼上架當下就已經沒用了，
	// 而且還要排隊審核。下界取「UTC 當日結束」而不是 time.Now()：app 送的是
	// 使用者當地時區的當日 23:59:59，直接跟 now 比會讓東半球的「今天」矇混過關。
	if req.ExpiresAt != nil {

		endOfTodayUTC := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		if !req.ExpiresAt.After(endOfTodayUTC) {
			badRequest(w, codeExpiryInPast, "有效期限最早只能是明天")
			return
		}
	}

	userID, _ := auth.UserID(ctx)

	// 免費方案限制同時上架的張數，Pro 不限。擋在這裡而不是只擋在 app 端 ——
	// client 端的判斷繞得過去，而排序的曝光是有限資源。
	// 兩種碼一起算：限制的是一個帳號佔掉多少曝光，不是佔在哪一種碼上。
	if pro, _ := s.isPro(r, userID); !pro {
		count, err := s.store.CountActiveCodesForUser(ctx, userID)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if count >= int64(s.cfg.FreeActiveCodeLimit) {
			// 這個 code 讓 app 知道要跳 paywall 而不是只顯示錯誤訊息。
			writeError(w, http.StatusForbidden, codeProRequired,
				fmt.Sprintf("免費方案最多同時上架 %d 個碼，升級 Pro 可以無限上架", s.cfg.FreeActiveCodeLimit))
			return
		}
	}

	created, err := s.store.CreateCode(ctx, dbgen.CreateCodeParams{
		UserID:     userID,
		MerchantID: merchant.ID,
		Code:       code,
		Note:       note,
		ExpiresAt:  req.ExpiresAt,
		CodeType:   codeType,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			// 唯一索引現在含 code_type，撞到的一定是同一種碼。
			conflict(w, codeCodeAlreadyListed, "你在這家服務商已經有一個上架中的同類型碼了")
			return
		}
		internalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListMyCodes(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	rows, err := s.store.ListMyCodes(r.Context(), userID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"codes": rows})
}

// handleDisableMyCode 讓上架者把自己的碼從架上撤下來。
//
// 是改 status 不是真刪：事件、回報、審核紀錄都掛在 code_id 上，真刪會把別人留下的
// 回報一起帶走，而且同一組碼重新上架一次就能把負評洗掉。
func (s *Server) handleDisableMyCode(w http.ResponseWriter, r *http.Request) {
	codeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	ctx := r.Context()
	code, err := s.store.GetCodeByID(ctx, codeID)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCodeNotFound, "找不到這個碼")
			return
		}
		internalError(w, r, err)
		return
	}

	userID, _ := auth.UserID(ctx)
	if code.UserID != userID {
		// 同 handleCodeStats：不回 403，免得從錯誤碼看得出這個 id 存不存在。
		notFound(w, codeCodeNotFound, "找不到這個碼")
		return
	}

	// 只有還在流程裡的能撤。已到期、已拒絕、已下架的再撤一次不會改變任何東西，
	// 回錯誤讓 app 知道要重抓列表——多半是列表拿的是舊資料。
	if code.Status != "active" && code.Status != "pending" {
		badRequest(w, codeCodeNotActive, "這個碼目前不在架上")
		return
	}

	var updated dbgen.ReferralCode
	if err := s.store.InTx(ctx, func(q *dbgen.Queries) error {
		updated, err = q.SetCodeStatus(ctx, dbgen.SetCodeStatusParams{
			ID:     codeID,
			Status: "disabled",
		})
		if err != nil {
			return err
		}
		// admin_id 留 null 就是「不是後台動的」，跟管理員下架的紀錄分得開。
		_, err = q.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID: codeID,
			Action: "disable",
			Reason: "上架者自行下架",
		})
		return err
	}); err != nil {
		internalError(w, r, err)
		return
	}

	slog.Info("上架者自行下架", "code_id", codeID, "user_id", userID)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleCodeStats(w http.ResponseWriter, r *http.Request) {
	codeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	ctx := r.Context()
	code, err := s.store.GetCodeByID(ctx, codeID)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCodeNotFound, "找不到這個碼")
			return
		}
		internalError(w, r, err)
		return
	}

	userID, _ := auth.UserID(ctx)
	if code.UserID != userID {
		// 不回 403：讓別人無法從錯誤碼推斷這個 id 是否存在。
		notFound(w, codeCodeNotFound, "找不到這個碼")
		return
	}

	// 數據儀表板是 Pro 賣點之一，免費方案不能看——paywall 上就是這樣賣的。
	if pro, _ := s.isPro(r, userID); !pro {
		writeError(w, http.StatusForbidden, codeProRequired, "查看曝光/點擊數據需要升級 Pro")
		return
	}

	days := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 365 {
		days = v
	}

	stats, err := s.store.GetCodeStats(ctx, dbgen.GetCodeStatsParams{
		CodeID:     codeID,
		WindowDays: int32(days),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code_id":       codeID,
		"window_days":   days,
		"impressions":   stats.Impressions,
		"clicks":        stats.Clicks,
		"copies":        stats.Copies,
		"quality_score": code.QualityScore,
		"status":        code.Status,
	})
}

// handleCreateEvent 收 client 端才知道發生的事件（點擊、複製）。
// impression 不收：那是伺服器決定要顯示哪些碼時記的，開放給 client 送等於開放灌水。
func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CodeID    uuid.UUID `json:"code_id"`
		EventType string    `json:"event_type"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if req.EventType != "click" && req.EventType != "copy" {
		badRequest(w, codeEventTypeInvalid, "event_type 只能是 click 或 copy")
		return
	}

	ctx := r.Context()
	code, err := s.store.GetCodeByID(ctx, req.CodeID)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCodeNotFound, "找不到這個碼")
			return
		}
		internalError(w, r, err)
		return
	}

	var userID *uuid.UUID
	if id, ok := auth.UserID(ctx); ok {
		userID = &id
	}

	if err := s.store.InsertEvents(ctx, []store.EventRow{{
		CodeID:     code.ID,
		MerchantID: code.MerchantID,
		EventType:  req.EventType,
		UserID:     userID,
		DeviceHash: deviceHash(r),
		IPHash:     ipHash(r),
	}}); err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// handleCreateReport 收「這個碼能不能用」。允許匿名 —— 大部分找碼的人不會註冊，
// 而他們才是真正試過碼的人，把回報限制在登入者身上會拿不到資料。
func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	codeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		badRequest(w, codeInvalidID, "id 格式錯誤")
		return
	}

	var req struct {
		Result string `json:"result"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	switch req.Result {
	case "worked", "failed", "invalid_code", "merchant_closed":
	default:
		badRequest(w, codeReportResultInvalid, "result 不在允許的值內")
		return
	}

	ctx := r.Context()
	code, err := s.store.GetCodeByID(ctx, codeID)
	if err != nil {
		if store.IsNotFound(err) {
			notFound(w, codeCodeNotFound, "找不到這個碼")
			return
		}
		internalError(w, r, err)
		return
	}

	var reporterID *uuid.UUID
	if id, ok := auth.UserID(ctx); ok {
		reporterID = &id
	}

	if _, err := s.store.CreateCodeReport(ctx, dbgen.CreateCodeReportParams{
		CodeID:     codeID,
		ReporterID: reporterID,
		DeviceHash: deviceHash(r),
		Result:     req.Result,
	}); err != nil {
		if store.IsNotFound(err) {
			// ON CONFLICT DO NOTHING 沒插入 —— 同一台裝置重複回報，當成已收到。
			writeJSON(w, http.StatusNoContent, nil)
			return
		}
		internalError(w, r, err)
		return
	}

	if err := s.applyReportOutcome(r, code); err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// applyReportOutcome 每收到一筆回報就重算品質分數，必要時自動下架。
// 放在回報的同一個請求裡做，是因為失效碼多留一分鐘就多坑一個人。
func (s *Server) applyReportOutcome(r *http.Request, code dbgen.ReferralCode) error {
	ctx := r.Context()

	stats, err := s.store.GetRecentReportStats(ctx, code.ID)
	if err != nil {
		return err
	}

	score := ranking.QualityScore(stats.Worked, stats.Failed)
	if err := s.store.UpdateCodeQualityScore(ctx, dbgen.UpdateCodeQualityScoreParams{
		ID:           code.ID,
		QualityScore: score,
	}); err != nil {
		return err
	}

	if code.Status != "active" {
		return nil
	}
	if !ranking.ShouldAutoDisable(stats.Failed, stats.Total, s.cfg.AutoDisableMinReports, s.cfg.AutoDisableFailRatio) {
		return nil
	}

	return s.store.InTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.SetCodeStatus(ctx, dbgen.SetCodeStatusParams{
			ID:     code.ID,
			Status: "disabled",
		}); err != nil {
			return err
		}
		_, err := q.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID: code.ID,
			Action: "auto_disable",
			Reason: "近期回報失效比例過高",
		})
		return err
	})
}
