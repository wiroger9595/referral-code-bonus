<script setup lang="ts">
import type { Category, MerchantSummary } from '~/types/api'

const route = useRoute()
const { public: cfg } = useRuntimeConfig()
const { t } = useI18n()
const localePath = useLocalePath()
const { authHeaders } = useAuth()
const slug = computed(() => String(route.params.slug))

// 用 /v1/categories/{slug} 而不是在完整列表裡找：後台改過 slug 之後，
// 舊網址不會出現在列表裡，但這支查得到（後端會回查歷史表）。
const { data: category, error } = await useFetch<Category>(() => `/v1/categories/${slug.value}`, {
  baseURL: cfg.apiBase,
})

// 分類不存在時回 404 而不是顯示空列表，免得產生一堆內容重複的可索引頁面。
if (error.value) {
  throw createError({ statusCode: 404, statusMessage: t('category.notFound'), fatal: true })
}

// slug 改過的話後端仍然查得到，但網址要換成現在的。用 301 而不是 302，
// 搜尋排名才會跟著轉到新網址（見 refcode-api 的 category_slug_history）。
if (category.value && category.value.slug !== slug.value) {
  await navigateTo(localePath(`/category/${category.value.slug}`), {
    redirectCode: 301,
    replace: true,
  })
}

// 帶上 token 後端才知道要不要把在地服務商排前面（沒登入就是原本的排序）。
// 查詢用正規化後的 slug，不是網址上那個（可能是舊的）。
const { data: merchants } = await useFetch<{ merchants: MerchantSummary[] }>('/v1/merchants', {
  baseURL: cfg.apiBase,
  query: { category: computed(() => category.value?.slug), limit: 50 },
  headers: authHeaders(),
})

useSeoMeta({
  title: () => t('category.seoTitle', { name: category.value?.name ?? '' }),
  description: () => t('category.seoDescription', { name: category.value?.name ?? '' }),
})
</script>

<template>
  <div>
    <NuxtLink :to="localePath('/')" class="text-sm text-neutral-500 hover:underline">
      ← {{ $t('category.backToAll') }}
    </NuxtLink>

    <h1 class="mt-3 mb-6 text-2xl font-semibold">
      {{ $t('category.heading', { name: category?.name ?? '' }) }}
    </h1>

    <div v-if="merchants?.merchants?.length" class="grid gap-3 sm:grid-cols-2">
      <MerchantCard v-for="m in merchants.merchants" :key="m.id" :merchant="m" />
    </div>

    <p v-else class="py-12 text-center text-neutral-500">{{ $t('category.empty') }}</p>
  </div>
</template>
