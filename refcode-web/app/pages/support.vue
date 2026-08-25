<script setup lang="ts">
// 兩家商店的「支援網址」都指到這裡（App Store 是必填欄位，審核員會實際點開）。
// 內容刻意做成「自助 + 一個真的會有人看的信箱」：只放一個 mailto 連結，
// 審核時看起來像敷衍，而且大多數問題其實在下面幾條就解決了。
const { t } = useI18n()
const { public: cfg } = useRuntimeConfig()
const localePath = useLocalePath()

useSeoMeta({
  title: () => t('support.seoTitle'),
  description: () => t('support.seoDescription'),
})

const faqs = ['code', 'expired', 'rewards', 'account'] as const
</script>

<template>
  <article class="space-y-6">
    <h1 class="text-2xl font-semibold">{{ $t('support.heading') }}</h1>
    <p class="text-muted">{{ $t('support.lead') }}</p>

    <section class="space-y-4">
      <h2 class="font-medium">{{ $t('support.faqTitle') }}</h2>
      <div v-for="k in faqs" :key="k" class="space-y-1">
        <h3 class="text-sm font-semibold">{{ $t(`support.faq.${k}.q`) }}</h3>
        <p class="text-sm text-muted">{{ $t(`support.faq.${k}.a`) }}</p>
      </div>
    </section>

    <section class="space-y-2">
      <h2 class="font-medium">{{ $t('support.contactTitle') }}</h2>
      <p class="text-muted">{{ $t('support.contactBody') }}</p>
      <!-- supportEmail 沒設就不顯示連結，但這一節照樣在 —— 空的 mailto: 比沒有更糟。 -->
      <p v-if="cfg.supportEmail">
        <a :href="`mailto:${cfg.supportEmail}`" class="font-semibold text-brand hover:underline">
          {{ cfg.supportEmail }}
        </a>
      </p>
    </section>

    <section class="space-y-2">
      <h2 class="font-medium">{{ $t('support.accountTitle') }}</h2>
      <p class="text-muted">{{ $t('support.accountBody') }}</p>
      <p class="flex flex-wrap items-center gap-3 text-sm">
        <NuxtLink :to="localePath('/delete-account')" class="text-brand hover:underline">
          {{ $t('support.deleteAccountLink') }}
        </NuxtLink>
        <span aria-hidden="true">·</span>
        <NuxtLink :to="localePath('/privacy')" class="text-brand hover:underline">
          {{ $t('legal.privacySeoTitle') }}
        </NuxtLink>
        <span aria-hidden="true">·</span>
        <NuxtLink :to="localePath('/terms')" class="text-brand hover:underline">
          {{ $t('legal.termsSeoTitle') }}
        </NuxtLink>
      </p>
    </section>
  </article>
</template>
