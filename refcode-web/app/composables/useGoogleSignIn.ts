// Google Identity Services 的網頁流程：Google 直接回一個簽好的 ID token
// （callback 裡的 credential），拿去打後端的 /v1/auth/oauth 就完成登入。
// 官網沒有自己的 OAuth redirect endpoint，也不需要 client secret。

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize(config: {
            client_id: string
            callback: (res: { credential: string }) => void
            auto_select?: boolean
          }): void
          renderButton(el: HTMLElement, options: Record<string, unknown>): void
        }
      }
    }
  }
}

let scriptPromise: Promise<void> | null = null
let scriptLocale = ''

// 按鈕的語言是靠載入 script 時的 hl 參數決定，renderButton() 的 locale 欄位不會生效
// （試過了：日英頁面上的按鈕文字還是中文）。locale 變了要重新載入一次 script。
function loadScript(locale: string): Promise<void> {
  if (scriptPromise && scriptLocale !== locale) {
    scriptPromise = null
  }
  scriptLocale = locale

  scriptPromise ??= new Promise<void>((resolve, reject) => {
    const s = document.createElement('script')
    s.src = `https://accounts.google.com/gsi/client?hl=${encodeURIComponent(locale)}`
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => {
      scriptPromise = null // 失敗就讓下一次重試，不要卡死在壞掉的 promise 上。
      reject(new Error('Google 登入元件載入失敗'))
    }
    document.head.appendChild(s)
  })
  return scriptPromise
}

/**
 * 把 Google 官方的登入按鈕掛到回傳的 target 上。
 * 沒設定 client id 就 enabled = false，呼叫端整塊不要顯示 ——
 * 半設定的按鈕按下去只會拿到看不懂的 Google 錯誤。
 */
export function useGoogleSignIn(onCredential: (idToken: string) => void) {
  const { public: cfg } = useRuntimeConfig()
  const { locale } = useI18n()

  const target = ref<HTMLElement | null>(null)
  const failed = ref(false)
  const enabled = computed(() => cfg.googleClientId !== '')

  onMounted(async () => {
    if (!enabled.value || !target.value) return

    try {
      await loadScript(locale.value)
      window.google!.accounts.id.initialize({
        client_id: cfg.googleClientId,
        callback: (res) => onCredential(res.credential),
        // 不要自動用上次的帳號登入 —— 登出後回到登入頁又被自動登回去，
        // 使用者會以為登出沒生效。
        auto_select: false,
      })
      window.google!.accounts.id.renderButton(target.value, {
        theme: 'outline',
        size: 'large',
        shape: 'rectangular',
        text: 'continue_with',
        width: 320,
      })
    } catch {
      failed.value = true
    }
  })

  return { target, enabled, failed }
}
