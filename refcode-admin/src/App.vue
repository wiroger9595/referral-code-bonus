<script setup lang="ts">
import { NConfigProvider, NDialogProvider, NMessageProvider, dateZhTW, zhTW } from 'naive-ui'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import AppShell from './components/AppShell.vue'

const route = useRoute()
const bare = computed(() => route.meta.public === true)
</script>

<template>
  <NConfigProvider :locale="zhTW" :date-locale="dateZhTW">
    <NMessageProvider>
      <NDialogProvider>
        <RouterView v-if="bare" />
        <AppShell v-else />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
