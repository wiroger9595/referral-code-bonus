// Package worker 放不由使用者請求觸發的週期性工作。
package worker

import (
	"context"
	"log/slog"
	"time"

	"refcode-api/internal/entitlement"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

type Worker struct {
	store *store.Store
	ent   *entitlement.Syncer
}

func New(st *store.Store, freeLimit int) *Worker {
	return &Worker{store: st, ent: entitlement.New(st, freeLimit)}
}

// Run 阻塞直到 ctx 取消。每件事都採「啟動時先跑一次」：
// 服務重啟後不必等一整個週期才收斂。
func (w *Worker) Run(ctx context.Context) {
	go w.loop(ctx, time.Hour, "expire-codes", w.expireCodes)
	go w.loop(ctx, 24*time.Hour, "ensure-partitions", w.ensurePartitions)
	// 訂閱到期是以「天」為單位的事，六小時一輪對使用者體感沒差別，
	// 但比一天一次能讓 webhook 漏掉的人早一點收斂。
	go w.loop(ctx, 6*time.Hour, "sync-entitlements", w.syncEntitlements)
	<-ctx.Done()
}

func (w *Worker) loop(ctx context.Context, interval time.Duration, name string, fn func(context.Context) error) {
	run := func() {
		if err := fn(ctx); err != nil {
			slog.Error("排程執行失敗", "job", name, "err", err)
		}
	}
	run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (w *Worker) expireCodes(ctx context.Context) error {
	rows, err := w.store.ExpireOverdueCodes(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	// 到期是上架者自己填的期限，不算違規，所以留 auto_expire 軌跡但不影響品質分數。
	for _, row := range rows {
		if _, err := w.store.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID: row.ID,
			Action: "auto_expire",
			Reason: "已過上架者設定的有效期限",
		}); err != nil {
			return err
		}
	}
	slog.Info("到期下架完成", "count", len(rows))
	return nil
}

// syncEntitlements 補 webhook 漏掉的訂閱降級與恢復。細節見 entitlement.Sweep ——
// 兩個方向都掃，而不是只處理到期。
func (w *Worker) syncEntitlements(ctx context.Context) error {
	downgraded, restored, err := w.ent.Sweep(ctx)
	if downgraded > 0 || restored > 0 {
		slog.Info("訂閱額度收斂完成", "downgraded_users", downgraded, "restored_users", restored)
	}
	return err
}

// ensurePartitions 補建事件表的月分區。沒有對應分區時 INSERT 會直接失敗，
// 所以提前開三個月的緩衝，這個排程斷掉幾天也不會出事。
func (w *Worker) ensurePartitions(ctx context.Context) error {
	now := time.Now()
	for i := 0; i < 3; i++ {
		month := now.AddDate(0, i, 0)
		if err := w.store.CreateEventPartition(ctx, month); err != nil {
			return err
		}
	}
	return nil
}
