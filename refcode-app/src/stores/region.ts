import { Preferences } from '@capacitor/preferences'
import { defineStore } from 'pinia'
import { ref } from 'vue'

import { deviceRegion } from '../countries'

const KEY = 'catalog_region'

// 使用者主動選了「顯示所有地區」時存這個值。用空字串沒辦法跟「還沒選過」分開，
// 而那兩件事要走不同的預設。後端也認這個字（見 resolveRegion）。
export const ALL_REGIONS = 'all'

// 目錄要限縮在哪個地區。跟 users.country 是兩件事：
// 這裡是「我現在想看哪一區的服務商」，帳號那個是「我住在哪」。
// 人在美國出差但想找台灣的碼時，改這裡就好，不用去動帳號設定。
export const useRegionStore = defineStore('region', () => {
  // null 代表使用者從來沒自己選過，這時才輪到帳號所在地與裝置地區。
  const stored = ref<string | null>(null)

  async function load() {
    const { value } = await Preferences.get({ key: KEY })
    stored.value = value || null
  }

  async function choose(next: string) {
    stored.value = next
    await Preferences.set({ key: KEY, value: next })
  }

  // 優先序：使用者自己選的 > 帳號填的所在地 > 裝置地區 > 不篩。
  // userCountry 由呼叫端傳進來，這個 store 才不用反過來依賴 auth store。
  // 用 || 而不是 ??：空字串代表「沒有」，要往下一個候選掉。
  function effective(userCountry?: string | null): string {
    return stored.value || userCountry || deviceRegion() || ALL_REGIONS
  }

  return { stored, load, choose, effective }
})
