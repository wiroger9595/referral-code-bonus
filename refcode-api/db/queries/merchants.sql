-- name: ListCategories :many
SELECT * FROM referral_code_bonus.merchant_categories ORDER BY sort_order, name;

-- name: CreateCategory :one
INSERT INTO referral_code_bonus.merchant_categories (name, sort_order, image_url, name_en, name_ja)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateCategory :one
UPDATE referral_code_bonus.merchant_categories
SET name = $2, sort_order = $3, image_url = $4, name_en = $5, name_ja = $6
WHERE id = $1
RETURNING *;

-- name: GetCategoryByID :one
SELECT * FROM referral_code_bonus.merchant_categories WHERE id = $1;

-- merchants.category_id 是 FK 且沒有 ON DELETE，還有服務商掛著的分類刪不掉，
-- 交給呼叫端把 23503 轉成看得懂的錯誤（見 store.IsForeignKeyViolation）。
-- name: DeleteCategory :exec
DELETE FROM referral_code_bonus.merchant_categories WHERE id = $1;

-- 目錄頁：帶上每家目前有幾個可用的碼，前端才能顯示「12 個可用推薦碼」。
-- soonest_expires_at 是這家最快到期的那個碼，目錄頁用它顯示倒數與分「快到期」區塊；
-- 一家都沒有可用的碼時是 NULL。
--
-- 兩個統計值走同一個 LATERAL，referral_codes 只掃一次。
--
-- sqlc 推不出 aggregate 的 nullability，soonest_expires_at 產出來是 interface{}，
-- 由 handler 收成 *time.Time。加 ::timestamptz 會讓它變成非 nullable 的 time.Time，
-- 掃到 NULL（這家沒有可用的碼）直接噴錯，所以不要加。
--
-- total_count 是套用 LIMIT 之前的總筆數（window function 比 LIMIT 早算），
-- 搜尋頁要靠它顯示「找到 N 家」，不必為了一個數字再打一次 count 查詢。
-- name: ListMerchants :many
SELECT
    m.*,
    c.name AS category_name,
    c.name_en AS category_name_en,
    c.name_ja AS category_name_ja,
    coalesce(stat.active_code_count, 0) AS active_code_count,
    stat.soonest_expires_at,
    count(*) OVER () AS total_count
FROM referral_code_bonus.merchants m
JOIN referral_code_bonus.merchant_categories c ON c.id = m.category_id
LEFT JOIN LATERAL (
    SELECT count(*) AS active_code_count, min(rc.expires_at) AS soonest_expires_at
    FROM referral_code_bonus.referral_codes rc
    WHERE rc.merchant_id = m.id AND rc.status = 'active' AND rc.expires_at > now()
) stat ON true
WHERE m.is_active
-- 分類篩選只認 id。
  AND (sqlc.narg(category_id)::uuid IS NULL OR m.category_id = sqlc.narg(category_id)::uuid)
-- 搜尋比對服務商名、獎勵說明、分類名，後兩者連 en/ja 一起比 —— 使用者記得的
-- 常常是「現金回饋」「銀行」這種說明或分類的字，不是品牌名。
--
-- 這兩個串接表達式要跟 00011_search.sql 的 trgm 索引寫得一模一樣，差一個空白
-- 就走不到索引。改這裡一定要回去改那裡。
--
-- 串起來比對的副作用是跨欄位誤中（'天 銀' 會命中 '樂天 銀行送500'），
-- 但那要求查詢字串自己帶空白，真實查詢幾乎不會長那樣，換一個索引很划算。
--
-- search 進來之前已經在 handler escape 過 % 與 _（見 escapeLike），
-- 這裡不必再處理萬用字元。
  AND (
    sqlc.narg(search)::text IS NULL
    OR (
        m.name || ' ' || m.reward_desc || ' ' ||
        coalesce(m.reward_desc_en, '') || ' ' || coalesce(m.reward_desc_ja, '')
    ) ILIKE '%' || sqlc.narg(search)::text || '%'
    OR (
        c.name || ' ' || coalesce(c.name_en, '') || ' ' || coalesce(c.name_ja, '')
    ) ILIKE '%' || sqlc.narg(search)::text || '%'
  )
-- 地區過濾。region 是 NULL 就完全不篩 —— 匿名訪客、沒填所在地、或使用者自己
-- 選了「所有地區」都走這條，官網匿名的 SSR 內容因此不會因人而異，SEO 不受影響。
--
-- countries 是空陣列代表不分地區（串流、雲端這種跨國服務），任何地區都該看得到，
-- 所以它要在過濾裡放行，不是被當成「哪裡都不能用」。
--
-- 只篩目錄，不篩 GetMerchantBySlug：從別人分享的連結點進來的人、或搜尋引擎爬到的
-- 服務商頁還是要打得開，否則跨地區分享的連結會全部變成 404。
  AND (
    sqlc.narg(region)::text IS NULL
    OR cardinality(m.countries) = 0
    OR m.countries @> ARRAY[sqlc.narg(region)::text]
  )
-- 地區優先是排序，跟上面的過濾是兩件事：使用者切到「所有地區」時不再過濾，
-- 但在地的仍然要排前面。
-- viewer_country 是 NULL（沒登入、或沒填所在地）時整個 CASE 都是 1，
-- 等於退回原本的排序。
ORDER BY
    -- 命中強度排在地區之前：搜「台新」的人要的是台新，不是「在你的國家、
    -- 而且名字裡剛好有新字」的那一家。沒搜尋時整個 CASE 都是 0，
    -- 排序完全退回原本的，首頁與分類頁的結果一個字都不會變。
    CASE
        WHEN sqlc.narg(search)::text IS NULL THEN 0
        WHEN m.name ILIKE sqlc.narg(search)::text THEN 0                 -- 名稱就是這個字
        WHEN m.name ILIKE sqlc.narg(search)::text || '%' THEN 1          -- 名稱開頭命中
        WHEN m.name ILIKE '%' || sqlc.narg(search)::text || '%' THEN 2   -- 名稱中間命中
        ELSE 3                                                            -- 只有說明或分類名命中
    END,
    CASE
        WHEN sqlc.narg(viewer_country)::text IS NULL THEN 1
        WHEN m.countries @> ARRAY[sqlc.narg(viewer_country)::text] THEN 0  -- 在地
        WHEN cardinality(m.countries) = 0 THEN 1                           -- 不分地區
        ELSE 2                                                             -- 外地
    END,
    active_code_count DESC, m.name
LIMIT $1 OFFSET $2;

-- 搜不到東西時的「你是不是要找 xxx」。只比服務商名 —— 建議要給得出一個
-- 可以直接點進去的對象，說明或分類名相近沒辦法變成一個連結。
--
-- 門檻 0.15：pg_trgm 預設的 0.3 是給整句英文用的，2-4 個字的品牌名打錯一個字
-- 就掉到 0.3 以下，等於整個建議功能不會出現。0.15 是先放寬到「還看得出關聯」
-- 的起點，等真實查詢累積起來再照 search_terms 裡搜不到的詞回頭調。
--
-- 中日文的效果本來就比英數字差（三連字元切中文詞切不出幾個組合），
-- 這條路對「rakuen → Rakuten」有效，對「台心 → 台新」幫助有限。
-- name: SuggestMerchants :many
SELECT m.slug, m.name
FROM referral_code_bonus.merchants m
WHERE m.is_active
  AND extensions.similarity(m.name, sqlc.arg(search)::text) >= 0.15
ORDER BY extensions.similarity(m.name, sqlc.arg(search)::text) DESC, m.name
LIMIT sqlc.arg(max_results)::int;

-- name: GetMerchantBySlug :one
SELECT m.*, c.name AS category_name, c.name_en AS category_name_en, c.name_ja AS category_name_ja
FROM referral_code_bonus.merchants m
JOIN referral_code_bonus.merchant_categories c ON c.id = m.category_id
WHERE m.slug = $1 AND m.is_active;

-- name: GetMerchantByID :one
SELECT * FROM referral_code_bonus.merchants WHERE id = $1;

-- name: CreateMerchant :one
INSERT INTO referral_code_bonus.merchants (slug, name, category_id, logo_url, signup_url, reward_desc, code_format_regex, countries, reward_desc_en, reward_desc_ja)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- 匯入用（cmd/appimport）。從 App Store 拉回來的只有名稱、圖示與官網，
-- 獎勵說明爬不到，所以一律建成停用的草稿，等後台補完 reward_desc 再上架 ——
-- 沒有獎勵說明的服務商放上目錄，使用者點進去只會看到空白。
-- name: CreateImportedMerchant :one
INSERT INTO referral_code_bonus.merchants (slug, name, category_id, logo_url, signup_url, countries, is_active)
VALUES ($1, $2, $3, $4, $5, $6, false)
RETURNING *;

-- 同一家 app 會同時出現在好幾個國家的排行榜。那是同一家服務商，不該建成好幾列
-- （slug 是唯一的、推薦碼掛在 merchant_id 上，拆列會把同一家的碼池切開），
-- 所以只把國別加進 countries 陣列。
--
-- 已經有這個國家就不動它（連 updated_at 都不碰），回傳的列數就是「有沒有真的加到」。
-- name: AddMerchantCountry :execrows
UPDATE referral_code_bonus.merchants
SET countries = (
        SELECT array_agg(c ORDER BY c) FROM (SELECT DISTINCT unnest(countries || @country::text) AS c) u
    ),
    updated_at = now()
WHERE slug = @slug AND NOT (countries @> ARRAY[@country::text]);

-- name: UpdateMerchant :one
UPDATE referral_code_bonus.merchants
SET slug = $2, name = $3, category_id = $4, logo_url = $5, signup_url = $6,
    reward_desc = $7, code_format_regex = $8, is_active = $9, countries = $10,
    reward_desc_en = $11, reward_desc_ja = $12, updated_at = now()
WHERE id = $1
RETURNING *;


-- 後台維護用：不過濾 is_active，而且要帶齊編輯表單需要的欄位
-- （公開的 ListMerchants 只回展示用的子集）。
-- name: ListMerchantsForAdmin :many
SELECT
    m.*,
    c.name AS category_name,
    (SELECT count(*) FROM referral_code_bonus.referral_codes rc
      WHERE rc.merchant_id = m.id AND rc.status = 'active' AND rc.expires_at > now()) AS active_code_count
FROM referral_code_bonus.merchants m
JOIN referral_code_bonus.merchant_categories c ON c.id = m.category_id
ORDER BY m.name;

-- sitemap 用：所有可索引的服務商 slug。
-- name: ListMerchantSlugs :many
SELECT slug, updated_at FROM referral_code_bonus.merchants WHERE is_active ORDER BY slug;
