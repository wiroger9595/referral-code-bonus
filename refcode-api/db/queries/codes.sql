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
-- 上架者是 Pro 才給排序加成（is_pro），跟免費上架張數上限是同一個賣點延伸。
SELECT
    c.id, c.user_id, c.code, c.note, c.quality_score, c.impressions,
    c.expires_at, c.created_at, c.activated_at,
    u.display_name AS owner_name,
    u.avatar_url   AS owner_avatar_url,
    (SELECT count(*) FROM referral_code_bonus.code_reports r
      WHERE r.code_id = c.id AND r.result = 'worked') AS worked_count,
    (SELECT count(*) FROM referral_code_bonus.code_reports r
      WHERE r.code_id = c.id AND r.result <> 'worked') AS failed_count,
    EXISTS (
        SELECT 1 FROM referral_code_bonus.subscriptions s
        WHERE s.user_id = c.user_id
          AND s.is_active
          AND (s.expires_at IS NULL OR s.expires_at > now())
    ) AS owner_is_pro
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.users u ON u.id = c.user_id
WHERE c.merchant_id = $1
  AND c.status = 'active'
  -- expires_at IS NULL 是永久有效的碼，永遠留在候選池裡。
  AND (c.expires_at IS NULL OR c.expires_at > now())
ORDER BY c.quality_score DESC, c.created_at DESC
LIMIT 200;

-- 待審佇列。Pro 的碼排在前面，兌現 paywall 的「優先審核」賣點——
-- 排序的新鮮度加成是從 activated_at 起算的，審核多等一天就等於黃金期晚一天開始，
-- 所以這裡不優先的話，Pro 買到的黃金期會被佇列的等待時間吃掉一部分。
-- 同一層級內仍照送審時間先後，免費的碼不會被無限期插隊。
-- name: ListPendingCodes :many
SELECT c.*, m.slug AS merchant_slug, m.name AS merchant_name,
       m.code_format_regex, u.email AS owner_email, u.display_name AS owner_name,
       EXISTS (
           SELECT 1 FROM referral_code_bonus.subscriptions s
           WHERE s.user_id = c.user_id
             AND s.is_active
             AND (s.expires_at IS NULL OR s.expires_at > now())
       ) AS owner_is_pro
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.merchants m ON m.id = c.merchant_id
JOIN referral_code_bonus.users u     ON u.id = c.user_id
WHERE c.status = 'pending'
ORDER BY owner_is_pro DESC, c.created_at
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
-- 永久碼（expires_at IS NULL）不用另外排除：NULL <= now() 的結果是 NULL 不是 true，
-- 本來就篩不到。
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
