import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/tabs/explore' },
  {
    path: '/tabs',
    component: () => import('../views/TabsPage.vue'),
    children: [
      { path: '', redirect: '/tabs/explore' },
      { path: 'explore', component: () => import('../views/ExploreTab.vue') },
      { path: 'category', component: () => import('../views/CategoryTab.vue') },
      { path: 'record', component: () => import('../views/RecordTab.vue') },
      {
        path: 'my-codes',
        component: () => import('../views/MyCodesTab.vue'),
        meta: { requiresAuth: true },
      },
      { path: 'account', component: () => import('../views/AccountTab.vue') },
    ],
  },
  { path: '/merchant/:slug', component: () => import('../views/MerchantPage.vue') },
  { path: '/category/:id', component: () => import('../views/CategoryPage.vue') },
  // 必須先登入才能到 paywall：沒登入時 RevenueCat 用的是匿名 app_user_id，
  // 購買的 webhook 回到後端會對不到帳號，訂閱等於跟著裝置而不是跟著人。
  {
    path: '/pro',
    component: () => import('../views/PaywallPage.vue'),
    meta: { requiresAuth: true },
  },
  { path: '/login', component: () => import('../views/LoginPage.vue') },
  { path: '/forgot-password', component: () => import('../views/ForgotPasswordPage.vue') },
  {
    path: '/add-code',
    component: () => import('../views/AddCodePage.vue'),
    meta: { requiresAuth: true },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // 冷啟動時第一個導航可能早於 restore 完成，等它一下再判斷。
  if (!auth.ready) await auth.restore()

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
})

export default router
