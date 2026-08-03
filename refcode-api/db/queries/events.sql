-- name: InsertEvent :exec
INSERT INTO referral_code_bonus.code_events (code_id, merchant_id, event_type, user_id, device_hash, ip_hash, is_billable)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- 點擊計費要求「先有曝光才算數」，Phase 3 會用這個擋直接打 API 刷點擊。
-- name: HasRecentImpression :one
SELECT EXISTS (
    SELECT 1 FROM referral_code_bonus.code_events
    WHERE code_id = $1
      AND device_hash = $2
      AND event_type = 'impression'
      AND created_at > now() - interval '30 minutes'
);

-- 上架者儀表板。
-- name: GetCodeStats :one
SELECT
    count(*) FILTER (WHERE event_type = 'impression') AS impressions,
    count(*) FILTER (WHERE event_type = 'click')      AS clicks,
    count(*) FILTER (WHERE event_type = 'copy')       AS copies
FROM referral_code_bonus.code_events
WHERE code_id = $1 AND created_at > now() - sqlc.arg(window_days)::int * interval '1 day';

-- name: CreateEventPartition :exec
SELECT referral_code_bonus.ensure_event_partition(sqlc.arg(month)::date);
