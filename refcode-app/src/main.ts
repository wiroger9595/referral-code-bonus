import { IonicVue } from '@ionic/vue'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { initTokens, setUnauthorizedHandler } from './api/client'
import { i18n, initLocale } from './i18n'
import router from './router'
import { useSubscriptionStore } from './stores/subscription'

import '@ionic/vue/css/core.css'
import '@ionic/vue/css/normalize.css'
import '@ionic/vue/css/structure.css'
import '@ionic/vue/css/typography.css'
// 深色模式跟著系統走。這份要在 theme/variables.css 之前，
// 後者只覆蓋品牌色與表面色，順序反了就會被 Ionic 的預設值蓋掉。
import '@ionic/vue/css/palettes/dark.system.css'
import './theme/variables.css'
import './style.css'

// token 存在 Capacitor Preferences，是 async 的 —— 要先讀回來才能發任何請求，
// 否則冷啟動的第一批請求會全部以未登入的身分送出。語言同一個道理，
// 晚一步讀回來畫面會先閃一次預設語言。
//
// 包 catch 是因為這是 top-level await：一旦 reject，模組評估就失敗，app 永遠不會
// mount，畫面全白且畫面上沒有任何線索。寧可用預設狀態把畫面開出來再說。
await Promise.all([initTokens(), initLocale()]).catch((e) => {
  console.error('[boot] token / 語言初始化失敗，改以預設狀態啟動', e)
})

const app = createApp(App)
const pinia = createPinia()
app.use(IonicVue)
app.use(pinia)
app.use(i18n)
app.use(router)

setUnauthorizedHandler(() => {
  router.replace('/login')
})

// 訂閱狀態可能在 app 外面變（商店設定裡取消、續訂扣款成功），
// RevenueCat 會主動推更新，不是只有開 app 時抓一次。
useSubscriptionStore(pinia).watch()

// 同理不讓它擋住 mount —— router guard 裡任何一個 await 失敗都會讓這裡 reject。
await router.isReady().catch((e) => {
  console.error('[boot] 初始導航失敗', e)
})
app.mount('#app')
