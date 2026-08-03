<script setup lang="ts">
import type { MerchantSummary } from '~/types/api'

defineProps<{ merchant: MerchantSummary }>()

const localePath = useLocalePath()
</script>

<template>
  <NuxtLink
    :to="localePath(`/referral/${merchant.slug}`)"
    class="block rounded-lg border border-neutral-200 p-4 transition hover:border-neutral-400 dark:border-neutral-800 dark:hover:border-neutral-600"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="truncate font-medium">{{ merchant.name }}</h3>
        <p class="mt-1 text-sm text-neutral-500">{{ merchant.reward_desc }}</p>
      </div>

      <span
        class="shrink-0 rounded-full px-2 py-1 text-xs"
        :class="
          merchant.active_code_count > 0
            ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
            : 'bg-neutral-100 text-neutral-500 dark:bg-neutral-900'
        "
      >
        {{ $t('merchant.codesAvailable', { count: merchant.active_code_count }) }}
      </span>
    </div>
  </NuxtLink>
</template>
