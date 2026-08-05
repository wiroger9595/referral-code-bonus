package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
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

// 有效期上限。太長的到期日等於沒有到期日，失效碼會一直留在列表上。
const maxCodeLifetime = 365 * 24 * time.Hour

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

	slog.Info("帳號已刪除", "user_id", userID)
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleCreateCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MerchantID uuid.UUID `json:"merchant_id"`
		Code       string    `json:"code"`
		Note       string    `json:"note"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
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

	// 推薦碼大小寫敏感，只去頭尾空白，中間原樣保留。
	code := strings.TrimSpace(req.Code)
	if code == "" {
		badRequest(w, codeCodeRequired, "推薦碼不能空白")
		return
	}
	if merchant.CodeFormatRegex != nil && *merchant.CodeFormatRegex != "" {
		re, err := regexp.Compile(*merchant.CodeFormatRegex)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if !re.MatchString(code) {
			badRequest(w, codeCodeFormatMismatch, "推薦碼格式不符合這家服務商的規則")
			return
		}
	}

	switch {
	case req.ExpiresAt.IsZero():
		badRequest(w, codeExpiryRequired, "請填寫有效期限")
		return
	case req.ExpiresAt.Before(time.Now()):
		badRequest(w, codeExpiryInPast, "有效期限不能是過去的時間")
		return
	case req.ExpiresAt.After(time.Now().Add(maxCodeLifetime)):
		badRequest(w, codeExpiryTooFar, "有效期限最長一年")
		return
	}

	userID, _ := auth.UserID(ctx)

	// 免費方案限制同時上架的張數，Pro 不限。擋在這裡而不是只擋在 app 端 ——
	// client 端的判斷繞得過去，而排序的曝光是有限資源。
	if pro, _ := s.isPro(r, userID); !pro {
		count, err := s.store.CountActiveCodesForUser(ctx, userID)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if count >= int64(s.cfg.FreeActiveCodeLimit) {
			// 這個 code 讓 app 知道要跳 paywall 而不是只顯示錯誤訊息。
			writeError(w, http.StatusForbidden, codeProRequired,
				fmt.Sprintf("免費方案最多同時上架 %d 個推薦碼，升級 Pro 可以無限上架", s.cfg.FreeActiveCodeLimit))
			return
		}
	}

	created, err := s.store.CreateCode(ctx, dbgen.CreateCodeParams{
		UserID:     userID,
		MerchantID: merchant.ID,
		Code:       code,
		Note:       strings.TrimSpace(req.Note),
		ExpiresAt:  req.ExpiresAt,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeCodeAlreadyListed, "你在這家服務商已經有一個上架中的推薦碼了")
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
			notFound(w, codeCodeNotFound, "找不到這個推薦碼")
			return
		}
		internalError(w, r, err)
		return
	}

	userID, _ := auth.UserID(ctx)
	if code.UserID != userID {
		// 不回 403：讓別人無法從錯誤碼推斷這個 id 是否存在。
		notFound(w, codeCodeNotFound, "找不到這個推薦碼")
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
			notFound(w, codeCodeNotFound, "找不到這個推薦碼")
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
			notFound(w, codeCodeNotFound, "找不到這個推薦碼")
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
