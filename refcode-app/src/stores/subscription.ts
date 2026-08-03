import type { CustomerInfo, PurchasesPackage } from '@revenuecat/purchases-capacitor'
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  buy,
  currentCustomerInfo,
  forgetUser,
  identify,
  isProActive,
  onCustomerInfoChange,
  proExpiresAt,
  proPackages,
  purchasesAvailable,
  restore,
} from '../api/purchases'

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

  async function purchase(pkg: PurchasesPackage) {
    info.value = await buy(pkg)
  }

  async function restorePurchases() {
    info.value = await restore()
    return isProActive(info.value)
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
    restorePurchases,
    watch,
  }
})
