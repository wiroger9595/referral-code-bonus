import { Preferences } from '@capacitor/preferences'
import { createI18n } from 'vue-i18n'

import { ApiError, setApiLang } from '../api/client'
import en from './locales/en.json'
import ja from './locales/ja.json'
import zhTW from './locales/zh-TW.json'

const KEY_LANG = 'refcode_lang'

// 日文先停用（付費服務還沒做日本市場的功能／文案），翻譯檔留著沒刪，
// 之後要重開直接把這行加回來就好。
export const SUPPORTED = [
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en', name: 'English' },
] as const

export type LocaleCode = (typeof SUPPORTED)[number]['code']

// 裝置語言可能是 zh-Hant-HK、ja-JP、en-GB 這種，只認前面的部分。
// 中文一律給繁體 —— 目前沒有簡體語系檔，硬給日文或英文更糟。
function fromDevice(): LocaleCode {
  const tags = navigator.languages?.length ? navigator.languages : [navigator.language]
  for (const tag of tags) {
    const lower = tag.toLowerCase()
    if (lower.startsWith('zh')) return 'zh-TW'
    if (lower.startsWith('en')) return 'en'
  }
  return 'zh-TW'
}

function isSupported(code: string | null): code is LocaleCode {
  return SUPPORTED.some((l) => l.code === code)
}

export const i18n = createI18n({
  // Composition API（useI18n）需要 legacy: false。
  legacy: false,
  locale: 'zh-TW',
  fallbackLocale: 'zh-TW',
  messages: { 'zh-TW': zhTW, ja, en },
})

// Preferences 是 async 的，跟 token 一樣要在 mount 之前讀回來，
// 否則畫面會先用預設語言 render 一次再跳語言。
export async function initLocale() {
  const saved = await Preferences.get({ key: KEY_LANG })
  i18n.global.locale.value = isSupported(saved.value) ? saved.value : fromDevice()
  setApiLang(i18n.global.locale.value)
}

// 使用者手動選過語言就記住，之後不再跟著裝置語言跑。
export async function setLocale(code: LocaleCode) {
  i18n.global.locale.value = code
  // 資料欄位的語言跟著切，不然換了介面語言之後分類名還停在上一個語言。
  setApiLang(code)
  await Preferences.set({ key: KEY_LANG, value: code })
}

// 到期倒數。目錄頁的服務商卡片與服務商頁的每個碼共用這一份，
// 分開寫的話同一個時間點會出現「今天到期」與「1 天後到期」兩種說法。
export function daysUntilExpiry(iso: string): number {
  return Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
}

// iso 是 null 代表這個碼沒有到期日，永遠有效。
export function expiryLabel(iso: string | null): string {
  const { t } = i18n.global
  if (iso === null) return t('common.noExpiry')
  const days = daysUntilExpiry(iso)
  return days <= 0 ? t('common.expiresToday') : t('common.expiresInDays', { count: days }, days)
}

// 從 App Store 匯入的服務商只有名稱、圖示與官網，獎勵說明要後台自己補
// （見 refcode-api 的 CreateImportedMerchant）。空字串直接 render 會變成一行
// 空白，看起來像卡片壞掉而不是「這家還沒有資訊」。
export function rewardText(desc: string): string {
  return desc || i18n.global.t('merchant.rewardPending')
}

// 一律走 code：後端的每個 code 都對應到單一句話（見 refcode-api 的 response.go），
// 這裡查得到就用譯文。查不到才退回後端那句中文 —— 那是後端加了新 code 但
// 語系檔還沒跟上，顯示中文至少比顯示 code 好。
// fallbackKey 是連不上後端（拿不到 ApiError）時要顯示的話。
export function apiErrorMessage(e: unknown, fallbackKey: string): string {
  const { t, te } = i18n.global

  if (!(e instanceof ApiError)) return t(fallbackKey)

  const key = `errors.${e.code}`
  return te(key) ? t(key) : e.message
}
