<script setup lang="ts">
import type { Category, MerchantSummary } from '~/types/api'

const route = useRoute()
const { public: cfg } = useRuntimeConfig()
const { t } = useI18n()
const localePath = useLocalePath()
const { authHeaders } = useAuth()
const lang = useApiLang()
// 分類一律用 id 認，網址上就是 id。
const id = computed(() => String(route.params.id))

const { data: category, error } = await useFetch<Category>(() => `/v1/categories/${id.value}`, {
  baseURL: cfg.apiBase,
  query: { lang },
})

// 分類不存在時回 404 而不是顯示空列表，免得產生一堆內容重複的可索引頁面。
if (error.value) {
  throw createError({ statusCode: 404, statusMessage: t('category.notFound'), fatal: true })
}

// 整份分類清單是為了頁尾的磁磚 —— 逛完一個分類的人接下來多半是換一個分類看，
// 不是回首頁再點一次。
const { data: categories } = await useFetch<{ categories: Category[] }>('/v1/categories', {
  baseURL: cfg.apiBase,
  query: { lang },
})

// 帶上 token 後端才知道要不要把在地服務商排前面（沒登入就是原本的排序）。
const { data: merchants } = await useFetch<{ merchants: MerchantSummary[] }>('/v1/merchants', {
  baseURL: cfg.apiBase,
  query: { category: computed(() => category.value?.id), limit: 50, lang },
  headers: authHeaders(),
})

const all = computed(() => merchants.value?.merchants ?? [])

useSeoMeta({
  title: () => t('category.seoTitle', { name: category.value?.name ?? '' }),
  description: () => t('category.seoDescription', { name: category.value?.name ?? '' }),
})
</script>

<template>
  <div>
    <NuxtLink
      :to="localePath('/')"
      class="inline-flex items-center gap-1 text-sm font-semibold text-muted hover:text-ink"
    >
      ← {{ $t('category.backToAll') }}
    </NuxtLink>

    <div class="mt-3 mb-6 flex flex-wrap items-baseline justify-between gap-2">
      <h1 class="text-2xl font-bold tracking-tight">
        {{ $t('category.heading', { name: category?.name ?? '' }) }}
      </h1>
      <p v-if="all.length" class="text-xs text-muted">
        {{ $t('home.summary', { count: all.length }, all.length) }}
      </p>
    </div>

    <div v-if="all.length" class="grid gap-3 sm:grid-cols-2">
      <MerchantCard v-for="m in all" :key="m.id" :merchant="m" />
    </div>

    <p v-else class="py-12 text-center text-muted">{{ $t('category.empty') }}</p>

    <section v-if="categories?.categories?.length" class="mt-12 border-t border-line pt-8">
      <h2 class="mb-4 text-lg font-bold tracking-tight">{{ $t('home.categories') }}</h2>
      <CategoryTiles :categories="categories.categories" :active-id="id" />
    </section>
  </div>
</template>
