import type {
  AdminCodeItem,
  AdminCodeStatus,
  AdminLoginResponse,
  AdminMerchant,
  AdminUserItem,
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

// 檔案上傳走 multipart，不能用 request()——那支固定送 JSON。
// 401 處理跟 request() 各自寫一份，換取這裡不用硬把 body 型別塞進共用函式。
async function uploadImage(file: File, folder: 'merchants' | 'categories'): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  form.append('folder', folder)

  const headers = new Headers()
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`)

  const res = await fetch(`${BASE_URL}/v1/admin/uploads/image`, {
    method: 'POST',
    headers,
    body: form,
  })

  if (res.status === 401) {
    setToken(null)
    onUnauthorized()
  }

  const body = await res.json().catch(() => null)

  if (!res.ok) {
    const detail = body?.error
    throw new ApiError(
      res.status,
      detail?.code ?? 'unknown',
      detail?.message ?? '上傳失敗，請稍後再試',
    )
  }
  return body.url as string
}

export const api = {
  uploadImage,

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

  // 已上架的碼（不含 pending）。後端把有負面回報的排在最前面，
  // 所以不翻頁也看得到該處理的那些。
  listCodes(
    opts: { status?: AdminCodeStatus | null; q?: string; limit?: number; offset?: number } = {},
  ) {
    const params = new URLSearchParams({
      limit: String(opts.limit ?? 50),
      offset: String(opts.offset ?? 0),
    })
    if (opts.status) params.set('status', opts.status)
    if (opts.q) params.set('q', opts.q)
    return request<{ codes: AdminCodeItem[]; total: number }>(`/v1/admin/codes?${params}`)
  },

  // 被系統自動打掉、還沒有人複核的碼。複核過（恢復或維持下架）就會離開這份清單。
  listAutoDisabledCodes(limit = 50, offset = 0) {
    return request<{ codes: AdminCodeItem[]; total: number }>(
      `/v1/admin/codes/auto-disabled?limit=${limit}&offset=${offset}`,
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

  createCategory(input: { name: string; name_en: string; name_ja: string; sort_order: number; image_url: string | null }) {
    return request<Category>('/v1/admin/categories', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  // 分類沒有 slug：網址與 ?category= 篩選都用 id。
  updateCategory(id: string, input: { name: string; name_en: string; name_ja: string; sort_order: number; image_url: string | null }) {
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

  // 服務商的 slug 改得動，但舊網址不會轉址，改了就是死連結。
  updateMerchant(id: string, input: MerchantInput) {
    return request<Merchant>(`/v1/admin/merchants/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    })
  },

  listUsers(q = '', limit = 50, offset = 0) {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (q) params.set('q', q)
    return request<{ users: AdminUserItem[]; total: number }>(`/v1/admin/users?${params}`)
  },

  // expiresAt 是 null 代表永久授權（promotional）。
  grantPro(id: string, expiresAt: string | null) {
    return request<void>(`/v1/admin/users/${id}/pro`, {
      method: 'POST',
      body: JSON.stringify({ expires_at: expiresAt }),
    })
  },

  revokePro(id: string) {
    return request<void>(`/v1/admin/users/${id}/pro`, { method: 'DELETE' })
  },
}
