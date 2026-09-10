-- name: UpsertJob :exec
-- worker 啟動時把程式裡註冊的每支 job 補進表裡。已經有的列不動 enabled 與
-- last_run_at —— 那是後台跟執行歷程的狀態，部署一次就被重設的話，「先關掉這支」
-- 會在下一次部署自己打開。
--
-- interval_seconds 只在沒被人改過時跟著程式走：改預設值要能發下去，但後台調過
-- 的那些不該被蓋回去，兩件事靠 interval_overridden 分開。
INSERT INTO referral_code_bonus.scheduled_jobs (name, interval_seconds)
VALUES (@name, @interval_seconds)
ON CONFLICT (name) DO UPDATE
SET interval_seconds = CASE
        WHEN referral_code_bonus.scheduled_jobs.interval_overridden
            THEN referral_code_bonus.scheduled_jobs.interval_seconds
            ELSE EXCLUDED.interval_seconds
    END,
    updated_at = now();

-- name: ClaimDueJob :one
-- 認領一支到期的 job。回一列代表「這個 instance 拿到了，可以跑」，回 no rows
-- 代表還沒到期、被關掉、或被別的 instance 搶走了。
--
-- 認領與更新 last_run_at 是同一個 statement，所以兩個 instance 同時醒來只有一個
-- 拿得到 —— 沒有這層的話，Northflank 上擴到兩個 replica 就會同時去爬同一批網址。
-- SKIP LOCKED 讓搶輸的那個立刻拿到 no rows 而不是卡著等。
--
-- 要用 CTE 是因為 RETURNING 看到的是更新後的列：run_requested_at 在 SET 裡被清成
-- NULL，直接 RETURNING 判斷永遠會是 false，分不出這輪是排程還是後台按的。
-- CTE 的輸出欄位不能也叫 name：UPDATE ... FROM due 之後 due 與被更新的
-- scheduled_jobs 在同一個 scope 裡，sqlc 解析 @name 時會判成 ambiguous。
-- ::boolean 的 cast 也不能省，不然 manual 會被推成 interface{}。
WITH due AS (
    SELECT s.name AS due_name, (s.run_requested_at IS NOT NULL)::boolean AS manual
    FROM referral_code_bonus.scheduled_jobs s
    WHERE s.name = @name
      AND s.enabled
      AND (
          -- 後台按了「立即執行」就不管間隔
          s.run_requested_at IS NOT NULL
          -- 從沒跑過（剛加進來的 job）
          OR s.last_run_at IS NULL
          OR s.last_run_at + make_interval(secs => s.interval_seconds) <= now()
      )
    FOR UPDATE SKIP LOCKED
)
UPDATE referral_code_bonus.scheduled_jobs j
SET last_run_at = now(),
    run_requested_at = NULL,
    updated_at = now()
FROM due
WHERE j.name = due.due_name
RETURNING due.manual;

-- name: ListJobs :many
-- 後台的排程列表，帶最近一次執行的結果。
SELECT
    j.name, j.enabled, j.interval_seconds, j.last_run_at, j.run_requested_at,
    j.interval_overridden, j.updated_at,
    coalesce(lr.status, '') AS last_status,
    coalesce(lr.summary, '') AS last_summary,
    coalesce(lr.error, '') AS last_error,
    lr.finished_at AS last_finished_at
FROM referral_code_bonus.scheduled_jobs j
-- 別名不用 last：那是 SQL 關鍵字，sqlc 的 parser 會把 last.status 解析壞掉。
LEFT JOIN LATERAL (
    SELECT r.status, r.summary, r.error, r.finished_at
    FROM referral_code_bonus.job_runs r
    WHERE r.job_name = j.name
    ORDER BY r.started_at DESC
    LIMIT 1
) lr ON true
ORDER BY j.name;

-- name: SetJobEnabled :one
UPDATE referral_code_bonus.scheduled_jobs
SET enabled = @enabled, updated_at = now()
WHERE name = @name
RETURNING *;

-- name: SetJobInterval :one
-- 標記 interval_overridden，之後部署不會把這個值蓋回程式裡的預設。
UPDATE referral_code_bonus.scheduled_jobs
SET interval_seconds = @interval_seconds,
    interval_overridden = true,
    updated_at = now()
WHERE name = @name
RETURNING *;

-- name: RequestJobRun :one
-- 後台按「立即執行」。只是留一個記號，真正跑起來是下一次輪詢時 ClaimDueJob 認領到
-- —— HTTP handler 不該等一支爬蟲跑完。
UPDATE referral_code_bonus.scheduled_jobs
SET run_requested_at = now(), updated_at = now()
WHERE name = @name AND enabled
RETURNING *;

-- name: StartJobRun :one
INSERT INTO referral_code_bonus.job_runs (job_name, trigger)
VALUES (@job_name, @trigger)
RETURNING *;

-- name: FinishJobRun :exec
UPDATE referral_code_bonus.job_runs
SET status = @status, summary = @summary, error = @error, finished_at = now()
WHERE id = @id;

-- name: ListJobRuns :many
SELECT * FROM referral_code_bonus.job_runs
WHERE job_name = @job_name
ORDER BY started_at DESC
LIMIT @row_limit;

-- name: FailStaleJobRuns :execrows
-- 啟動時把上一個 process 留下的 running 收乾淨。API 被硬殺（部署、OOM）時，
-- FinishJobRun 沒機會跑，那列會永遠停在 running，後台看起來像「還在跑」。
--
-- 只收「這次啟動之前就開始」的，不然多 instance 情況下會把別人正在跑的那列
-- 標成失敗。
UPDATE referral_code_bonus.job_runs
SET status = 'failed',
    error = '服務重啟，這輪沒有正常結束',
    finished_at = now()
WHERE status = 'running' AND started_at < @before;

-- name: DeleteOldJobRuns :execrows
-- 執行紀錄是診斷用的，留一段時間就好。按天數清而不是「每支留 N 筆」：
-- 後者要 window function，而 sqlc 看不到 derived table 的 rn 欄位。
DELETE FROM referral_code_bonus.job_runs
WHERE started_at < now() - make_interval(days => @keep_days::int);
