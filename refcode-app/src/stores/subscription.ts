import type { CustomerInfo, PurchasesPackage } from '@revenuecat/purchases-capacitor'
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { api } from '../api/client'
import {
  buy,
  currentCustomerInfo,
  forgetUser,
  identify,
  isIdentifiedAs,
  isProActive,
  onCustomerInfoChange,
  proExpiresAt,
  proPackages,
  purchasesAvailable,
  restore,
} from '../api/purchases'

// 購買前發現 SDK 還停在匿名身分。呼叫端要能分辨這個跟一般購買失敗 ——
// 這種情況不能讓他繼續付錢。
export class NotIdentifiedError extends Error {}

export const useSubscriptionStore = defineStore('subscription', () => {
  const info = ref<CustomerInfo | null>(null)
  const packages = ref<PurchasesPackage[]>([])
  const loading = ref(false)

  // 瀏覽器裡沒有 RevenueCat SDK，這時改用後端 /v1/me 回的 is_pro。
  // 原生上仍以 SDK 為準：後端那份要等 webhook，剛買完的那幾秒會落後。
  const serverIsPro = ref(false)

  const isPro = computed(() => (purchasesAvailable ? isProActive(info.value) : serverIsPro.value))
  const expiresAt = computed(() => proExpiresAt(info.value))
  const available = computed(() => purchasesAvailable)

  function setServerState(pro: boolean) {
    serverIsPro.value = pro
  }

  // 登入後把 RevenueCat 的身分綁到我們的 user id，之後 webhook 才對得到帳號。
  async function linkUser(userID: string) {
    if (!purchasesAvailable) return
    try {
      await identify(userID)
      info.value = await currentCustomerInfo()
    } catch (e) {
      // 綁定失敗不該擋住登入 —— 使用者還是能用 app，只是買不了 Pro。
      console.warn('[purchases] 綁定使用者失敗', e)
    }
  }

  async function unlinkUser() {
    if (!purchasesAvailable) return
    try {
      await forgetUser()
    } catch (e) {
      console.warn('[purchases] 解除綁定失敗', e)
    }
    info.value = null
    serverIsPro.value = false
  }

  async function refresh() {
    if (!purchasesAvailable) return
    info.value = await currentCustomerInfo()
  }

  async function loadPackages() {
    if (!purchasesAvailable) return
    loading.value = true
    try {
      packages.value = await proPackages()
    } finally {
      loading.value = false
    }
  }

  // userID 由呼叫端傳進來（跟 linkUser 一樣），這個 store 才不用反過來依賴 auth store。
  async function purchase(pkg: PurchasesPackage, userID: string) {
    // 綁定當初可能失敗過（linkUser 是刻意不擋登入的），所以買之前再確認一次。
    // 補綁得起來就繼續，補不起來就別讓他付錢 —— 錢扣了但後端認不得，
    // 比買不了難處理得多。
    if (!(await isIdentifiedAs(userID))) {
      await linkUser(userID)
      if (!(await isIdentifiedAs(userID))) throw new NotIdentifiedError()
    }

    info.value = await buy(pkg)

    // SDK 這邊已經是 Pro 了，但後端那份要等 webhook。買完馬上去上架第 4 個碼
    // 會被後端用舊狀態擋下來，所以主動同步一次 —— webhook 還沒到就維持原狀，
    // 下次進 app 的 restore 會再補上。
    await syncServerState()
  }

  // 從後端重新取一次 is_pro。失敗不拋出去：這只是讓兩邊早點一致，
  // 真正的授權判斷在後端每支 API 自己會做。
  async function syncServerState() {
    try {
      const me = await api.me()
      serverIsPro.value = me.is_pro
    } catch (e) {
      console.warn('[purchases] 同步後端訂閱狀態失敗', e)
    }
  }

  async function restorePurchases() {
    info.value = await restore()
    const pro = isProActive(info.value)
    // 恢復購買常見於換手機重裝，後端那份也可能是舊的，一起同步。
    if (pro) await syncServerState()
    return pro
  }

  // 在 app 外面發生的變化（設定裡取消、續訂扣款）SDK 會推過來。
  async function watch() {
    await onCustomerInfoChange((next) => {
      info.value = next
    })
  }

  return {
    info,
    packages,
    loading,
    isPro,
    expiresAt,
    available,
    setServerState,
    linkUser,
    unlinkUser,
    refresh,
    loadPackages,
    purchase,
    syncServerState,
    restorePurchases,
    watch,
  }
})
