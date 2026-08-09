-- 熱門關鍵字。只有使用者明確送出的查詢才進來（API 的 commit 參數），
-- 逐字輸入不記 —— 打一次「台新銀行」會留下四筆前綴，榜單就毀了。
-- name: UpsertSearchTerm :exec
INSERT INTO referral_code_bonus.search_terms (term, lang)
VALUES (sqlc.arg(term)::text, sqlc.arg(lang)::text)
ON CONFLICT (term, lang) DO UPDATE
SET hits = referral_code_bonus.search_terms.hits + 1,
    last_searched_at = now();

-- 榜單只看最近這段時間有人搜過的詞。不設時間窗的話，早期洗上去的詞會永遠卡在
-- 前面，新的熱門服務商永遠擠不進來。
-- name: ListPopularSearchTerms :many
SELECT term, hits
FROM referral_code_bonus.search_terms
WHERE lang = sqlc.arg(lang)::text
  AND last_searched_at > now() - sqlc.arg(window_days)::int * interval '1 day'
ORDER BY hits DESC, term
LIMIT sqlc.arg(max_results)::int;
