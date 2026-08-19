import type { CodeType } from './api/types'

// 後台是單一語言，直接寫死中文，不走 i18n（三個前端才有語系檔）。
export const CODE_TYPE_LABELS: Record<CodeType, string> = {
  referral: '推薦碼',
  discount: '折扣碼',
}
