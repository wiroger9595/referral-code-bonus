import { Preferences } from '@capacitor/preferences'
import { defineStore } from 'pinia'
import { ref } from 'vue'

const KEY = 'onboarding_seen'

// 首次開啟的引導頁看過沒。存 Preferences 而不是後端：這是「這台裝置上的人
// 認不認得這個 app」，跟帳號無關 —— 綁帳號的話換手機重裝就不會再看到，
// 而那正是最需要它的時候。
//
// 用 null 而不是 false 當初始值：冷啟動時還沒讀回來，那時候還不知道
// 該不該導去引導頁。router guard 要等它讀完（見 router/index.ts）。
export const useOnboardingStore = defineStore('onboarding', () => {
  const seen = ref<boolean | null>(null)

  async function load() {
    if (seen.value !== null) return
    const { value } = await Preferences.get({ key: KEY })
    seen.value = value === '1'
  }

  // 看完或按跳過都算看過 —— 強迫看完不會讓人更想用，只會讓人更想解除安裝。
  async function markSeen() {
    seen.value = true
    await Preferences.set({ key: KEY, value: '1' })
  }

  return { seen, load, markSeen }
})
