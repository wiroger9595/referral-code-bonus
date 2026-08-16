package httpapi

import (
	"testing"
	"time"
)

// 這幾條分支錯了都是真金白銀的客訴：CANCELLATION 提早收回等於偷走已付費的期間，
// EXPIRATION 沒收回等於白送。所以逐一釘住。
func TestDecideSubscription(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	future := now.Add(30 * 24 * time.Hour).UnixMilli()
	past := now.Add(-24 * time.Hour).UnixMilli()

	tests := []struct {
		name          string
		eventType     string
		expirationMs  int64
		entitlements  []string
		wantSkip      bool
		wantActive    bool
		wantWillRenew bool
	}{
		{
			name: "購買後在有效期內", eventType: "INITIAL_PURCHASE", expirationMs: future,
			wantActive: true, wantWillRenew: true,
		},
		{
			name: "續訂", eventType: "RENEWAL", expirationMs: future,
			wantActive: true, wantWillRenew: true,
		},
		{
			// 取消只是關掉自動續訂，付到的那段還是要能用。
			name: "取消後付到的期間仍有效", eventType: "CANCELLATION", expirationMs: future,
			wantActive: true, wantWillRenew: false,
		},
		{
			// 扣款失敗時 RevenueCat 還在重試，這段寬限期不能斷人服務。
			name: "帳單問題仍在寬限期內", eventType: "BILLING_ISSUE", expirationMs: future,
			wantActive: true, wantWillRenew: false,
		},
		{
			name: "到期", eventType: "EXPIRATION", expirationMs: past,
			wantActive: false, wantWillRenew: false,
		},
		{
			// 退款要立刻收回，就算到期日還在未來。
			name: "退款立刻收回", eventType: "REFUND", expirationMs: future,
			wantActive: false, wantWillRenew: false,
		},
		{
			name: "暫停訂閱", eventType: "SUBSCRIPTION_PAUSED", expirationMs: future,
			wantActive: false, wantWillRenew: false,
		},
		{
			// 事件可能遲到：型別看起來是購買，但到期日已經過了就不算數。
			name: "遲到的購買事件已過期", eventType: "INITIAL_PURCHASE", expirationMs: past,
			wantActive: false, wantWillRenew: true,
		},
		{
			// promotional / 買斷沒有到期日，要當成無限遠而不是「立刻過期」。
			name: "沒有到期日視為永久", eventType: "NON_RENEWING_PURCHASE", expirationMs: 0,
			wantActive: true, wantWillRenew: true,
		},
		{
			// 之後在 RevenueCat 上多開別的方案時，不該把 pro 狀態蓋掉。
			name: "別的 entitlement 不處理", eventType: "INITIAL_PURCHASE", expirationMs: future,
			entitlements: []string{"vip"}, wantSkip: true,
		},
		{
			name: "同時含 pro 的多 entitlement 要處理", eventType: "INITIAL_PURCHASE", expirationMs: future,
			entitlements: []string{"vip", "pro"}, wantActive: true, wantWillRenew: true,
		},
		{
			// 有些事件型別不帶 entitlement_ids，不能因此當成不相干而略過。
			name: "沒帶 entitlement 照樣處理", eventType: "RENEWAL", expirationMs: future,
			entitlements: nil, wantActive: true, wantWillRenew: true,
		},
		{
			// Paywall events 不帶 entitlement_ids 也不帶到期日。走白名單之前
			// 這會寫成 is_active + expires_at NULL，開一下 paywall 就變永久 Pro。
			name: "paywall 曝光不動訂閱", eventType: "PAYWALL_IMPRESSION", expirationMs: 0,
			wantSkip: true,
		},
		{
			name: "關閉 paywall 不動訂閱", eventType: "PAYWALL_CLOSE", expirationMs: 0,
			wantSkip: true,
		},
		{
			name: "離開挽留優惠不動訂閱", eventType: "PAYWALL_EXIT_OFFER", expirationMs: 0,
			wantSkip: true,
		},
		{
			// RevenueCat 之後新增的事件型別一律當成不相干，不要猜。
			name: "認不得的事件型別不動訂閱", eventType: "SOMETHING_NEW", expirationMs: future,
			entitlements: []string{"pro"}, wantSkip: true,
		},
		{
			// 轉移不帶到期日，要當成不相干而不是「有效且永不過期」。
			name: "訂閱轉移不動訂閱", eventType: "TRANSFER", expirationMs: 0,
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := revenueCatEvent{
				Type:           tt.eventType,
				EntitlementIDs: tt.entitlements,
				ExpirationAtMs: tt.expirationMs,
			}
			got := decideSubscription(ev, "pro", now)

			if got.Skip != tt.wantSkip {
				t.Fatalf("Skip = %v, 想要 %v", got.Skip, tt.wantSkip)
			}
			if tt.wantSkip {
				return
			}
			if got.IsActive != tt.wantActive {
				t.Errorf("IsActive = %v, 想要 %v", got.IsActive, tt.wantActive)
			}
			if got.WillRenew != tt.wantWillRenew {
				t.Errorf("WillRenew = %v, 想要 %v", got.WillRenew, tt.wantWillRenew)
			}
		})
	}
}

// 到期日要原樣傳下去（UTC），沒帶就是 nil —— 存成零值時間的話
// isPro 會把永久授權判成 1970 年就過期了。
func TestDecideSubscriptionExpiresAt(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	want := now.Add(7 * 24 * time.Hour)

	got := decideSubscription(revenueCatEvent{
		Type:           "INITIAL_PURCHASE",
		ExpirationAtMs: want.UnixMilli(),
	}, "pro", now)

	if got.ExpiresAt == nil {
		t.Fatal("ExpiresAt 不該是 nil")
	}
	if !got.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, 想要 %v", got.ExpiresAt, want)
	}

	noExpiry := decideSubscription(revenueCatEvent{Type: "NON_RENEWING_PURCHASE"}, "pro", now)
	if noExpiry.ExpiresAt != nil {
		t.Errorf("沒有到期日時 ExpiresAt 應為 nil，得到 %v", noExpiry.ExpiresAt)
	}
}
