<script setup lang="ts">
import type { Category, MerchantSummary } from '~/types/api'

const { public: cfg } = useRuntimeConfig()
const { t } = useI18n()
const localePath = useLocalePath()
const { authHeaders } = useAuth()
const { daysLeft, expiryLabel, isUrgent } = useExpiry()

const { data: categories } = await useFetch<{ categories: Category[] }>('/v1/categories', {
  baseURL: cfg.apiBase,
})

// 帶上 token 後端才知道要不要把在地服務商排前面（沒登入就是原本的排序）。
const { data: merchants } = await useFetch<{ merchants: MerchantSummary[] }>('/v1/merchants', {
  baseURL: cfg.apiBase,
  query: { limit: 30 },
  headers: authHeaders(),
})

const all = computed(() => merchants.value?.merchants ?? [])

// 橫幅上的數字算的是這一頁列出來的，不是全站總數（API 一次只給 30 筆）。
// 文案也是照這個寫的，別改成「全站共 N 組」。
const totalCodes = computed(() => all.value.reduce((sum, m) => sum + m.active_code_count, 0))

// 進「快到期」區塊的門檻。門檻拉太寬會變成每一家都在倒數，倒數就不再是訊號了。
const EXPIRING_DAYS = 7

const expiring = computed(() =>
  all.value
    .filter((m) => m.soonest_expires_at !== null && daysLeft(m.soonest_expires_at) <= EXPIRING_DAYS)
    .sort((a, b) => (a.soonest_expires_at ?? '').localeCompare(b.soonest_expires_at ?? '')),
)

// 後端已經照 active_code_count 由大到小排好，這裡不用再排一次。
const hot = computed(() => all.value.filter((m) => m.active_code_count > 0).slice(0, 8))

useSeoMeta({
  title: () => t('home.seoTitle'),
  description: () => t('home.seoDescription'),
  ogTitle: () => t('home.ogTitle'),
  ogDescription: () => t('home.ogDescription'),
})
</script>

<template>
  <div>
    <!-- 首屏橫幅。沒有活動檔期可以輪播，所以不做 carousel，改成講「碼是審過的」
         這件事 —— 那才是使用者留下來而不是回去翻論壇的理由。
         h1 留在這裡，橫幅同時是這頁對搜尋引擎的標題。 -->
    <section
      class="mb-10 rounded-tile bg-linear-to-br from-brand to-brand-2 p-6 text-on-brand sm:p-8"
    >
      <span
        class="inline-flex items-center gap-1.5 rounded-full bg-on-brand/20 px-2.5 py-1 text-xs font-bold"
      >
        <AppIcon name="shield" />
        {{ $t('home.bannerTag') }}
      </span>

      <h1 class="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">{{ $t('home.heading') }}</h1>
      <p class="mt-2 max-w-2xl text-sm/relaxed opacity-90">{{ $t('home.lead') }}</p>

      <div class="mt-6 flex gap-8 border-t border-on-brand/25 pt-4">
        <div>
          <strong class="block text-2xl font-bold tracking-tight">{{ all.length }}</strong>
          <span class="text-xs font-semibold opacity-85">{{ $t('home.statMerchants') }}</span>
        </div>
        <div>
          <strong class="block text-2xl font-bold tracking-tight">{{ totalCodes }}</strong>
          <span class="text-xs font-semibold opacity-85">{{ $t('home.statCodes') }}</span>
        </div>
      </div>
    </section>

    <section v-if="categories?.categories?.length" class="mb-10">
      <h2 class="mb-4 text-lg font-bold tracking-tight">{{ $t('home.categories') }}</h2>
      <CategoryTiles :categories="categories.categories" />
    </section>

    <!-- 快到期擺在熱門前面：這一排是會消失的東西，熱門明天再看還在。 -->
    <section v-if="expiring.length" class="mb-10">
      <h2 class="mb-4 flex items-center gap-1.5 text-lg font-bold tracking-tight">
        <AppIcon name="clock" class="text-brand" />
        {{ $t('home.sectionExpiring') }}
      </h2>

      <div class="rail pb-1">
        <NuxtLink
          v-for="m in expiring"
          :key="m.id"
          :to="localePath(`/referral/${m.slug}`)"
          class="app-card app-card-link relative w-44 shrink-0 snap-start p-4"
        >
          <span
            class="absolute top-3 right-3 rounded-full bg-alert px-2 py-1 text-[11px] leading-none font-bold text-on-alert"
          >
            {{ $t('home.limited') }}
          </span>
          <span
            class="grid size-10 place-items-center overflow-hidden rounded-card bg-brand-soft font-bold text-brand-ink"
          >
            <img
              v-if="m.logo_url"
              :src="m.logo_url"
              :alt="m.name"
              class="size-full object-cover"
            />
            <template v-else>{{ m.name.trim().charAt(0) }}</template>
          </span>
          <p class="mt-3 truncate text-xs font-semibold text-muted">{{ m.name }}</p>
          <p class="mt-0.5 line-clamp-2 text-sm font-bold text-brand">{{ m.reward_desc }}</p>
          <p
            v-if="m.soonest_expires_at"
            class="mt-2 text-xs font-semibold"
            :class="isUrgent(m.soonest_expires_at) ? 'text-alert' : 'text-muted'"
          >
            {{ expiryLabel(m.soonest_expires_at) }}
          </p>
        </NuxtLink>
      </div>
    </section>

    <section v-if="hot.length" class="mb-10">
      <h2 class="mb-4 flex items-center gap-1.5 text-lg font-bold tracking-tight">
        <AppIcon name="flame" class="text-brand" />
        {{ $t('home.sectionHot') }}
      </h2>

      <div class="rail pb-1">
        <NuxtLink
          v-for="(m, i) in hot"
          :key="m.id"
          :to="localePath(`/referral/${m.slug}`)"
          class="app-card app-card-link relative w-44 shrink-0 snap-start p-4"
        >
          <!-- 名次是這排唯一跟「快到期」不一樣的地方，兩排卡片長得一樣時
               角標是唯一能讓人知道自己在看哪一排的東西。 -->
          <span
            class="absolute top-3 right-3 grid size-5 place-items-center rounded-full bg-brand-soft text-[11px] font-bold text-brand-ink"
          >
            {{ i + 1 }}
          </span>
          <span
            class="grid size-10 place-items-center overflow-hidden rounded-card bg-brand-soft font-bold text-brand-ink"
          >
            <img
              v-if="m.logo_url"
              :src="m.logo_url"
              :alt="m.name"
              class="size-full object-cover"
            />
            <template v-else>{{ m.name.trim().charAt(0) }}</template>
          </span>
          <p class="mt-3 truncate text-xs font-semibold text-muted">{{ m.name }}</p>
          <p class="mt-0.5 line-clamp-2 text-sm font-bold text-brand">{{ m.reward_desc }}</p>
          <p class="mt-2 text-xs font-semibold text-muted">
            {{ $t('merchant.codesAvailable', { count: m.active_code_count }) }}
          </p>
        </NuxtLink>
      </div>
    </section>

    <section>
      <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2">
        <h2 class="text-lg font-bold tracking-tight">{{ $t('home.allMerchants') }}</h2>
        <p class="text-xs text-muted">
          {{ $t('home.summary', { count: all.length }, all.length) }}
        </p>
      </div>

      <div v-if="all.length" class="grid gap-3 sm:grid-cols-2">
        <MerchantCard v-for="m in all" :key="m.id" :merchant="m" />
      </div>

      <p v-else class="py-12 text-center text-muted">{{ $t('home.empty') }}</p>
    </section>
  </div>
</template>
