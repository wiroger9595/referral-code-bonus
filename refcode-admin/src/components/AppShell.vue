<script setup lang="ts">
import { NButton, NLayout, NLayoutHeader, NLayoutSider, NMenu, NText } from 'naive-ui'
import { computed, h } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const menuOptions = computed(() => {
  const items = [
    {
      label: () => h(RouterLink, { to: { name: 'review' } }, () => '審核佇列'),
      key: 'review',
    },
    // 已上架的碼與使用者回報，reviewer 也要能處理，不放進 owner 那一段。
    {
      label: () => h(RouterLink, { to: { name: 'codes' } }, () => '上架的碼'),
      key: 'codes',
    },
  ]
  // 服務商目錄只有 owner 能維護，reviewer 看不到入口。
  if (auth.isOwner) {
    items.push(
      {
        label: () => h(RouterLink, { to: { name: 'suggestions' } }, () => '平台建議'),
        key: 'suggestions',
      },
      {
        label: () => h(RouterLink, { to: { name: 'merchants' } }, () => '服務商'),
        key: 'merchants',
      },
      {
        label: () => h(RouterLink, { to: { name: 'categories' } }, () => '分類'),
        key: 'categories',
      },
      {
        label: () => h(RouterLink, { to: { name: 'users' } }, () => '使用者'),
        key: 'users',
      },
    )
  }

  return items
})

function logout() {
  auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <NLayout position="absolute">
    <NLayoutHeader bordered class="header">
      <strong>推薦碼平台後台</strong>
      <div class="header-right">
        <NText depth="3">{{ auth.admin?.email }}（{{ auth.admin?.role }}）</NText>
        <NButton size="small" quaternary @click="logout">登出</NButton>
      </div>
    </NLayoutHeader>

    <NLayout position="absolute" style="top: 56px" has-sider>
      <NLayoutSider bordered :width="180" content-style="padding-top: 8px">
        <NMenu :value="String(route.name)" :options="menuOptions" />
      </NLayoutSider>
      <NLayout content-style="padding: 24px">
        <RouterView />
      </NLayout>
    </NLayout>
  </NLayout>
</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 20px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
