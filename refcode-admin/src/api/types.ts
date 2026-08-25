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
  name: string
  // 譯文。null 代表還沒填，公開站會退回中文那份。
  name_en: string | null
  name_ja: string | null
  sort_order: number
  image_url: string | null
  created_at: string
}

// 上架的碼有兩種來源：使用者自己的推薦碼（雙方各拿獎勵），
// 或手上的折扣碼（只有使用的人拿到折扣）。
// 兩種的欄位一模一樣，折扣碼的優惠內容寫在 note 裡。
export type CodeType = 'referral' | 'discount'

export interface Merchant {
  id: string
  slug: string
  name: string
  category_id: string
  logo_url: string | null
  signup_url: string
  reward_desc: string
  // 獎勵說明的譯文。null 代表還沒填，公開站會退回中文那份。
  // 服務商名沒有譯文欄位 —— 那是品牌名，不翻。
  reward_desc_en: string | null
  reward_desc_ja: string | null
  code_format_regex: string | null
  // 折扣碼的格式規則。跟推薦碼分開驗 —— 推薦碼多半是系統發的固定格式，
  // 折扣碼是行銷活動字串，共用一條會把其中一種全部誤擋。
  discount_code_format_regex: string | null
  // 這家收哪幾種碼。至少會有一種；沒有推薦計畫的服務商只勾折扣碼。
  allowed_code_types: CodeType[]
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
  category_name: string
  active_code_count: number
}

// 後台列表：包含停用的服務商，欄位比公開的 MerchantSummary 齊全。
export interface AdminMerchant extends Merchant {
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
  code_type: CodeType
  // null 代表永久有效，沒有到期日。
  expires_at: string | null
  quality_score: number
  impressions: number
  created_at: string
  activated_at: string | null
  updated_at: string
  merchant_slug: string
  merchant_name: string
  code_format_regex: string | null
  discount_code_format_regex: string | null
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
  code_type: CodeType
  // null 代表永久有效，沒有到期日。
  expires_at: string | null
  quality_score: number
  impressions: number
  created_at: string
  activated_at: string | null
  updated_at: string
}

// 後台碼列表能篩的狀態。pending 不在裡面 —— 那批在審核佇列處理。
export type AdminCodeStatus = Exclude<CodeStatus, 'pending'>

// 後台的已上架推薦碼，比 ReferralCode 多了服務商、上架者與使用者回報統計。
export interface AdminCodeItem extends ReferralCode {
  merchant_slug: string
  merchant_name: string
  owner_email: string
  owner_name: string
  // 使用者回報。四種分開數，因為處理方式不同：invalid_code 要找上架者，
  // merchant_closed 代表整家服務商的活動可能都結束了。
  report_total: number
  report_worked: number
  report_failed: number
  report_invalid_code: number
  report_merchant_closed: number
  // 「內容令人反感」的檢舉。跟上面四種分開看：那四種在講碼還能不能用，
  // 這個是 UGC 政策要處理的內容問題，不影響品質分數也不會自動下架，
  // 所以只有人工看到才會有動作。
  report_objectionable: number
  // null 代表這個碼還沒有人回報過。
  last_reported_at: string | null
  // 只有自動下架清單那支會帶：系統把它打掉的時間。
  disabled_at?: string
}

// 使用者提報「希望上架的平台」。目錄只由 admin 維護，這是使用者唯一能把
// 「你們少了這一家」講出來的管道（見 refcode-api 的 00016 migration）。
// 通過審核時後台補 slug 與分類，直接建成停用的服務商草稿。
export interface MerchantSuggestion {
  id: string
  user_id: string
  name: string
  signup_url: string
  note: string
  // 佇列只回 pending；另外兩個狀態是審完之後的紀錄，後台不再列出來。
  status: 'pending' | 'approved' | 'rejected'
  reviewed_by: string | null
  reviewed_at: string | null
  review_reason: string
  // 通過時建出來的那家服務商。
  merchant_id: string | null
  created_at: string
  owner_email: string
  owner_name: string
}

// 建議單只會被審一次，沒有下架、恢復這種後續動作。
export type SuggestionReviewAction = 'approve' | 'reject'

// 客服查帳號、查訂閱狀態用的（退款爭議、手動補發/撤銷 Pro）。
export interface AdminUserItem {
  id: string
  email: string
  display_name: string
  status: 'active' | 'suspended' | 'deleted'
  created_at: string
  is_pro: boolean
  pro_expires_at: string | null
  pro_store: string | null
  pro_product_id: string | null
}

export interface MerchantInput {
  slug: string
  name: string
  category_id: string
  logo_url: string | null
  signup_url: string
  reward_desc: string
  reward_desc_en: string | null
  reward_desc_ja: string | null
  code_format_regex: string | null
  discount_code_format_regex: string | null
  allowed_code_types: CodeType[]
  is_active?: boolean
  countries: string[]
}
