package entitlement

import (
	"testing"
	"time"
)

// 這條分支錯了會讓從沒過審的碼直接見客：降級是撤在 pending 跟 active 兩種狀態上
// （額度算的是兩種相加），恢復時如果一律放回 active，那些還在審核佇列裡就被撤掉的
// 碼會跳過審核直接上架。
func TestRestoredStatus(t *testing.T) {
	activated := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	// 曾經上架但時間是零值 —— 指標非 nil 就算過審，不要改成看 IsZero()。
	zero := time.Time{}

	tests := []struct {
		name        string
		activatedAt *time.Time
		want        string
	}{
		{name: "曾經過審上架過的回架上", activatedAt: &activated, want: "active"},
		{name: "還在審核就被撤的回審核佇列", activatedAt: nil, want: "pending"},
		{name: "activated_at 是零值仍算過審", activatedAt: &zero, want: "active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := restoredStatus(tt.activatedAt); got != tt.want {
				t.Errorf("restoredStatus() = %q, 預期 %q", got, tt.want)
			}
		})
	}
}
