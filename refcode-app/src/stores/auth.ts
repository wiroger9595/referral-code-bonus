import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { api, clearTokens, hasSession, saveTokens } from '../api/client'
import { fetchIdToken, signOutProviders } from '../api/social'
import { identifyTawkUser } from '../api/tawk'
import type { OAuthProvider, User } from '../api/types'
import { useSubscriptionStore } from './subscription'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const ready = ref(false)

  const isLoggedIn = computed(() => user.value !== null)

  // 每個「剛知道使用者是誰」的入口（restore／login／register／…）都要做兩件事：
  // 把 RevenueCat 的身分綁到這個帳號（不綁的話購買會掛在匿名 ID 上，webhook 回來
  // 後端認不得，訂閱就跟著裝置而不是跟著人），以及讓客服 widget 知道是誰在聊，
  // 不用對方自己報信箱。
  async function onAuthed(u: User) {
    const subs = useSubscriptionStore()
    subs.setServerState(u.is_pro)
    await subs.linkUser(u.id)
    identifyTawkUser(u.display_name, u.email)
  }

  // app 冷啟動時 token 已經從 Preferences 讀回來了，但還不知道它有沒有過期，
  // 打一次 /v1/me 確認 —— 失敗就當沒登入，不要卡在載入畫面。
  async function restore() {
    if (!hasSession()) {
      ready.value = true
      return
    }
    try {
      user.value = await api.me()
      await onAuthed(user.value)
    } catch {
      await clearTokens()
      user.value = null
    } finally {
      ready.value = true
    }
  }

  async function login(email: string, password: string) {
    const res = await api.login(email, password)
    await saveTokens(res.tokens)
    user.value = res.user
    await onAuthed(res.user)
  }

  async function register(
    email: string,
    password: string,
    displayName: string,
    country: string,
  ) {
    const res = await api.register(email, password, displayName, country)
    await saveTokens(res.tokens)
    user.value = res.user
    await onAuthed(res.user)
  }

  // 重設密碼後端會順便發新 token，所以這裡跟 login 一樣直接讓人進到已登入狀態。
  async function resetPassword(email: string, code: string, password: string) {
    const res = await api.resetPassword(email, code, password)
    await saveTokens(res.tokens)
    user.value = res.user
    await onAuthed(res.user)
  }

  // country 只在這次社群登入要建新帳號時有用，後端會忽略已有帳號的人帶來的值。
  async function loginWithProvider(provider: OAuthProvider, country = '') {
    const idToken = await fetchIdToken(provider)
    const res = await api.oauthLogin(provider, idToken, country)
    await saveTokens(res.tokens)
    user.value = res.user
    await onAuthed(res.user)
  }

  // 刪除帳號之後本機要跟登出走同一條路：清 token、解除 RevenueCat 綁定、清狀態。
  // 後端已經把資料刪掉了，這裡不必再打 logout（那支會 401）。
  async function deleteAccount(confirm: string) {
    await api.deleteAccount(confirm)
    await signOutProviders()
    await useSubscriptionStore().unlinkUser()
    await clearTokens()
    user.value = null
  }

  async function logout() {
    try {
      await api.logout()
    } catch {
      // 後端撤銷失敗也要讓本機登出，不然使用者會卡在登不出的狀態。
    }
    await signOutProviders()
    await useSubscriptionStore().unlinkUser()
    await clearTokens()
    user.value = null
  }

  // 帳號頁改所在地。後端那支是整份覆寫，所以要把現有的顯示名稱一起送回去。
  async function setCountry(country: string) {
    if (!user.value) return
    user.value = await api.updateMe({
      display_name: user.value.display_name,
      avatar_url: user.value.avatar_url,
      country,
    })
  }

  // 大頭照後端上傳完會直接寫回 users，所以回來的就是最新的整份資料。
  async function setAvatar(image: Blob) {
    if (!user.value) return
    user.value = await api.uploadAvatar(image)
  }

  return {
    user,
    ready,
    isLoggedIn,
    restore,
    login,
    register,
    resetPassword,
    loginWithProvider,
    setCountry,
    setAvatar,
    logout,
    deleteAccount,
  }
})
