-- +goose Up
-- +goose StatementBegin

-- 排程的設定與執行紀錄。在這之前，週期性工作的間隔寫死在 worker.Run 裡
-- （`go w.loop(ctx, time.Hour, ...)`），要停掉或改頻率只能改程式重新部署。
--
-- 搬進資料庫換到三件事：
--
--   1. 後台可以開關、改間隔、手動觸發，不必為了「這支先別跑」而發一版。
--   2. last_run_at 存在資料庫裡，重啟不會歸零。原本的 loop 是「啟動先跑一次」，
--      對只碰自家資料的工作沒差，但 app-import / logo-backfill 這種要打外站的
--      排程，一天部署三次就等於多打三輪 —— 對方看到的是同一個 IP 反覆掃。
--   3. 認領是一次 UPDATE ... RETURNING（見 ClaimDueJob），多個 API instance
--      同時醒來只有一個搶得到。Northflank 上擴到兩個 replica 時，沒有這層的話
--      兩邊會同時去爬同一批網址。
CREATE TABLE referral_code_bonus.scheduled_jobs (
    -- 程式裡註冊的 job 名稱（expire-codes、app-import…）。不是流水號 uuid：
    -- 這張表的列是由程式碼定義的，不是使用者建的，用名字當鍵讓 worker 啟動時
    -- 的 upsert 直接對得上。
    name             text PRIMARY KEY,

    enabled          boolean NOT NULL DEFAULT true,
    interval_seconds integer NOT NULL,

    -- 上次「開始」跑的時間，不是跑完的時間。認領時就寫下去，所以一支跑很久的
    -- 工作不會因為還沒結束而被下一輪重複認領。
    last_run_at      timestamptz,

    -- 後台按「立即執行」時寫 now()，認領成功後清掉。用時間戳而不是 boolean，
    -- 是為了在後台顯示「已排入，等下一次輪詢」而不只是一個閃一下的旗標。
    run_requested_at timestamptz,

    -- 程式改了預設間隔時要不要覆蓋資料庫裡的值：不要。這欄記的是「這列的間隔
    -- 是不是人改過的」，worker 啟動時的 upsert 只更新沒被人改過的那些，
    -- 後台調過的設定不會被下一次部署蓋回去。
    interval_overridden boolean NOT NULL DEFAULT false,

    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    -- 下限擋在一分鐘：這些工作最快的（expire-codes）也是一小時一輪，
    -- 會填到一分鐘以下的只有手滑，而爬蟲類的填錯就是拿自己的 IP 去撞對方。
    CONSTRAINT scheduled_jobs_interval_check CHECK (interval_seconds >= 60)
);

-- 每次執行留一列。排程失敗現在只會噴一行 slog，Northflank 上的 log 有保留期限，
-- 「這支上禮拜是不是一直在失敗」事後查不到 —— 爬蟲尤其需要，被對方擋掉是
-- 慢慢發生的，不是某一次爆掉。
CREATE TABLE referral_code_bonus.job_runs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name    text NOT NULL REFERENCES referral_code_bonus.scheduled_jobs(name) ON DELETE CASCADE,

    started_at  timestamptz NOT NULL DEFAULT now(),
    -- running 的列還沒有結束時間。API 被硬殺時這列會永遠停在 running，
    -- 那是刻意的：看到一列卡著就是「上次沒有正常結束」，比事後補一個假的
    -- 結束時間誠實。
    finished_at timestamptz,

    -- running = 還在跑，ok = 正常結束，failed = 回了 error
    status      text NOT NULL DEFAULT 'running',

    -- 這輪做了什麼的一句話（「新增 3 家、補國別 12 家」）。給後台列表直接顯示，
    -- 不必為了看結果去翻 log。
    summary     text NOT NULL DEFAULT '',
    error       text NOT NULL DEFAULT '',

    -- schedule = 排程觸發，manual = 後台按的
    trigger     text NOT NULL DEFAULT 'schedule',

    CONSTRAINT job_runs_status_check CHECK (status IN ('running', 'ok', 'failed')),
    CONSTRAINT job_runs_trigger_check CHECK (trigger IN ('schedule', 'manual'))
);

-- 後台列表是「某支 job 的最近幾次」，一律照這個順序取。
CREATE INDEX job_runs_job_idx ON referral_code_bonus.job_runs (job_name, started_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS referral_code_bonus.job_runs;
DROP TABLE IF EXISTS referral_code_bonus.scheduled_jobs;

-- +goose StatementEnd
