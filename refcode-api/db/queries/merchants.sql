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
-- name: ListMerchants :many
SELECT
    m.*,
    c.name AS category_name,
    c.name_en AS category_name_en,
    c.name_ja AS category_name_ja,
    coalesce(stat.active_code_count, 0) AS active_code_count,
    stat.soonest_expires_at
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
  AND (sqlc.narg(search)::text IS NULL OR m.name ILIKE '%' || sqlc.narg(search)::text || '%')
-- 地區優先只是排序，不過濾：外地的服務商照樣看得到，只是排在後面。
-- viewer_country 是 NULL（沒登入、或沒填所在地）時整個 CASE 都是 1，
-- 等於退回原本的排序 —— 匿名訪客拿到的 SSR 內容不會因人而異，SEO 才不會受影響。
ORDER BY
    CASE
        WHEN sqlc.narg(viewer_country)::text IS NULL THEN 1
        WHEN m.countries @> ARRAY[sqlc.narg(viewer_country)::text] THEN 0  -- 在地
        WHEN cardinality(m.countries) = 0 THEN 1                           -- 不分地區
        ELSE 2                                                             -- 外地
    END,
    active_code_count DESC, m.name
LIMIT $1 OFFSET $2;

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
