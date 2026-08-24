-- 使用者提報希望上架的平台，後台審核後建成停用的服務商草稿。
-- 資料模型與各欄位的取捨見 db/migrations/00016_merchant_suggestions.sql。

-- name: CreateMerchantSuggestion :one
INSERT INTO referral_code_bonus.merchant_suggestions (user_id, name, signup_url, note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- 一個人同時掛著幾筆待審。上限由 handler 決定 —— 這裡只負責數。
-- name: CountPendingSuggestionsByUser :one
SELECT count(*) FROM referral_code_bonus.merchant_suggestions
WHERE user_id = $1 AND status = 'pending';

-- 提報之前先確認目錄裡沒有這家（同名、不分大小寫）。
-- 只看上架中的：停用的那些是還沒補完資料的草稿，使用者在 app 裡看不到，
-- 他提報是合理的，那筆建議也剛好提醒後台「有人在等這家」。
-- name: CountActiveMerchantsByName :one
SELECT count(*) FROM referral_code_bonus.merchants
WHERE is_active AND lower(name) = lower(sqlc.arg(name)::text);

-- 後台待審清單。最舊的排前面，先進先審。
-- name: ListPendingMerchantSuggestions :many
SELECT
    s.*,
    u.email AS owner_email,
    u.display_name AS owner_name,
    count(*) OVER () AS total_count
FROM referral_code_bonus.merchant_suggestions s
JOIN referral_code_bonus.users u ON u.id = s.user_id
WHERE s.status = 'pending'
ORDER BY s.created_at
LIMIT $1 OFFSET $2;

-- name: GetMerchantSuggestionByID :one
SELECT * FROM referral_code_bonus.merchant_suggestions WHERE id = $1;

-- 審核只會發生一次，所以條件帶上 status = 'pending'：兩個 admin 同時按下通過時，
-- 第二個會撈不到列（回 ErrNoRows），由 handler 轉成「這筆已經審過了」，
-- 而不是把服務商建成兩家。
-- name: ReviewMerchantSuggestion :one
UPDATE referral_code_bonus.merchant_suggestions
SET status = sqlc.arg(status)::text,
    reviewed_by = sqlc.arg(reviewed_by),
    reviewed_at = now(),
    review_reason = sqlc.arg(review_reason),
    merchant_id = sqlc.narg(merchant_id)
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING *;
