import type {
  AdminLoginResponse,
  AdminMerchant,
  Category,
  Merchant,
  MerchantInput,
  PendingCode,
  ReferralCode,
  ReviewAction,
} from './types'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:7802'

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

// 後端不發 refresh token 給後台（session 短、過期就重登），
// 所以這裡只存 access token，401 時交給呼叫端導回登入頁。
let accessToken: string | null = localStorage.getItem('refcode_admin_token')

export function setToken(token: string | null) {
  accessToken = token
  if (token) localStorage.setItem('refcode_admin_token', token)
  else localStorage.removeItem('refcode_admin_token')
}

export function getToken() {
  return accessToken
}

type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler = () => {}

export function setUnauthorizedHandler(fn: UnauthorizedHandler) {
  onUnauthorized = fn
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`)

  const res = await fetch(`${BASE_URL}${path}`, { ...init, headers })

  if (res.status === 401) {
    setToken(null)
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
  login(email: string, password: string) {
    return request<AdminLoginResponse>('/v1/admin/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
  },

  listPendingCodes(limit = 50, offset = 0) {
    return request<{ codes: PendingCode[]; total: number }>(
      `/v1/admin/codes/pending?limit=${limit}&offset=${offset}`,
    )
  },

  reviewCode(id: string, action: ReviewAction, reason: string) {
    return request<ReferralCode>(`/v1/admin/codes/${id}/review`, {
      method: 'POST',
      body: JSON.stringify({ action, reason }),
    })
  },

  listCategories() {
    return request<{ categories: Category[] }>('/v1/categories')
  },

  createCategory(input: { slug: string; name: string; sort_order: number }) {
    return request<Category>('/v1/admin/categories', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  // slug 改得動。舊網址不會死 —— 後端把舊的存進 category_slug_history，
  // 官網抓到舊 slug 會 301 轉到新的。
  updateCategory(id: string, input: { slug: string; name: string; sort_order: number }) {
    return request<Category>(`/v1/admin/categories/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    })
  },

  deleteCategory(id: string) {
    return request<void>(`/v1/admin/categories/${id}`, { method: 'DELETE' })
  },

  // 後台一律用這支：公開的 /v1/merchants 會過濾掉停用的服務商，
  // 而且缺 category_id、code_format_regex 這些編輯表單需要的欄位。
  listMerchants() {
    return request<{ merchants: AdminMerchant[] }>('/v1/admin/merchants')
  },

  createMerchant(input: MerchantInput) {
    return request<Merchant>('/v1/admin/merchants', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  // slug 改得動，舊網址由後端的 merchant_slug_history 撐著（官網會 301）。
  updateMerchant(id: string, input: MerchantInput) {
    return request<Merchant>(`/v1/admin/merchants/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    })
  },
}
