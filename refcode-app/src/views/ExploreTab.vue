<script setup lang="ts">
import {
  IonContent,
  IonHeader,
  IonIcon,
  IonPage,
  IonRefresher,
  IonRefresherContent,
  IonSearchbar,
  IonTitle,
  IonToolbar,
} from '@ionic/vue'
import type { RefresherCustomEvent } from '@ionic/vue'
import { alertCircleOutline, chevronForward, searchOutline } from 'ionicons/icons'
import { computed, onMounted, ref } from 'vue'

import { api } from '../api/client'
import type { Category, MerchantSummary } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { apiErrorMessage, daysUntilExpiry, expiryLabel } from '../i18n'

const merchants = ref<MerchantSummary[]>([])
const categories = ref<Category[]>([])
const activeCategory = ref<string | null>(null)
const search = ref('')
const loading = ref(false)
const errorMessage = ref('')

// 進「快到期」區塊的門檻。門檻拉太寬會變成每一家都在倒數，倒數就不再是訊號了。
const EXPIRING_DAYS = 7
// 倒數標紅的門檻。這幾天內不去用就真的沒了，值得用主色喊一下。
const URGENT_DAYS = 3

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await api.listMerchants({
      category: activeCategory.value ?? undefined,
      q: search.value || undefined,
    })
    merchants.value = res.merchants
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'common.connectionFailed')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  categories.value = (await api.listCategories().catch(() => ({ categories: [] }))).categories
  await load()
})

function pickCategory(slug: string | null) {
  activeCategory.value = activeCategory.value === slug ? null : slug
  load()
}

async function refresh(event: RefresherCustomEvent) {
  await load()
  event.target.complete()
}

// 沒有 logo 的服務商用名稱首字當替代，空白方框比缺圖的破圖好看。
function initial(name: string) {
  return name.trim().charAt(0)
}

function isUrgent(iso: string) {
  return daysUntilExpiry(iso) <= URGENT_DAYS
}

// 使用者已經在搜尋或篩分類時就不分區塊 —— 他要的是篩出來的那一份清單，
// 再拆成三堆只會讓剛篩到的東西不見。
const showSections = computed(() => !search.value && activeCategory.value === null)

const expiring = computed(() => {
  if (!showSections.value) return []
  return merchants.value
    .filter((m) => m.soonest_expires_at !== null && daysUntilExpiry(m.soonest_expires_at) <= EXPIRING_DAYS)
    .sort((a, b) => (a.soonest_expires_at ?? '').localeCompare(b.soonest_expires_at ?? ''))
})

// 後端已經照 active_code_count 由大到小排好，這裡不用再排一次。
const hot = computed(() =>
  showSections.value ? merchants.value.filter((m) => m.active_code_count > 0).slice(0, 10) : [],
)
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonTitle>{{ $t('explore.title') }}</IonTitle>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <IonRefresher slot="fixed" @ion-refresh="refresh">
        <IonRefresherContent />
      </IonRefresher>

      <div class="page-pad top">
        <IonSearchbar
          v-model="search"
          class="app-search"
          :placeholder="$t('explore.searchPlaceholder')"
          :debounce="400"
          @ion-input="load"
        />
      </div>

      <!-- 分類橫向捲動，不要換行 —— 換行會把首屏的服務商卡片推出畫面外。 -->
      <div v-if="categories.length" class="chips">
        <button class="chip" :class="{ on: activeCategory === null }" @click="pickCategory(null)">
          {{ $t('explore.all') }}
        </button>
        <button
          v-for="c in categories"
          :key="c.id"
          class="chip"
          :class="{ on: activeCategory === c.slug }"
          @click="pickCategory(c.slug)"
        >
          {{ c.name }}
        </button>
      </div>

      <SkeletonList v-if="loading && merchants.length === 0" />

      <EmptyState
        v-else-if="errorMessage"
        :icon="alertCircleOutline"
        tone="danger"
        :title="$t('common.loadFailed')"
        :description="errorMessage"
      />

      <EmptyState
        v-else-if="merchants.length === 0"
        :icon="searchOutline"
        :title="$t('explore.noMatchTitle')"
        :description="$t('explore.noMatchDesc')"
      />

      <template v-else>
        <!-- 區塊：橫向捲動的小卡。獎勵內容是這張卡唯一的重點，服務商名只是註記。 -->
        <template v-for="rail in [
          { key: 'expiring', title: $t('explore.sectionExpiring'), items: expiring },
          { key: 'hot', title: $t('explore.sectionHot'), items: hot },
        ]" :key="rail.key">
          <section v-if="rail.items.length" class="section">
            <h2 class="section-title page-pad">{{ rail.title }}</h2>
            <div class="rail">
              <div
                v-for="m in rail.items"
                :key="m.id"
                class="app-card tappable tile"
                @click="$router.push(`/merchant/${m.slug}`)"
              >
                <div class="logo sm">
                  <img v-if="m.logo_url" :src="m.logo_url" :alt="m.name" />
                  <span v-else>{{ initial(m.name) }}</span>
                </div>
                <p class="brand">{{ m.name }}</p>
                <p class="reward tile-reward">{{ m.reward_desc }}</p>
                <span
                  v-if="m.soonest_expires_at"
                  class="countdown"
                  :class="{ urgent: isUrgent(m.soonest_expires_at) }"
                >
                  ・{{ expiryLabel(m.soonest_expires_at) }}
                </span>
              </div>
            </div>
          </section>
        </template>

        <h2 v-if="showSections && (expiring.length || hot.length)" class="section-title page-pad">
          {{ $t('explore.allMerchants') }}
        </h2>

        <p class="count page-pad tiny muted">
          {{ $t('explore.summary', { count: merchants.length }, merchants.length) }}
        </p>

        <div class="stack page-pad list">
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
              <h3 class="reward">{{ m.reward_desc }}</h3>
              <div class="meta">
                <span class="pill neutral">{{ m.category_name }}</span>
                <span
                  v-if="m.soonest_expires_at"
                  class="countdown"
                  :class="{ urgent: isUrgent(m.soonest_expires_at) }"
                >
                  ・{{ expiryLabel(m.soonest_expires_at) }}
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
.top {
  padding-top: 4px;
}

.chips {
  display: flex;
  gap: 8px;
  padding: 12px 16px 4px;
  overflow-x: auto;
  scrollbar-width: none;
}

.chips::-webkit-scrollbar {
  display: none;
}

.chip {
  flex: none;
  padding: 7px 14px;
  /* 灰底上的白色 chip 要用深一階的框線才看得出邊，subtle 那條會不見。 */
  border: 1px solid var(--app-line-strong);
  border-radius: var(--app-radius-full);
  background: var(--app-surface);
  color: var(--ion-text-color);
  font-family: inherit;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.chip.on {
  border-color: transparent;
  background: var(--ion-color-primary);
  color: var(--ion-color-primary-contrast);
}

/* ── 區塊 ───────────────────────────────────────────── */

.section {
  margin-top: 18px;
}

.section-title {
  margin: 0 0 10px;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

/* 橫向捲動的軌道。左右 padding 要跟 .page-pad 對齊，第一張卡才不會比下面的清單凸出來。 */
.rail {
  display: flex;
  gap: 12px;
  padding: 2px 16px 4px;
  overflow-x: auto;
  scroll-snap-type: x proximity;
  scrollbar-width: none;
}

.rail::-webkit-scrollbar {
  display: none;
}

.tile {
  flex: none;
  width: 150px;
  padding: 14px;
  scroll-snap-align: start;
}

/* 卡片高度要一致，獎勵內容長短不一，截在兩行。 */
.tile-reward {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

/* ── 清單 ───────────────────────────────────────────── */

.count {
  margin: 0 0 10px;
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

.logo.sm {
  width: 38px;
  height: 38px;
  margin-bottom: 10px;
  font-size: 16px;
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

/* 服務商名降成註記，獎勵內容才是使用者在比較的東西 —— 對應 ShopBack 把
   回饋率放最大、品牌名放小的做法。 */
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

.tile .countdown {
  display: inline-block;
  margin-top: 8px;
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
