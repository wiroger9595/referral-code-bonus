<script setup lang="ts">
import type { Category, MerchantSummary } from '~/types/api'

const { public: cfg } = useRuntimeConfig()
const { t } = useI18n()
const localePath = useLocalePath()
const { authHeaders } = useAuth()

const { data: categories } = await useFetch<{ categories: Category[] }>('/v1/categories', {
  baseURL: cfg.apiBase,
})

// 帶上 token 後端才知道要不要把在地服務商排前面（沒登入就是原本的排序）。
const { data: merchants } = await useFetch<{ merchants: MerchantSummary[] }>('/v1/merchants', {
  baseURL: cfg.apiBase,
  query: { limit: 30 },
  headers: authHeaders(),
})

useSeoMeta({
  title: () => t('home.seoTitle'),
  description: () => t('home.seoDescription'),
  ogTitle: () => t('home.ogTitle'),
  ogDescription: () => t('home.ogDescription'),
})
</script>

<template>
  <div>
    <section class="mb-10">
      <h1 class="text-2xl font-semibold">{{ $t('home.heading') }}</h1>
      <p class="mt-2 text-neutral-600 dark:text-neutral-400">
        {{ $t('home.lead') }}
      </p>
    </section>

    <section v-if="categories?.categories?.length" class="mb-10">
      <h2 class="mb-3 text-sm font-medium text-neutral-500">{{ $t('home.categories') }}</h2>
      <div class="flex flex-wrap gap-2">
        <NuxtLink
          v-for="c in categories.categories"
          :key="c.id"
          :to="localePath(`/category/${c.slug}`)"
          class="rounded-full border border-neutral-200 px-3 py-1.5 text-sm transition hover:border-neutral-400 dark:border-neutral-800 dark:hover:border-neutral-600"
        >
          {{ c.name }}
        </NuxtLink>
      </div>
    </section>

    <section>
      <h2 class="mb-3 text-sm font-medium text-neutral-500">{{ $t('home.allMerchants') }}</h2>

      <div v-if="merchants?.merchants?.length" class="grid gap-3 sm:grid-cols-2">
        <MerchantCard v-for="m in merchants.merchants" :key="m.id" :merchant="m" />
      </div>

      <p v-else class="py-12 text-center text-neutral-500">{{ $t('home.empty') }}</p>
    </section>
  </div>
</template>
