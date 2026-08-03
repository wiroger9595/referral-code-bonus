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
