-- +goose Up
-- +goose StatementBegin

CREATE TABLE referral_code_bonus.users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             text NOT NULL,
    email_verified_at timestamptz,
    display_name      text NOT NULL DEFAULT '',
    avatar_url        text,
    -- 只有 email 註冊者有值；純社群登入者為 NULL。
    password_hash     text,
    status            text NOT NULL DEFAULT 'active',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_status_check CHECK (status IN ('active', 'suspended', 'deleted'))
);

-- email 比對一律不分大小寫。
CREATE UNIQUE INDEX users_email_lower_idx ON referral_code_bonus.users (lower(email));

CREATE TABLE referral_code_bonus.oauth_identities (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    provider         text NOT NULL,
    provider_user_id text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT oauth_provider_check CHECK (provider IN ('google', 'apple')),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX oauth_identities_user_idx ON referral_code_bonus.oauth_identities (user_id);

-- Refresh token 用 rotating：每次換發舊的標記 rotated_at，
-- 舊 token 再被使用代表外洩，該裝置整條鏈全撤銷（see internal/auth）。
CREATE TABLE referral_code_bonus.refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL UNIQUE,
    -- 同一裝置換發出來的 token 共用一個 family，偵測到重用就整族撤銷。
    family_id   uuid NOT NULL,
    expires_at  timestamptz NOT NULL,
    rotated_at  timestamptz,
    revoked_at  timestamptz,
    user_agent  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_family_idx ON referral_code_bonus.refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_idx   ON referral_code_bonus.refresh_tokens (user_id);

-- admin 後台的登入帳號跟一般使用者完全分開，避免權限混在同一張表。
CREATE TABLE referral_code_bonus.admins (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL,
    password_hash text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    role          text NOT NULL DEFAULT 'reviewer',
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT admins_role_check CHECK (role IN ('owner', 'reviewer'))
);

CREATE UNIQUE INDEX admins_email_lower_idx ON referral_code_bonus.admins (lower(email));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_code_bonus.admins;
DROP TABLE IF EXISTS referral_code_bonus.refresh_tokens;
DROP TABLE IF EXISTS referral_code_bonus.oauth_identities;
DROP TABLE IF EXISTS referral_code_bonus.users;
-- +goose StatementEnd
