const HISTORY_KEY = 'refcode_search_history'

// 留幾筆。搜尋頁的空白狀態要一眼看完，超過這個數量就得捲，
// 而會去捲搜尋歷史的人本來就會直接重打。
const MAX_ITEMS = 8

// 搜尋歷史只存在這台瀏覽器，不進後端 —— 「這個人搜過什麼」是很個人的東西，
// 而平台需要的熱門榜在後端已經有聚合版本（search_terms），不需要個人層級的紀錄。
export function useSearchHistory() {
  const items = ref<string[]>([])

  function read(): string[] {
    if (import.meta.server) return []
    try {
      const raw = localStorage.getItem(HISTORY_KEY)
      const parsed: unknown = raw ? JSON.parse(raw) : []
      // 使用者自己改壞了、或舊版本存過別的格式時，寧可當作沒有歷史，
      // 也不要讓整頁掛在 map 上。
      return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : []
    } catch {
      return []
    }
  }

  function write(next: string[]) {
    items.value = next
    if (import.meta.server) return
    try {
      localStorage.setItem(HISTORY_KEY, JSON.stringify(next))
    } catch {
      // 無痕模式或空間滿了。歷史掉了不值得打斷搜尋。
    }
  }

  // localStorage 在 SSR 階段讀不到，所以載入要等到掛載之後 —— 直接在 setup
  // 裡讀會讓伺服器與瀏覽器的第一次 render 不一致（hydration mismatch）。
  function load() {
    items.value = read()
  }

  function add(term: string) {
    const t = term.trim()
    if (!t) return
    // 重搜同一個詞是把它移到最前面，不是留兩筆。
    write([t, ...read().filter((v) => v !== t)].slice(0, MAX_ITEMS))
  }

  function remove(term: string) {
    write(read().filter((v) => v !== term))
  }

  function clear() {
    write([])
  }

  return { items, load, add, remove, clear }
}
