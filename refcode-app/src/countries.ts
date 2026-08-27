import { i18n } from './i18n'

// 帳號「所在地」的常用選項。這不是白名單 —— 後端只驗格式（refcode-api 的
// internal/geo），沒列在這裡的國家不會被擋，只是選單上沒有。
//
// 跟目錄的地區選單是兩回事，那份照實際有服務商的國家給（見 regionOptions）。
// 這份可以更寬：人住在日本是事實，不會因為目錄還沒有日本的服務商就不成立。
// 但反過來不行 —— 目錄有的國家這裡一定要有，否則紐西蘭、澳門的使用者
// 連「我住在這裡」都填不了。
const COUNTRY_CODES = [
  'TW', 'JP', 'HK', 'MO', 'SG', 'MY', 'KR', 'CN', 'US', 'GB', 'AU', 'NZ', 'CA',
]

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

// 地區選單的選項。照後端給的順序（服務商多的在前），不重新排序 ——
// 使用者要找的多半是自己那一區，而那通常也是服務商多的那幾個。
// 帶上家數：選之前就看得出哪一區有東西，不用選進去才發現是空的。
export function regionOptions(
  regions: { code: string; merchant_count: number }[],
): { code: string; label: string }[] {
  return regions.map((r) => ({
    code: r.code,
    label: `${countryName(r.code)}（${r.merchant_count}）`,
  }))
}

export function defaultCountry(): string {
  return DEFAULT_BY_LOCALE[currentLocale()] ?? ''
}

// 時區前綴 → 國家。只列目錄實際涵蓋得到的範圍（見 /v1/regions），
// 沒對到的時區就回空字串，交給呼叫端當「判不出來」處理。
//
// 用前綴比對而不是列完整時區名：America/Toronto、America/Vancouver、
// America/Edmonton⋯⋯ 加拿大一國就有六七個，而 IANA 每年都在增修。
const TZ_PREFIX_TO_COUNTRY: [string, string][] = [
  ['Asia/Taipei', 'TW'],
  ['Asia/Hong_Kong', 'HK'],
  ['Asia/Macau', 'MO'],
  ['Asia/Macao', 'MO'],
  ['Asia/Singapore', 'SG'],
  ['Pacific/Auckland', 'NZ'],
  ['Pacific/Chatham', 'NZ'],
  ['Australia/', 'AU'],
  // 北美要逐一列：America/ 底下同時有美國、加拿大與整個中南美洲，
  // 用 America/ 當前綴會把墨西哥、巴西全部誤判成美國。
  ['America/Toronto', 'CA'],
  ['America/Vancouver', 'CA'],
  ['America/Edmonton', 'CA'],
  ['America/Winnipeg', 'CA'],
  ['America/Halifax', 'CA'],
  ['America/St_Johns', 'CA'],
  ['America/Regina', 'CA'],
  ['America/Whitehorse', 'CA'],
  ['America/Yellowknife', 'CA'],
  ['America/Iqaluit', 'CA'],
  ['America/Moncton', 'CA'],
  ['America/Dawson_Creek', 'CA'],
  ['America/New_York', 'US'],
  ['America/Chicago', 'US'],
  ['America/Denver', 'US'],
  ['America/Los_Angeles', 'US'],
  ['America/Phoenix', 'US'],
  ['America/Anchorage', 'US'],
  ['America/Detroit', 'US'],
  ['America/Indiana/', 'US'],
  ['America/Kentucky/', 'US'],
  ['America/Boise', 'US'],
  ['America/Juneau', 'US'],
  ['America/Sitka', 'US'],
  ['America/Nome', 'US'],
  ['America/Adak', 'US'],
  ['America/Menominee', 'US'],
  ['America/North_Dakota/', 'US'],
  ['Pacific/Honolulu', 'US'],
]

// 比對順序是「長的優先」，否則 Australia/ 這種短前綴會先吃掉 Australia/Perth。
// 分成兩行不是排版偏好：接在陣列字面值後面的話，上面那個 [string, string][]
// 會變成 .sort() 回傳值的標註，字面值自己被推成 string[][]，兩邊對不起來。
TZ_PREFIX_TO_COUNTRY.sort((a, b) => b[0].length - a[0].length)

// 裝置所在地，用來當目錄地區篩選的初值 —— app 不像官網有 SSR 的包袱，
// 匿名使用者第一次打開就該看到自己這邊用得到的服務商。
//
// **這個值只拿來篩畫面，永遠不寫進 users.country。** 判錯的人要能在畫面上
// 自己改掉，而不是被悄悄記成他的所在地（見 db/migrations/00006_geo.sql）。
//
// 看時區不看語言：navigator.languages 給的是「介面想用什麼語言」，
// 跟人在哪裡是兩件事 —— 在台灣把手機設成英文的人不算少，那樣會被判成美國
// 或整個落空。時區則是裝置對「我現在在哪」的認知，換時區旅行會跟著變，
// 而且拿它不需要任何權限。
//
// 判不出來就回空字串（呼叫端會退回不篩）。使用者手動改過時區、或 VPN
// 配合改時區會判錯，但那跟 IP 判定會被 VPN 騙是同一級的問題，
// 只有 GPS 躲得掉，而那要位置權限。
export function deviceRegion(): string {
  try {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone
    if (!tz) return ''
    for (const [prefix, country] of TZ_PREFIX_TO_COUNTRY) {
      if (tz === prefix || tz.startsWith(prefix)) return country
    }
  } catch {
    // Intl 不完整的舊 WebView。判不出來就不篩，不要讓目錄整個掛掉。
  }
  return ''
}
