-- +goose Up
-- +goose StatementBegin

-- 使用者提報「希望上架的平台」。
--
-- 服務商目錄一直只由 admin 維護（見 00001 的註解），代價是使用者手上有一家
-- 目錄裡沒有的平台的碼時，整條上架路徑就斷在那裡 —— 他沒有任何管道把那家
-- 講出來，我們也不知道少了哪些家。這張表就是那個管道：使用者只留名稱與官網，
-- 通過審核之後由後台建成停用的服務商草稿，補完獎勵說明再上架。
--
-- 刻意不讓通過就直接進目錄：使用者填不出 slug、分類與獎勵說明，那三樣缺一
-- 公開頁面就是空的（同 cmd/appimport 匯進來的那批，一律建成 is_active=false）。
CREATE TABLE referral_code_bonus.merchant_suggestions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- 提報者。帳號刪掉時建議跟著走 —— 它沒有獨立價值，通過的那些已經變成
    -- merchants 了，留著沒人聯絡得上的待審建議只是佔審核佇列。
    user_id     uuid NOT NULL REFERENCES referral_code_bonus.users(id) ON DELETE CASCADE,
    -- 使用者說得出來的就這三樣，分類、獎勵說明、logo 全部由後台事後補。
    name        text NOT NULL,
    signup_url  text NOT NULL,
    note        text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'pending',

    -- 審核結果。跟 code_reviews 不同，建議單不需要獨立的軌跡表：它只會被審一次
    -- （通過或拒絕之後就離開佇列，沒有下架、恢復這種後續動作）。
    reviewed_by   uuid REFERENCES referral_code_bonus.admins(id),
    reviewed_at   timestamptz,
    review_reason text NOT NULL DEFAULT '',
    -- 通過時建出來的那家。後台要能從建議點回服務商去補資料，
    -- 服務商真的被刪掉時建議仍然留著當紀錄，所以是 SET NULL 不是 CASCADE。
    merchant_id   uuid REFERENCES referral_code_bonus.merchants(id) ON DELETE SET NULL,

    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT merchant_suggestions_status_check
        CHECK (status IN ('pending', 'approved', 'rejected')),
    -- 長度擋在資料庫這層：這是使用者直接寫進來的自由文字，而且提交點不只一個
    -- （app 與官網各一份表單），只靠前端擋等於沒擋。
    CONSTRAINT merchant_suggestions_name_check
        CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT merchant_suggestions_url_check
        CHECK (signup_url ~ '^https://' AND char_length(signup_url) <= 500),
    CONSTRAINT merchant_suggestions_note_check
        CHECK (char_length(note) <= 500),
    -- 審過的一定有時間與審核者，還沒審的一定都是空的 —— 半套的狀態會讓
    -- 後台列表顯示成「已通過但不知道誰審的」。
    CONSTRAINT merchant_suggestions_reviewed_shape_check CHECK (
        (status = 'pending') = (reviewed_at IS NULL)
    ),
    -- 通過一定會伴隨一家新建的服務商；拒絕的不會有。
    CONSTRAINT merchant_suggestions_merchant_shape_check CHECK (
        status = 'approved' OR merchant_id IS NULL
    )
);

-- 同一個人不要把同一家送兩次。只在待審那一段擋 —— 被拒絕過的平台過一陣子
-- 真的開了推薦計畫，使用者應該可以再提一次。
CREATE UNIQUE INDEX merchant_suggestions_user_pending_idx
    ON referral_code_bonus.merchant_suggestions (user_id, lower(name))
    WHERE status = 'pending';

-- 後台的待審清單，最舊的排前面（先進先審）。
CREATE INDEX merchant_suggestions_pending_idx
    ON referral_code_bonus.merchant_suggestions (created_at)
    WHERE status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS referral_code_bonus.merchant_suggestions;

-- +goose StatementEnd
