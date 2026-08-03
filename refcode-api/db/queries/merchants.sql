-- name: ListCategories :many
SELECT * FROM referral_code_bonus.merchant_categories ORDER BY sort_order, name;

-- name: CreateCategory :one
INSERT INTO referral_code_bonus.merchant_categories (slug, name, sort_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateCategory :one
UPDATE referral_code_bonus.merchant_categories
SET slug = $2, name = $3, sort_order = $4
WHERE id = $1
RETURNING *;

-- 舊 slug 也要找得到。live 的一律勝出（ORDER BY），所以就算某個字串同時是
-- A 的舊網址又被 B 拿去當現用網址，也不會回錯人。
-- name: GetCategoryBySlug :one
SELECT * FROM referral_code_bonus.merchant_categories c
WHERE c.slug = $1
   OR c.id IN (
        SELECT h.category_id FROM referral_code_bonus.category_slug_history h WHERE h.slug = $1
      )
ORDER BY (c.slug = $1) DESC
LIMIT 1;

-- name: GetCategoryByID :one
SELECT * FROM referral_code_bonus.merchant_categories WHERE id = $1;

-- name: RecordCategorySlugChange :exec
INSERT INTO referral_code_bonus.category_slug_history (slug, category_id)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET category_id = EXCLUDED.category_id, replaced_at = now();

-- 一個 slug 變成現用的，就不該再是誰的舊網址。
-- name: ClaimCategorySlug :exec
DELETE FROM referral_code_bonus.category_slug_history WHERE slug = $1;

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
    c.slug AS category_slug,
    c.name AS category_name,
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
  AND (sqlc.narg(category_slug)::text IS NULL OR c.slug = sqlc.narg(category_slug)::text)
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

-- 舊 slug 也要找得到，呼叫端比對回來的 m.slug 決定要不要 301（見 handleGetMerchant）。
-- live 的一律勝出（ORDER BY），所以就算某個字串同時是 A 的舊網址又被 B 拿去當現用網址，
-- 也不會回錯家。
-- name: GetMerchantBySlug :one
SELECT m.*, c.slug AS category_slug, c.name AS category_name
FROM referral_code_bonus.merchants m
JOIN referral_code_bonus.merchant_categories c ON c.id = m.category_id
WHERE m.is_active
  AND (m.slug = $1
       OR m.id IN (
            SELECT h.merchant_id FROM referral_code_bonus.merchant_slug_history h WHERE h.slug = $1
          ))
ORDER BY (m.slug = $1) DESC
LIMIT 1;

-- name: GetMerchantByID :one
SELECT * FROM referral_code_bonus.merchants WHERE id = $1;

-- name: CreateMerchant :one
INSERT INTO referral_code_bonus.merchants (slug, name, category_id, logo_url, signup_url, reward_desc, code_format_regex, countries)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateMerchant :one
UPDATE referral_code_bonus.merchants
SET slug = $2, name = $3, category_id = $4, logo_url = $5, signup_url = $6,
    reward_desc = $7, code_format_regex = $8, is_active = $9, countries = $10, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RecordMerchantSlugChange :exec
INSERT INTO referral_code_bonus.merchant_slug_history (slug, merchant_id)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET merchant_id = EXCLUDED.merchant_id, replaced_at = now();

-- 一個 slug 變成現用的，就不該再是誰的舊網址。
-- name: ClaimMerchantSlug :exec
DELETE FROM referral_code_bonus.merchant_slug_history WHERE slug = $1;

-- 後台維護用：不過濾 is_active，而且要帶齊編輯表單需要的欄位
-- （公開的 ListMerchants 只回展示用的子集）。
-- name: ListMerchantsForAdmin :many
SELECT
    m.*,
    c.slug AS category_slug,
    c.name AS category_name,
    (SELECT count(*) FROM referral_code_bonus.referral_codes rc
      WHERE rc.merchant_id = m.id AND rc.status = 'active' AND rc.expires_at > now()) AS active_code_count
FROM referral_code_bonus.merchants m
JOIN referral_code_bonus.merchant_categories c ON c.id = m.category_id
ORDER BY m.name;

-- sitemap 用：所有可索引的服務商 slug。
-- name: ListMerchantSlugs :many
SELECT slug, updated_at FROM referral_code_bonus.merchants WHERE is_active ORDER BY slug;
