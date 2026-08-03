-- +goose Up
-- +goose StatementBegin

-- 訂閱狀態的真相在 RevenueCat，這裡只留一份供伺服器端判斷用的副本
-- （client 端自己讀 SDK，不靠這張表）。一個使用者只會有一個生效中的
-- entitlement，所以 user_id 直接當主鍵，webhook 進來就 upsert。
CREATE TABLE referral_code_bonus.subscriptions (
    user_id        uuid PRIMARY KEY REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    entitlement    text        NOT NULL,          -- 目前只有 'pro'
    product_id     text        NOT NULL,
    store          text        NOT NULL,          -- app_store / play_store / stripe / promotional
    is_active      boolean     NOT NULL,
    will_renew     boolean     NOT NULL DEFAULT false,
    -- 沒有到期日代表是永久授權（promotional / 買斷），判斷生效時要當成無限遠。
    expires_at     timestamptz,
    rc_app_user_id text        NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- 只寫不改的事件紀錄。RevenueCat 送不到會重試，rc_event_id 拿來做冪等；
-- 對帳與退款爭議時這張表是唯一能重建時序的地方。
CREATE TABLE referral_code_bonus.subscription_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rc_event_id text NOT NULL UNIQUE,
    -- app_user_id 對不到本地帳號時（匿名 ID、測試事件）還是要留紀錄，所以可為空。
    user_id     uuid REFERENCES referral_code_bonus.users(id) ON DELETE SET NULL,
    event_type  text        NOT NULL,
    payload     jsonb       NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX subscription_events_user_idx
    ON referral_code_bonus.subscription_events (user_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_code_bonus.subscription_events;
DROP TABLE IF EXISTS referral_code_bonus.subscriptions;
-- +goose StatementEnd
