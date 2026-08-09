import { Preferences } from '@capacitor/preferences'
import { defineStore } from 'pinia'
import { ref } from 'vue'

const KEY = 'search_history'

// 留幾筆。chips 只佔一排，超過這個數量就得左右撥，而會去撥搜尋歷史的人
// 本來就會直接重打。
const MAX_ITEMS = 8

// 搜尋歷史只留在這台裝置，不進後端 —— 「這個人搜過什麼」是很個人的東西，
// 而平台需要的熱門榜後端已經有聚合版本（search_terms），不需要個人層級的紀錄。
// 用 Preferences 而不是 localStorage：WebView 的 localStorage 會被系統清掉。
export const useSearchHistoryStore = defineStore('searchHistory', () => {
  const items = ref<string[]>([])

  async function load() {
    const { value } = await Preferences.get({ key: KEY })
    if (!value) return
    try {
      const parsed: unknown = JSON.parse(value)
      // 舊版本存過別的格式、或值被改壞時，寧可當作沒有歷史，
      // 也不要讓整個探索頁掛在 v-for 上。
      items.value = Array.isArray(parsed)
        ? parsed.filter((v): v is string => typeof v === 'string')
        : []
    } catch {
      items.value = []
    }
  }

  async function persist(next: string[]) {
    items.value = next
    await Preferences.set({ key: KEY, value: JSON.stringify(next) })
  }

  async function add(term: string) {
    const t = term.trim()
    if (!t) return
    // 重搜同一個詞是把它移到最前面，不是留兩筆。
    await persist([t, ...items.value.filter((v) => v !== t)].slice(0, MAX_ITEMS))
  }

  async function remove(term: string) {
    await persist(items.value.filter((v) => v !== term))
  }

  async function clear() {
    await persist([])
  }

  return { items, load, add, remove, clear }
})
