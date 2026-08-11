import { Preferences } from '@capacitor/preferences'
import { defineStore } from 'pinia'
import { ref } from 'vue'

const KEY_CODES = 'record_copied_codes'
const KEY_MERCHANTS = 'record_viewed_merchants'

// 兩份各留幾筆。紀錄頁是拿來「找回剛剛那個」的，不是完整流水帳；
// 留太多只會讓要找的那筆更難找，也讓 Preferences 一直長大。
const MAX_ITEMS = 30

export interface CopiedCode {
  codeId: string
  code: string
  merchantSlug: string
  merchantName: string
  merchantLogo: string | null
  expiresAt: string
  copiedAt: string
}

export interface ViewedMerchant {
  slug: string
  name: string
  logo: string | null
  viewedAt: string
}

// 只存不會隨語言改變的欄位。獎勵說明與分類名是後端依 ?lang= 回的，
// 存下來的話使用者切了語言，紀錄頁會留著上一個語言的字 —— 服務商名是品牌名
// （後端也沒有譯文欄位），碼本身更不會變，這兩個才存得住。
//
// 跟搜尋歷史同一個立場：這是「這個人看過什麼、複製過什麼」，很個人的東西，
// 只留在這台裝置，不上傳後端。後端要的統計已經有 events 那張表的聚合版本。
export const useRecordStore = defineStore('record', () => {
  const codes = ref<CopiedCode[]>([])
  const merchants = ref<ViewedMerchant[]>([])

  // 存壞掉或舊版本存過別的格式時當作沒有紀錄，不要讓整頁掛在 v-for 上。
  async function read<T>(key: string, valid: (v: unknown) => v is T): Promise<T[]> {
    const { value } = await Preferences.get({ key })
    if (!value) return []
    try {
      const parsed: unknown = JSON.parse(value)
      return Array.isArray(parsed) ? parsed.filter(valid) : []
    } catch {
      return []
    }
  }

  function isCopiedCode(v: unknown): v is CopiedCode {
    return typeof v === 'object' && v !== null && typeof (v as CopiedCode).codeId === 'string'
  }

  function isViewedMerchant(v: unknown): v is ViewedMerchant {
    return typeof v === 'object' && v !== null && typeof (v as ViewedMerchant).slug === 'string'
  }

  async function load() {
    ;[codes.value, merchants.value] = await Promise.all([
      read(KEY_CODES, isCopiedCode),
      read(KEY_MERCHANTS, isViewedMerchant),
    ])
  }

  // 同一個碼複製第二次是把它移到最前面，不是留兩筆 —— 使用者要的是
  // 「我最近拿過哪些碼」，同一組出現三次只是把別的擠掉。
  async function addCode(entry: Omit<CopiedCode, 'copiedAt'>) {
    const next = [
      { ...entry, copiedAt: new Date().toISOString() },
      ...codes.value.filter((c) => c.codeId !== entry.codeId),
    ].slice(0, MAX_ITEMS)
    codes.value = next
    await Preferences.set({ key: KEY_CODES, value: JSON.stringify(next) })
  }

  async function addMerchant(entry: Omit<ViewedMerchant, 'viewedAt'>) {
    const next = [
      { ...entry, viewedAt: new Date().toISOString() },
      ...merchants.value.filter((m) => m.slug !== entry.slug),
    ].slice(0, MAX_ITEMS)
    merchants.value = next
    await Preferences.set({ key: KEY_MERCHANTS, value: JSON.stringify(next) })
  }

  async function clearCodes() {
    codes.value = []
    await Preferences.remove({ key: KEY_CODES })
  }

  async function clearMerchants() {
    merchants.value = []
    await Preferences.remove({ key: KEY_MERCHANTS })
  }

  return { codes, merchants, load, addCode, addMerchant, clearCodes, clearMerchants }
})
