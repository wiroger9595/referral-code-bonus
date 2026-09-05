// Package entitlement 處理訂閱狀態變動之後要跟著收斂的東西。
//
// 免費方案限制同時上架的張數，但那個限制原本只擋在「新增」的路徑上
// （handleCreateCode），沒有人在 Pro 失效時回頭處理已經在架上的碼 ——
// 結果付過一次錢的人可以無限期保留超額曝光。這個 package 補的就是那一段。
//
// webhook（即時）與排程（兜底）共用同一份邏輯：兩邊各寫一份會分岔成
// 「webhook 撤掉、排程又恢復」的來回，而使用者只會看到碼一直在閃。
package entitlement

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

type Syncer struct {
	store *store.Store
	// 免費方案能同時上架幾個碼。跟 handleCreateCode 讀的是同一個設定值，
	// 不然新增擋在 3 個、降級收斂到 5 個，兩邊永遠對不起來。
	freeLimit int
}

func New(st *store.Store, freeLimit int) *Syncer {
	return &Syncer{store: st, freeLimit: freeLimit}
}

// Downgrade 把超出免費額度的碼撤下來，留最舊的幾個，回傳撤掉的數量。
//
// 呼叫端要先確定這個使用者現在沒有生效中的 Pro —— 這裡不自己查，
// webhook 手上已經有剛算好的判斷，排程那邊是用 SQL 篩出來的。
func (s *Syncer) Downgrade(ctx context.Context, userID uuid.UUID) (int, error) {
	var ids []uuid.UUID

	// 撤碼跟寫軌跡要在同一個交易裡：只撤沒留軌跡的話，使用者看到碼從架上
	// 消失而我們查不出是誰撤的；續訂時也認不出哪些該恢復。
	err := s.store.InTx(ctx, func(q *dbgen.Queries) error {
		var err error
		ids, err = q.DowngradeExcessCodesForUser(ctx, dbgen.DowngradeExcessCodesForUserParams{
			UserID: userID,
			Keep:   int32(s.freeLimit),
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := q.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
				CodeID: id,
				// admin_id 留空代表不是後台動的，跟管理員違規下架分得開。
				Action: "downgrade",
				Reason: fmt.Sprintf("Pro 已失效，超出免費方案的 %d 個額度", s.freeLimit),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if len(ids) > 0 {
		slog.Info("Pro 失效，超額的碼已下架",
			"user_id", userID, "count", len(ids), "kept", s.freeLimit)
	}
	return len(ids), nil
}

// Restore 把當初因為降級被撤掉的碼放回架上，回傳恢復的數量。
//
// 不用管張數：恢復的前提是 Pro 生效，而 Pro 不限張數。降級期間使用者
// 自己又上架的碼也留著，總數超過免費額度沒關係 —— 等下次 Pro 再失效時
// Downgrade 會重新收斂。
func (s *Syncer) Restore(ctx context.Context, userID uuid.UUID) (int, error) {
	rows, err := s.store.ListDowngradedCodesForUser(ctx, userID)
	if err != nil {
		return 0, err
	}

	restored := 0
	for _, row := range rows {
		status := restoredStatus(row.ActivatedAt)

		// 一筆碼一個交易，不是整批一個 —— 下面那個 unique violation 要能
		// 只跳過單筆，包成一個大交易的話一筆撞到就全部回滾。
		err := s.store.InTx(ctx, func(q *dbgen.Queries) error {
			if _, err := q.SetCodeStatus(ctx, dbgen.SetCodeStatusParams{
				ID:     row.ID,
				Status: status,
			}); err != nil {
				return err
			}
			_, err := q.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
				CodeID: row.ID,
				Action: "restore",
				Reason: "Pro 已恢復，重新上架",
			})
			return err
		})
		if err != nil {
			// codes_user_merchant_type_live_idx 只涵蓋 pending/active，所以
			// 降級期間使用者可以在同一家同類型重新上架一個新碼。那筆才是他
			// 現在在用的，舊的維持 disabled，不要把新的擠掉。
			if store.IsUniqueViolation(err) {
				slog.Info("降級期間已重新上架同類型的碼，舊的不恢復",
					"user_id", userID, "code_id", row.ID)
				continue
			}
			return restored, err
		}
		restored++
	}

	if restored > 0 {
		slog.Info("Pro 恢復，被降級撤掉的碼已放回架上",
			"user_id", userID, "count", restored)
	}
	return restored, nil
}

// restoredStatus 決定被降級撤掉的碼要恢復成哪個狀態。從沒上架過的（還在審核
// 就被撤）要回 pending，直接放成 active 等於讓它跳過審核 —— activated_at
// 有值代表它曾經過審上架。
//
// 抽成函式純粹是為了測得到：Restore 其餘每一步都要碰 DB，而這條分支錯了會讓
// 沒審過的碼直接見客。
func restoredStatus(activatedAt *time.Time) string {
	if activatedAt == nil {
		return "pending"
	}
	return "active"
}

// Sweep 是 webhook 的兜底，兩個方向都掃：訂閱沒了還超額的降級，Pro 生效
// 卻還躺著降級碼的恢復。回傳各自處理了幾個使用者。
//
// 需要兜底是因為 webhook 只有一次機會 —— 事件寫進 subscription_events 之後
// 靠 rc_event_id 去重，RevenueCat 重送會被當成 duplicate 直接跳過，不會重跑
// 到收斂那一步。webhook 本身也會漏：送不到、app_user_id 對不到本地帳號
// （刪帳號重建）、或我們回了 500 之後它放棄重送。
func (s *Syncer) Sweep(ctx context.Context) (downgraded, restored int, err error) {
	overLimit, err := s.store.ListUsersOverFreeLimit(ctx, int32(s.freeLimit))
	if err != nil {
		return 0, 0, err
	}
	for _, userID := range overLimit {
		// 一個人失敗不該讓整輪停掉，下一輪還會再掃到他。
		if n, err := s.Downgrade(ctx, userID); err != nil {
			slog.Error("排程降級失敗", "user_id", userID, "err", err)
		} else if n > 0 {
			downgraded++
		}
	}

	pending, err := s.store.ListProUsersWithDowngradedCodes(ctx)
	if err != nil {
		// 降級那半已經做完了，不要把它的成果連帶回報成失敗。
		return downgraded, 0, err
	}
	for _, userID := range pending {
		if n, err := s.Restore(ctx, userID); err != nil {
			slog.Error("排程恢復失敗", "user_id", userID, "err", err)
		} else if n > 0 {
			restored++
		}
	}
	return downgraded, restored, nil
}
