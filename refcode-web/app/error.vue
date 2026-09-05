<script setup lang="ts">
import type { NuxtError } from '#app'

// Nuxt 的內建錯誤頁會吐出「404 - Page not found: /xxx | Nuxt」，訊息重複兩次、
// 按鈕是英文的 Go back home，而且沒有站頭也沒有頁尾。這個站的流量幾乎都從
// 自然搜尋進來，死連結是常態（服務商下架、網址被截斷），落地頁不能是一個
// 沒有任何回站路徑的框架預設畫面。
const props = defineProps<{ error: NuxtError }>()

const { t } = useI18n()
const localePath = useLocalePath()

const isNotFound = computed(() => props.error?.statusCode === 404)

// 站內自己 throw 的 404 會在 data 裡標 localized，那時 statusMessage 是譯過的
// 句子（「找不到這個服務商」），拿來當標題比通用的那句精準。
//
// 沒有這個標記就不能信 statusMessage：路由沒匹配到時 Nuxt 自己塞的是
// 「Page not found: /whatever」，直接顯示等於把英文的框架訊息和原始網址
// 端到使用者面前，還會一路寫進 <title>。
const heading = computed(() => {
  const localized = (props.error?.data as { localized?: boolean } | undefined)?.localized
  const fromServer = props.error?.statusMessage?.trim()
  if (localized && fromServer) return fromServer
  return isNotFound.value ? t('errorPage.notFoundTitle') : t('errorPage.genericTitle')
})

const body = computed(() =>
  isNotFound.value ? t('errorPage.notFoundBody') : t('errorPage.genericBody'),
)

// error.vue 完全取代 app.vue，所以 app.vue 那邊設的 titleTemplate 到不了這裡，
// 站名要自己接上去。少了這行，分頁標題就是框架預設的「… | Nuxt」。
useHead(() => ({
  title: t('site.titleTemplate', { title: heading.value }),
  // 錯誤頁不該被收錄：它會用各種不存在的網址被爬到，每一個都是一頁重複內容。
  meta: [{ name: 'robots', content: 'noindex' }],
}))

// 一定要走 clearError，不能只放一個 NuxtLink —— 錯誤狀態不清掉的話，
// 使用者按了之後畫面還是停在這一頁。
function goHome() {
  clearError({ redirect: localePath('/') })
}
</script>

<template>
  <div class="flex min-h-screen flex-col bg-page text-ink">
    <!-- 站頭只留品牌與回首頁，不重複 app.vue 那套完整導覽：錯誤頁的工作是把人
         送回站內，不是提供第二份選單。 -->
    <header class="border-b border-line bg-surface">
      <div class="mx-auto flex max-w-5xl items-center px-4 py-3">
        <NuxtLink
          :to="localePath('/')"
          class="flex items-center gap-2 text-lg font-bold tracking-tight"
        >
          <img src="/images/app-icon-1024.png" alt="" class="size-8 shrink-0 rounded-[22%]" />
          {{ $t('site.name') }}
        </NuxtLink>
      </div>
    </header>

    <main class="mx-auto flex w-full max-w-xl flex-1 flex-col justify-center px-4 py-16">
      <p class="text-sm font-bold text-muted">{{ error?.statusCode ?? 500 }}</p>
      <h1 class="mt-2 text-2xl font-bold tracking-tight sm:text-3xl">{{ heading }}</h1>
      <p class="mt-3 text-sm/relaxed text-muted">{{ body }}</p>

      <!-- 搜尋框是這一頁最有價值的出路：會走到這裡的人心裡有一個具體的服務商，
           把他丟回首頁自己找，不如當下就讓他搜。 -->
      <div v-if="isNotFound" class="mt-8">
        <p class="mb-2 text-sm font-semibold">{{ $t('errorPage.searchHint') }}</p>
        <SearchBox size="lg" />
      </div>

      <div class="mt-8">
        <button class="btn btn-primary" @click="goHome">{{ $t('errorPage.backHome') }}</button>
      </div>
    </main>

    <footer class="border-t border-line py-8 text-center text-sm text-muted">
      <p class="flex flex-wrap items-center justify-center gap-3">
        <NuxtLink :to="localePath('/about')" class="hover:text-ink">
          {{ $t('nav.about') }}
        </NuxtLink>
        <span aria-hidden="true">·</span>
        <NuxtLink :to="localePath('/support')" class="hover:text-ink">
          {{ $t('support.heading') }}
        </NuxtLink>
        <span aria-hidden="true">·</span>
        <NuxtLink :to="localePath('/privacy')" class="hover:text-ink">
          {{ $t('legal.privacySeoTitle') }}
        </NuxtLink>
        <span aria-hidden="true">·</span>
        <NuxtLink :to="localePath('/terms')" class="hover:text-ink">
          {{ $t('legal.termsSeoTitle') }}
        </NuxtLink>
      </p>
    </footer>
  </div>
</template>
