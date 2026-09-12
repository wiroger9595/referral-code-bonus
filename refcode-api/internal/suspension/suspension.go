// Package suspension 處理停權與解除停權要連帶收拾的東西。
//
// 停權不只是把 users.status 改掉：他架上的碼要跟著下架（不然使用者複製到的是
// 一個不會再有人維護的碼），refresh token 要撤掉（不然那張還沒過期的 access
// token 仍然通得過 middleware），而每一個被下架的碼都要留軌跡，解除時才分得出
// 哪些是「因為停權才下架」該還回去的。
//
// 後台（即時）與排程（期滿放人）共用同一份邏輯：兩邊各寫一份會分岔成「後台解除
// 有還碼、排程解除沒還」這種只有當事人會發現的差別。理由與 entitlement 那個
// package 相同。
package suspension

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// ErrNotSuspendable 代表這個人不在可以執行該動作的狀態 —— 停權時是「不存在、
// 已停權或已刪除」，解除時是「目前沒有被停權」。呼叫端轉成 409，不當成系統錯誤：
// 兩個 admin 同時按下同一顆按鈕時第二個就會走到這裡。
var ErrNotSuspendable = errors.New("使用者不在可執行此動作的狀態")

type Manager struct {
	store *store.Store
}

func New(st *store.Store) *Manager {
	return &Manager{store: st}
}

// Suspend 停權一個使用者，回傳連帶下架的碼數。
//
// until 是 nil 代表停到有人手動解除為止；有值的話 Sweep 會在期滿那一輪放人。
//
// 三步刻意不包在同一個交易裡：每一步都是冪等的（狀態條件都帶在 WHERE 上），
// 中途失敗重按一次就會補完剩下的，比為此把 store 的介面全部改成收 tx 划算。
func (m *Manager) Suspend(ctx context.Context, userID uuid.UUID, adminID *uuid.UUID, until *time.Time) (int, error) {
	n, err := m.store.SuspendUser(ctx, dbgen.SuspendUserParams{
		ID:             userID,
		SuspendedUntil: until,
	})
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ErrNotSuspendable
	}

	reason := "上架者已被停權"
	if until != nil {
		// 日期寫進 reason 而不只是存在 users.suspended_until：那一欄解除時會被
		// 清掉，而軌跡要能在事後回答「當初是停到什麼時候」。
		reason = fmt.Sprintf("上架者已被停權至 %s", until.Format("2006-01-02"))
	}

	ids, err := m.store.DisableCodesForSuspendedUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if _, err := m.store.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID:  id,
			AdminID: adminID,
			Action:  "suspend",
			Reason:  reason,
		}); err != nil {
			return 0, err
		}
	}

	if err := m.store.RevokeAllUserTokens(ctx, userID); err != nil {
		return 0, err
	}

	slog.Info("停權使用者", "user_id", userID, "until", until, "disabled_codes", len(ids))
	return len(ids), nil
}

// Reinstate 解除停權，並把「因為停權才被下架」的碼還給他，回傳還原的碼數。
//
// 只還最後一筆軌跡是 suspend 的那些（見 RestoreCodesSuspendedWithUser）：停權期間
// admin 又個別處理過的碼，最新那筆才代表現在的決定，不該被解除停權一起復活。
// 還原也留一列 restore 的軌跡 —— 沒有軌跡的話，事後看到一個 active 的碼查不出
// 它中間被下架過。
//
// adminID 是 nil 代表排程期滿自動放人，不是後台按的。
func (m *Manager) Reinstate(ctx context.Context, userID uuid.UUID, adminID *uuid.UUID) (int, error) {
	n, err := m.store.ReinstateUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ErrNotSuspendable
	}

	reason := "上架者已解除停權"
	if adminID == nil {
		reason = "停權期間屆滿，自動解除"
	}

	ids, err := m.store.RestoreCodesSuspendedWithUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if _, err := m.store.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID:  id,
			AdminID: adminID,
			Action:  "restore",
			Reason:  reason,
		}); err != nil {
			return 0, err
		}
	}

	slog.Info("解除停權", "user_id", userID, "restored_codes", len(ids))
	return len(ids), nil
}

// Sweep 把期滿的停權放掉，回傳解除的人數。排程每輪呼叫一次。
//
// 一個人失敗不中斷其他人：期滿放人是「時間到了就該發生」的事，其中一個因為碼的
// 狀態異常卡住，不該讓排在後面的人多關一輪。錯誤記 log，下一輪會再試 ——
// 那個人仍然留在 ListDueSuspensions 的結果裡。
func (m *Manager) Sweep(ctx context.Context) (int, error) {
	ids, err := m.store.ListDueSuspensions(ctx)
	if err != nil {
		return 0, err
	}

	released := 0
	for _, id := range ids {
		if _, err := m.Reinstate(ctx, id, nil); err != nil {
			// 這一輪撈出來之後才被 admin 手動解除的話會走到這裡，那不是錯誤。
			if errors.Is(err, ErrNotSuspendable) {
				continue
			}
			slog.Error("期滿解除停權失敗，下一輪再試", "user_id", id, "err", err)
			continue
		}
		released++
	}
	return released, nil
}
