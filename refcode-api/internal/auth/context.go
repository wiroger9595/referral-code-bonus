package auth

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyAdmin
)

type AdminInfo struct {
	ID   uuid.UUID
	Role string
}

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// UserID 的 ok 為 false 代表匿名訪客 —— 瀏覽與複製都允許匿名，
// 所以呼叫端要能區分「沒登入」和「登入了但不是本人」。
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

func WithAdmin(ctx context.Context, info AdminInfo) context.Context {
	return context.WithValue(ctx, ctxKeyAdmin, info)
}

func Admin(ctx context.Context) (AdminInfo, bool) {
	info, ok := ctx.Value(ctxKeyAdmin).(AdminInfo)
	return info, ok
}
