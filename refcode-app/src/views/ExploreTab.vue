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
import {
  airplaneOutline,
  alertCircleOutline,
  appsOutline,
  bagHandleOutline,
  cardOutline,
  cellularOutline,
  chevronForward,
  cloudOutline,
  fastFoodOutline,
  flameOutline,
  gameControllerOutline,
  playCircleOutline,
  searchOutline,
  shieldCheckmarkOutline,
  timeOutline,
  trendingUpOutline,
} from 'ionicons/icons'
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

// 分類磁磚的圖示。分類在後端只有 id 與 name（slug 已經拿掉了），沒有可以對應
// 圖示的穩定欄位，所以只能拿名稱去比。比不到就給通用圖示，不會壞掉，
// 但要新增分類時記得回來補一條 —— 正解是後端在分類上加一個 icon 欄位。
const CATEGORY_ICONS: { match: RegExp; icon: string }[] = [
  { match: /銀行|信用卡|カード|銀行|bank|card/i, icon: cardOutline },
  { match: /券商|投資|証券|投資|invest|broker|stock/i, icon: trendingUpOutline },
  { match: /外送|外食|デリバリー|食|delivery|food/i, icon: fastFoodOutline },
  { match: /影音|串流|動画|音楽|stream|video|music/i, icon: playCircleOutline },
  { match: /電信|通訊|通信|携帯|telecom|mobile/i, icon: cellularOutline },
  { match: /旅遊|訂房|旅行|ホテル|travel|hotel|flight/i, icon: airplaneOutline },
  { match: /購物|電商|通販|shop|retail|commerce/i, icon: bagHandleOutline },
  { match: /遊戲|ゲーム|game/i, icon: gameControllerOutline },
  { match: /軟體|訂閱|クラウド|saas|software|cloud/i, icon: cloudOutline },
]

function categoryIcon(name: string) {
  return CATEGORY_ICONS.find((c) => c.match.test(name))?.icon ?? appsOutline
}

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

function pickCategory(id: string | null) {
  activeCategory.value = activeCategory.value === id ? null : id
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

// 橫幅上的兩個數字算的是「這一頁列出來的」，不是全站總數 —— API 一次只給 50 筆。
// 文案也是照這個寫的，別改成「全站共 N 組」。
const totalCodes = computed(() => merchants.value.reduce((sum, m) => sum + m.active_code_count, 0))

// 排序在前端做。清單一次就全抓回來了，改排序再打一次 API 只是讓畫面白一下。
type SortKey = 'recommended' | 'codes' | 'expiring'
const SORTS: { key: SortKey; label: string }[] = [
  { key: 'recommended', label: 'explore.sortRecommended' },
  { key: 'codes', label: 'explore.sortCodes' },
  { key: 'expiring', label: 'explore.sortExpiring' },
]
const sort = ref<SortKey>('recommended')

const sorted = computed(() => {
  const list = [...merchants.value]
  if (sort.value === 'codes') {
    return list.sort((a, b) => b.active_code_count - a.active_code_count)
  }
  if (sort.value === 'expiring') {
    // 沒有可用的碼就沒有到期日，那些排最後 —— 空值排在前面會讓整份清單看起來沒東西。
    return list.sort((a, b) =>
      (a.soonest_expires_at ?? '9999').localeCompare(b.soonest_expires_at ?? '9999'),
    )
  }
  return list // 後端排過的順序（含在地優先），不要動
})
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

      <!-- 首屏橫幅。這裡沒有可以輪播的活動檔期，所以不做 carousel，改成講
           「碼是審過的」這件事 —— 那才是使用者願意用這個 app 而不是搜論壇的理由。 -->
      <section v-if="showSections && !loading" class="banner page-pad">
        <div class="banner-inner">
          <span class="banner-tag">
            <IonIcon :icon="shieldCheckmarkOutline" />
            {{ $t('explore.bannerTag') }}
          </span>
          <h2>{{ $t('explore.bannerTitle') }}</h2>
          <p>{{ $t('explore.bannerLead') }}</p>

          <div class="banner-stats">
            <div>
              <strong>{{ merchants.length }}</strong>
              <span>{{ $t('explore.statMerchants') }}</span>
            </div>
            <div>
              <strong>{{ totalCodes }}</strong>
              <span>{{ $t('explore.statCodes') }}</span>
            </div>
          </div>
        </div>
      </section>

      <div class="page-pad top">
        <IonSearchbar
          v-model="search"
          class="app-search"
          :placeholder="$t('explore.searchPlaceholder')"
          :debounce="400"
          @ion-input="load"
        />
      </div>

      <!-- 分類做成磁磚而不是橫向 chips：一眼看得完的分類數（個位數）用磁磚
           比較好按，也不用左右撥。選中的那格套主色外框，不整格變橘 ——
           一格橘色磁磚在一片淡色磁磚裡會被當成廣告。 -->
      <section v-if="categories.length" class="cats page-pad">
        <div class="cat-grid">
          <button class="cat" :class="{ on: activeCategory === null }" @click="pickCategory(null)">
            <span class="cat-ico a0"><IonIcon :icon="appsOutline" /></span>
            <span class="cat-name">{{ $t('explore.all') }}</span>
          </button>
          <button
            v-for="(c, i) in categories"
            :key="c.id"
            class="cat"
            :class="{ on: activeCategory === c.id }"
            @click="pickCategory(c.id)"
          >
            <span class="cat-ico" :class="`a${(i % 4) + 1}`">
              <IonIcon :icon="categoryIcon(c.name)" />
            </span>
            <span class="cat-name">{{ c.name }}</span>
          </button>
        </div>
      </section>

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
          { key: 'expiring', title: $t('explore.sectionExpiring'), icon: timeOutline, items: expiring, rank: false },
          { key: 'hot', title: $t('explore.sectionHot'), icon: flameOutline, items: hot, rank: true },
        ]" :key="rail.key">
          <section v-if="rail.items.length" class="section">
            <h2 class="section-title page-pad">
              <IonIcon :icon="rail.icon" />
              {{ rail.title }}
            </h2>
            <div class="rail">
              <div
                v-for="(m, i) in rail.items"
                :key="m.id"
                class="app-card tappable tile"
                @click="$router.push(`/merchant/${m.slug}`)"
              >
                <!-- 熱門給名次、快到期給「限時」：兩排卡片長得一樣時，角標是唯一
                     能讓人知道自己在看哪一排的東西。 -->
                <span v-if="rail.rank" class="corner rank">{{ i + 1 }}</span>
                <span v-else class="corner urgent">{{ $t('explore.limited') }}</span>

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
                  {{ expiryLabel(m.soonest_expires_at) }}
                </span>
              </div>
            </div>
          </section>
        </template>

        <div class="list-head page-pad">
          <h2 v-if="showSections && (expiring.length || hot.length)" class="section-title">
            {{ $t('explore.allMerchants') }}
          </h2>
          <p class="count tiny muted">
            {{ $t('explore.summary', { count: merchants.length }, merchants.length) }}
          </p>
        </div>

        <!-- 排序列。三個以內就全部攤開，藏進 action sheet 反而多一次點擊。 -->
        <div class="sorts">
          <button
            v-for="s in SORTS"
            :key="s.key"
            class="sort"
            :class="{ on: sort === s.key }"
            @click="sort = s.key"
          >
            {{ $t(s.label) }}
          </button>
        </div>

        <div class="stack page-pad list">
          <div
            v-for="m in sorted"
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

/* ── 橫幅 ───────────────────────────────────────────── */

.banner {
  padding-top: 4px;
  padding-bottom: 14px;
}

.banner-inner {
  padding: 18px;
  border-radius: var(--app-radius-lg);
  background: linear-gradient(135deg, var(--ion-color-primary), var(--ion-color-primary-tint));
  color: var(--ion-color-primary-contrast);
}

.banner-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: var(--app-radius-full);
  /* 主色底上的標籤用半透明白，給實色會變成第二個按鈕的樣子。 */
  background: rgba(var(--ion-color-primary-contrast-rgb), 0.18);
  font-size: 11px;
  font-weight: 700;
}

.banner-inner h2 {
  margin: 10px 0 0;
  font-size: 21px;
  font-weight: 700;
  line-height: 1.3;
  letter-spacing: -0.02em;
}

.banner-inner p {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.55;
  opacity: 0.88;
}

.banner-stats {
  display: flex;
  gap: 28px;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid rgba(var(--ion-color-primary-contrast-rgb), 0.24);
}

.banner-stats div {
  display: flex;
  flex-direction: column;
}

.banner-stats strong {
  font-size: 22px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.banner-stats span {
  font-size: 11px;
  font-weight: 600;
  opacity: 0.85;
}

/* ── 分類 ───────────────────────────────────────────── */

.cats {
  padding-top: 16px;
  padding-bottom: 2px;
}

.cat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px 4px;
}

.cat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--ion-text-color);
  font-family: inherit;
  cursor: pointer;
}

.cat-ico {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border-radius: var(--app-radius-lg);
  font-size: 23px;
  /* 選中時的外框畫在磁磚外面，不然 52px 的方塊會被框線吃掉一圈。 */
  box-shadow: 0 0 0 0 transparent;
  transition: box-shadow 120ms ease, transform 120ms ease;
}

.cat:active .cat-ico {
  transform: scale(0.94);
}

.cat.on .cat-ico {
  box-shadow: 0 0 0 2px var(--ion-background-color), 0 0 0 4px var(--ion-color-primary);
}

.cat.on .cat-name {
  color: var(--ion-color-primary);
}

.cat-ico.a0 {
  background: rgba(var(--ion-color-medium-rgb), 0.14);
  color: var(--app-muted);
}

.cat-ico.a1 {
  background: var(--app-accent-1);
  color: var(--app-accent-1-ink);
}

.cat-ico.a2 {
  background: var(--app-accent-2);
  color: var(--app-accent-2-ink);
}

.cat-ico.a3 {
  background: var(--app-accent-3);
  color: var(--app-accent-3-ink);
}

.cat-ico.a4 {
  background: var(--app-accent-4);
  color: var(--app-accent-4-ink);
}

/* 分類名長度不一，截在一行；四欄的格子寬度不夠塞兩行還維持對齊。 */
.cat-name {
  max-width: 100%;
  overflow: hidden;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.3;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── 區塊 ───────────────────────────────────────────── */

.section {
  margin-top: 18px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 10px;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.section-title ion-icon {
  color: var(--ion-color-primary);
  font-size: 17px;
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
  position: relative;
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

.corner {
  position: absolute;
  top: 10px;
  right: 10px;
  border-radius: var(--app-radius-full);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.corner.rank {
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  background: var(--app-tint);
  color: var(--app-tint-ink);
}

.corner.urgent {
  padding: 4px 8px;
  background: var(--ion-color-danger);
  color: var(--ion-color-danger-contrast);
}

/* ── 清單 ───────────────────────────────────────────── */

.list-head {
  margin-top: 22px;
}

.count {
  margin: 4px 0 0;
}

.sorts {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  overflow-x: auto;
  scrollbar-width: none;
}

.sorts::-webkit-scrollbar {
  display: none;
}

.sort {
  flex: none;
  padding: 6px 13px;
  /* 灰底上的白色按鈕要用深一階的框線才看得出邊，subtle 那條會不見。 */
  border: 1px solid var(--app-line-strong);
  border-radius: var(--app-radius-full);
  background: var(--app-surface);
  color: var(--app-muted);
  font-family: inherit;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.sort.on {
  border-color: transparent;
  background: var(--ion-color-primary);
  color: var(--ion-color-primary-contrast);
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
