-- 這一輪該檢查的服務商：距離上次檢查已經超過 @stale_days 天的。
--
-- 「誰滿一週了」用資料庫裡的時間算，不靠排程自己的 ticker 記 —— API 一週內
-- 重啟過（部署、擴縮）ticker 就歸零，168 小時的 ticker 在正常部署頻率下幾乎
-- 不會有觸發的一天。改成每天醒來問這支查詢，每家仍然是一週一次，而且一輪跑到
-- 一半被中斷（部署、ctx 取消），下次醒來會接著跑剩下的那些。
--
-- last_result 是上一次的結論，排程靠它判斷「連續兩次都沒找到」才下架。
--
-- coalesce 不能省：result 在來源表是 NOT NULL，sqlc 就照著推成非 nullable 的
-- string，但這裡是 LEFT JOIN —— 從沒檢查過的那些掃回來是 NULL，會直接掃爆。
-- 空字串代表「從沒檢查過」，跟三個 result 值不會撞。
--
-- active_code_count 是下架前的第二道關卡，見 DeactivateMerchantForCodeAudit。
-- 這裡也帶一份是為了在軌跡的 note 上寫清楚「為什麼滿兩次還是沒下架」。
-- name: ListMerchantsDueForCodeAudit :many
SELECT
    m.id, m.name, m.signup_url,
    coalesce(last.result, '') AS last_result,
    -- 「可用的碼」照 ListMerchants（merchants.sql）那份定義寫，兩邊分岔的話，
    -- 目錄上顯示「3 個可用推薦碼」的服務商會被稽核當成沒人上架而下架。
    (SELECT count(*) FROM referral_code_bonus.referral_codes rc
      WHERE rc.merchant_id = m.id AND rc.status = 'active'
        AND (rc.expires_at IS NULL OR rc.expires_at > now())) AS active_code_count
FROM referral_code_bonus.merchants m
LEFT JOIN LATERAL (
    SELECT a.result, a.created_at
    FROM referral_code_bonus.merchant_code_audits a
    WHERE a.merchant_id = m.id
    ORDER BY a.created_at DESC
    LIMIT 1
) last ON true
-- 停用的也查，包含 appimport 匯進來還沒補完的草稿。目錄的流程是「爬榜匯進來 →
-- 確認有沒有在發碼 → 人工補完再上架」，只掃已上架的話新匯進來的那批永遠得不到
-- 判定，整條線就斷在中間 —— 而判定結果正是複檢草稿的人最需要的資訊。
-- 排程只回報不下架，多掃停用的那些沒有副作用，只是多幾次 HTTP 請求。
WHERE last.created_at IS NULL OR last.created_at < now() - make_interval(days => @stale_days::int)
-- 從沒檢查過的排最前面（ASC 預設是 NULLS LAST，這裡要反過來）：新匯進來的
-- 服務商最可能根本沒在發碼，先問它們。
ORDER BY last.created_at ASC NULLS FIRST, m.name;

-- name: CreateMerchantCodeAudit :one
INSERT INTO referral_code_bonus.merchant_code_audits
    (merchant_id, result, checked_urls, matched, note, deactivated)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- 目前沒有呼叫者：排程改成只回報不下架（誤判率見 merchantaudit 的 package 註解），
-- 「該不該下架」由後台的人看過 ListMerchantsFlaggedByCodeAudit 之後自己決定。
-- 留著是因為判定準確度若哪天夠了，要接回去只差 Sweep 裡那一次呼叫。
--
-- 稽核判定沒在發碼時下架。只動 is_active，不走 UpdateMerchant 整列覆蓋 ——
-- 理由同 SetMerchantLogo：全欄位更新會把後台剛改好的欄位蓋回舊值。
--
-- 回傳列數是「這次有沒有真的下架」。條件帶 is_active 是因為排程跑一輪要好幾
-- 分鐘，這段期間後台可能已經自己把它停用了，那次不該再算成稽核下架的成果。
--
-- NOT EXISTS 那段是爬蟲誤判的擋牆，不能省。實測 10 個確定有推薦計畫的平台，
-- 抓得到頁面的 8 家裡有 4 家被判成 missing —— 推薦計畫多半在登入牆後面
-- （Dropbox 的 /referrals 直接轉去 login），公開 HTML 上什麼都看不到。
-- 「架上還有使用者上傳的可用碼」是跟爬蟲完全獨立的一個訊號，而且比爬蟲強：
-- 有人正在用這家的碼，就是它還在發碼的活證據。兩個訊號都指向沒在發碼才下架。
--
-- 條件寫在 UPDATE 裡而不是只在 Go 那邊判：一輪要跑好幾分鐘，這段期間有人上架
-- 新的碼是常態，用查詢當下的快照決定會把剛救活的那家照樣下架。
-- name: DeactivateMerchantForCodeAudit :execrows
UPDATE referral_code_bonus.merchants m
SET is_active = false, updated_at = now()
WHERE m.id = @id AND m.is_active
  AND NOT EXISTS (
      SELECT 1 FROM referral_code_bonus.referral_codes rc
      WHERE rc.merchant_id = m.id AND rc.status = 'active'
        AND (rc.expires_at IS NULL OR rc.expires_at > now())
  );

-- 連續兩次都判 missing 的服務商，等後台人工複檢。
--
-- 排程只回報不下架（實測誤判率 43%，見 merchantaudit 的 package 註解），所以
-- 「該不該下架」的決定留在這張清單上由人做。判定條件跟原本的自動下架一樣 ——
-- 連續兩次 missing，中間夾 unreachable 的不算，因為那次根本沒看到頁面。
--
-- active_code_count 一起帶出來，讓複檢的人有第二個跟爬蟲無關的訊號：架上還有人
-- 在用的碼，就是這家還在發碼的活證據。定義照 ListMerchantsDueForCodeAudit 那份寫。
--
-- checked_urls 與 note 是最近一次的，複檢時要先確認爬蟲抓的是不是正確的頁面 ——
-- 抓錯頁（例如抓到登入牆）跟這家真的收掉推薦計畫，結論完全不同。
-- name: ListMerchantsFlaggedByCodeAudit :many
WITH recent AS (
    SELECT
        a.merchant_id, a.result, a.checked_urls, a.note, a.created_at,
        row_number() OVER (PARTITION BY a.merchant_id ORDER BY a.created_at DESC) AS rn
    FROM referral_code_bonus.merchant_code_audits a
)
SELECT
    m.id, m.name, m.signup_url,
    latest.checked_urls, latest.note,
    latest.created_at AS last_checked_at,
    (SELECT count(*) FROM referral_code_bonus.referral_codes rc
      WHERE rc.merchant_id = m.id AND rc.status = 'active'
        AND (rc.expires_at IS NULL OR rc.expires_at > now())) AS active_code_count,
    count(*) OVER () AS total_count
FROM referral_code_bonus.merchants m
JOIN recent latest ON latest.merchant_id = m.id AND latest.rn = 1
JOIN recent prev ON prev.merchant_id = m.id AND prev.rn = 2
WHERE m.is_active
  AND latest.result = 'missing'
  AND prev.result = 'missing'
ORDER BY latest.created_at DESC
LIMIT $1 OFFSET $2;
