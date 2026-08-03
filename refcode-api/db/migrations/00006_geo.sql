-- +goose Up
-- +goose StatementBegin

-- 使用者的所在地（ISO 3166-1 alpha-2），註冊時自己選。
-- 不用 IP 判定也不用介面語言推斷：旅居海外的人語言跟所在地常常不一樣，
-- IP 碰到 VPN／行動網路會猜錯，而且兩種都沒辦法讓使用者自己修正。
-- NULL 代表沒填（社群登入建立的帳號、或這次改動之前註冊的人），排序時視同沒有偏好。
ALTER TABLE referral_code_bonus.users
    ADD COLUMN country text CHECK (country ~ '^[A-Z]{2}$');

-- 這家服務商在哪些國家能用。空陣列代表不分地區（串流、雲端這種跨國服務），
-- 排序時當中性：排在在地服務商後面、外地服務商前面。
-- 不另外開國家表也不做外鍵 —— 這是一份幾乎不會變的靜態清單，
-- 格式在 API 進 admin 時驗（internal/geo）。
ALTER TABLE referral_code_bonus.merchants
    ADD COLUMN countries text[] NOT NULL DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE referral_code_bonus.merchants DROP COLUMN IF EXISTS countries;
ALTER TABLE referral_code_bonus.users DROP COLUMN IF EXISTS country;
-- +goose StatementEnd
