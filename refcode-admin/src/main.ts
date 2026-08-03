import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { setUnauthorizedHandler } from './api/client'
import router from './router'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// token 過期時後端會回 401，client 清掉 token 後由這裡把人帶回登入頁。
setUnauthorizedHandler(() => {
  if (router.currentRoute.value.name !== 'login') {
    router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
  }
})

app.mount('#app')
