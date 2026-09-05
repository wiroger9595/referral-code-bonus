<script setup lang="ts">
import type { MerchantSummary } from '~/types/api'

defineProps<{ merchant: MerchantSummary }>()

const localePath = useLocalePath()
const { expiryLabel, isUrgent } = useExpiry()
const { rewardText, rewardTone } = useReward()
</script>

<template>
  <!-- min-w-0 是給外層 grid 用的：grid item 預設 min-width 是 auto，會被內容的
       max-content 撐開，而下面那個 h3 是 truncate（nowrap），最長的一句獎勵說明
       就足以把整條 grid track 撐得比視窗還寬，整頁在手機上變成可以左右拖。
       truncate 要真的截斷也得靠這個。 -->
  <NuxtLink
    :to="localePath(`/referral/${merchant.slug}`)"
    class="app-card app-card-link flex min-w-0 items-center gap-4 p-4"
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
      <h3 class="mt-0.5 truncate text-base font-bold tracking-tight" :class="rewardTone(merchant.reward_desc)">
        {{ rewardText(merchant.reward_desc) }}
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
