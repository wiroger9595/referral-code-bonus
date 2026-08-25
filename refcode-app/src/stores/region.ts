import { Preferences } from '@capacitor/preferences'
import { defineStore } from 'pinia'
import { ref } from 'vue'

import { api } from '../api/client'
import type { Region } from '../api/types'
import { deviceRegion } from '../countries'

const KEY = 'catalog_region'
// 支援的地區清單也存一份：冷啟動時 /v1/regions 還沒回來，選單不該先空一拍，
// 判定也不該因為這樣就退回不篩。上一次拿到的清單幾乎一定還是對的。
const KEY_SUPPORTED = 'catalog_supported_regions'

// 使用者主動選了「顯示所有地區」時存這個值。用空字串沒辦法跟「還沒選過」分開，
// 而那兩件事要走不同的預設。後端也認這個字（見 resolveRegion）。
export const ALL_REGIONS = 'all'

// 目錄要限縮在哪個地區。跟 users.country 是兩件事：
// 這裡是「我現在想看哪一區的服務商」，帳號那個是「我住在哪」。
// 人在美國出差但想找台灣的碼時，改這裡就好，不用去動帳號設定。
export const useRegionStore = defineStore('region', () => {
  // null 代表使用者從來沒自己選過，這時才輪到帳號所在地與裝置地區。
  const stored = ref<string | null>(null)
  // 目錄實際涵蓋的國家，由後端算（見 /v1/regions）。空陣列代表還沒拿到，
  // 這時所有「有沒有支援」的判斷都要放行，不能因為清單沒到就把人擋成不篩。
  const supported = ref<Region[]>([])

  async function load() {
    const [{ value }, cached] = await Promise.all([
      Preferences.get({ key: KEY }),
      Preferences.get({ key: KEY_SUPPORTED }),
    ])
    stored.value = value || null
    if (cached.value) {
      try {
        supported.value = JSON.parse(cached.value)
      } catch {
        // 存壞了就當沒有，下面的 API 會補回來。
      }
    }
  }

  // 跟 load 分開呼叫：load 只讀本機、一定會成功，這支要打網路。
  // 失敗就沿用快取那份，不要讓地區選單因為一次連線失敗就變空的。
  async function refreshSupported() {
    try {
      const { regions } = await api.listRegions()
      supported.value = regions
      await Preferences.set({ key: KEY_SUPPORTED, value: JSON.stringify(regions) })
    } catch {
      // 沿用 load() 讀進來的快取。
    }
  }

  // 清單還沒拿到時一律回 true —— 這個判斷是拿來擋「選了會看到空目錄」的，
  // 在不知道有哪些國家的情況下擋人，只會把正確的偏好也一起擋掉。
  function isSupported(code: string): boolean {
    if (supported.value.length === 0) return true
    return supported.value.some((r) => r.code === code)
  }

  async function choose(next: string) {
    stored.value = next
    await Preferences.set({ key: KEY, value: next })
  }

  // 優先序：使用者自己選的 > 帳號填的所在地 > 裝置時區 > 不篩。
  // userCountry 由呼叫端傳進來，這個 store 才不用反過來依賴 auth store。
  // 用 || 而不是 ??：空字串代表「沒有」，要往下一個候選掉。
  //
  // 每一層都要通過 isSupported：目錄沒有那個國家的服務商時，篩下去就是一片空白。
  // 這對三層都成立 —— 包含使用者自己選的，因為選單曾經寫死過一份清單，
  // 裡面有幾個國家一家服務商都沒有（JP、KR、CN⋯⋯），那些舊值還留在裝置上。
  function effective(userCountry?: string | null): string {
    if (stored.value === ALL_REGIONS) return ALL_REGIONS
    for (const candidate of [stored.value, userCountry, deviceRegion()]) {
      if (candidate && isSupported(candidate)) return candidate
    }
    return ALL_REGIONS
  }

  return { stored, supported, load, refreshSupported, isSupported, choose, effective }
})
