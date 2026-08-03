<script setup lang="ts">
const { t } = useI18n()

useSeoMeta({
  title: () => t('about.seoTitle'),
  description: () => t('about.seoDescription'),
})
</script>

<template>
  <article class="space-y-6">
    <h1 class="text-2xl font-semibold">{{ $t('about.heading') }}</h1>

    <section class="space-y-2">
      <h2 class="font-medium">{{ $t('about.noRewardsTitle') }}</h2>
      <p class="text-neutral-600 dark:text-neutral-400">{{ $t('about.noRewardsBody') }}</p>
    </section>

    <section class="space-y-2">
      <h2 class="font-medium">{{ $t('about.expiredTitle') }}</h2>
      <p class="text-neutral-600 dark:text-neutral-400">{{ $t('about.expiredBody') }}</p>
    </section>

    <section class="space-y-2">
      <h2 class="font-medium">{{ $t('about.rankingTitle') }}</h2>
      <p class="text-neutral-600 dark:text-neutral-400">{{ $t('about.rankingBody') }}</p>
    </section>
  </article>
</template>
