-- +goose Up
-- +goose StatementBegin

-- 後台可以幫分類上傳一張圖（列表用），跟 merchants.logo_url 同款：
-- 存 Cloudinary 回的 secure_url，NULL 代表還沒上傳。
ALTER TABLE referral_code_bonus.merchant_categories
    ADD COLUMN image_url text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE referral_code_bonus.merchant_categories DROP COLUMN IF EXISTS image_url;
-- +goose StatementEnd
