<script setup lang="ts">
import type { CodeItem, MerchantDetail, ReportResult } from '~/types/api'

const route = useRoute()
const { public: cfg } = useRuntimeConfig()
const { track, report } = useTracking()
const { t } = useI18n()
const localePath = useLocalePath()

// 要註冊才能拿到推薦碼：帶著 redirect 讓登入完直接回到這頁。
const loginLink = computed(() => ({ path: localePath('/login'), query: { redirect: route.fullPath } }))

const slug = computed(() => String(route.params.slug))

// 曝光是由後端在決定顯示哪些碼時記錄的。SSR 階段打 API 的是 server，
// 不轉發的話後端看到的 UA/IP 全是 node 的 —— 曝光會歸錯，bot 也擋不掉。
const { data, error } = await useFetch<MerchantDetail>(() => `/v1/merchants/${slug.value}`, {
  baseURL: cfg.apiBase,
  headers: useRequestHeaders(['user-agent', 'x-forwarded-for']),
})

if (error.value) {
  throw createError({ statusCode: 404, statusMessage: t('referral.notFound'), fatal: true })
}

const merchant = computed(() => data.value?.merchant)
const codes = computed(() => data.value?.codes ?? [])

// slug 改過的話後端仍然查得到（回查歷史表），但網址要換成現在的。
// 用 301 而不是 302，搜尋排名才會跟著轉過去 —— 這頁是全站流量最集中的一頁。
if (merchant.value && merchant.value.slug !== slug.value) {
  await navigateTo(localePath(`/referral/${merchant.value.slug}`), {
    redirectCode: 301,
    replace: true,
  })
}

// 複製過的碼才問「能不能用」—— 沒複製的人根本沒試過，問了也是雜訊。
const copiedId = ref<string | null>(null)
const reportedIds = ref(new Set<string>())
const copyFailed = ref(false)

async function copyCode(code: CodeItem) {
  if (code.masked || code.code === null) return // 按鈕在遮碼狀態下不會出現，這只是保險
  copyFailed.value = false
  try {
    await navigator.clipboard.writeText(code.code)
    copiedId.value = code.id
    track(code.id, 'copy')
  } catch {
    // Safari 在非使用者手勢的情境會擋 clipboard，讓使用者自己選取。
    copyFailed.value = true
  }
}

async function sendReport(code: CodeItem, result: ReportResult) {
  reportedIds.value = new Set(reportedIds.value).add(code.id)
  try {
    await report(code.id, result)
  } catch {
    // 回報失敗不用打擾使用者，重複回報後端本來就會去重。
  }
}

function goSignup(code: CodeItem) {
  if (!merchant.value) return
  track(code.id, 'click')
  window.open(merchant.value.signup_url, '_blank', 'noopener')
}

function daysLeft(iso: string) {
  return Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
}

useSeoMeta({
  title: () => t('referral.seoTitle', { name: merchant.value?.name ?? '' }),
  description: () =>
    t('referral.seoDescription', {
      name: merchant.value?.name ?? '',
      reward: merchant.value?.reward_desc ?? '',
      count: data.value?.total ?? 0,
    }),
  ogTitle: () => t('referral.seoTitle', { name: merchant.value?.name ?? '' }),
  ogDescription: () => merchant.value?.reward_desc ?? '',
})

// 給搜尋引擎看的結構化資料，讓服務商頁在搜尋結果有機會拿到 rich result。
// Googlebot 沒有 token，SSR 給它的 codes 永遠是遮碼版（c.code 是 null）——
// name 不能直接塞碼，改用「誰分享的」，碼本身不該被索引成公開內容。
useHead({
  script: [
    {
      type: 'application/ld+json',
      innerHTML: computed(() =>
        JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'ItemList',
          name: t('referral.seoTitle', { name: merchant.value?.name ?? '' }),
          numberOfItems: data.value?.total ?? 0,
          itemListElement: codes.value.slice(0, 10).map((c, i) => ({
            '@type': 'ListItem',
            position: i + 1,
            name: t('referral.sharedBy', { name: c.owner_name }),
          })),
        }),
      ),
    },
  ],
})
</script>

<template>
  <div v-if="merchant">
    <NuxtLink
      :to="localePath(`/category/${merchant.category_slug}`)"
      class="text-sm text-neutral-500 hover:underline"
    >
      ← {{ merchant.category_name }}
    </NuxtLink>

    <h1 class="mt-3 text-2xl font-semibold">
      {{ $t('referral.heading', { name: merchant.name }) }}
    </h1>
    <p class="mt-2 text-neutral-600 dark:text-neutral-400">{{ merchant.reward_desc }}</p>

    <p v-if="copyFailed" class="mt-4 rounded-md bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950 dark:text-amber-200">
      {{ $t('referral.copyBlocked') }}
    </p>

    <section class="mt-8">
      <h2 class="mb-3 text-sm font-medium text-neutral-500">
        {{ $t('referral.activeCodes', { count: data?.total ?? 0 }, data?.total ?? 0) }}
      </h2>

      <ul v-if="codes.length" class="space-y-3">
        <li
          v-for="code in codes"
          :key="code.id"
          class="rounded-lg border border-neutral-200 p-4 dark:border-neutral-800"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <code v-if="code.masked" class="inline-flex items-center gap-1.5 font-mono text-lg font-medium text-neutral-400 italic dark:text-neutral-600">
                {{ $t('referral.maskedPlaceholder') }}
              </code>
              <code v-else class="font-mono text-lg font-medium">{{ code.code }}</code>
              <p class="mt-1 text-xs text-neutral-500">
                {{ $t('referral.sharedBy', { name: code.owner_name }) }}
                ・{{ $t('referral.expiresIn', { count: daysLeft(code.expires_at) }, daysLeft(code.expires_at)) }}
                <template v-if="code.worked_count + code.failed_count > 0">
                  ・{{ $t('referral.workedReports', { count: code.worked_count }, code.worked_count) }}
                </template>
              </p>
              <p v-if="code.note" class="mt-1 text-sm text-neutral-600 dark:text-neutral-400">
                {{ code.note }}
              </p>
            </div>

            <!-- 要註冊才能拿到推薦碼：遮碼狀態下只給一顆「登入查看」，不給複製或
                 前往註冊——沒有碼就跑去註冊服務商，使用者到了那邊也不知道要填什麼。 -->
            <div v-if="code.masked" class="flex shrink-0">
              <NuxtLink
                :to="loginLink"
                class="rounded-md bg-neutral-900 px-3 py-1.5 text-sm whitespace-nowrap text-white transition hover:bg-neutral-700 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-neutral-300"
              >
                {{ $t('referral.loginToReveal') }}
              </NuxtLink>
            </div>
            <div v-else class="flex shrink-0 gap-2">
              <button
                class="rounded-md border border-neutral-300 px-3 py-1.5 text-sm transition hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-900"
                @click="copyCode(code)"
              >
                {{ copiedId === code.id ? $t('referral.copied') : $t('referral.copy') }}
              </button>
              <button
                class="rounded-md bg-neutral-900 px-3 py-1.5 text-sm text-white transition hover:bg-neutral-700 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-neutral-300"
                @click="goSignup(code)"
              >
                {{ $t('referral.goSignup') }}
              </button>
            </div>
          </div>

          <div
            v-if="copiedId === code.id"
            class="mt-3 flex items-center gap-3 border-t border-neutral-200 pt-3 text-sm dark:border-neutral-800"
          >
            <template v-if="!reportedIds.has(code.id)">
              <span class="text-neutral-500">{{ $t('referral.askFeedback') }}</span>
              <button class="text-emerald-600 hover:underline" @click="sendReport(code, 'worked')">
                {{ $t('referral.worked') }}
              </button>
              <button class="text-red-600 hover:underline" @click="sendReport(code, 'failed')">
                {{ $t('referral.failed') }}
              </button>
            </template>
            <span v-else class="text-neutral-500">{{ $t('referral.thanks') }}</span>
          </div>
        </li>
      </ul>

      <p v-else class="py-12 text-center text-neutral-500">
        {{ $t('referral.empty') }}
      </p>
    </section>
  </div>
</template>
