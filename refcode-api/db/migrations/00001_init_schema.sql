-- +goose Up
-- +goose StatementBegin

-- 同一台 Postgres 上還有別的 side project，一律用獨立 schema 隔開。
-- 應用端連線不設 search_path，所有 query 明確寫 referral_code_bonus.xxx。
CREATE SCHEMA IF NOT EXISTS referral_code_bonus;

CREATE TABLE referral_code_bonus.merchant_categories (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text NOT NULL UNIQUE,
    name        text NOT NULL,
    sort_order  int  NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- 服務商目錄只由 admin 維護，使用者不能自建，避免同一家被建成五筆。
CREATE TABLE referral_code_bonus.merchants (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug              text NOT NULL UNIQUE,
    name              text NOT NULL,
    category_id       uuid NOT NULL REFERENCES referral_code_bonus.merchant_categories(id),
    logo_url          text,
    signup_url        text NOT NULL,
    reward_desc       text NOT NULL DEFAULT '',
    -- 擋掉明顯亂填的碼，例如只收 8 碼英數。留空代表不驗。
    code_format_regex text,
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX merchants_category_idx ON referral_code_bonus.merchants (category_id) WHERE is_active;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_code_bonus.merchants;
DROP TABLE IF EXISTS referral_code_bonus.merchant_categories;
DROP SCHEMA IF EXISTS referral_code_bonus;
-- +goose StatementEnd
