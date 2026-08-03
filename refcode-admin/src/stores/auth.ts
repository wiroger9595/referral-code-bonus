import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { api, getToken, setToken } from '../api/client'
import type { AdminUser } from '../api/types'

const ADMIN_KEY = 'refcode_admin_user'

// token 存在 client.ts，這裡另外記 admin 本人的資料，
// 重新整理後才知道該不該顯示 owner 專屬的選單。
function restoreAdmin(): AdminUser | null {
  const raw = localStorage.getItem(ADMIN_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as AdminUser
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const admin = ref<AdminUser | null>(restoreAdmin())

  const isLoggedIn = computed(() => admin.value !== null && getToken() !== null)
  const isOwner = computed(() => admin.value?.role === 'owner')

  async function login(email: string, password: string) {
    const res = await api.login(email, password)
    setToken(res.access_token)
    admin.value = res.admin
    localStorage.setItem(ADMIN_KEY, JSON.stringify(res.admin))
  }

  function logout() {
    setToken(null)
    admin.value = null
    localStorage.removeItem(ADMIN_KEY)
  }

  return { admin, isLoggedIn, isOwner, login, logout }
})
