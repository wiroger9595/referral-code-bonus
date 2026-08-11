<script setup lang="ts">
import {
  IonBackButton,
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonIcon,
  IonPage,
  IonTitle,
  IonToolbar,
  actionSheetController,
} from '@ionic/vue'
import { alertCircleOutline, chevronForward, earthOutline, searchOutline } from 'ionicons/icons'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { ApiError, api } from '../api/client'
import type { Category, MerchantSummary } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { countryName, countryOptions } from '../countries'
import { apiErrorMessage, daysUntilExpiry, expiryLabel, rewardText } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { ALL_REGIONS, useRegionStore } from '../stores/region'

const route = useRoute()
const auth = useAuthStore()
const regionStore = useRegionStore()
const { t, locale } = useI18n()

// 分類一律用 id 認，網址上就是 id（跟 refcode-web 的 /category/[id] 對齊）。
const categoryId = computed(() => String(route.params.id))
const category = ref<Category | null>(null)
const merchants = ref<MerchantSummary[]>([])
const loading = ref(true)
const errorMessage = ref('')
// 分類不存在（被刪了、或網址亂打）跟一般連線失敗要分開顯示，前者給的是
// 「回探索頁」，後者給的是「重試」，安慰的話不一樣。
const notFound = ref(false)

const region = computed(() => regionStore.effective(auth.user?.country))
const regionLabel = computed(() =>
  region.value === ALL_REGIONS ? t('explore.allRegions') : countryName(region.value),
)
const filteringByRegion = computed(() => region.value !== ALL_REGIONS)

const URGENT_DAYS = 3
function isUrgent(iso: string) {
  return daysUntilExpiry(iso) <= URGENT_DAYS
}

function initial(name: string) {
  return name.trim().charAt(0)
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  notFound.value = false
  try {
    const [cat, list] = await Promise.all([
      api.getCategory(categoryId.value),
      api.listMerchants({ category: categoryId.value, region: region.value }),
    ])
    category.value = cat
    merchants.value = list.merchants
  } catch (e) {
    if (e instanceof ApiError && e.code === 'category_not_found') {
      notFound.value = true
    } else {
      errorMessage.value = apiErrorMessage(e, 'common.connectionFailed')
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await regionStore.load().catch(() => {})
  await load()
})

// 分類名與獎勵說明是後端依 ?lang= 回的，Ionic 把這頁留在導覽堆疊裡，
// 從別的分頁切語言再走回來不會重跑 onMounted，所以要自己盯著 locale（跟 MerchantPage 一樣）。
watch(locale, load)

async function pickRegion() {
  const sheet = await actionSheetController.create({
    header: t('explore.regionHeader'),
    buttons: [
      ...countryOptions().map((c) => ({ text: c.label, handler: () => applyRegion(c.code) })),
      { text: t('explore.allRegions'), handler: () => applyRegion(ALL_REGIONS) },
      { text: t('common.cancel'), role: 'cancel' },
    ],
  })
  await sheet.present()
}

async function applyRegion(next: string) {
  await regionStore.choose(next)
  await load()
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonButtons slot="start">
          <!-- 深連結進來（沒有上一頁可回）時才會用到這個目標，現在分類自己有分頁了，
               退回分類列表比退回探索頁貼切。從探索頁的磁磚點進來的走的是真的返回。 -->
          <IonBackButton default-href="/tabs/category" text="" />
        </IonButtons>
        <IonTitle>{{ category?.name ?? '' }}</IonTitle>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <SkeletonList v-if="loading" />

      <EmptyState
        v-else-if="notFound"
        :icon="alertCircleOutline"
        tone="danger"
        :title="$t('category.notFound')"
      >
        <IonButton router-link="/tabs/category" router-direction="back" class="wide">
          {{ $t('category.backToList') }}
        </IonButton>
      </EmptyState>

      <EmptyState
        v-else-if="errorMessage"
        :icon="alertCircleOutline"
        tone="danger"
        :title="$t('common.loadFailed')"
        :description="errorMessage"
      />

      <template v-else>
        <div class="page-pad head">
          <p class="tiny muted count">{{ $t('category.summary', { count: merchants.length }, merchants.length) }}</p>
          <button class="sort region" @click="pickRegion">
            <IonIcon :icon="earthOutline" />
            {{ regionLabel }}
          </button>
        </div>

        <EmptyState
          v-if="merchants.length === 0"
          :icon="searchOutline"
          :title="$t('category.emptyTitle')"
          :description="$t('category.emptyDesc')"
        >
          <IonButton v-if="filteringByRegion" fill="outline" size="small" @click="applyRegion(ALL_REGIONS)">
            {{ $t('explore.showAllRegions') }}
          </IonButton>
        </EmptyState>

        <div v-else class="stack page-pad list">
          <div
            v-for="m in merchants"
            :key="m.id"
            class="app-card tappable row"
            @click="$router.push(`/merchant/${m.slug}`)"
          >
            <div class="logo">
              <img v-if="m.logo_url" :src="m.logo_url" :alt="m.name" />
              <span v-else>{{ initial(m.name) }}</span>
            </div>

            <div class="body">
              <p class="brand">{{ m.name }}</p>
              <h3 class="reward" :class="{ pending: !m.reward_desc }">
                {{ rewardText(m.reward_desc) }}
              </h3>
              <div class="meta">
                <span
                  v-if="m.soonest_expires_at"
                  class="countdown"
                  :class="{ urgent: isUrgent(m.soonest_expires_at) }"
                >
                  {{ expiryLabel(m.soonest_expires_at) }}
                </span>
              </div>
            </div>

            <div class="end">
              <span class="pill" :class="m.active_code_count > 0 ? 'success' : 'neutral'">
                {{ $t('explore.codeCount', { count: m.active_code_count }, m.active_code_count) }}
              </span>
              <IonIcon :icon="chevronForward" class="chevron" />
            </div>
          </div>
        </div>
      </template>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 14px;
  padding-bottom: 10px;
}

.count {
  margin: 0;
}

.sort.region {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 5px;
  padding: 6px 13px;
  border: 1px solid var(--app-line-strong);
  border-radius: var(--app-radius-full);
  background: var(--app-surface);
  color: var(--ion-text-color);
  font-family: inherit;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.list {
  padding-bottom: 24px;
}

.row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px;
}

.logo {
  display: grid;
  place-items: center;
  flex: none;
  width: 46px;
  height: 46px;
  border-radius: var(--app-radius);
  overflow: hidden;
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 19px;
  font-weight: 700;
}

.logo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.body {
  flex: 1;
  min-width: 0;
}

.brand {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--app-muted);
}

.reward {
  margin: 2px 0 0;
  font-size: 16px;
  font-weight: 700;
  line-height: 1.35;
  letter-spacing: -0.01em;
  color: var(--ion-color-primary);
}

.reward.pending {
  font-weight: 600;
  color: var(--app-muted);
}

.meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 7px;
}

.countdown {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-muted);
  white-space: nowrap;
}

.countdown.urgent {
  color: var(--ion-color-danger);
}

.end {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
}

.chevron {
  color: var(--app-muted);
  font-size: 16px;
  opacity: 0.6;
}
</style>
