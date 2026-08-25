-- name: CreateUser :one
INSERT INTO referral_code_bonus.users (email, display_name, avatar_url, password_hash, email_verified_at, country)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM referral_code_bonus.users
WHERE lower(email) = lower(sqlc.arg(email)::text) AND status <> 'deleted';

-- name: GetUserByID :one
SELECT * FROM referral_code_bonus.users
WHERE id = $1 AND status <> 'deleted';

-- name: MarkEmailVerified :exec
UPDATE referral_code_bonus.users
SET email_verified_at = now(), updated_at = now()
WHERE id = $1 AND email_verified_at IS NULL;

-- name: UpdateUserPassword :exec
UPDATE referral_code_bonus.users
SET password_hash = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateUserProfile :one
UPDATE referral_code_bonus.users
SET display_name = $2, avatar_url = $3, country = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- 只動大頭照。走 UpdateUserProfile 的話得把顯示名稱與所在地一起送，
-- 上傳圖片的當下前端手上那份可能已經是舊的。
-- name: UpdateUserAvatar :one
UPDATE referral_code_bonus.users
SET avatar_url = $2, avatar_public_id = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetOAuthIdentity :one
SELECT * FROM referral_code_bonus.oauth_identities
WHERE provider = $1 AND provider_user_id = $2;

-- name: CreateOAuthIdentity :one
INSERT INTO referral_code_bonus.oauth_identities (user_id, provider, provider_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateRefreshToken :one
INSERT INTO referral_code_bonus.refresh_tokens (user_id, token_hash, family_id, expires_at, user_agent)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM referral_code_bonus.refresh_tokens WHERE token_hash = $1;

-- name: MarkRefreshTokenRotated :exec
UPDATE referral_code_bonus.refresh_tokens SET rotated_at = now() WHERE id = $1;

-- 偵測到舊 token 被重用時整族撤銷：那代表 token 外洩，該裝置全部作廢。
-- name: RevokeTokenFamily :exec
UPDATE referral_code_bonus.refresh_tokens
SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- 重設密碼後把這個人的所有 session 作廢。會忘記密碼常常是因為帳號被別人拿走了，
-- 只換密碼而留著既有的 refresh token 等於沒把對方踢出去。
-- name: RevokeAllUserTokens :exec
UPDATE referral_code_bonus.refresh_tokens
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: GetAdminByEmail :one
SELECT * FROM referral_code_bonus.admins
WHERE lower(email) = lower(sqlc.arg(email)::text) AND is_active;

-- name: GetAdminByID :one
SELECT * FROM referral_code_bonus.admins WHERE id = $1 AND is_active;

-- name: CreateAdmin :one
INSERT INTO referral_code_bonus.admins (email, password_hash, display_name, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- 刪除帳號。code_events 沒有外鍵（分區表），所以要自己把 user_id 抹掉；
-- 抹掉而不是刪除，是因為那些曝光/點擊已經計入推薦碼的排序與品質分數，
-- 整批刪掉會讓別人的碼排序莫名跳動。抹完就跟這個人無關了。
-- name: AnonymizeUserEvents :exec
UPDATE referral_code_bonus.code_events
SET user_id = NULL
WHERE user_id = $1;

-- 其餘子表靠外鍵處理：oauth_identities / refresh_tokens / referral_codes /
-- subscriptions 是 ON DELETE CASCADE，code_reports.reporter_id 與
-- subscription_events.user_id 是 ON DELETE SET NULL（回報內容留著但匿名）。
-- name: DeleteUser :exec
DELETE FROM referral_code_bonus.users WHERE id = $1;

-- 後台的使用者查詢，客服/退款爭議時用來看誰是 Pro、手動補發或撤銷。
-- is_pro 的判斷跟 s.isPro() 同一套邏輯（is_active 且未過期），這裡用 SQL 重算一次
-- 是因為要一次列一頁，不能逐筆呼叫 Go 那個函式。
-- name: ListUsersAdmin :many
SELECT
    u.id, u.email, u.display_name, u.status, u.created_at,
    COALESCE(s.is_active, false) AND (s.expires_at IS NULL OR s.expires_at > now()) AS is_pro,
    s.expires_at  AS pro_expires_at,
    s.store       AS pro_store,
    s.product_id  AS pro_product_id
FROM referral_code_bonus.users u
LEFT JOIN referral_code_bonus.subscriptions s ON s.user_id = u.id
WHERE u.status <> 'deleted'
  AND (sqlc.arg(q)::text = '' OR u.email ILIKE '%' || sqlc.arg(q)::text || '%')
ORDER BY u.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsersAdmin :one
SELECT count(*) FROM referral_code_bonus.users u
WHERE u.status <> 'deleted'
  AND (sqlc.arg(q)::text = '' OR u.email ILIKE '%' || sqlc.arg(q)::text || '%');

-- 客服手動撤銷 Pro（退款爭議、誤發）。只關掉 is_active，事件紀錄不動——
-- 跟 RevenueCat webhook 進來的撤銷用同一份狀態，之後真的 webhook 送到時會再 upsert 一次。
-- name: RevokeSubscription :execrows
UPDATE referral_code_bonus.subscriptions
SET is_active = false, will_renew = false, updated_at = now()
WHERE user_id = $1 AND is_active;

-- 封鎖上架者。UGC 政策要的「使用者能自己封鎖濫用者」（見 00012_user_blocks.sql）。
-- 重複封鎖同一個人不是錯誤，當成已經封鎖了。
-- name: BlockUser :exec
INSERT INTO referral_code_bonus.user_blocks (blocker_id, blocked_id)
VALUES ($1, $2)
ON CONFLICT (blocker_id, blocked_id) DO NOTHING;

-- name: UnblockUser :exec
DELETE FROM referral_code_bonus.user_blocks
WHERE blocker_id = $1 AND blocked_id = $2;

-- 帳號頁的「已封鎖的上架者」。封鎖是單向且不通知對方的，所以這份清單
-- 只有封鎖者自己看得到 —— 沒有這一頁，被誤封的人就永遠救不回來。
-- name: ListMyBlocks :many
SELECT b.blocked_id, b.created_at, u.display_name, u.avatar_url
FROM referral_code_bonus.user_blocks b
JOIN referral_code_bonus.users u ON u.id = b.blocked_id
WHERE b.blocker_id = $1
ORDER BY b.created_at DESC;
