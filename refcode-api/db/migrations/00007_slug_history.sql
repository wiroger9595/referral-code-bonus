-- +goose Up
-- +goose StatementBegin

-- slug 現在可以改了（後台編輯服務商與分類時）。改掉之後舊網址還在外面流傳 ——
-- 分享出去的連結、Google 已經索引的頁面 —— 直接 404 的話搜尋排名要重新累積，
-- 而官網的流量幾乎全部來自長尾搜尋。所以記住每個用過的 slug，
-- 抓不到就回查這裡並 301 轉到現在的網址。
--
-- slug 當主鍵：同一個字串不會同時是兩家的舊網址，先搶先贏。
-- 一筆 slug 同時是 A 的舊網址、又被 B 拿去當現用網址時，查詢一律讓「現用」勝出
-- （見 db/queries/merchants.sql 的 GetMerchantBySlug），而且寫入端會把它從歷史移除。
CREATE TABLE referral_code_bonus.merchant_slug_history (
    slug        text PRIMARY KEY,
    merchant_id uuid NOT NULL REFERENCES referral_code_bonus.merchants(id) ON DELETE CASCADE,
    replaced_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX merchant_slug_history_merchant_idx
    ON referral_code_bonus.merchant_slug_history (merchant_id);

CREATE TABLE referral_code_bonus.category_slug_history (
    slug        text PRIMARY KEY,
    category_id uuid NOT NULL REFERENCES referral_code_bonus.merchant_categories(id) ON DELETE CASCADE,
    replaced_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX category_slug_history_category_idx
    ON referral_code_bonus.category_slug_history (category_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_code_bonus.category_slug_history;
DROP TABLE IF EXISTS referral_code_bonus.merchant_slug_history;
-- +goose StatementEnd
