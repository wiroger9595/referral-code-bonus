// 到期倒數。首頁的卡片與服務商頁的每個碼共用這一份，分開寫的話同一個
// 時間點會出現「今天到期」與「1 天後到期」兩種說法。
export function useExpiry() {
  const { t } = useI18n()

  // 這幾天內不去用就真的沒了，值得標紅喊一下。
  const URGENT_DAYS = 3

  function daysLeft(iso: string) {
    return Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
  }

  // iso 是 null 代表這個碼沒有到期日，永遠有效，也就永遠不緊急。
  function expiryLabel(iso: string | null) {
    if (iso === null) return t('common.noExpiry')
    const days = daysLeft(iso)
    return days <= 0 ? t('common.expiresToday') : t('common.expiresInDays', { count: days }, days)
  }

  function isUrgent(iso: string | null) {
    return iso !== null && daysLeft(iso) <= URGENT_DAYS
  }

  return { daysLeft, expiryLabel, isUrgent }
}
