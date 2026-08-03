// 對應 refcode-api 的回傳。四個 repo 各自獨立，這份是手寫的；
// 後端補上 OpenAPI spec 之後應該改成從 spec 產生，避免兩邊分岔。

export type CodeStatus = 'pending' | 'active' | 'rejected' | 'expired' | 'disabled'
export type AdminRole = 'owner' | 'reviewer'
export type ReviewAction = 'approve' | 'reject' | 'disable' | 'restore'

export interface AdminUser {
  id: string
  email: string
  display_name: string
  role: AdminRole
}

export interface AdminLoginResponse {
  access_token: string
  expires_at: string
  admin: AdminUser
}

export interface Category {
  id: string
  slug: string
  name: string
  sort_order: number
  created_at: string
}

export interface Merchant {
  id: string
  slug: string
  name: string
  category_id: string
  logo_url: string | null
  signup_url: string
  reward_desc: string
  code_format_regex: string | null
  is_active: boolean
  // 適用國家（ISO 3166-1 alpha-2）。空陣列代表不分地區。
  countries: string[]
  created_at: string
  updated_at: string
}

// 目錄列表回的是帶統計的版本，欄位跟 Merchant 不完全一樣。
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
}

// 後台列表：包含停用的服務商，欄位比公開的 MerchantSummary 齊全。
export interface AdminMerchant extends Merchant {
  category_slug: string
  category_name: string
  active_code_count: number
}

export interface PendingCode {
  id: string
  user_id: string
  merchant_id: string
  code: string
  note: string
  status: CodeStatus
  expires_at: string
  quality_score: number
  impressions: number
  created_at: string
  activated_at: string | null
  updated_at: string
  merchant_slug: string
  merchant_name: string
  code_format_regex: string | null
  owner_email: string
  owner_name: string
}

export interface ReferralCode {
  id: string
  user_id: string
  merchant_id: string
  code: string
  note: string
  status: CodeStatus
  expires_at: string
  quality_score: number
  impressions: number
  created_at: string
  activated_at: string | null
  updated_at: string
}

export interface MerchantInput {
  slug: string
  name: string
  category_id: string
  logo_url: string | null
  signup_url: string
  reward_desc: string
  code_format_regex: string | null
  is_active?: boolean
  countries: string[]
}
