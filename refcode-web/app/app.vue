<script setup lang="ts">
const route = useRoute()
const { user, isLoggedIn, logout } = useAuth()
const { t } = useI18n()
const localePath = useLocalePath()

// footer 的「聯絡我們」。信箱沒設定就不顯示那個連結 —— 一個點了沒反應的
// mailto 比沒有更糟。
const { public: cfg } = useRuntimeConfig()

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
const isSearchPage = computed(() => route.path === localePath('/search'))
const authRedirect = computed(() => (isAuthPage.value ? {} : { redirect: route.fullPath }))
const loginTo = computed(() => ({ path: localePath('/login'), query: authRedirect.value }))
const registerTo = computed(() => ({ path: localePath('/register'), query: authRedirect.value }))

async function signOut() {
  await logout()
  // 目前沒有需要登入才能看的頁面，留在原地就好。
}
</script>

<template>
  <div class="min-h-screen bg-page text-ink">
    <NuxtRouteAnnouncer />

    <!-- 站頭固定在上面：這個站是拿來一直往下滑列表的，捲到一半想換分類或登入
         的時候，不該要求使用者先滑回頂端。 -->
    <header class="sticky top-0 z-20 border-b border-line bg-surface/90 backdrop-blur-md">
      <div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
        <NuxtLink
          :to="localePath('/')"
          class="flex items-center gap-2 text-lg font-bold tracking-tight"
        >
          <img src="/images/app-icon-1024.png" alt="" class="size-8 shrink-0 rounded-[22%]" />
          {{ $t('site.name') }}
        </NuxtLink>

        <!-- 搜尋頁自己有一個大的輸入框，站頭這個再出現一次只是兩個一樣的東西。 -->
        <SearchBox v-if="!isSearchPage" class="hidden max-w-xs flex-1 sm:block" />

        <nav class="flex items-center gap-3 text-sm">
          <!-- 窄螢幕放不下輸入框，改成一個進搜尋頁的圖示 —— 搜尋頁的空白狀態
               本來就是為了這條路徑設計的（歷史 + 熱門）。 -->
          <NuxtLink
            v-if="!isSearchPage"
            :to="localePath('/search')"
            class="text-muted hover:text-ink sm:hidden"
            :aria-label="$t('search.placeholder')"
          >
            <AppIcon name="search" class="text-xl" />
          </NuxtLink>

          <NuxtLink
            :to="localePath('/about')"
            class="hidden font-semibold text-muted hover:text-ink sm:block"
          >
            {{ $t('nav.about') }}
          </NuxtLink>

          <template v-if="isLoggedIn">
            <!-- 只有登入者才有自己的碼可看，沒登入時擺出來只會導到登入頁。 -->
            <NuxtLink
              :to="localePath('/my-codes')"
              class="hidden font-semibold text-muted hover:text-ink sm:block"
            >
              {{ $t('nav.myCodes') }}
            </NuxtLink>

            <NuxtLink
              :to="localePath('/account')"
              class="flex min-w-0 items-center gap-2 font-semibold text-muted hover:text-ink"
            >
              <span
                class="grid size-7 shrink-0 place-items-center overflow-hidden rounded-full bg-brand-soft text-xs font-bold text-brand-ink"
              >
                <img
                  v-if="user?.avatar_url"
                  :src="user.avatar_url"
                  alt=""
                  class="size-full object-cover"
                />
                <template v-else>{{ (user?.display_name || '').trim().charAt(0).toUpperCase() }}</template>
              </span>
              <span class="max-w-32 truncate">{{ user?.display_name }}</span>
            </NuxtLink>
            <button class="font-semibold text-muted hover:text-ink" @click="signOut">
              {{ $t('nav.logout') }}
            </button>
          </template>

          <template v-else>
            <NuxtLink
              v-if="route.path !== localePath('/register')"
              :to="registerTo"
              class="font-semibold text-muted hover:text-ink"
            >
              {{ $t('nav.register') }}
            </NuxtLink>
            <NuxtLink :to="loginTo" class="btn btn-primary">{{ $t('nav.login') }}</NuxtLink>
          </template>

          <LangSwitcher />
        </nav>
      </div>
    </header>

    <main class="mx-auto max-w-5xl px-4 py-8">
      <NuxtPage />
    </main>

    <footer class="mt-16 border-t border-line py-8 text-center text-sm text-muted">
      <p>{{ $t('footer.disclaimer') }}</p>
      <p class="mt-3 flex items-center justify-center gap-3">
        <NuxtLink :to="localePath('/privacy')" class="hover:text-ink">
          {{ $t('legal.privacySeoTitle') }}
        </NuxtLink>
        <span aria-hidden="true">·</span>
        <NuxtLink :to="localePath('/terms')" class="hover:text-ink">
          {{ $t('legal.termsSeoTitle') }}
        </NuxtLink>
        <template v-if="cfg.supportEmail">
          <span aria-hidden="true">·</span>
          <a :href="`mailto:${cfg.supportEmail}`" class="hover:text-ink">
            {{ $t('footer.contact') }}
          </a>
        </template>
      </p>
    </footer>
  </div>
</template>
