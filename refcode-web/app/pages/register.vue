<script setup lang="ts">
const { t } = useI18n()

definePageMeta({
  middleware: [() => (useAuth().isLoggedIn.value ? navigateTo(useLocalePath()('/')) : undefined)],
})

useSeoMeta({
  title: () => t('auth.registerTitle'),
  robots: 'noindex, follow',
})
</script>

<template>
  <AuthPanel mode="register" />
</template>
