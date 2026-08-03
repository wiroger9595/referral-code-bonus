// 註冊表單的所在地選項。這是「常用選項」不是白名單 —— 後端只驗格式
// （refcode-api 的 internal/geo），沒列在這裡的國家不會被擋，只是選單上沒有。
const COUNTRY_CODES = ['TW', 'JP', 'HK', 'SG', 'MY', 'KR', 'CN', 'US', 'GB', 'AU', 'CA']

// 介面語言對應的預設所在地。只是幫使用者少點一次，猜錯他自己會改 ——
// 所以不能反過來用語言直接當所在地存進資料庫。
const DEFAULT_BY_LOCALE: Record<string, string> = {
  'zh-TW': 'TW',
  ja: 'JP',
}

export function useCountries() {
  const { locale } = useI18n()

  // 國家名稱交給 Intl，不進語系檔：三種語言 × 十幾個國家的譯名沒必要自己維護，
  // 而且 Node 端也有完整 ICU，SSR 跟 client 出來的字一樣。
  const options = computed(() => {
    const names = new Intl.DisplayNames([locale.value], { type: 'region' })
    return COUNTRY_CODES.map((code) => ({ code, label: names.of(code) ?? code })).sort((a, b) =>
      a.label.localeCompare(b.label, locale.value),
    )
  })

  const defaultCountry = computed(() => DEFAULT_BY_LOCALE[locale.value] ?? '')

  function countryName(code: string): string {
    return new Intl.DisplayNames([locale.value], { type: 'region' }).of(code) ?? code
  }

  return { options, defaultCountry, countryName }
}
