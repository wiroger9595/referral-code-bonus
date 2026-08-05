// 到期倒數。首頁的卡片與服務商頁的每個碼共用這一份，分開寫的話同一個
// 時間點會出現「今天到期」與「1 天後到期」兩種說法。
export function useExpiry() {
  const { t } = useI18n()

  // 這幾天內不去用就真的沒了，值得標紅喊一下。
  const URGENT_DAYS = 3

  function daysLeft(iso: string) {
    return Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
  }

  function expiryLabel(iso: string) {
    const days = daysLeft(iso)
    return days <= 0 ? t('common.expiresToday') : t('common.expiresInDays', { count: days }, days)
  }

  function isUrgent(iso: string) {
    return daysLeft(iso) <= URGENT_DAYS
  }

  return { daysLeft, expiryLabel, isUrgent }
}
