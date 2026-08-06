-- +goose Up
-- +goose StatementBegin

-- 分類名與獎勵說明要能跟著介面語言換。服務商名（merchants.name）刻意不做 ——
-- 那是品牌名，Netflix 不會因為換語言就變成別的字。
--
-- 兩個欄位都可以是 NULL，代表「這個語言還沒填」，API 會退回中文那份。
-- 不給 NOT NULL DEFAULT '' 是因為空字串跟「沒填」在顯示上要做不同的事：
-- 空字串會讓英文站出現空白的分類磁磚，NULL 則是老老實實顯示中文。
ALTER TABLE referral_code_bonus.merchant_categories
    ADD COLUMN name_en text,
    ADD COLUMN name_ja text;

ALTER TABLE referral_code_bonus.merchants
    ADD COLUMN reward_desc_en text,
    ADD COLUMN reward_desc_ja text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE referral_code_bonus.merchant_categories
    DROP COLUMN IF EXISTS name_en,
    DROP COLUMN IF EXISTS name_ja;

ALTER TABLE referral_code_bonus.merchants
    DROP COLUMN IF EXISTS reward_desc_en,
    DROP COLUMN IF EXISTS reward_desc_ja;
-- +goose StatementEnd
