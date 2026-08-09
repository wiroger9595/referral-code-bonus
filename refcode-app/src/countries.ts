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

// 裝置本身的地區，用來當目錄地區篩選的初值 —— app 不像官網有 SSR 的包袱，
// 匿名使用者第一次打開就該看到自己這邊用得到的服務商。
//
// **這個值只拿來篩畫面，永遠不寫進 users.country。** 語言不等於所在地
// （見 db/migrations/00006_geo.sql），猜錯的人要能在畫面上自己改掉，
// 而不是被悄悄記成他的所在地。
//
// navigator.language 在 WebView 裡拿得到裝置的語言標籤（zh-TW、en-US）。
// 只有語言沒有地區的標籤（單純的 "en"）取不出 region，那就回空字串當「不篩」。
export function deviceRegion(): string {
  try {
    for (const tag of navigator.languages ?? [navigator.language]) {
      const region = new Intl.Locale(tag).region
      if (region) return region.toUpperCase()
    }
  } catch {
    // Intl.Locale 不存在的舊 WebView。拿不到就不篩，不要讓目錄整個掛掉。
  }
  return ''
}
