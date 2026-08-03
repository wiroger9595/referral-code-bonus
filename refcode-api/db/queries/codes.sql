-- name: CreateCode :one
INSERT INTO referral_code_bonus.referral_codes (user_id, merchant_id, code, note, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCodeByID :one
SELECT * FROM referral_code_bonus.referral_codes WHERE id = $1;

-- name: ListMyCodes :many
SELECT c.*, m.slug AS merchant_slug, m.name AS merchant_name, m.logo_url AS merchant_logo_url
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.merchants m ON m.id = c.merchant_id
WHERE c.user_id = $1
ORDER BY c.created_at DESC;

-- 服務商頁的候選池。權重計算放在 Go 端（internal/ranking），SQL 只負責撈素材：
-- 一次上限 200 筆，超過這個數量的長尾對抽樣結果影響已經很小。
-- name: ListActiveCodesForMerchant :many
SELECT
    c.id, c.user_id, c.code, c.note, c.quality_score, c.impressions,
    c.expires_at, c.created_at, c.activated_at,
    u.display_name AS owner_name,
    u.avatar_url   AS owner_avatar_url,
    (SELECT count(*) FROM referral_code_bonus.code_reports r
      WHERE r.code_id = c.id AND r.result = 'worked') AS worked_count,
    (SELECT count(*) FROM referral_code_bonus.code_reports r
      WHERE r.code_id = c.id AND r.result <> 'worked') AS failed_count
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.users u ON u.id = c.user_id
WHERE c.merchant_id = $1
  AND c.status = 'active'
  AND c.expires_at > now()
ORDER BY c.quality_score DESC, c.created_at DESC
LIMIT 200;

-- name: ListPendingCodes :many
SELECT c.*, m.slug AS merchant_slug, m.name AS merchant_name,
       m.code_format_regex, u.email AS owner_email, u.display_name AS owner_name
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.merchants m ON m.id = c.merchant_id
JOIN referral_code_bonus.users u     ON u.id = c.user_id
WHERE c.status = 'pending'
ORDER BY c.created_at
LIMIT $1 OFFSET $2;

-- name: CountPendingCodes :one
SELECT count(*) FROM referral_code_bonus.referral_codes WHERE status = 'pending';

-- name: SetCodeStatus :one
UPDATE referral_code_bonus.referral_codes
SET status = $2,
    activated_at = CASE WHEN $2 = 'active' AND activated_at IS NULL THEN now() ELSE activated_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCodeQualityScore :exec
UPDATE referral_code_bonus.referral_codes
SET quality_score = $2, updated_at = now()
WHERE id = $1;

-- 到期下架排程用。回傳受影響的碼以便通知上架者。
-- name: ExpireOverdueCodes :many
UPDATE referral_code_bonus.referral_codes
SET status = 'expired', updated_at = now()
WHERE status = 'active' AND expires_at <= now()
RETURNING id, user_id, merchant_id;

-- name: AddCodeImpressions :exec
UPDATE referral_code_bonus.referral_codes
SET impressions = impressions + sqlc.arg(delta)::bigint
WHERE id = ANY(sqlc.arg(code_ids)::uuid[]);

-- name: CreateCodeReview :one
INSERT INTO referral_code_bonus.code_reviews (code_id, admin_id, action, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListCodeReviews :many
SELECT * FROM referral_code_bonus.code_reviews WHERE code_id = $1 ORDER BY created_at DESC;

-- name: CreateCodeReport :one
INSERT INTO referral_code_bonus.code_reports (code_id, reporter_id, device_hash, result)
VALUES ($1, $2, $3, $4)
ON CONFLICT (code_id, device_hash) DO NOTHING
RETURNING *;

-- 自動下架的判定素材：只看最近 10 筆，讓碼有機會從舊的負評裡翻身
-- （服務商重啟活動時，舊的 failed 不該永遠壓著）。
-- name: GetRecentReportStats :one
SELECT
    count(*)                                        AS total,
    count(*) FILTER (WHERE result = 'worked')       AS worked,
    count(*) FILTER (WHERE result <> 'worked')      AS failed
FROM (
    SELECT result FROM referral_code_bonus.code_reports
    WHERE code_id = $1
    ORDER BY created_at DESC
    LIMIT 10
) recent;
