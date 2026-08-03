package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// RevenueCat webhook 的 event 形狀。只取用得到的欄位，其餘原樣存進
// subscription_events 的 payload —— 之後要對帳或查退款爭議時翻那張表。
type revenueCatEvent struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	AppUserID      string   `json:"app_user_id"`
	ProductID      string   `json:"product_id"`
	EntitlementIDs []string `json:"entitlement_ids"`
	Store          string   `json:"store"`
	Environment    string   `json:"environment"`
	ExpirationAtMs int64    `json:"expiration_at_ms"`
}

// 這幾種事件代表授權已經沒了。其餘（購買、續訂、取消、帳單問題、產品變更…）
// 都還在有效期內 —— 尤其 CANCELLATION 只是關掉自動續訂，使用者付到的那段還是要能用，
// 提早收回會變成盜刷等級的客訴。
var revokingEventTypes = []string{"EXPIRATION", "REFUND", "SUBSCRIPTION_PAUSED"}

// 這幾種代表不會再自動續訂了，但不影響現在是否有效。
var nonRenewingEventTypes = []string{"CANCELLATION", "EXPIRATION", "BILLING_ISSUE", "REFUND", "SUBSCRIPTION_PAUSED"}

// handleRevenueCatWebhook 接 RevenueCat 的訂閱事件。
//
// 回應碼的意義跟一般 API 不一樣：RevenueCat 只要收到非 2xx 就會重試，
// 所以「我們處理不了但重送也沒用」的情況（認不得的 app_user_id、測試事件）
// 一律回 200，只把事件記下來。真正該重試的只有我們自己壞掉的時候。
func (s *Server) handleRevenueCatWebhook(w http.ResponseWriter, r *http.Request) {
	// 沒設定共用密鑰就當這支路由不存在，不要無驗證地收單。
	if s.cfg.RevenueCatWebhookAuth == "" {
		notFound(w, codeNotFound, "找不到這個資源")
		return
	}
	if r.Header.Get("Authorization") != s.cfg.RevenueCatWebhookAuth {
		unauthorized(w, codeWebhookAuthFailed, "webhook 驗證失敗")
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		badRequest(w, codeInvalidRequest, "讀取請求失敗")
		return
	}

	var body struct {
		Event revenueCatEvent `json:"event"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	ev := body.Event
	if ev.ID == "" {
		badRequest(w, codeMissingEventID, "缺少 event id")
		return
	}

	ctx := r.Context()

	// app_user_id 是我們在 Purchases.logIn() 時給的使用者 UUID。
	// 匿名購買（$RCAnonymousID:...）或測試事件會對不到帳號，照樣記錄但不改狀態。
	var userID *uuid.UUID
	if parsed, err := uuid.Parse(ev.AppUserID); err == nil {
		userID = &parsed
	}

	// 先寫事件。rc_event_id 有 unique 限制，重送的事件會在這裡被擋掉，
	// 不會重複套用一次狀態變更。
	if _, err := s.store.InsertSubscriptionEvent(ctx, dbgen.InsertSubscriptionEventParams{
		RcEventID: ev.ID,
		UserID:    userID,
		EventType: ev.Type,
		Payload:   raw,
	}); err != nil {
		if store.IsNotFound(err) {
			slog.Info("RevenueCat 事件重複，略過", "event_id", ev.ID, "type", ev.Type)
			writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
			return
		}
		internalError(w, r, err)
		return
	}

	if userID == nil {
		slog.Warn("RevenueCat 事件的 app_user_id 對不到本地帳號",
			"event_id", ev.ID, "type", ev.Type, "app_user_id", ev.AppUserID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	// 只處理我們認得的 entitlement。RevenueCat 上之後多開別的方案時，
	// 沒對應到的事件不該把 pro 狀態蓋掉。
	if len(ev.EntitlementIDs) > 0 && !slices.Contains(ev.EntitlementIDs, s.cfg.ProEntitlement) {
		slog.Info("RevenueCat 事件不屬於這個 entitlement，略過",
			"event_id", ev.ID, "entitlements", ev.EntitlementIDs)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	var expiresAt *time.Time
	if ev.ExpirationAtMs > 0 {
		t := time.UnixMilli(ev.ExpirationAtMs).UTC()
		expiresAt = &t
	}

	revoked := slices.Contains(revokingEventTypes, ev.Type)
	// 到期日已經過了就不算有效，不管事件類型是什麼 —— 事件可能遲到。
	expired := expiresAt != nil && expiresAt.Before(time.Now())

	if _, err := s.store.UpsertSubscription(ctx, dbgen.UpsertSubscriptionParams{
		UserID:      *userID,
		Entitlement: s.cfg.ProEntitlement,
		ProductID:   ev.ProductID,
		Store:       ev.Store,
		IsActive:    !revoked && !expired,
		WillRenew:   !slices.Contains(nonRenewingEventTypes, ev.Type),
		ExpiresAt:   expiresAt,
		RcAppUserID: ev.AppUserID,
	}); err != nil {
		internalError(w, r, err)
		return
	}

	slog.Info("訂閱狀態已更新",
		"user_id", *userID, "type", ev.Type, "product", ev.ProductID, "env", ev.Environment)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// isPro 回傳這個使用者現在有沒有生效中的 Pro。查不到就是沒有 —— 訂閱是加值，
// 查詢失敗時放行比擋住合理（最糟只是多讓人上架幾個碼）。
func (s *Server) isPro(r *http.Request, userID uuid.UUID) (bool, *time.Time) {
	sub, err := s.store.GetActiveSubscription(r.Context(), userID)
	if err != nil {
		if !store.IsNotFound(err) && !errors.Is(err, r.Context().Err()) {
			slog.Error("查訂閱狀態失敗", "user_id", userID, "err", err)
		}
		return false, nil
	}
	return true, sub.ExpiresAt
}
