-- name: UpsertSubscription :one
INSERT INTO referral_code_bonus.subscriptions (
    user_id, entitlement, product_id, store, is_active, will_renew, expires_at, rc_app_user_id, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (user_id) DO UPDATE SET
    entitlement    = EXCLUDED.entitlement,
    product_id     = EXCLUDED.product_id,
    store          = EXCLUDED.store,
    is_active      = EXCLUDED.is_active,
    will_renew     = EXCLUDED.will_renew,
    expires_at     = EXCLUDED.expires_at,
    rc_app_user_id = EXCLUDED.rc_app_user_id,
    updated_at     = now()
RETURNING *;

-- 到期日是 NULL 代表永久授權（promotional / 買斷），要當成還沒到期。
-- name: GetActiveSubscription :one
SELECT * FROM referral_code_bonus.subscriptions
WHERE user_id = $1
  AND is_active
  AND (expires_at IS NULL OR expires_at > now());

-- name: CountActiveCodesForUser :one
SELECT count(*) FROM referral_code_bonus.referral_codes
WHERE user_id = $1 AND status IN ('pending', 'active');

-- webhook 會重送，靠 rc_event_id 擋掉重複處理。
-- name: InsertSubscriptionEvent :one
INSERT INTO referral_code_bonus.subscription_events (rc_event_id, user_id, event_type, payload)
VALUES ($1, $2, $3, $4)
ON CONFLICT (rc_event_id) DO NOTHING
RETURNING *;
