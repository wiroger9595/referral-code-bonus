<script setup lang="ts">
import type { CodeItem, MerchantDetail, ReportResult } from '~/types/api'

const route = useRoute()
const { public: cfg } = useRuntimeConfig()
const { track, report } = useTracking()
const { t } = useI18n()
const localePath = useLocalePath()
const { authHeaders } = useAuth()
const { expiryLabel, isUrgent } = useExpiry()
const { rewardText } = useReward()
const lang = useApiLang()

// 要註冊才能拿到推薦碼：帶著 redirect 讓登入完直接回到這頁。
const loginLink = computed(() => ({ path: localePath('/login'), query: { redirect: route.fullPath } }))

const slug = computed(() => String(route.params.slug))

// 曝光是由後端在決定顯示哪些碼時記錄的。SSR 階段打 API 的是 server，
// 不轉發的話後端看到的 UA/IP 全是 node 的 —— 曝光會歸錯，bot 也擋不掉。
//
// token 也要一起帶：後端靠它決定碼要不要遮起來（見 refcode-api 的 revealCode）。
// 少了它，登入過的人在這頁一樣只看得到「登入才能看到完整碼」，而這頁是官網
// 唯一的取碼入口 —— 等於登入在官網上沒有任何作用。
const { data, error } = await useFetch<MerchantDetail>(() => `/v1/merchants/${slug.value}`, {
  baseURL: cfg.apiBase,
  query: { lang },
  headers: { ...useRequestHeaders(['user-agent', 'x-forwarded-for']), ...authHeaders() },
})

if (error.value) {
  // data.localized 是給 error.vue 看的：它靠這個分辨 statusMessage 是這裡譯好的
  // 句子，還是 Nuxt 自己塞的「Page not found: /xxx」。
  throw createError({
    statusCode: 404,
    statusMessage: t('referral.notFound'),
    data: { localized: true },
    fatal: true,
  })
}

const merchant = computed(() => data.value?.merchant)
const codes = computed(() => data.value?.codes ?? [])

// 手機上的固定操作列拿的是清單第一個碼 —— 後端已經照品質排過，第一個就是最推薦的那組。
const topCode = computed(() => codes.value[0] ?? null)

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

useSeoMeta({
  title: () => t('referral.seoTitle', { name: merchant.value?.name ?? '' }),
  description: () =>
    t('referral.seoDescription', {
      name: merchant.value?.name ?? '',
      // 獎勵說明還沒補的服務商不能讓描述變成「xxx推薦碼：。目前有…」，
      // 那一句斷掉的話在搜尋結果上很難看。
      reward: rewardText(merchant.value?.reward_desc ?? ''),
      count: data.value?.total ?? 0,
    }),
  ogTitle: () => t('referral.seoTitle', { name: merchant.value?.name ?? '' }),
  ogDescription: () => rewardText(merchant.value?.reward_desc ?? ''),
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
  <!-- 手機上底部有固定的操作列，內容要多留一段才不會被蓋住。 -->
  <div v-if="merchant" class="pb-24 sm:pb-0">
    <NuxtLink
      :to="localePath(`/category/${merchant.category_id}`)"
      class="inline-flex items-center gap-1 text-sm font-semibold text-muted hover:text-ink"
    >
      ← {{ merchant.category_name }}
    </NuxtLink>

    <!-- 商品頁的品牌頭：獎勵內容是使用者往下滑之前唯一想知道的事，用最大的字。 -->
    <div class="app-card mt-3 p-5 sm:p-6">
      <span
        class="grid size-14 place-items-center overflow-hidden rounded-tile bg-brand-soft text-2xl font-bold text-brand-ink"
      >
        <img
          v-if="merchant.logo_url"
          :src="merchant.logo_url"
          :alt="merchant.name"
          class="size-full object-cover"
        />
        <template v-else>{{ merchant.name.trim().charAt(0) }}</template>
      </span>

      <!-- 有獎勵說明時它就是 h1：那是使用者往下滑之前唯一想知道的事。
           還沒補說明的服務商（多半是剛匯入的）改用服務商名當 h1 ——
           空的 h1 對 SEO 是硬傷，而這一頁本來就是在講這家服務商。 -->
      <template v-if="merchant.reward_desc">
        <p class="mt-4 text-sm font-semibold text-muted">{{ merchant.name }}</p>
        <h1 class="mt-1 text-2xl font-bold tracking-tight sm:text-3xl">{{ merchant.reward_desc }}</h1>
      </template>
      <template v-else>
        <h1 class="mt-4 text-2xl font-bold tracking-tight sm:text-3xl">{{ merchant.name }}</h1>
        <p class="mt-1 text-sm text-muted">{{ $t('merchant.rewardPending') }}</p>
      </template>

      <div class="mt-4 flex flex-wrap gap-2">
        <span class="pill">{{ merchant.category_name }}</span>
        <span class="pill" :class="(data?.total ?? 0) > 0 ? 'pill-ok' : ''">
          {{ $t('referral.activeCodes', { count: data?.total ?? 0 }, data?.total ?? 0) }}
        </span>
      </div>

      <p class="mt-4 flex items-center gap-1.5 border-t border-line pt-3 text-xs font-semibold text-muted">
        <AppIcon name="shield" class="text-ok" />
        {{ $t('referral.reviewed') }}
      </p>
    </div>

    <p v-if="copyFailed" class="mt-4 rounded-card bg-brand-soft p-3 text-sm text-brand-ink">
      {{ $t('referral.copyBlocked') }}
    </p>

    <section class="mt-8">
      <h2 class="mb-4 text-lg font-bold tracking-tight">
        {{ $t('referral.activeCodes', { count: data?.total ?? 0 }, data?.total ?? 0) }}
      </h2>

      <ul v-if="codes.length" class="grid gap-3">
        <li v-for="code in codes" :key="code.id" class="app-card p-4 sm:p-5">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <!-- 兩種碼混在同一份清單，第一眼要分得出這是誰的推薦碼還是一組折扣碼。
                   折扣碼的優惠內容在下面的備註那行，上架時強制要填。 -->
              <p class="mb-2">
                <span class="pill">{{ $t(`codeType.${code.code_type}`) }}</span>
              </p>
              <p
                v-if="code.masked"
                class="flex items-center gap-1.5 font-mono text-lg font-bold text-muted italic"
              >
                <AppIcon name="lock" />
                {{ $t('referral.maskedPlaceholder') }}
              </p>
              <p v-else class="font-mono text-xl font-bold tracking-wider break-all">
                {{ code.code }}
              </p>

              <p class="mt-1.5 flex flex-wrap items-center gap-x-1.5 text-xs text-muted">
                <span>{{ $t('referral.sharedBy', { name: code.owner_name }) }}</span>
                <span>・</span>
                <span :class="isUrgent(code.expires_at) ? 'font-semibold text-alert' : ''">
                  {{ expiryLabel(code.expires_at) }}
                </span>
                <template v-if="code.worked_count > 0">
                  <span>・</span>
                  <span class="font-semibold text-ok-ink">
                    {{ $t('referral.workedReports', { count: code.worked_count }, code.worked_count) }}
                  </span>
                </template>
              </p>

              <p v-if="code.note" class="mt-2 text-sm text-muted">{{ code.note }}</p>
            </div>

            <!-- 要註冊才能拿到推薦碼：遮碼狀態下只給一顆「登入查看」，不給複製或
                 前往註冊——沒有碼就跑去註冊服務商，使用者到了那邊也不知道要填什麼。 -->
            <div v-if="code.masked" class="shrink-0">
              <NuxtLink :to="loginLink" class="btn btn-primary">
                <AppIcon name="lock" />
                {{ $t('referral.loginToReveal') }}
              </NuxtLink>
            </div>
            <div v-else class="flex shrink-0 gap-2">
              <button class="btn btn-outline" @click="copyCode(code)">
                <AppIcon :name="copiedId === code.id ? 'check' : 'copy'" />
                {{ copiedId === code.id ? $t('referral.copied') : $t('referral.copy') }}
              </button>
              <button class="btn btn-primary" @click="goSignup(code)">
                {{ $t('referral.goSignup') }}
                <AppIcon name="external" />
              </button>
            </div>
          </div>

          <div
            v-if="copiedId === code.id"
            class="mt-4 flex flex-wrap items-center gap-3 border-t border-line pt-3 text-sm"
          >
            <template v-if="!reportedIds.has(code.id)">
              <span class="text-muted">{{ $t('referral.askFeedback') }}</span>
              <button class="font-semibold text-ok-ink hover:underline" @click="sendReport(code, 'worked')">
                {{ $t('referral.worked') }}
              </button>
              <button
                class="font-semibold text-alert-ink hover:underline"
                @click="sendReport(code, 'failed')"
              >
                {{ $t('referral.failed') }}
              </button>
            </template>
            <span v-else class="text-muted">{{ $t('referral.thanks') }}</span>
          </div>
        </li>
      </ul>

      <p v-else class="py-12 text-center text-muted">{{ $t('referral.empty') }}</p>
    </section>

    <!-- 手機上捲過兩三張卡就看不到按鈕了，這條列是那時候唯一的出口。
         桌機的卡片一直在視野裡，再固定一條只是擋畫面。 -->
    <div
      v-if="topCode"
      class="fixed inset-x-0 bottom-0 z-20 border-t border-line bg-surface px-4 py-3 shadow-[0_-2px_12px_rgba(64,72,90,0.1)] sm:hidden"
    >
      <NuxtLink v-if="topCode.masked" :to="loginLink" class="btn btn-primary w-full">
        <AppIcon name="lock" />
        {{ $t('referral.loginToReveal') }}
      </NuxtLink>

      <template v-else>
        <p class="mb-2 text-xs text-muted">{{ $t('referral.ctaBest') }}</p>
        <div class="flex gap-2">
          <button class="btn btn-outline" @click="copyCode(topCode)">
            <AppIcon :name="copiedId === topCode.id ? 'check' : 'copy'" />
            {{ copiedId === topCode.id ? $t('referral.copied') : $t('referral.copy') }}
          </button>
          <button class="btn btn-primary flex-1" @click="goSignup(topCode)">
            {{ $t('referral.goSignup') }}
            <AppIcon name="external" />
          </button>
        </div>
      </template>
    </div>
  </div>
</template>
