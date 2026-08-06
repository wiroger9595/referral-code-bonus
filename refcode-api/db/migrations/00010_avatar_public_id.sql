-- +goose Up
-- +goose StatementBegin

-- 換掉或刪帳號時要把 Cloudinary 上那張圖也刪掉（Apple 5.1.1(v) 要求刪帳號時
-- 連使用者產生的內容一起刪，圖檔本身算在內）。刪圖要 public_id，
-- 從 secure_url 反推得出來，但 URL 中間可能有版本號或 transformation，
-- 反推容易錯，直接把上傳當下 Cloudinary 回的值存起來。
--
-- 可以是 NULL：社群登入帶進來的頭像不是我們上傳的，沒有 public_id，也不該由我們刪。
ALTER TABLE referral_code_bonus.users
    ADD COLUMN avatar_public_id text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE referral_code_bonus.users
    DROP COLUMN avatar_public_id;

-- +goose StatementEnd
