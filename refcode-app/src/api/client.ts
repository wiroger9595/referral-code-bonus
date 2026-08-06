import { Preferences } from '@capacitor/preferences'

import type {
  AuthResponse,
  Category,
  CodeStats,
  MerchantDetail,
  MerchantSummary,
  MyCode,
  OAuthProvider,
  ReportResult,
  TokenPair,
  User,
} from './types'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:7802'

const KEY_ACCESS = 'refcode_access_token'
const KEY_REFRESH = 'refcode_refresh_token'
const KEY_DEVICE = 'refcode_device_id'

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

let accessToken: string | null = null
let refreshToken: string | null = null
let deviceId = ''

// Preferences 是 async 的，所以 app 啟動時要先跑這個才能發請求。
export async function initTokens() {
  const [a, r, d] = await Promise.all([
    Preferences.get({ key: KEY_ACCESS }),
    Preferences.get({ key: KEY_REFRESH }),
    Preferences.get({ key: KEY_DEVICE }),
  ])
  accessToken = a.value
  refreshToken = r.value

  deviceId = d.value ?? crypto.randomUUID()
  if (!d.value) await Preferences.set({ key: KEY_DEVICE, value: deviceId })
}

export async function saveTokens(pair: TokenPair) {
  accessToken = pair.access_token
  refreshToken = pair.refresh_token
  await Promise.all([
    Preferences.set({ key: KEY_ACCESS, value: pair.access_token }),
    Preferences.set({ key: KEY_REFRESH, value: pair.refresh_token }),
  ])
}

export async function clearTokens() {
  accessToken = null
  refreshToken = null
  await Promise.all([
    Preferences.remove({ key: KEY_ACCESS }),
    Preferences.remove({ key: KEY_REFRESH }),
  ])
}

export function hasSession() {
  return accessToken !== null
}

// 分類名與獎勵說明是資料庫欄位，要靠 ?lang= 決定後端回哪個語言（見 refcode-api 的
// pickLang）。這裡用 setter 而不是直接 import i18n —— i18n/index.ts 已經 import 了
// 這個檔（拿 ApiError），反向再 import 一次就成了循環相依。
let apiLang = 'zh'

export function setApiLang(locale: string) {
  apiLang = locale.startsWith('zh') ? 'zh' : locale
}

let onUnauthorized: () => void = () => {}

export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

// refresh token 是 rotating 的：同時發兩個換發請求，第二個會被後端當成
// token 重用而撤銷整族。所以併發的 401 必須共用同一個 in-flight 換發。
let refreshing: Promise<boolean> | null = null

async function refreshSession(): Promise<boolean> {
  if (!refreshToken) return false

  refreshing ??= (async () => {
    try {
      const res = await fetch(`${BASE_URL}/v1/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
      if (!res.ok) {
        await clearTokens()
        return false
      }
      await saveTokens((await res.json()) as TokenPair)
      return true
    } catch {
      return false
    } finally {
      refreshing = null
    }
  })()

  return refreshing
}

async function request<T>(path: string, init: RequestInit = {}, allowRetry = true): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  headers.set('X-Device-ID', deviceId)
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`)

  const res = await fetch(`${BASE_URL}${path}`, { ...init, headers })

  if (res.status === 401 && allowRetry && refreshToken) {
    if (await refreshSession()) return request<T>(path, init, false)
    onUnauthorized()
  }

  if (res.status === 204) return undefined as T

  const body = await res.json().catch(() => null)

  if (!res.ok) {
    const detail = body?.error
    throw new ApiError(
      res.status,
      detail?.code ?? 'unknown',
      detail?.message ?? '連線失敗，請稍後再試',
    )
  }
  return body as T
}

export const api = {
  // country 是選填（空字串＝不指定），後端拿它把在地的服務商排前面。
  register(email: string, password: string, displayName: string, country: string) {
    return request<AuthResponse>('/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, display_name: displayName, country }),
    })
  },

  login(email: string, password: string) {
    return request<AuthResponse>('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
  },

  // 原生的 Google / Apple 登入 plugin 還沒接（需要真實的 client id 才有辦法測），
  // 拿到 id_token 之後打這支就能完成登入。
  // country 只有在這次要建新帳號時後端才會採用，已經有帳號的人會忽略它。
  oauthLogin(provider: OAuthProvider, idToken: string, country = '') {
    return request<AuthResponse>('/v1/auth/oauth', {
      method: 'POST',
      body: JSON.stringify({ provider, id_token: idToken, country }),
    })
  },

  // 不管這個 email 有沒有註冊過都回 204，前端不能拿它來判斷帳號存不存在
  // （後端刻意不區分，避免被當成帳號探測工具）。
  // locale 決定信件用哪種語言 —— 信是後端寄的，前端沒有機會翻譯。
  forgotPassword(email: string, locale: string) {
    return request<void>('/v1/auth/password/forgot', {
      method: 'POST',
      body: JSON.stringify({ email, locale }),
    })
  },

  // 驗證碼對了就直接回一組新 token，不用再叫使用者登入一次。
  resetPassword(email: string, code: string, password: string) {
    return request<AuthResponse>('/v1/auth/password/reset', {
      method: 'POST',
      body: JSON.stringify({ email, code, password }),
    })
  },

  // 刪除帳號。confirm 要是帳號本身的 email，後端會比對 —— 這是防誤觸，
  // 不是驗證身分（身分靠 token）。
  deleteAccount(confirm: string) {
    return request<void>('/v1/me', { method: 'DELETE', body: JSON.stringify({ confirm }) })
  },

  logout() {
    return request<void>('/v1/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
  },

  me() {
    return request<User>('/v1/me')
  },

  // 後端這支是整份覆寫，所以顯示名稱要一起送 —— 只想改所在地也一樣。
  updateMe(input: { display_name: string; avatar_url: string | null; country: string }) {
    return request<User>('/v1/me', { method: 'PATCH', body: JSON.stringify(input) })
  },

  listCategories() {
    return request<{ categories: Category[] }>(`/v1/categories?lang=${apiLang}`)
  },

  listMerchants(params: { category?: string; q?: string } = {}) {
    const qs = new URLSearchParams({ limit: '50', lang: apiLang })
    if (params.category) qs.set('category', params.category)
    if (params.q) qs.set('q', params.q)
    return request<{ merchants: MerchantSummary[] }>(`/v1/merchants?${qs}`)
  },

  getMerchant(slug: string) {
    return request<MerchantDetail>(`/v1/merchants/${slug}?lang=${apiLang}`)
  },

  listMyCodes() {
    return request<{ codes: MyCode[] }>('/v1/me/codes')
  },

  createCode(input: { merchant_id: string; code: string; note: string; expires_at: string }) {
    return request<MyCode>('/v1/codes', { method: 'POST', body: JSON.stringify(input) })
  },

  codeStats(id: string, days = 30) {
    return request<CodeStats>(`/v1/codes/${id}/stats?days=${days}`)
  },

  track(codeId: string, eventType: 'click' | 'copy') {
    return request<void>('/v1/events', {
      method: 'POST',
      body: JSON.stringify({ code_id: codeId, event_type: eventType }),
    })
  },

  report(codeId: string, result: ReportResult) {
    return request<void>(`/v1/codes/${codeId}/reports`, {
      method: 'POST',
      body: JSON.stringify({ result }),
    })
  },
}
