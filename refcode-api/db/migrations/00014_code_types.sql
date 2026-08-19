-- +goose Up
-- +goose StatementBegin

-- 一個碼有兩種來源：使用者自己的推薦碼（推薦人與被推薦人各拿獎勵），
-- 以及使用者手上的折扣碼（活動碼、會員專屬碼，只有使用的人拿到折扣）。
-- 之前整張表都假設是前者，但很多服務商根本沒有推薦計畫，只發折扣碼——
-- 那些服務商在目錄裡是空的，使用者也沒地方分享手上的折扣碼。
--
-- 現有的碼全部視為 referral：這張表在這次改動之前只收得下推薦碼。
ALTER TABLE referral_code_bonus.referral_codes
    ADD COLUMN code_type text NOT NULL DEFAULT 'referral',
    -- 以下三欄只有 discount 用得到，形狀由 codes_discount_shape_check 管。
    --
    -- percent     → discount_value 是折扣的百分比（20 = 打八折，off 20%）。
    --               存 off 的數字而不是台灣習慣的「9 折」：介面有中英日三語，
    --               只有 off 這個表達法三邊都轉得出去（zh-TW 顯示時再換算成折數）。
    -- amount      → discount_value 是折抵金額，幣別看 discount_currency。
    -- free_trial  → discount_value 是免費試用的天數。
    -- other       → 不規則的優惠（買一送一、送贈品），value 留空、靠 note 描述。
    ADD COLUMN discount_kind text,
    -- 整數不是疏忽：numeric 到了 Go 是 pgtype.Numeric，序列化出去三個前端都要
    -- 額外處理，而折抵金額實務上就是整數（TWD／JPY 本來就沒有小數位）。
    -- 真的碰到 $9.99 這種，上架者填 other 用文字寫。
    ADD COLUMN discount_value int,
    -- ISO 4217 三碼。由上架者自己選而不是跟著服務商的 countries 推導：
    -- 跨國服務商的 countries 是空的，推不出幣別，而同一家底下本來就可能
    -- 同時有台幣碼跟日圓碼。
    ADD COLUMN discount_currency text,

    ADD CONSTRAINT codes_code_type_check CHECK (code_type IN ('referral', 'discount')),
    ADD CONSTRAINT codes_discount_kind_check
        CHECK (discount_kind IS NULL OR discount_kind IN ('percent', 'amount', 'free_trial', 'other')),

    -- 欄位形狀擋在資料庫這一層，因為寫入點不只一個（API、將來的匯入工具），
    -- 而「discount_kind 是 amount 卻沒有幣別」這種半套資料前端顯示不出東西。
    ADD CONSTRAINT codes_discount_shape_check CHECK (
        CASE code_type
            WHEN 'referral' THEN
                discount_kind IS NULL AND discount_value IS NULL AND discount_currency IS NULL
            WHEN 'discount' THEN
                discount_kind IS NOT NULL
                -- other 以外都要有數值，other 一定沒有。
                AND (discount_kind = 'other') = (discount_value IS NULL)
                -- 幣別只有 amount 有意義，percent 帶幣別是填錯了。
                AND (discount_kind = 'amount') = (discount_currency IS NOT NULL)
            ELSE false
        END
    ),

    -- 範圍分開驗，訊息才對得上是哪一格填錯。
    -- percent 上界取 99：100% off 是免費送，那不是折扣碼該表達的東西。
    ADD CONSTRAINT codes_discount_value_range_check CHECK (
        discount_value IS NULL
        OR (discount_kind = 'percent'    AND discount_value BETWEEN 1 AND 99)
        OR (discount_kind = 'amount'     AND discount_value > 0)
        OR (discount_kind = 'free_trial' AND discount_value > 0)
    ),
    ADD CONSTRAINT codes_discount_currency_check
        CHECK (discount_currency IS NULL OR discount_currency ~ '^[A-Z]{3}$');

-- 一人一家一個碼的限制改成一人一家「每種類型」一個：推薦碼跟折扣碼是兩件
-- 不同的東西，手上兩種都有的人不該被逼著二選一。同一類型仍然只能有一個，
-- 擋重複上架洗榜的原意不變。
DROP INDEX referral_code_bonus.codes_user_merchant_live_idx;
CREATE UNIQUE INDEX codes_user_merchant_type_live_idx
    ON referral_code_bonus.referral_codes (user_id, merchant_id, code_type)
    WHERE status IN ('pending', 'active');

-- 這家服務商收哪幾種碼。
--
-- 預設兩種都收，既有服務商也一起（ADD COLUMN 帶 DEFAULT 會把現有的列一併填上，
-- 不必另外 UPDATE）。代價是沒有推薦計畫的服務商底下也收得到推薦碼，那種假碼
-- 要靠人工審核擋下來 —— 這是刻意換的：一上線就有得選，比逐家打開快。
-- 之後真的被假推薦碼洗，就到後台把那幾家的推薦碼取消勾選。
--
-- 跟 countries 一樣不另外開表：這是一份極短的靜態清單，值域用 CHECK 管就夠。
ALTER TABLE referral_code_bonus.merchants
    ADD COLUMN allowed_code_types text[] NOT NULL DEFAULT '{referral,discount}',
    -- 折扣碼的格式跟推薦碼不一樣：推薦碼多半是系統發的固定格式（8 碼英數），
    -- 折扣碼是行銷活動字串（SUMMER2026）。共用一個 regex 會把其中一種全部誤擋，
    -- 所以分開各驗各的。NULL 跟 code_format_regex 一樣代表不驗。
    ADD COLUMN discount_code_format_regex text,

    ADD CONSTRAINT merchants_allowed_code_types_check CHECK (
        cardinality(allowed_code_types) > 0
        AND allowed_code_types <@ ARRAY['referral', 'discount']
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE referral_code_bonus.merchants
    DROP CONSTRAINT IF EXISTS merchants_allowed_code_types_check,
    DROP COLUMN IF EXISTS discount_code_format_regex,
    DROP COLUMN IF EXISTS allowed_code_types;

-- 回滾前要先把折扣碼清掉，不然舊的唯一索引建不起來（同一人同一家可能有兩筆）。
-- 直接刪而不是改成 referral：那些碼在回滾後的資料模型裡沒有對應的東西，
-- 留著只會變成一批沒有優惠內容、看起來像推薦碼的髒資料。
DELETE FROM referral_code_bonus.referral_codes WHERE code_type = 'discount';

DROP INDEX referral_code_bonus.codes_user_merchant_type_live_idx;
CREATE UNIQUE INDEX codes_user_merchant_live_idx
    ON referral_code_bonus.referral_codes (user_id, merchant_id)
    WHERE status IN ('pending', 'active');

ALTER TABLE referral_code_bonus.referral_codes
    DROP CONSTRAINT IF EXISTS codes_discount_currency_check,
    DROP CONSTRAINT IF EXISTS codes_discount_value_range_check,
    DROP CONSTRAINT IF EXISTS codes_discount_shape_check,
    DROP CONSTRAINT IF EXISTS codes_discount_kind_check,
    DROP CONSTRAINT IF EXISTS codes_code_type_check,
    DROP COLUMN IF EXISTS discount_currency,
    DROP COLUMN IF EXISTS discount_value,
    DROP COLUMN IF EXISTS discount_kind,
    DROP COLUMN IF EXISTS code_type;

-- +goose StatementEnd
