// 對應 refcode-api 的回傳。後端補上 OpenAPI spec 之後應該改成從 spec 產生。

// 官網只做得到 Google（網頁版的 Sign in with Apple 要另外申請 Services ID，
// 目前只有 app 那邊在用），但型別跟著後端的契約走。
export type OAuthProvider = 'google' | 'apple'

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: string
}

export interface User {
  id: string
  email: string
  display_name: string
  avatar_url: string | null
  email_verified: boolean
  // 註冊時自己選的所在地（ISO 3166-1 alpha-2）。null 代表沒填。
  country: string | null
  created_at: string
}

export interface AuthResponse {
  user: User
  tokens: TokenPair
}

export interface Category {
  id: string
  slug: string
  name: string
  sort_order: number
}

export interface MerchantSummary {
  id: string
  slug: string
  name: string
  logo_url: string | null
  signup_url: string
  reward_desc: string
  category_slug: string
  category_name: string
  active_code_count: number
  // 這家在哪些國家能用。空陣列代表不分地區。
  countries: string[]
}

export interface CodeItem {
  id: string
  // 要註冊才能拿到推薦碼：沒登入時後端不會把碼本身送過來，code 是 null、masked 是 true。
  // Googlebot 沒有 token，所以 SSR 給爬蟲看到的永遠是遮碼版——這是刻意的，
  // 服務商頁本身仍值得被收錄，但碼不該被索引成公開內容。
  code: string | null
  masked: boolean
  note: string
  owner_name: string
  owner_avatar_url: string | null
  quality_score: number
  worked_count: number
  failed_count: number
  expires_at: string
  created_at: string
}

export interface MerchantDetail {
  merchant: MerchantSummary
  codes: CodeItem[]
  total: number
}

export type ReportResult = 'worked' | 'failed' | 'invalid_code' | 'merchant_closed'
