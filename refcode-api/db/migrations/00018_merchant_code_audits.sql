-- +goose Up
-- +goose StatementBegin

-- 每週去各家平台的頁面上確認「這家還有沒有在發推薦碼／折扣碼」，一次檢查留一列。
--
-- 目錄裡的服務商是 cmd/appimport 匯進來或後台建的，沒有人回頭看它們是不是還在
-- 跑推薦計畫。平台把計畫收掉之後，那一頁在目錄上就變成使用者點進去才發現
-- 「根本沒有這回事」的死頁面。
--
-- 為什麼要留一張軌跡表，而不是在 merchants 上加兩個欄位：這裡的結論會直接把
-- 服務商下架（is_active = false），而 is_active = false 在目錄裡本來就有另一種
-- 意思 —— appimport 匯進來還沒補完的草稿也是停用的。沒有軌跡的話，後台看到一家
-- 停用的服務商分不出是「草稿」還是「稽核判定沒在發碼」，也查不到判定的依據。
--
-- 排程只掃 is_active 的服務商，所以被判定下架之後就不會再被檢查，也不會自動
-- 恢復 —— 要不要救回來是後台的人看過軌跡再決定。
CREATE TABLE referral_code_bonus.merchant_code_audits (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- 服務商真的被刪掉時稽核紀錄沒有獨立價值，跟著走。
    merchant_id uuid NOT NULL REFERENCES referral_code_bonus.merchants(id) ON DELETE CASCADE,

    -- found       = 頁面上找得到推薦／折扣碼的字樣
    -- missing     = 頁面抓得到，但整頁沒有任何相關字樣
    -- unreachable = 連不上、非 200、或抓回來的內容短到不像真的頁面（SPA 空殼、
    --               擋爬蟲的錯誤頁）。這種不算證據，不會累積成下架的理由
    result      text NOT NULL,
    -- 這次實際抓了哪些網址（註冊頁與同網域首頁）。判斷錯的時候要追得回來是不是抓錯頁。
    checked_urls text[] NOT NULL DEFAULT '{}',
    -- found 時命中的字樣。後台複檢看這個最快 —— 命中「折扣碼」跟命中「推薦計畫」
    -- 的意義差很多。
    matched     text[] NOT NULL DEFAULT '{}',
    -- unreachable 的原因（HTTP 403、timeout…）。判成 missing 但其中一頁抓失敗時
    -- 也會留，那是複檢時的重要旁證。
    note        text NOT NULL DEFAULT '',
    -- 這次檢查有沒有真的把服務商下架。連續兩次 missing 才會是 true。
    deactivated boolean NOT NULL DEFAULT false,

    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT merchant_code_audits_result_check
        CHECK (result IN ('found', 'missing', 'unreachable'))
);

-- 排程每輪都要問「這家上次什麼時候檢查的、結果是什麼」，兩個問題同一支查詢
-- （ListMerchantsDueForCodeAudit）用這個索引取最新那一列。
CREATE INDEX merchant_code_audits_merchant_idx
    ON referral_code_bonus.merchant_code_audits (merchant_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS referral_code_bonus.merchant_code_audits;

-- +goose StatementEnd
