// Package worker 放不由使用者請求觸發的週期性工作。
//
// 排程的開關與間隔存在資料庫（scheduled_jobs），不寫死在這裡 —— 後台可以停掉
// 某一支、改頻率、或手動觸發一次，不必為此發一版。程式碼這邊只負責「有哪些
// job、預設多久一次、實際做什麼」。
//
// 每輪由 tick 掃過所有註冊的 job，各自向資料庫認領（ClaimDueJob）。認領是一次
// atomic 的 UPDATE，所以 API 擴到多個 instance 時同一支 job 只會有一個在跑。
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"refcode-api/internal/appimport"
	"refcode-api/internal/entitlement"
	"refcode-api/internal/logobackfill"
	"refcode-api/internal/merchantaudit"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// tickInterval 是排程器自己醒來的頻率，不是任何一支 job 的間隔 ——
// 每次醒來問資料庫「誰到期了」。後台按「立即執行」之後最多等這麼久才會跑起來。
const tickInterval = 30 * time.Second

// runRetentionDays 執行紀錄保留天數。夠久到能看出「這支是不是這幾天一直在失敗」，
// 又不會讓表無限長大。
const runRetentionDays = 30

// Job 是一支可排程的工作。
//
// Fn 回傳的字串會存進 job_runs.summary 給後台直接顯示，所以要寫成一句人看得懂的
// 結果（「新增 3 家、補國別 12 家」）；這輪沒事做就回空字串，後台會顯示成沒有變動。
type Job struct {
	Name string
	// 預設間隔。資料庫裡沒有這一列時用它建；之後改這個值只會影響「沒有被後台
	// 調整過」的那些（見 UpsertJob）。
	DefaultInterval time.Duration
	// 後台列表上的說明，讓按下「立即執行」的人知道這支會做什麼。
	Description string
	Fn          func(context.Context) (string, error)
}

// Deps 是各支 job 需要的外部依賴。集中成一個 struct 而不是攤在 New 的參數列 ——
// 爬蟲類的 job 之後還會增加，每加一支就改一次 New 的簽名太吵。
type Deps struct {
	FreeActiveCodeLimit int
	// 補 logo 要上傳 Cloudinary。nil 代表沒設定，那支 job 就不會被註冊。
	Images logobackfill.Uploader
	// app-import 要匯哪些國別的排行榜。空的話那支 job 不會被註冊。
	ImportCountries []string
}

type Worker struct {
	store *store.Store
	jobs  []Job
}

func New(st *store.Store, deps Deps) *Worker {
	ent := entitlement.New(st, deps.FreeActiveCodeLimit)
	audit := merchantaudit.New(st)

	w := &Worker{store: st}
	w.jobs = []Job{
		{
			Name:            "expire-codes",
			DefaultInterval: time.Hour,
			Description:     "把過了上架者設定期限的推薦碼下架",
			Fn:              func(ctx context.Context) (string, error) { return expireCodes(ctx, st) },
		},
		{
			Name:            "ensure-partitions",
			DefaultInterval: 24 * time.Hour,
			Description:     "補建事件表未來三個月的分區",
			Fn:              func(ctx context.Context) (string, error) { return ensurePartitions(ctx, st) },
		},
		{
			// 訂閱到期是以「天」為單位的事，六小時一輪對使用者體感沒差別，
			// 但比一天一次能讓 webhook 漏掉的人早一點收斂。
			Name:            "sync-entitlements",
			DefaultInterval: 6 * time.Hour,
			Description:     "補 webhook 漏掉的訂閱降級與恢復",
			Fn:              func(ctx context.Context) (string, error) { return syncEntitlements(ctx, ent) },
		},
		{
			// 每家平台一週查一次，但排程每天醒來 —— 「誰滿一週了」是查資料庫算的，
			// 不是靠這裡的間隔記，細節見 ListMerchantsDueForCodeAudit。
			Name:            "audit-merchant-codes",
			DefaultInterval: 24 * time.Hour,
			Description:     "爬各平台頁面確認服務商還有沒有在發推薦碼（只回報不下架）",
			Fn:              func(ctx context.Context) (string, error) { return auditMerchantCodes(ctx, audit) },
		},
		{
			Name:            "prune-job-runs",
			DefaultInterval: 24 * time.Hour,
			Description:     "清掉超過 30 天的排程執行紀錄",
			Fn:              func(ctx context.Context) (string, error) { return pruneJobRuns(ctx, st) },
		},
	}

	// 兩支爬蟲原本只有 CLI（cmd/appimport、cmd/logobackfill），現在也掛進排程。
	// 缺少必要設定時不註冊，而不是註冊了每輪失敗一次 —— 後台列表上該看到的是
	// 「這支不存在」，不是「這支一直紅著」。
	if len(deps.ImportCountries) > 0 {
		imp := appimport.New(st, deps.ImportCountries)
		w.jobs = append(w.jobs, Job{
			Name: "app-import",
			// 排行榜的變動是以週為單位的，一天爬一次只是多打 Apple 六次。
			DefaultInterval: 7 * 24 * time.Hour,
			Description:     "爬 App Store 排行榜，把新的 app 匯進服務商目錄（一律是停用草稿）",
			Fn:              imp.Sweep,
		})
	}
	if deps.Images != nil {
		bf := logobackfill.New(st, deps.Images)
		w.jobs = append(w.jobs, Job{
			Name:            "logo-backfill",
			DefaultInterval: 24 * time.Hour,
			Description:     "幫沒有 logo 的服務商從官網或 iTunes 補圖並上傳 Cloudinary",
			Fn:              bf.Sweep,
		})
	}

	return w
}

// Jobs 給 httpapi 用：後台列表要顯示每支的說明，而說明只有程式碼這邊有
// （資料庫那張表只存開關與間隔）。
func (w *Worker) Jobs() []Job { return w.jobs }

// Run 阻塞直到 ctx 取消。
func (w *Worker) Run(ctx context.Context) {
	w.register(ctx)

	// 上一個 process 被硬殺時，job_runs 會留下永遠停在 running 的列。啟動時收乾淨
	// —— 用「這次啟動的時間」當界線，不會誤傷別的 instance 正在跑的那些。
	if n, err := w.store.FailStaleJobRuns(ctx, time.Now()); err != nil {
		slog.Error("清理未結束的執行紀錄失敗", "err", err)
	} else if n > 0 {
		slog.Warn("上次有排程沒有正常結束", "count", n)
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// register 把程式裡定義的 job 補進資料庫。已經存在的列只有間隔可能被更新，
// enabled 與 last_run_at 不動（見 UpsertJob）。
func (w *Worker) register(ctx context.Context) {
	for _, j := range w.jobs {
		err := w.store.UpsertJob(ctx, dbgen.UpsertJobParams{
			Name:            j.Name,
			IntervalSeconds: int32(j.DefaultInterval.Seconds()),
		})
		if err != nil {
			slog.Error("註冊排程失敗", "job", j.Name, "err", err)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	for _, j := range w.jobs {
		// 每支各開一個 goroutine：一支爬蟲跑半小時不該擋住 expire-codes。
		// 重複觸發由 ClaimDueJob 擋 —— last_run_at 在認領當下就寫下去了，
		// 還在跑的那支不會被下一輪再認領一次。
		go w.claimAndRun(ctx, j)
	}
}

func (w *Worker) claimAndRun(ctx context.Context, j Job) {
	manual, err := w.store.ClaimDueJob(ctx, j.Name)
	if err != nil {
		// no rows = 還沒到期、被關掉、或被別的 instance 搶走了，都是正常的。
		if !store.IsNotFound(err) {
			slog.Error("認領排程失敗", "job", j.Name, "err", err)
		}
		return
	}

	trigger := "schedule"
	if manual {
		trigger = "manual"
	}

	run, err := w.store.StartJobRun(ctx, dbgen.StartJobRunParams{
		JobName: j.Name,
		Trigger: trigger,
	})
	if err != nil {
		slog.Error("寫入執行紀錄失敗", "job", j.Name, "err", err)
		return
	}

	started := time.Now()
	summary, runErr := j.Fn(ctx)

	status, errText := "ok", ""
	if runErr != nil {
		status, errText = "failed", runErr.Error()
		slog.Error("排程執行失敗", "job", j.Name, "trigger", trigger, "err", runErr)
	} else if summary != "" {
		slog.Info("排程完成", "job", j.Name, "trigger", trigger,
			"summary", summary, "took", time.Since(started).Round(time.Second))
	}

	// ctx 已經取消時（服務要關了）不能拿它去寫收尾 —— 那個寫入一定失敗，
	// 這列就會留在 running。另開一個短 timeout 的 ctx 把結果寫完。
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := w.store.FinishJobRun(finishCtx, dbgen.FinishJobRunParams{
		ID:      run.ID,
		Status:  status,
		Summary: summary,
		Error:   errText,
	}); err != nil {
		slog.Error("更新執行紀錄失敗", "job", j.Name, "err", err)
	}
}

func expireCodes(ctx context.Context, st *store.Store) (string, error) {
	rows, err := st.ExpireOverdueCodes(ctx)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}

	// 到期是上架者自己填的期限，不算違規，所以留 auto_expire 軌跡但不影響品質分數。
	for _, row := range rows {
		if _, err := st.CreateCodeReview(ctx, dbgen.CreateCodeReviewParams{
			CodeID: row.ID,
			Action: "auto_expire",
			Reason: "已過上架者設定的有效期限",
		}); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("到期下架 %d 個碼", len(rows)), nil
}

// syncEntitlements 補 webhook 漏掉的訂閱降級與恢復。細節見 entitlement.Sweep ——
// 兩個方向都掃，而不是只處理到期。
func syncEntitlements(ctx context.Context, ent *entitlement.Syncer) (string, error) {
	downgraded, restored, err := ent.Sweep(ctx)
	if err != nil {
		return "", err
	}
	if downgraded == 0 && restored == 0 {
		return "", nil
	}
	return fmt.Sprintf("降級 %d 人、恢復 %d 人", downgraded, restored), nil
}

// auditMerchantCodes 去平台頁面上確認目錄裡的服務商還有沒有在發推薦碼，
// 連續兩週找不到的列進後台的複檢清單。只回報不下架 —— 判斷方式、實測誤判率
// 與為什麼不能自動下架，見 merchantaudit 的 package 註解。
func auditMerchantCodes(ctx context.Context, audit *merchantaudit.Auditor) (string, error) {
	checked, flagged, blocked, err := audit.Sweep(ctx)
	if err != nil {
		return "", err
	}
	// 一輪常常是 0 家到期（都還沒滿一週），那不值得留一筆紀錄。
	if checked == 0 {
		return "", nil
	}
	// 被擋的家數寫進 summary 而不只是 log：Northflank 的 log 有保留期限，
	// 而「這支是不是被對方越擋越多」要看的是跨週的趨勢，那得留在 job_runs 上。
	summary := fmt.Sprintf("檢查 %d 家、列入複檢 %d 家", checked, flagged)
	if blocked > 0 {
		summary += fmt.Sprintf("、被反爬蟲擋下 %d 家", blocked)
	}
	return summary, nil
}

// ensurePartitions 補建事件表的月分區。沒有對應分區時 INSERT 會直接失敗，
// 所以提前開三個月的緩衝，這個排程斷掉幾天也不會出事。
func ensurePartitions(ctx context.Context, st *store.Store) (string, error) {
	now := time.Now()
	for i := 0; i < 3; i++ {
		month := now.AddDate(0, i, 0)
		if err := st.CreateEventPartition(ctx, month); err != nil {
			return "", err
		}
	}
	return "", nil
}

func pruneJobRuns(ctx context.Context, st *store.Store) (string, error) {
	n, err := st.DeleteOldJobRuns(ctx, runRetentionDays)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	return fmt.Sprintf("清掉 %d 筆舊紀錄", n), nil
}
