-- +goose Up
-- +goose StatementBegin

-- 後台停權使用者時，要把他上架中的碼一起下架，而每一次下架都得在 code_reviews
-- 留下依據（見 00003 建表時的理由）。現有的 action 值裡沒有一個講得出「這個碼
-- 不是自己有問題，是上架者被停權了」：
--
--   disable      = admin 針對這一個碼做的決定
--   auto_disable = 檢舉統計判定這個碼不能用了
--   auto_expire  = 過了上架者自己設的期限
--
-- 混用 disable 的話，解除停權時分不出「哪些碼是因為停權被下架的、該還給他」
-- 與「哪些是本來就被個別下架的、不該一起復活」。ReinstateUser 那條線靠的就是
-- 這個值，所以它必須是獨立的一種。
--
-- users.status 不用改，00002 建表時的 CHECK 就含 'suspended' 了 —— 那個值一直
-- 存在但沒有任何地方會設它，這次是把它接起來。
ALTER TABLE referral_code_bonus.code_reviews
    DROP CONSTRAINT reviews_action_check,
    ADD CONSTRAINT reviews_action_check CHECK (
        action IN ('approve', 'reject', 'disable', 'restore',
                   'auto_disable', 'auto_expire', 'downgrade', 'suspend')
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滾前要先把 suspend 的軌跡改掉，不然舊的 CHECK 加不回去。改成 disable 而不是
-- 刪掉：那些碼確實被下架過，軌跡消失會讓「這個碼為什麼是 disabled」查不到。
-- 代價是回滾後解除停權沒辦法只還原停權下架的那批，那是回滾本身的取捨。
UPDATE referral_code_bonus.code_reviews SET action = 'disable' WHERE action = 'suspend';

ALTER TABLE referral_code_bonus.code_reviews
    DROP CONSTRAINT reviews_action_check,
    ADD CONSTRAINT reviews_action_check CHECK (
        action IN ('approve', 'reject', 'disable', 'restore',
                   'auto_disable', 'auto_expire', 'downgrade')
    );

-- +goose StatementEnd
