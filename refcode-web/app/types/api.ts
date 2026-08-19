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

// 上架的碼有兩種來源：自己的推薦碼（雙方各拿獎勵），或手上的折扣碼（只有使用的人拿到折扣）。
// 兩種的欄位一模一樣，折扣碼的優惠內容寫在 note 裡。
export type CodeType = 'referral' | 'discount'

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
  category_id: string
  category_name: string
  active_code_count: number
  // 這家最快到期的那個碼。沒有可用的碼時是 null。
  soonest_expires_at: string | null
  // 這家在哪些國家能用。空陣列代表不分地區。
  countries: string[]
  // 這家收哪幾種碼。沒有推薦計畫的服務商只會有 discount。
  allowed_code_types: CodeType[]
}

// 搜不到東西時後端給的「你是不是要找」。有結果時是空陣列。
export interface SearchSuggestion {
  slug: string
  name: string
}

// /v1/merchants 的回傳。total 是套用 limit 之前的總筆數，
// suggestions 只有帶了 ?q= 而且一筆都沒搜到時才有東西。
export interface MerchantListResponse {
  merchants: MerchantSummary[]
  total: number
  suggestions?: SearchSuggestion[]
}

export interface PopularTerm {
  term: string
  hits: number
}

export interface CodeItem {
  id: string
  // 要註冊才能拿到推薦碼：沒登入時後端不會把碼本身送過來，code 是 null、masked 是 true。
  // Googlebot 沒有 token，所以 SSR 給爬蟲看到的永遠是遮碼版——這是刻意的，
  // 服務商頁本身仍值得被收錄，但碼不該被索引成公開內容。
  code: string | null
  masked: boolean
  note: string
  code_type: CodeType
  owner_name: string
  owner_avatar_url: string | null
  quality_score: number
  worked_count: number
  failed_count: number
  // null 代表永久有效，沒有到期日。
  expires_at: string | null
  created_at: string
}

export interface MerchantDetail {
  merchant: MerchantSummary
  codes: CodeItem[]
  total: number
}

export type ReportResult = 'worked' | 'failed' | 'invalid_code' | 'merchant_closed'

// 對應 referral_codes.status 的 CHECK 約束（refcode-api 的 00003_referral_codes.sql）。
export type CodeStatus = 'pending' | 'active' | 'rejected' | 'expired' | 'disabled'

// /v1/me/codes 的一列。跟公開列表的 CodeItem 形狀不一樣 ——
// 這裡碼一定看得到（是自己的），而且帶了服務商資訊與只有自己看得到的狀態。
export interface MyCode {
  id: string
  merchant_id: string
  code: string
  note: string
  status: CodeStatus
  code_type: CodeType
  // null 代表永久有效，沒有到期日。
  expires_at: string | null
  quality_score: number
  impressions: number
  created_at: string
  merchant_slug: string
  merchant_name: string
  merchant_logo_url: string | null
  // 最近一次被拒的理由，沒被拒過是空字串。被拒之後又被恢復的碼也會留著上一次的
  // 理由，所以要連 status 一起看才知道現在該不該顯示。
  reject_reason: string
}
