import { Capacitor } from '@capacitor/core'
import { Purchases } from '@revenuecat/purchases-capacitor'
import type { CustomerInfo, PurchasesPackage } from '@revenuecat/purchases-capacitor'

// RevenueCat 的 API key 是分平台的，兩把都是 public key，可以進前端。
const IOS_KEY = import.meta.env.VITE_REVENUECAT_IOS_KEY ?? ''
const ANDROID_KEY = import.meta.env.VITE_REVENUECAT_ANDROID_KEY ?? ''

// 後端的 PRO_ENTITLEMENT 要跟這個一致，兩邊對的是 RevenueCat 上同一個 entitlement。
export const PRO_ENTITLEMENT = import.meta.env.VITE_REVENUECAT_ENTITLEMENT ?? 'pro'

const platform = Capacitor.getPlatform()
const apiKey = platform === 'ios' ? IOS_KEY : platform === 'android' ? ANDROID_KEY : ''

// 這個 plugin 的 web 實作要另外接 RevenueCat 的 web billing，沒接就不能用。
// 瀏覽器裡開發時整組購買功能停用，Pro 狀態改看後端 /v1/me 的 is_pro。
export const purchasesAvailable = Capacitor.isNativePlatform() && apiKey !== ''

// 使用者按取消不是錯誤，呼叫端要能分辨。
export class PurchaseCancelled extends Error {}

let configuring: Promise<void> | null = null

function ensureConfigured() {
  configuring ??= Purchases.configure({ apiKey }).catch((e) => {
    configuring = null // 失敗讓下一次重試，不要卡在壞掉的 promise 上。
    throw e
  })
  return configuring
}

// 把 RevenueCat 的 app user id 綁成我們的使用者 UUID —— webhook 送回來的
// app_user_id 就是這個值，後端靠它把訂閱掛到帳號上。沒綁的話會是匿名 ID，
// 後端認不得，訂閱就跟著裝置而不是跟著帳號。
export async function identify(userID: string) {
  if (!purchasesAvailable) return
  await ensureConfigured()
  await Purchases.logIn({ appUserID: userID })
}

export async function forgetUser() {
  if (!purchasesAvailable) return
  await ensureConfigured()
  // 登出後回到匿名身分，否則下一個在同一台裝置登入的人會看到前一個人的訂閱。
  await Purchases.logOut()
}

export function isProActive(info: CustomerInfo | null): boolean {
  return Boolean(info?.entitlements.active[PRO_ENTITLEMENT])
}

export function proExpiresAt(info: CustomerInfo | null): string | null {
  return info?.entitlements.active[PRO_ENTITLEMENT]?.expirationDate ?? null
}

export async function currentCustomerInfo(): Promise<CustomerInfo | null> {
  if (!purchasesAvailable) return null
  await ensureConfigured()
  return (await Purchases.getCustomerInfo()).customerInfo
}

// 取目前 offering 的方案。RevenueCat 後台改了 offering 這裡不用動 ——
// 價格、方案數量、排序全部由後台決定，app 只負責 render。
export async function proPackages(): Promise<PurchasesPackage[]> {
  if (!purchasesAvailable) return []
  await ensureConfigured()
  const { current } = await Purchases.getOfferings()
  return current?.availablePackages ?? []
}

export async function buy(pkg: PurchasesPackage): Promise<CustomerInfo> {
  await ensureConfigured()
  try {
    const { customerInfo } = await Purchases.purchasePackage({ aPackage: pkg })
    return customerInfo
  } catch (e) {
    if ((e as { userCancelled?: boolean }).userCancelled) throw new PurchaseCancelled()
    throw e
  }
}

// 換手機或重裝之後要拿回訂閱。兩家商店都要求 app 內有這個入口，
// 不是「最好有」——沒有會被退件。
export async function restore(): Promise<CustomerInfo | null> {
  if (!purchasesAvailable) return null
  await ensureConfigured()
  const { customerInfo } = await Purchases.restorePurchases()
  return customerInfo
}

// 訂閱狀態可能在 app 外面改變（App Store 設定裡取消、續訂扣款成功），
// SDK 會主動推更新，不要只在開 app 時抓一次。
export async function onCustomerInfoChange(fn: (info: CustomerInfo) => void) {
  if (!purchasesAvailable) return
  await ensureConfigured()
  await Purchases.addCustomerInfoUpdateListener(fn)
}
