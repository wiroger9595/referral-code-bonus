// Tawk.to 即時客服。只在 client 掛：script 直接操作 DOM，SSR 階段執行沒意義。
// TAWK_PROPERTY_ID／TAWK_WIDGET_ID 沒填就整個不載入，本機開發不用先申請帳號。
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

export default defineNuxtPlugin(() => {
  const { public: cfg } = useRuntimeConfig()
  if (!cfg.tawkPropertyId || !cfg.tawkWidgetId) return

  window.Tawk_API = window.Tawk_API || {}
  window.Tawk_LoadStart = new Date()

  // 登入的人讓客服看得到是誰在問，不用對方自己報信箱。widget 是非同步載入的，
  // 掛在 onLoad 裡才能保證 setAttributes 已經存在。
  const { user } = useAuth()
  window.Tawk_API.onLoad = () => {
    if (user.value) {
      window.Tawk_API?.setAttributes?.(
        { name: user.value.display_name, email: user.value.email },
        () => {},
      )
    }
  }
  // 已經載入完之後才登入／換帳號的情況：widget 還在，直接補送一次屬性。
  watch(user, (u) => {
    if (u) window.Tawk_API?.setAttributes?.({ name: u.display_name, email: u.email }, () => {})
  })

  const script = document.createElement('script')
  script.async = true
  script.src = `https://embed.tawk.to/${cfg.tawkPropertyId}/${cfg.tawkWidgetId}`
  script.charset = 'UTF-8'
  script.setAttribute('crossorigin', '*')
  document.head.appendChild(script)
})
