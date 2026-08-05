<script setup lang="ts">
import type { MerchantSummary } from '~/types/api'

defineProps<{ merchant: MerchantSummary }>()

const localePath = useLocalePath()
const { expiryLabel, isUrgent } = useExpiry()
</script>

<template>
  <NuxtLink
    :to="localePath(`/referral/${merchant.slug}`)"
    class="app-card app-card-link flex items-center gap-4 p-4"
  >
    <!-- 沒有 logo 的服務商用名稱首字當替代，空白方框比缺圖的破圖好看。 -->
    <span
      class="grid size-12 shrink-0 place-items-center overflow-hidden rounded-card bg-brand-soft text-xl font-bold text-brand-ink"
    >
      <img
        v-if="merchant.logo_url"
        :src="merchant.logo_url"
        :alt="merchant.name"
        class="size-full object-cover"
      />
      <template v-else>{{ merchant.name.trim().charAt(0) }}</template>
    </span>

    <div class="min-w-0 flex-1">
      <!-- 服務商名降成註記，獎勵內容才是使用者在比較的東西。 -->
      <p class="text-xs font-semibold text-muted">{{ merchant.name }}</p>
      <h3 class="mt-0.5 truncate text-base font-bold tracking-tight text-brand">
        {{ merchant.reward_desc }}
      </h3>
      <div class="mt-2 flex flex-wrap items-center gap-2">
        <span class="pill">{{ merchant.category_name }}</span>
        <span
          v-if="merchant.soonest_expires_at"
          class="text-xs font-semibold"
          :class="isUrgent(merchant.soonest_expires_at) ? 'text-alert' : 'text-muted'"
        >
          {{ expiryLabel(merchant.soonest_expires_at) }}
        </span>
      </div>
    </div>

    <span class="pill shrink-0" :class="merchant.active_code_count > 0 ? 'pill-ok' : ''">
      {{ $t('merchant.codesAvailable', { count: merchant.active_code_count }) }}
    </span>
  </NuxtLink>
</template>
