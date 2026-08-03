package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type EventRow struct {
	CodeID     uuid.UUID
	MerchantID uuid.UUID
	EventType  string
	UserID     *uuid.UUID
	DeviceHash string
	IPHash     string
}

// InsertEvents 一次寫入多筆事件。曝光是「一次請求 N 筆」的形狀，
// 逐筆 INSERT 會讓服務商頁面的 DB round-trip 直接乘以 N。
func (s *Store) InsertEvents(ctx context.Context, rows []EventRow) error {
	if len(rows) == 0 {
		return nil
	}

	_, err := s.Pool.CopyFrom(ctx,
		pgx.Identifier{"referral_code_bonus", "code_events"},
		[]string{"code_id", "merchant_id", "event_type", "user_id", "device_hash", "ip_hash"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			r := rows[i]
			return []any{r.CodeID, r.MerchantID, r.EventType, r.UserID, r.DeviceHash, r.IPHash}, nil
		}),
	)
	return err
}
