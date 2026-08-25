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
    -- expires_at IS NULL 是永久有效的碼，要算進 active_code_count。
    -- min() 會忽略 NULL，所以 soonest_expires_at 仍是「最快到期的那個碼」，
    -- 全部都是永久碼時回 NULL，前端就不顯示倒數。
    WHERE rc.merchant_id = m.id AND rc.status = 'active'
      AND (rc.expires_at IS NULL OR rc.expires_at > now())
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
--
-- 最後那條是打錯字的救援：ILIKE 是子字串比對，'coupng' 少一個字母就一筆都不中。
--
-- 用 word_similarity 而不是 similarity：後者比的是整個字串，名字帶後綴時分數
-- 會被稀釋掉 —— 'amazn' 對 'Amazon Shopping' 只有 0.222，對 'Amazon' 才夠高，
-- 而目錄裡的名字十家有八家帶後綴。word_similarity 是拿查詢去比對方最像的那一段，
-- 同一組實測 0.667。參數順序不能反：第一個是要拿去找的詞，第二個是被找的字串。
--
-- 門檻 0.5 是照現有 263 家實測出來的：典型 typo 最低的是 'coupng'→'Coupang' 的
-- 0.571，抓得到；再往上一階到 pg_trgm 預設的 0.6 就會漏掉它。
--
-- length >= 4 這道更重要。三個字母的查詢 trigram 太少，'pay' 在 0.5 會撈出
-- Paramount+、Papa Johns、Panera Bread 這種只是開頭像的；四個字以上就乾淨了
-- （bank / shop / food / card 實測都沒有多撈）。短查詢本來就容易精確命中，
-- 不需要模糊救。中日文查詢通常只有兩三個字，會被這道擋掉 —— 但三連字元對中文
-- 本來就切不出東西（見 SuggestMerchants 的說明），擋掉沒有損失。
--
-- 比對的是 search_raw（未 escape）。word_similarity 不吃 LIKE 的萬用字元，
-- 餵 escape 過的字串進去，反斜線會變成要比對的內容之一。
--
-- 這條走不到 merchants_name_trgm 那個 GIN 索引（要 <% 運算子才走得到，但它的
-- 門檻是 session 層級的 GUC，連線池底下設了會殘留給下一個請求），所以是全表計算。
-- 263 家的規模這個代價無所謂，真的長到幾萬家再回頭換成 <% 加 set_limit。
  AND (
    sqlc.narg(search)::text IS NULL
    OR (
        m.name || ' ' || m.reward_desc || ' ' ||
        coalesce(m.reward_desc_en, '') || ' ' || coalesce(m.reward_desc_ja, '')
    ) ILIKE '%' || sqlc.narg(search)::text || '%'
    OR (
        c.name || ' ' || coalesce(c.name_en, '') || ' ' || coalesce(c.name_ja, '')
    ) ILIKE '%' || sqlc.narg(search)::text || '%'
    OR (
        length(sqlc.narg(search_raw)::text) >= 4
        AND extensions.word_similarity(sqlc.narg(search_raw)::text, m.name) >= 0.5
    )
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
        WHEN (
            m.name || ' ' || m.reward_desc || ' ' ||
            coalesce(m.reward_desc_en, '') || ' ' || coalesce(m.reward_desc_ja, '')
        ) ILIKE '%' || sqlc.narg(search)::text || '%'
          OR (
            c.name || ' ' || coalesce(c.name_en, '') || ' ' || coalesce(c.name_ja, '')
        ) ILIKE '%' || sqlc.narg(search)::text || '%' THEN 3              -- 說明或分類名命中
        ELSE 4                                                            -- 名字只是「像」，靠 similarity 撈回來的
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
INSERT INTO referral_code_bonus.merchants (slug, name, category_id, logo_url, signup_url, reward_desc, code_format_regex, countries, reward_desc_en, reward_desc_ja, allowed_code_types, discount_code_format_regex)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
    reward_desc_en = $11, reward_desc_ja = $12,
    allowed_code_types = $13, discount_code_format_regex = $14, updated_at = now()
WHERE id = $1
RETURNING *;


-- 後台維護用：不過濾 is_active，而且要帶齊編輯表單需要的欄位
-- （公開的 ListMerchants 只回展示用的子集）。
-- name: ListMerchantsForAdmin :many
SELECT
    m.*,
    c.name AS category_name,
    (SELECT count(*) FROM referral_code_bonus.referral_codes rc
      WHERE rc.merchant_id = m.id AND rc.status = 'active'
        AND (rc.expires_at IS NULL OR rc.expires_at > now())) AS active_code_count
FROM referral_code_bonus.merchants m
JOIN referral_code_bonus.merchant_categories c ON c.id = m.category_id
ORDER BY m.name;

-- app 的地區選單要照「實際有服務商的國家」給，不是寫死一份清單 ——
-- 目錄是從 App Store 排行榜匯進來的，涵蓋哪些國家會隨著匯入與停用一直變。
-- 寫死的清單遲早會同時出現「選了卻是空的」與「有服務商卻選不到」兩種錯。
--
-- countries 是空陣列的服務商（不分地區）不會貢獻任何國家，這是對的：
-- 它們在任何地區都看得到，不構成「選這個國家」的理由。
-- name: ListActiveMerchantCountries :many
-- ::text 不能省：sqlc 推不出 unnest() 的型別，沒有 cast 會產出 interface{}。
SELECT c::text AS country, count(*) AS merchant_count
FROM referral_code_bonus.merchants m, unnest(m.countries) AS c
WHERE m.is_active
GROUP BY c
ORDER BY merchant_count DESC, c;

-- sitemap 用：所有可索引的服務商 slug。
-- name: ListMerchantSlugs :many
SELECT slug, updated_at FROM referral_code_bonus.merchants WHERE is_active ORDER BY slug;

-- logo 補圖用（cmd/logobackfill）。只動 logo_url，不像 UpdateMerchant 那樣整列覆蓋 ——
-- 補圖跟後台編輯是兩件事，用全欄位更新會把後台剛改好的欄位一起蓋回舊值。
-- 只補還沒有圖的，重複跑不會覆蓋已經有圖的（包含後台手動換過的）。
-- name: SetMerchantLogo :execrows
UPDATE referral_code_bonus.merchants
SET logo_url = @logo_url, updated_at = now()
WHERE id = @id AND logo_url IS NULL;

-- 補圖用：還沒有 logo 的啟用中服務商。
-- name: ListMerchantsWithoutLogo :many
SELECT id, slug, name, signup_url FROM referral_code_bonus.merchants
WHERE is_active AND logo_url IS NULL ORDER BY name;
