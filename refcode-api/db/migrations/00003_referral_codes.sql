-- +goose Up
-- +goose StatementBegin

CREATE TABLE referral_code_bonus.referral_codes (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    merchant_id   uuid NOT NULL REFERENCES referral_code_bonus.merchants(id),
    -- 推薦碼大小寫敏感，照使用者輸入原樣存，不做 normalize。
    code          text NOT NULL,
    note          text NOT NULL DEFAULT '',
    status        text NOT NULL DEFAULT 'pending',
    -- 必填。碼會自然老化，沒有到期日就會累積一堆沒人維護的死碼。
    expires_at    timestamptz NOT NULL,
    -- 0~100，由 code_reports 統計而來。新碼給 60：有曝光機會但壓不過已驗證的碼。
    quality_score int NOT NULL DEFAULT 60,
    -- 排序權重用的曝光累計，由 worker 從 code_events 回填，不即時寫。
    impressions   bigint NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    activated_at  timestamptz,
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT codes_status_check CHECK (status IN ('pending', 'active', 'rejected', 'expired', 'disabled')),
    CONSTRAINT codes_quality_check CHECK (quality_score BETWEEN 0 AND 100)
);

-- 一人一家服務商只能有一個生效中的碼，擋重複上架洗榜。
CREATE UNIQUE INDEX codes_user_merchant_live_idx
    ON referral_code_bonus.referral_codes (user_id, merchant_id)
    WHERE status IN ('pending', 'active');

-- 服務商頁面撈候選池用；只有 active 的碼會進排序。
CREATE INDEX codes_merchant_active_idx
    ON referral_code_bonus.referral_codes (merchant_id, quality_score DESC)
    WHERE status = 'active';

-- 到期下架排程掃這個。
CREATE INDEX codes_expiring_idx
    ON referral_code_bonus.referral_codes (expires_at)
    WHERE status = 'active';

CREATE INDEX codes_user_idx ON referral_code_bonus.referral_codes (user_id, created_at DESC);

-- 審核佇列：pending 的碼按上架時間排。
CREATE INDEX codes_pending_idx
    ON referral_code_bonus.referral_codes (created_at)
    WHERE status = 'pending';

-- 人工審核軌跡。只增不改，申訴時要能還原當初為什麼被拒。
CREATE TABLE referral_code_bonus.code_reviews (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code_id    uuid NOT NULL REFERENCES referral_code_bonus.referral_codes(id) ON DELETE CASCADE,
    admin_id   uuid REFERENCES referral_code_bonus.admins(id),
    action     text NOT NULL,
    reason     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT reviews_action_check CHECK (action IN ('approve', 'reject', 'disable', 'restore', 'auto_disable', 'auto_expire'))
);

CREATE INDEX code_reviews_code_idx ON referral_code_bonus.code_reviews (code_id, created_at DESC);

-- 使用者回報。人工審核擋不住「上架時有效、兩週後活動停辦」，靠這個補。
CREATE TABLE referral_code_bonus.code_reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code_id     uuid NOT NULL REFERENCES referral_code_bonus.referral_codes(id) ON DELETE CASCADE,
    reporter_id uuid REFERENCES referral_code_bonus.users(id) ON DELETE SET NULL,
    -- 匿名使用者也能回報，用裝置雜湊去重。
    device_hash text NOT NULL,
    result      text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT reports_result_check CHECK (result IN ('worked', 'failed', 'invalid_code', 'merchant_closed'))
);

-- 同一裝置對同一個碼只能回報一次，擋刷負評打對手。
CREATE UNIQUE INDEX code_reports_device_idx ON referral_code_bonus.code_reports (code_id, device_hash);
CREATE INDEX code_reports_code_recent_idx   ON referral_code_bonus.code_reports (code_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_code_bonus.code_reports;
DROP TABLE IF EXISTS referral_code_bonus.code_reviews;
DROP TABLE IF EXISTS referral_code_bonus.referral_codes;
-- +goose StatementEnd
