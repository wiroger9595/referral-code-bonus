-- name: CreateCode :one
INSERT INTO referral_code_bonus.referral_codes (
    user_id, merchant_id, code, note, expires_at, code_type
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCodeByID :one
SELECT * FROM referral_code_bonus.referral_codes WHERE id = $1;

-- reject_reason 是最近一次被拒的理由，上架者要知道自己該改什麼才有得申訴。
-- 沒有被拒過就是空字串 —— code_reviews.reason 本身就是 NOT NULL DEFAULT ''，
-- 空字串代表「沒有理由」是既有語意。這裡的 coalesce 不能省：LEFT JOIN 沒配到時
-- 那一格是 NULL，而 sqlc 看的是欄位本身的 NOT NULL，會產出非指標的 string，
-- 掃到 NULL 直接噴錯。
-- 沒有限制 c.status：被拒之後又被 restore 的碼照樣帶著上一次的理由，
-- 現在是什麼狀態該不該顯示交給前端判斷，SQL 這裡只負責把事實撈出來。
-- name: ListMyCodes :many
SELECT c.*, m.slug AS merchant_slug, m.name AS merchant_name, m.logo_url AS merchant_logo_url,
       coalesce(rej.reason, '') AS reject_reason
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.merchants m ON m.id = c.merchant_id
LEFT JOIN LATERAL (
    SELECT r.reason
    FROM referral_code_bonus.code_reviews r
    WHERE r.code_id = c.id AND r.action = 'reject'
    ORDER BY r.created_at DESC
    LIMIT 1
) rej ON true
WHERE c.user_id = $1
ORDER BY c.created_at DESC;

-- 服務商頁的候選池。權重計算放在 Go 端（internal/ranking），SQL 只負責撈素材：
-- 一次上限 200 筆，超過這個數量的長尾對抽樣結果影響已經很小。
-- name: ListActiveCodesForMerchant :many
-- 上架者是 Pro 才給排序加成（is_pro），跟免費上架張數上限是同一個賣點延伸。
SELECT
    c.id, c.user_id, c.code, c.note, c.quality_score, c.impressions,
    c.expires_at, c.created_at, c.activated_at,
    c.code_type,
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
       m.code_format_regex, m.discount_code_format_regex,
       u.email AS owner_email, u.display_name AS owner_name,
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

-- 後台的「已上架推薦碼」列表，帶使用者回報統計。
--
-- pending 不在這裡：那批還沒進過候選池、沒有人看得到，回報永遠是 0，
-- 列出來只會讓審核佇列的工作在兩個地方各做一半。
--
-- 四種回報分開數而不是收成「成功／失敗」兩欄，因為處理方式不同：
-- invalid_code 是碼本身有問題（找上架者），merchant_closed 是活動結束
-- （整家服務商都該檢查），failed 才是單純沒拿到獎勵。
--
-- total_count 跟 ListMerchants 一樣用 window function 帶出來，不必為了一個數字
-- 再打一次 count 查詢。
-- name: ListCodesForAdmin :many
SELECT
    c.*,
    m.slug AS merchant_slug,
    m.name AS merchant_name,
    u.email AS owner_email,
    u.display_name AS owner_name,
    coalesce(rs.total, 0) AS report_total,
    coalesce(rs.worked, 0) AS report_worked,
    coalesce(rs.failed, 0) AS report_failed,
    coalesce(rs.invalid_code, 0) AS report_invalid_code,
    coalesce(rs.merchant_closed, 0) AS report_merchant_closed,
    rs.last_reported_at,
    count(*) OVER () AS total_count
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.merchants m ON m.id = c.merchant_id
JOIN referral_code_bonus.users u     ON u.id = c.user_id
LEFT JOIN LATERAL (
    SELECT
        count(*)                                              AS total,
        count(*) FILTER (WHERE r.result = 'worked')           AS worked,
        count(*) FILTER (WHERE r.result = 'failed')           AS failed,
        count(*) FILTER (WHERE r.result = 'invalid_code')     AS invalid_code,
        count(*) FILTER (WHERE r.result = 'merchant_closed')  AS merchant_closed,
        max(r.created_at)                                     AS last_reported_at
    FROM referral_code_bonus.code_reports r
    WHERE r.code_id = c.id
) rs ON true
WHERE c.status <> 'pending'
  AND (sqlc.narg(status)::text IS NULL OR c.status = sqlc.narg(status)::text)
-- 搜尋比對碼本身、服務商名與上架者 email：後台找碼的起點不外乎這三個
-- （使用者來信抱怨某個碼、某家服務商出事、某個帳號在洗榜）。
  AND (
    sqlc.narg(search)::text IS NULL
    OR c.code ILIKE '%' || sqlc.narg(search)::text || '%'
    OR m.name ILIKE '%' || sqlc.narg(search)::text || '%'
    OR u.email ILIKE '%' || sqlc.narg(search)::text || '%'
  )
-- 被回報成用不了的排最前面：這頁存在的理由就是把這些碼撈出來處理，
-- 沒有負評的碼再新也不需要 admin 看。三種負面回報一起算，
-- 同分再讓品質分數低的、以及最近才動過的排前面。
ORDER BY
    coalesce(rs.failed, 0) + coalesce(rs.invalid_code, 0) + coalesce(rs.merchant_closed, 0) DESC,
    c.quality_score,
    c.updated_at DESC
LIMIT $1 OFFSET $2;

-- 自動下架後還沒有人複核過的碼。
--
-- 判定看「最後一筆審核紀錄是不是 auto_disable」，不另外開欄位記已處理：
-- admin 按了維持下架或恢復上架都會再寫一筆 code_reviews，那個碼自然就離開清單。
--
-- 這份清單非看不可的原因是誤判會沉默地發生：ShouldAutoDisable 只看最近 10 筆回報，
-- 湊得出幾筆負評就能打掉競爭對手的碼，沒有人複核的話上架者只會覺得平台不可靠。
-- name: ListAutoDisabledCodes :many
SELECT
    c.*,
    m.slug AS merchant_slug,
    m.name AS merchant_name,
    u.email AS owner_email,
    u.display_name AS owner_name,
    coalesce(rs.total, 0) AS report_total,
    coalesce(rs.worked, 0) AS report_worked,
    coalesce(rs.failed, 0) AS report_failed,
    coalesce(rs.invalid_code, 0) AS report_invalid_code,
    coalesce(rs.merchant_closed, 0) AS report_merchant_closed,
    rs.last_reported_at,
    last_review.created_at AS disabled_at,
    count(*) OVER () AS total_count
FROM referral_code_bonus.referral_codes c
JOIN referral_code_bonus.merchants m ON m.id = c.merchant_id
JOIN referral_code_bonus.users u     ON u.id = c.user_id
LEFT JOIN LATERAL (
    SELECT
        count(*)                                              AS total,
        count(*) FILTER (WHERE r.result = 'worked')           AS worked,
        count(*) FILTER (WHERE r.result = 'failed')           AS failed,
        count(*) FILTER (WHERE r.result = 'invalid_code')     AS invalid_code,
        count(*) FILTER (WHERE r.result = 'merchant_closed')  AS merchant_closed,
        max(r.created_at)                                     AS last_reported_at
    FROM referral_code_bonus.code_reports r
    WHERE r.code_id = c.id
) rs ON true
JOIN LATERAL (
    SELECT rv.action, rv.created_at
    FROM referral_code_bonus.code_reviews rv
    WHERE rv.code_id = c.id
    ORDER BY rv.created_at DESC
    LIMIT 1
) last_review ON true
WHERE c.status = 'disabled'
  AND last_review.action = 'auto_disable'
ORDER BY last_review.created_at DESC
LIMIT $1 OFFSET $2;

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
