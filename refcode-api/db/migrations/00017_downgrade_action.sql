-- +goose Up
-- +goose StatementBegin

-- Pro 到期時要把超出免費額度的碼撤下來，留一筆軌跡說明「為什麼它不見了」。
--
-- 不借用 auto_disable：那個是品質分數觸發的自動下架，後台
-- /admin/codes/auto-disabled 專門撈那批來人工複檢，混進來會讓審核佇列
-- 塞滿根本沒有品質問題的碼。
--
-- 恢復沿用既有的 restore，不另外開一個值：續訂恢復與後台恢復做的事一樣
-- （轉回架上），差別在 admin_id 是不是空的，那個欄位已經分得出來。
ALTER TABLE referral_code_bonus.code_reviews
    DROP CONSTRAINT reviews_action_check,
    ADD CONSTRAINT reviews_action_check CHECK (
        action IN ('approve', 'reject', 'disable', 'restore',
                   'auto_disable', 'auto_expire', 'downgrade')
    );

-- 續訂恢復要找「當初是被降級撤掉的」那些碼，條件是「最後一筆軌跡是 downgrade」。
-- 這支查詢跑在 webhook 的同步路徑上，讓它走索引而不是逐個 code 掃全部軌跡。
CREATE INDEX code_reviews_downgrade_idx
    ON referral_code_bonus.code_reviews (code_id, created_at DESC)
    WHERE action = 'downgrade';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX referral_code_bonus.code_reviews_downgrade_idx;

-- 回滾前要先清掉新值，不然 CHECK 建不回去。這些碼本身不動 —— 它們已經是
-- disabled，少一筆軌跡只是查不到當初為什麼被撤，比連碼一起刪安全。
DELETE FROM referral_code_bonus.code_reviews WHERE action = 'downgrade';

ALTER TABLE referral_code_bonus.code_reviews
    DROP CONSTRAINT reviews_action_check,
    ADD CONSTRAINT reviews_action_check CHECK (
        action IN ('approve', 'reject', 'disable', 'restore',
                   'auto_disable', 'auto_expire')
    );

-- +goose StatementEnd
