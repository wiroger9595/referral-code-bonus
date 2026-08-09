// 從 App Store 匯入的服務商只有名稱、圖示與官網，獎勵說明要後台自己補
// （見 refcode-api 的 CreateImportedMerchant）。空字串直接 render 會變成一行
// 空白，看起來像卡片壞掉而不是「這家還沒有資訊」。
export function useReward() {
  const { t } = useI18n()

  function rewardText(desc: string): string {
    return desc || t('merchant.rewardPending')
  }

  // 沒有獎勵說明的那行要降成一般說明文字。跟真的有獎勵的那行共用主色，
  // 會讓「還沒有資訊」看起來像一個賣點。
  function rewardTone(desc: string): string {
    return desc ? 'text-brand' : 'text-muted'
  }

  return { rewardText, rewardTone }
}
