<script setup lang="ts">
const { t } = useI18n()

definePageMeta({
  // 已經登入的人不需要看到這頁。auth plugin 在 middleware 之前跑完，這裡讀得到狀態。
  // 轉回首頁要走 localePath，否則從 /ja/login 會被丟到中文首頁。
  middleware: [() => (useAuth().isLoggedIn.value ? navigateTo(useLocalePath()('/')) : undefined)],
})

// 登入頁沒有給搜尋引擎看的價值，而且會跟首頁搶「推薦碼」這類字。
useSeoMeta({
  title: () => t('auth.loginTitle'),
  robots: 'noindex, follow',
})
</script>

<template>
  <AuthPanel mode="login" />
</template>
