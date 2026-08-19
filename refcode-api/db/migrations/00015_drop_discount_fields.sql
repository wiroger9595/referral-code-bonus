-- +goose Up
-- +goose StatementBegin

-- 折扣碼的結構化欄位全部收回去，優惠內容改成寫在 note 裡。
--
-- 00014 加這三欄的想法是「結構化才顯示得一致、才排得了序」，但真的要填的時候
-- 每一種都得再想一次規則：金額有幣別與門檻、百分比有「打幾折」與「折掉幾 %」
-- 兩種講法、試用天數又只有訂閱制用得到。上架者要為了一組碼填三格，而讀的人
-- 真正需要的其實就是一句「這是什麼優惠」—— 那句話 note 一直都能裝。
--
-- 留下來的只有 code_type：推薦碼與折扣碼在排序、審核、上架限制上是兩種東西，
-- 那個區分有實際作用，優惠內容的格式沒有。
--
-- 現在一筆折扣碼都還沒有（套用前查過是 0 筆），下面的 UPDATE 是保險：
-- 萬一有人搶先上架，把已經填好的優惠併進備註而不是直接丟掉。
UPDATE referral_code_bonus.referral_codes
SET note = trim(both ' ' from
        CASE discount_kind
            WHEN 'percent'    THEN discount_value::text || '% OFF　'
            WHEN 'amount'     THEN '折抵 ' || discount_value::text || ' ' || coalesce(discount_currency, '') || '　'
            WHEN 'free_trial' THEN '免費試用 ' || discount_value::text || ' 天　'
            ELSE ''
        END || note)
WHERE discount_kind IS NOT NULL;

-- 三條 CHECK 都引用了這幾欄，DROP COLUMN 會一起帶走；明確列出來是為了讓
-- 這支 migration 讀起來就知道約束也一併消失了，不是被漏掉。
-- codes_code_type_check 不動 —— code_type 留著。
ALTER TABLE referral_code_bonus.referral_codes
    DROP CONSTRAINT IF EXISTS codes_discount_shape_check,
    DROP CONSTRAINT IF EXISTS codes_discount_value_range_check,
    DROP CONSTRAINT IF EXISTS codes_discount_currency_check,
    DROP CONSTRAINT IF EXISTS codes_discount_kind_check,
    DROP COLUMN discount_currency,
    DROP COLUMN discount_value,
    DROP COLUMN discount_kind;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 只把欄位與約束加回來，不還原資料 —— 上面併進 note 的文字沒辦法可靠地
-- 再拆回結構化欄位（備註是自由文字，使用者可能已經改過）。
-- 加回來的欄位全部是 NULL，形狀約束對 referral 與「還沒填」的碼都成立。
ALTER TABLE referral_code_bonus.referral_codes
    ADD COLUMN discount_kind text,
    ADD COLUMN discount_value int,
    ADD COLUMN discount_currency text,

    ADD CONSTRAINT codes_discount_kind_check
        CHECK (discount_kind IS NULL OR discount_kind IN ('percent', 'amount', 'free_trial', 'other')),
    ADD CONSTRAINT codes_discount_currency_check
        CHECK (discount_currency IS NULL OR discount_currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT codes_discount_value_range_check CHECK (
        discount_value IS NULL
        OR (discount_kind = 'percent'    AND discount_value BETWEEN 1 AND 99)
        OR (discount_kind = 'amount'     AND discount_value > 0)
        OR (discount_kind = 'free_trial' AND discount_value > 0)
    );

-- +goose StatementEnd
