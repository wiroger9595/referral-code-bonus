// Tawk.to 即時客服。跟 purchases.ts／social.ts 同一套邏輯：沒設定 property/widget id
// 就整個不載入，本機開發不用先申請帳號。
const PROPERTY_ID = import.meta.env.VITE_TAWK_PROPERTY_ID ?? ''
const WIDGET_ID = import.meta.env.VITE_TAWK_WIDGET_ID ?? ''

export const tawkAvailable = PROPERTY_ID !== '' && WIDGET_ID !== ''

type TawkAPI = {
  setAttributes?: (attrs: Record<string, string>, callback: (err: unknown) => void) => void
  onLoad?: () => void
}

declare global {
  interface Window {
    Tawk_API?: TawkAPI
    Tawk_LoadStart?: Date
  }
}

let loaded = false

// 掛進 App 這個 WebView 裡的客服泡泡。全體使用者都看得到，不分有沒有登入 ——
// 還沒登入的人一樣會遇到問題（例如不知道怎麼註冊）。
export function initTawk() {
  if (!tawkAvailable || loaded) return
  loaded = true

  window.Tawk_API = window.Tawk_API || {}
  window.Tawk_LoadStart = new Date()

  const script = document.createElement('script')
  script.async = true
  script.src = `https://embed.tawk.to/${PROPERTY_ID}/${WIDGET_ID}`
  script.charset = 'UTF-8'
  script.setAttribute('crossorigin', '*')
  document.head.appendChild(script)
}

// 登入的人讓客服看得到是誰在問，不用對方自己報信箱。widget 是非同步載入的
// script，掛載當下 setAttributes 不一定已經存在 —— 還沒好的話接到 onLoad 後面，
// 不能直接吞掉這次呼叫。
export function identifyTawkUser(name: string, email: string) {
  if (!tawkAvailable) return
  const api = window.Tawk_API
  if (!api) return

  if (api.setAttributes) {
    api.setAttributes({ name, email }, () => {})
    return
  }
  const prevOnLoad = api.onLoad
  api.onLoad = () => {
    prevOnLoad?.()
    api.setAttributes?.({ name, email }, () => {})
  }
}
