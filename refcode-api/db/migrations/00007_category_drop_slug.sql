-- +goose Up
-- +goose StatementBegin

-- 分類一律用 id 認人。slug 原本同時是網址（/category/{slug}）與 ?category= 的篩選值，
-- 現在兩邊都改用 merchant_category_id，這個欄位沒有任何讀者了。
--
-- 服務商的 slug 不動：/referral/{slug} 還是靠它，那是官網吃自然搜尋的主力頁。
ALTER TABLE referral_code_bonus.merchant_categories DROP COLUMN slug;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 回不去原本的值 —— 欄位刪掉時那些字串就沒了。這裡只把欄位補回來，
-- 用 id 當佔位值滿足 NOT NULL 與 UNIQUE，之後要用的話得自己重填。
ALTER TABLE referral_code_bonus.merchant_categories ADD COLUMN slug text;
UPDATE referral_code_bonus.merchant_categories SET slug = id::text WHERE slug IS NULL;
ALTER TABLE referral_code_bonus.merchant_categories ALTER COLUMN slug SET NOT NULL;
ALTER TABLE referral_code_bonus.merchant_categories ADD CONSTRAINT merchant_categories_slug_key UNIQUE (slug);
-- +goose StatementEnd
