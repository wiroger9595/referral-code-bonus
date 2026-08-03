import type { AuthResponse, OAuthProvider, TokenPair, User } from '~/types/api'

const ACCESS_COOKIE = 'refcode_access_token'
const REFRESH_COOKIE = 'refcode_refresh_token'

// 對齊後端的 REFRESH_TOKEN_TTL（720h）。access token 自己帶到期時間，
// cookie 不需要跟著它的 15 分鐘 —— 過期會拿到 401，換發完再重試一次。
const SESSION_MAX_AGE = 30 * 24 * 60 * 60

// 一律走 code：後端的每個 code 都對應到單一句話（見 refcode-api 的 response.go），
// 這裡查得到就用譯文。查不到才退回後端那句中文 —— 那代表後端加了新 code 但
// 語系檔還沒跟上，顯示中文至少比顯示 code 好。
// 要在 setup 階段取得 t，回傳的函式才能在 catch 裡（已經脫離 setup context）安全呼叫。
export function useApiError() {
  const { t, te } = useI18n()

  return function apiErrorMessage(e: unknown): string {
    const detail = (e as { data?: { error?: { code?: string; message?: string } } }).data?.error

    const key = `errors.${detail?.code}`
    if (detail?.code && te(key)) return t(key)
    return detail?.message ?? t('errors.network')
  }
}

function statusOf(e: unknown): number {
  return (e as { response?: { status?: number } }).response?.status ?? 0
}

export function useAuth() {
  const { public: cfg } = useRuntimeConfig()

  // 用 cookie 而不是 localStorage：SSR 階段就要知道有沒有登入，
  // 否則 header 會先 render 成未登入、hydrate 之後才跳成已登入。
  const cookieOpts = {
    path: '/',
    maxAge: SESSION_MAX_AGE,
    sameSite: 'lax' as const,
    secure: !import.meta.dev,
  }
  const accessToken = useCookie<string | null>(ACCESS_COOKIE, cookieOpts)
  const refreshToken = useCookie<string | null>(REFRESH_COOKIE, cookieOpts)

  const user = useState<User | null>('auth:user', () => null)
  // SSR 確認過的結果會經由 payload 帶到 client，hydrate 後不要再打一次 /v1/me。
  const restored = useState<boolean>('auth:restored', () => false)

  const isLoggedIn = computed(() => user.value !== null)

  function applySession(res: AuthResponse) {
    accessToken.value = res.tokens.access_token
    refreshToken.value = res.tokens.refresh_token
    user.value = res.user
    restored.value = true
  }

  function clearSession() {
    accessToken.value = null
    refreshToken.value = null
    user.value = null
    restored.value = true
  }

  // refresh token 是 rotating 的，換發成功要把新的一組寫回去，
  // 舊的再用一次會被後端當成重用而撤銷整族。
  async function refreshSession(): Promise<boolean> {
    if (!refreshToken.value) return false
    try {
      const pair = await $fetch<TokenPair>('/v1/auth/refresh', {
        baseURL: cfg.apiBase,
        method: 'POST',
        body: { refresh_token: refreshToken.value },
      })
      accessToken.value = pair.access_token
      refreshToken.value = pair.refresh_token
      return true
    } catch {
      clearSession()
      return false
    }
  }

  async function authedFetch<T>(
    path: string,
    opts: { method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'; body?: unknown } = {},
    allowRetry = true,
  ): Promise<T> {
    try {
      return await $fetch<T>(path, {
        baseURL: cfg.apiBase,
        ...opts,
        headers: accessToken.value ? { Authorization: `Bearer ${accessToken.value}` } : {},
      })
    } catch (e) {
      if (allowRetry && statusOf(e) === 401 && (await refreshSession())) {
        return authedFetch<T>(path, opts, false)
      }
      throw e
    }
  }

  // 沒有 cookie 就不打 API —— 站上絕大多數流量是沒登入的搜尋訪客，
  // 每個 SSR 都多一次往返只是拖慢首頁。
  async function restore() {
    if (restored.value) return
    if (!accessToken.value && !refreshToken.value) {
      restored.value = true
      return
    }
    try {
      user.value = await authedFetch<User>('/v1/me')
    } catch {
      clearSession()
    } finally {
      restored.value = true
    }
  }

  async function login(email: string, password: string) {
    applySession(
      await $fetch<AuthResponse>('/v1/auth/login', {
        baseURL: cfg.apiBase,
        method: 'POST',
        body: { email, password },
      }),
    )
  }

  // country 是選填，空字串代表不指定；後端拿它把在地的服務商排前面。
  async function register(
    email: string,
    password: string,
    displayName: string,
    country: string,
  ) {
    applySession(
      await $fetch<AuthResponse>('/v1/auth/register', {
        baseURL: cfg.apiBase,
        method: 'POST',
        body: { email, password, display_name: displayName, country },
      }),
    )
  }

  // provider 簽的 ID token 交給後端驗（比對 iss 與 aud，見 refcode-api 的
  // internal/auth/oidc.go）。這裡不解 token，client 端解出來的 claim 不可信。
  // country 只有在這次社群登入是「第一次、要建新帳號」時才會被採用，
  // 已經有帳號的人後端會忽略它 —— 不該讓一次登入覆寫掉他自己設過的所在地。
  async function loginWithProvider(provider: OAuthProvider, idToken: string, country = '') {
    applySession(
      await $fetch<AuthResponse>('/v1/auth/oauth', {
        baseURL: cfg.apiBase,
        method: 'POST',
        body: { provider, id_token: idToken, country },
      }),
    )
  }

  // 不管這個 email 有沒有註冊過都回 204 —— 後端刻意不區分，避免這支被當成
  // 「哪些 email 有註冊」的查詢工具，所以前端也不能從結果推論帳號存不存在。
  // locale 決定信件用哪種語言 —— 信是後端寄的，前端沒有機會翻譯。
  async function forgotPassword(email: string, locale: string) {
    await $fetch('/v1/auth/password/forgot', {
      baseURL: cfg.apiBase,
      method: 'POST',
      body: { email, locale },
    })
  }

  // 驗證碼對了後端會直接發一組新 token，所以重設完就是已登入狀態。
  async function resetPassword(email: string, code: string, password: string) {
    applySession(
      await $fetch<AuthResponse>('/v1/auth/password/reset', {
        baseURL: cfg.apiBase,
        method: 'POST',
        body: { email, code, password },
      }),
    )
  }

  // 刪除帳號。Play 要求提供一個不必安裝 app 就能送出刪除請求的入口，
  // 這頁就是那個入口 —— 所以它必須能獨立完成整件事，不能只留一個說明。
  async function deleteAccount(confirm: string) {
    await authedFetch<void>('/v1/me', { method: 'DELETE', body: { confirm } })
    clearSession()
  }

  async function logout() {
    if (refreshToken.value) {
      try {
        await $fetch('/v1/auth/logout', {
          baseURL: cfg.apiBase,
          method: 'POST',
          body: { refresh_token: refreshToken.value },
        })
      } catch {
        // 後端撤銷失敗也要讓本機登出，不然使用者會卡在登不出的狀態。
      }
    }
    clearSession()
  }

  // 目錄類的請求帶上它，後端才知道要不要套地區排序。這幾支是 optionalUser，
  // token 過期也只是拿到不分地區的排序，不會整頁失敗。
  function authHeaders(): Record<string, string> {
    return accessToken.value ? { Authorization: `Bearer ${accessToken.value}` } : {}
  }

  return {
    user,
    isLoggedIn,
    restore,
    login,
    register,
    forgotPassword,
    resetPassword,
    loginWithProvider,
    deleteAccount,
    logout,
    authHeaders,
  }
}
