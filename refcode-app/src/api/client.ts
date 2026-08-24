import { Preferences } from '@capacitor/preferences'

import type {
  AuthResponse,
  Category,
  CodeStats,
  CodeType,
  MerchantDetail,
  MerchantListResponse,
  MerchantSuggestion,
  MyCode,
  OAuthProvider,
  PopularTerm,
  ReportResult,
  TokenPair,
  User,
} from './types'

// 預設走 localhost：模擬器與瀏覽器跟這台機器共用網路，換 WiFi、IP 變了都不影響。
// 只有實機測試連不到 localhost，那時才要把 .env 的 VITE_API_BASE_URL 換成區網 IP
// （`./dev.sh android-ip` 會自動偵測並寫進去）。不要在這裡寫死某個 IP 當預設 ——
// 換個地方那個值就是死的，而且失效時的症狀是整個 app 連不上，很難一眼看出原因。
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

// crypto.randomUUID 只有 secure context 才有。iOS 的 WebView 跑在
// capacitor://localhost，那個 custom scheme 不算 secure context，randomUUID 會是
// undefined —— 直接呼叫就是 TypeError，而這行踩在 main.ts 的 top-level await 上，
// 一炸整個 app 就不 mount，畫面全白、連錯誤畫面都沒有。
// 只有全新安裝（Preferences 還沒存過 device id）才會走到這裡，所以很難重現。
function newDeviceId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  // 退路自己組一組 v4。getRandomValues 的支援範圍比 randomUUID 廣得多，
  // 兩個都沒有才退到 Math.random —— 這個值只用來匿名去重，不是安全用途。
  const bytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256)
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80

  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

// Preferences 是 async 的，所以 app 啟動時要先跑這個才能發請求。
export async function initTokens() {
  const [a, r, d] = await Promise.all([
    Preferences.get({ key: KEY_ACCESS }),
    Preferences.get({ key: KEY_REFRESH }),
    Preferences.get({ key: KEY_DEVICE }),
  ])
  accessToken = a.value
  refreshToken = r.value

  deviceId = d.value ?? newDeviceId()
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
  // FormData 的 Content-Type 要由瀏覽器帶 boundary 一起產生，自己設就送不出去了。
  if (!(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
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

  // 大頭照是上傳完後端直接寫回 users，所以回的是整份使用者資料，不用再打一次 PATCH。
  uploadAvatar(image: Blob) {
    const form = new FormData()
    form.append('file', image, 'avatar.jpg')
    return request<User>('/v1/me/avatar', { method: 'POST', body: form })
  },

  listCategories() {
    return request<{ categories: Category[] }>(`/v1/categories?lang=${apiLang}`)
  },

  getCategory(id: string) {
    return request<Category>(`/v1/categories/${id}?lang=${apiLang}`)
  },

  // commit 代表這是使用者確定要搜的（按下搜尋、點了熱門或歷史），後端才會把它
  // 計進熱門榜。逐字輸入不要帶，不然打一次「台新銀行」會在榜上留下四筆前綴。
  // region 是「只看這一區的服務商」，'all' 代表不篩。不帶的話後端會退回
  // 登入者填的所在地（匿名就是不篩），但 app 一律自己算好再送 ——
  // 匿名使用者也要吃得到裝置地區。
  listMerchants(params: { category?: string; q?: string; commit?: boolean; region?: string } = {}) {
    const qs = new URLSearchParams({ limit: '50', lang: apiLang })
    if (params.category) qs.set('category', params.category)
    if (params.q) qs.set('q', params.q)
    if (params.commit) qs.set('commit', '1')
    if (params.region) qs.set('region', params.region)
    return request<MerchantListResponse>(`/v1/merchants?${qs}`)
  },

  listPopularSearches() {
    return request<{ terms: PopularTerm[] }>(`/v1/search/popular?lang=${apiLang}`)
  },

  getMerchant(slug: string) {
    return request<MerchantDetail>(`/v1/merchants/${slug}?lang=${apiLang}`)
  },

  listMyCodes() {
    return request<{ codes: MyCode[] }>('/v1/me/codes')
  },

  // expires_at 傳 null 是「永久有效」，後端會存成 NULL。
  createCode(input: {
    merchant_id: string
    code: string
    note: string
    expires_at: string | null
    code_type: CodeType
  }) {
    return request<MyCode>('/v1/codes', { method: 'POST', body: JSON.stringify(input) })
  },

  // 提報一家目錄裡沒有的平台。這支不會建立任何公開的東西，只是把名字送進
  // 後台的審核佇列；通過之後那家才會出現在服務商清單裡。
  suggestMerchant(input: { name: string; signup_url: string; note: string }) {
    return request<MerchantSuggestion>('/v1/merchant-suggestions', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  // 下架不是刪除：碼會留在「我的推薦碼」裡標成已下架，公開列表看不到。
  // 回的是 referral_codes 那一列，沒有服務商欄位，所以只取得到狀態。
  disableCode(id: string) {
    return request<Pick<MyCode, 'id' | 'status'>>(`/v1/codes/${id}/disable`, { method: 'POST' })
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
