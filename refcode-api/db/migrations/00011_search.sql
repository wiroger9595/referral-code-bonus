-- +goose Up
-- +goose StatementBegin

-- 搜尋要能比對詞中間（'%銀行%'），前置萬用字元讓 B-tree 完全用不上，資料量一大
-- 就是全表掃。pg_trgm 的 GIN 索引是唯一吃得到這種查詢的做法，順便換來
-- similarity()，沒有結果時可以拿它找「你是不是要找 xxx」。
--
-- 這台機器上還有別的 side project，extension 一律裝在 extensions schema
-- （pgcrypto、uuid-ossp 都在那），不要再往 public 丟一個 —— public 已經被
-- 別人的 vector 佔過了。也因此下面的 opclass 與查詢裡的 similarity() 都寫完整
-- 路徑，不靠連線的 search_path。
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA extensions;

-- 索引的表達式要跟 ListMerchants 的 WHERE 寫得一模一樣，差一個空白就不會被用到。
-- 改任何一邊都要回來對另一邊。
--
-- 服務商名不翻（品牌名），所以只有一份；獎勵說明與分類名有三種語言，
-- 全部串進同一個索引 —— 日文使用者搜英文字、中文使用者搜日文字都該找得到，
-- 分成三個索引反而要在查詢裡分語言 branch。
CREATE INDEX IF NOT EXISTS merchants_search_trgm
    ON referral_code_bonus.merchants
    USING gin ((
        name || ' ' || reward_desc || ' ' ||
        coalesce(reward_desc_en, '') || ' ' || coalesce(reward_desc_ja, '')
    ) extensions.gin_trgm_ops);

CREATE INDEX IF NOT EXISTS merchant_categories_search_trgm
    ON referral_code_bonus.merchant_categories
    USING gin ((
        name || ' ' || coalesce(name_en, '') || ' ' || coalesce(name_ja, '')
    ) extensions.gin_trgm_ops);

-- 找相近服務商（SuggestMerchants）只比對名稱，不能共用上面那個串起來的索引。
CREATE INDEX IF NOT EXISTS merchants_name_trgm
    ON referral_code_bonus.merchants
    USING gin (name extensions.gin_trgm_ops);

-- 熱門關鍵字。**只存詞與次數，不存 user_id、device、IP** —— 熱門榜要的是聚合，
-- 接上任何識別欄位就變成「誰搜過什麼」的紀錄，那是完全不同等級的東西。
--
-- lang 進主鍵：中文站的「銀行」跟英文站的「bank」是兩份榜，混在一起會讓
-- 英文使用者看到一排中文詞。
--
-- 只有使用者「確定要搜這個」時才寫進來（API 的 commit 參數），逐字輸入不記 ——
-- 不然打一次「台新銀行」會留下「台」「台新」「台新銀」四筆垃圾。
CREATE TABLE referral_code_bonus.search_terms (
    term             text NOT NULL,
    lang             text NOT NULL,
    hits             bigint NOT NULL DEFAULT 1,
    last_searched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (term, lang)
);

-- 榜單查詢是「某語言、最近有人搜過的、次數前 N 名」。不同的詞數量級很小
-- （幾千筆頂天），時間條件留給 filter 做就好，不值得再開一個複合索引。
CREATE INDEX IF NOT EXISTS search_terms_popular
    ON referral_code_bonus.search_terms (lang, hits DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS referral_code_bonus.search_terms_popular;
DROP TABLE IF EXISTS referral_code_bonus.search_terms;
DROP INDEX IF EXISTS referral_code_bonus.merchants_name_trgm;
DROP INDEX IF EXISTS referral_code_bonus.merchant_categories_search_trgm;
DROP INDEX IF EXISTS referral_code_bonus.merchants_search_trgm;

-- pg_trgm 故意不 DROP。這是整個資料庫共用的東西，這裡 rollback 不代表
-- 同一台上的別的 side project 沒在用它，砍掉會連累別人。

-- +goose StatementEnd
