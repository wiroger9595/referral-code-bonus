-- +goose Up
-- +goose StatementBegin

-- 封鎖上架者。Apple 1.2 與 Play 的 UGC 政策都要求「使用者能封鎖濫用者」，
-- 光有檢舉不算數 —— 檢舉是給平台看的，封鎖要讓當事人自己馬上看不到對方。
--
-- 這個 app 沒有使用者之間的直接互動，能被濫用的介面只有「別人上架的碼與備註」，
-- 所以封鎖的語意就是：被我封鎖的人，他的碼不再出現在我的目錄與服務商頁。
-- 單向，不通知對方，也不影響對方帳號 —— 那是檢舉之後由後台決定的事。
CREATE TABLE referral_code_bonus.user_blocks (
    blocker_id uuid NOT NULL REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    blocked_id uuid NOT NULL REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (blocker_id, blocked_id),
    -- 自己封鎖自己沒有意義，而且會讓「我的碼」憑空消失。
    CONSTRAINT user_blocks_not_self CHECK (blocker_id <> blocked_id)
);

-- 服務商頁每次撈碼都要問一次「這些人裡誰被我封鎖了」，走 blocker_id 這一側。
-- 主鍵已經是 (blocker_id, blocked_id)，前綴就夠用，不必再開索引。

-- 檢舉不當內容。原本的四種 result 都是在講「這組碼還能不能用」，那是功能性回報，
-- 不是 UGC 政策要的「檢舉令人反感的內容」—— 審核時這兩件事會被分開看。
--
-- objectionable 進同一張表是為了共用既有的一裝置一次去重（reports_unique_idx）
-- 與統計管線，但它不該影響品質分數：內容不當跟碼能不能用是兩回事，
-- 混進去會讓被亂檢舉的碼被自動下架（見 internal/ranking 的 ShouldAutoDisable）。
ALTER TABLE referral_code_bonus.code_reports
    DROP CONSTRAINT reports_result_check;

ALTER TABLE referral_code_bonus.code_reports
    ADD CONSTRAINT reports_result_check
    CHECK (result IN ('worked', 'failed', 'invalid_code', 'merchant_closed', 'objectionable'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 先把新值清掉，否則加回舊的 CHECK 會因為現存資料違反而失敗。
DELETE FROM referral_code_bonus.code_reports WHERE result = 'objectionable';

ALTER TABLE referral_code_bonus.code_reports
    DROP CONSTRAINT reports_result_check;

ALTER TABLE referral_code_bonus.code_reports
    ADD CONSTRAINT reports_result_check
    CHECK (result IN ('worked', 'failed', 'invalid_code', 'merchant_closed'));

DROP TABLE IF EXISTS referral_code_bonus.user_blocks;

-- +goose StatementEnd
