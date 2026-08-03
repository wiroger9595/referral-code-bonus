import { i18n } from './i18n'

// 所在地選單的常用選項。這不是白名單 —— 後端只驗格式（refcode-api 的 internal/geo），
// 沒列在這裡的國家不會被擋，只是選單上沒有。
const COUNTRY_CODES = ['TW', 'JP', 'HK', 'SG', 'MY', 'KR', 'CN', 'US', 'GB', 'AU', 'CA']

// 介面語言對應的預設所在地，只是幫使用者少點一次；猜錯他自己會改，
// 所以不能反過來拿語言當所在地存進資料庫。
const DEFAULT_BY_LOCALE: Record<string, string> = {
  'zh-TW': 'TW',
  ja: 'JP',
}

function currentLocale(): string {
  return i18n.global.locale.value
}

// 國家名稱交給 Intl，不進語系檔：三種語言 × 十幾個國家的譯名沒必要自己維護。
export function countryName(code: string): string {
  return new Intl.DisplayNames([currentLocale()], { type: 'region' }).of(code) ?? code
}

export function countryOptions(): { code: string; label: string }[] {
  const locale = currentLocale()
  return COUNTRY_CODES.map((code) => ({ code, label: countryName(code) })).sort((a, b) =>
    a.label.localeCompare(b.label, locale),
  )
}

export function defaultCountry(): string {
  return DEFAULT_BY_LOCALE[currentLocale()] ?? ''
}
