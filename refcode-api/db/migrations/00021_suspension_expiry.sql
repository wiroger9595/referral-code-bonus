-- +goose Up
-- +goose StatementBegin

-- 有期限的停權。00020 把停權接起來時只有「停到有人手動解除為止」一種，但實際
-- 處分多數是「停七天」這種 —— 沒有這個欄位的話，期滿要靠 admin 自己記得回來按
-- 解除，漏掉就變成永久停權，而被停的人看到的畫面跟永久停權完全一樣。
--
-- NULL 代表無限期，跟 subscriptions.expires_at、referral_codes.expires_at 的
-- 語意一致（那兩個的 NULL 也是「沒有終點」），後台那三個日期欄位才不會各有各的解讀。
--
-- 到期不是靠查詢時比對時間，而是由 reinstate-suspensions 排程把 status 改回 active
-- （見 internal/suspension）。理由是 users.status 必須是唯一的真相：如果判斷的地方
-- 各自算「status='suspended' 但 suspended_until 已過 = 其實可以用」，那 status 欄位
-- 說的話就跟系統的實際行為不一致，而停權期間被下架的那些碼仍然躺在 disabled ——
-- 兩份狀態分岔之後，「這個人現在到底能不能上架」會變成每個呼叫端各自解釋。
ALTER TABLE referral_code_bonus.users
    ADD COLUMN suspended_until timestamptz;

-- 排程每輪要問「誰的停權到期了」。partial index 只收停權中的那幾列 —— 全站絕大多數
-- 使用者是 active，把他們也放進索引只是讓每次寫入多付一份維護成本。
CREATE INDEX users_suspension_due_idx
    ON referral_code_bonus.users (suspended_until)
    WHERE status = 'suspended';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 欄位刪掉之後，有期限的停權會全部變成無限期（status 仍是 suspended）。
-- 這是回滾本身的取捨：期限只存在這一欄，沒有別的地方留著副本可以還原。
DROP INDEX IF EXISTS referral_code_bonus.users_suspension_due_idx;

ALTER TABLE referral_code_bonus.users
    DROP COLUMN IF EXISTS suspended_until;

-- +goose StatementEnd
