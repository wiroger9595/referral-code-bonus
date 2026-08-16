-- +goose Up
-- +goose StatementBegin

-- 推薦碼可以沒有到期日。NULL = 永久有效，跟 subscriptions.expires_at 同一個約定
-- （見 00005_subscriptions.sql）—— 服務商的常駐推薦計畫沒有活動檔期，
-- 逼使用者填一個假的到期日只會讓碼在還能用的時候被排程下架。
--
-- codes_expiring_idx 不動：NULL 進得了 index，但 `expires_at <= now()` 永遠不會
-- 命中，到期下架排程自然就跳過永久碼了，不值得為了幾筆 NULL 重建 index。
ALTER TABLE referral_code_bonus.referral_codes
    ALTER COLUMN expires_at DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滾要先把 NULL 填掉才加得回 NOT NULL。填一年後而不是隨便一個遠期日期：
-- 這些碼回滾後會重新受排程管理，給一年是讓上架者有機會回來自己調整。
UPDATE referral_code_bonus.referral_codes
SET expires_at = now() + interval '1 year'
WHERE expires_at IS NULL;

ALTER TABLE referral_code_bonus.referral_codes
    ALTER COLUMN expires_at SET NOT NULL;

-- +goose StatementEnd
