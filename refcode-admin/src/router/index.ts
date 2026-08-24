import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      redirect: '/review',
    },
    {
      path: '/review',
      name: 'review',
      component: () => import('../views/ReviewQueueView.vue'),
    },
    {
      path: '/codes',
      name: 'codes',
      component: () => import('../views/CodesView.vue'),
    },
    {
      path: '/suggestions',
      name: 'suggestions',
      component: () => import('../views/SuggestionsView.vue'),
      meta: { ownerOnly: true },
    },
    {
      path: '/merchants',
      name: 'merchants',
      component: () => import('../views/MerchantsView.vue'),
      meta: { ownerOnly: true },
    },
    {
      path: '/categories',
      name: 'categories',
      component: () => import('../views/CategoriesView.vue'),
      meta: { ownerOnly: true },
    },
    {
      path: '/users',
      name: 'users',
      component: () => import('../views/UsersView.vue'),
      meta: { ownerOnly: true },
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (!to.meta.public && !auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.public && auth.isLoggedIn) {
    return { name: 'review' }
  }
  // 這裡擋只是為了不讓 reviewer 看到會失敗的頁面；真正的權限在後端。
  if (to.meta.ownerOnly && !auth.isOwner) {
    return { name: 'review' }
  }
})

export default router
