<script setup lang="ts">
const route = useRoute()
const { user, isLoggedIn, logout } = useAuth()
const { t } = useI18n()
const localePath = useLocalePath()

// htmlAttrs 的 lang、canonical、以及各語言的 hreflang alternate 都由這裡產生。
// 手寫 canonical 會讓三種語言指向同一個網址，等於叫 Google 不要收日英版。
const localeHead = useLocaleHead()

useHead(() => ({
  htmlAttrs: localeHead.value.htmlAttrs,
  link: localeHead.value.link,
  meta: localeHead.value.meta,
  titleTemplate: (title?: string) =>
    title ? t('site.titleTemplate', { title }) : t('site.name'),
}))

// 登入完回到原本在看的頁面 —— 從服務商頁點登入的人是想分享碼，不是想去首頁。
// 但不要把 /login、/register 自己塞進 redirect，那會繞回登入頁。
const isAuthPage = computed(
  () => route.path === localePath('/login') || route.path === localePath('/register'),
)
const authRedirect = computed(() => (isAuthPage.value ? {} : { redirect: route.fullPath }))
const loginTo = computed(() => ({ path: localePath('/login'), query: authRedirect.value }))
const registerTo = computed(() => ({ path: localePath('/register'), query: authRedirect.value }))

async function signOut() {
  await logout()
  // 目前沒有需要登入才能看的頁面，留在原地就好。
}
</script>

<template>
  <div class="min-h-screen bg-white text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
    <NuxtRouteAnnouncer />

    <header class="border-b border-neutral-200 dark:border-neutral-800">
      <div class="mx-auto flex max-w-4xl items-center justify-between px-4 py-4">
        <NuxtLink :to="localePath('/')" class="text-lg font-semibold">{{ $t('site.name') }}</NuxtLink>

        <nav class="flex items-center gap-4 text-sm">
          <NuxtLink
            :to="localePath('/about')"
            class="text-neutral-500 hover:text-neutral-900 dark:hover:text-neutral-100"
          >
            {{ $t('nav.about') }}
          </NuxtLink>

          <template v-if="isLoggedIn">
            <span class="max-w-32 truncate text-neutral-500">{{ user?.display_name }}</span>
            <button
              class="text-neutral-500 hover:text-neutral-900 dark:hover:text-neutral-100"
              @click="signOut"
            >
              {{ $t('nav.logout') }}
            </button>
          </template>

          <template v-else>
            <NuxtLink
              v-if="route.path !== localePath('/register')"
              :to="registerTo"
              class="text-neutral-500 hover:text-neutral-900 dark:hover:text-neutral-100"
            >
              {{ $t('nav.register') }}
            </NuxtLink>
            <NuxtLink
              :to="loginTo"
              class="rounded-md border border-neutral-300 px-3 py-1.5 transition hover:border-neutral-400 dark:border-neutral-700 dark:hover:border-neutral-500"
            >
              {{ $t('nav.login') }}
            </NuxtLink>
          </template>

          <LangSwitcher />
        </nav>
      </div>
    </header>

    <main class="mx-auto max-w-4xl px-4 py-8">
      <NuxtPage />
    </main>

    <footer
      class="mt-16 border-t border-neutral-200 py-8 text-center text-sm text-neutral-500 dark:border-neutral-800"
    >
      {{ $t('footer.disclaimer') }}
    </footer>
  </div>
</template>
