-- +goose Up
-- +goose StatementBegin

-- 曝光/點擊/複製三種事件量差一個數量級以上，之後清理與統計都按時間切，
-- 所以一開始就做成 range partition，免得資料長大之後再改表。
-- is_billable 現在恆為 false（Phase 1 不計費），Phase 3 CPC 上線後才會有 true。
-- 無效點擊也照樣寫一筆但標記 false —— 廣告主質疑帳單時要拿得出完整紀錄。
CREATE TABLE referral_code_bonus.code_events (
    id          uuid NOT NULL DEFAULT gen_random_uuid(),
    code_id     uuid NOT NULL,
    merchant_id uuid NOT NULL,
    event_type  text NOT NULL,
    user_id     uuid,
    device_hash text NOT NULL DEFAULT '',
    ip_hash     text NOT NULL DEFAULT '',
    is_billable boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT events_type_check CHECK (event_type IN ('impression', 'click', 'copy')),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX code_events_code_idx     ON referral_code_bonus.code_events (code_id, event_type, created_at DESC);
CREATE INDEX code_events_merchant_idx ON referral_code_bonus.code_events (merchant_id, created_at DESC);

-- +goose StatementEnd

-- +goose StatementBegin
-- 補建分區的入口，排程每月呼叫一次（見 internal/store/partition.go）。
-- 沒有對應分區時 INSERT 會直接失敗，所以這件事不能漏。
CREATE OR REPLACE FUNCTION referral_code_bonus.ensure_event_partition(m date) RETURNS void AS $$
DECLARE
    start_month date := date_trunc('month', m)::date;
BEGIN
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS referral_code_bonus.code_events_%s PARTITION OF referral_code_bonus.code_events
         FOR VALUES FROM (%L) TO (%L)',
        to_char(start_month, 'YYYYMM'),
        start_month,
        start_month + interval '1 month'
    );
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
-- 先開好未來 18 個月，排程掛掉也還有很長的緩衝。
DO $$
DECLARE
    m date := date_trunc('month', now())::date;
BEGIN
    FOR i IN 0..17 LOOP
        PERFORM referral_code_bonus.ensure_event_partition((m + (i || ' month')::interval)::date);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_code_bonus.code_events;
DROP FUNCTION IF EXISTS referral_code_bonus.ensure_event_partition(date);
-- +goose StatementEnd
