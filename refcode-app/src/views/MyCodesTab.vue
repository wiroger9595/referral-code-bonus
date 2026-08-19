<script setup lang="ts">
import {
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonIcon,
  IonPage,
  IonRefresher,
  IonRefresherContent,
  IonSpinner,
  IonTitle,
  IonToolbar,
  alertController,
  onIonViewWillEnter,
  toastController,
} from '@ionic/vue'
import type { RefresherCustomEvent } from '@ionic/vue'
import { addOutline, alertCircleOutline, arrowDownCircleOutline, funnelOutline, pricetagsOutline, refreshOutline } from 'ionicons/icons'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { api } from '../api/client'
import type { CodeStatus, MyCode } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { apiErrorMessage } from '../i18n'

const codes = ref<MyCode[]>([])
const loading = ref(false)
const errorMessage = ref('')

const { t, locale } = useI18n()

function statusLabel(status: CodeStatus) {
  return t(`myCodes.status.${status}`)
}

// 對應 style.css 的 .pill 變體，不是 Ionic 的 color。
const statusTone: Record<CodeStatus, string> = {
  pending: 'warning',
  active: 'success',
  rejected: 'danger',
  expired: 'neutral',
  disabled: 'danger',
}

// 'all' 之後照使用者實際會找的順序排：還在架上的擺前面，
// 已經沒救的（拒絕、到期）擺最後。
const FILTERS = ['all', 'active', 'pending', 'disabled', 'expired', 'rejected'] as const
type Filter = (typeof FILTERS)[number]

const filter = ref<Filter>('all')

// 清單一頁就抓完（免費 3 個、Pro 也不會多到哪去），篩選純在前端做 ——
// 切籤不用重打 API，換來的體感差很多。
const visible = computed(() =>
  filter.value === 'all' ? codes.value : codes.value.filter((c) => c.status === filter.value),
)

// 籤上直接標數量，才不用逐一點進去才知道哪個籤是空的。
const counts = computed(() => {
  const map = { all: codes.value.length } as Record<Filter, number>
  for (const f of FILTERS) {
    if (f !== 'all') map[f] = codes.value.filter((c) => c.status === f).length
  }
  return map
})

function filterLabel(f: Filter) {
  return f === 'all' ? t('myCodes.filterAll') : statusLabel(f)
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    codes.value = (await api.listMyCodes()).codes
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'common.loadFailed')
  } finally {
    loading.value = false
  }
}

// 每次切回這一頁都重抓：碼的狀態是後台審完才改的，app 這邊不會收到通知，
// 只能靠回到頁面時再問一次。不能用 onMounted —— Ionic 把離開的頁面留在 DOM 裡，
// 它只會跑第一次；也不能用 Vue 的 onActivated —— 那要有 <KeepAlive> 才觸發，
// 而 Ionic 的 router outlet 自己管頁面堆疊，沒有用 KeepAlive，掛上去等於沒掛。
onIonViewWillEnter(load)

async function refresh(event: RefresherCustomEvent) {
  await load()
  event.target.complete()
}

// 狀態沒變的時候畫面不會有任何動靜，按了像沒反應，所以成功一定要給回饋 ——
// 下拉刷新有收合動畫可以看，按鈕沒有。失敗的訊息 load() 已經寫進 errorMessage
// 並顯示在內容區，這裡不再跳一次。
async function manualRefresh() {
  await load()
  if (errorMessage.value) return
  const toast = await toastController.create({ message: t('myCodes.refreshed'), duration: 1600 })
  await toast.present()
}

// 已經在架上或還在審的才撤得掉，其餘狀態後端會擋。
function canDisable(status: CodeStatus) {
  return status === 'active' || status === 'pending'
}

const disablingID = ref('')

async function disable(c: MyCode) {
  const alert = await alertController.create({
    header: t('myCodes.disableTitle'),
    message: t('myCodes.disableWarning', { merchant: c.merchant_name }),
    buttons: [
      { text: t('myCodes.disableCancel'), role: 'cancel' },
      { text: t('myCodes.disableConfirm'), role: 'destructive' },
    ],
  })
  await alert.present()
  const { role } = await alert.onDidDismiss()
  if (role !== 'destructive') return

  disablingID.value = c.id
  try {
    const updated = await api.disableCode(c.id)
    // 只改狀態不重抓整份列表：使用者要的是碼從公開列表消失，
    // 它自己這張卡留在原位標成「已下架」比較不會以為資料不見了。
    // 停在「上架中」籤時它會從畫面上消失，那正是那個籤的意思，
    // 而籤上的數量同時會變，看得出來它是換籤了而不是被刪掉。
    c.status = updated.status
    const toast = await toastController.create({ message: t('myCodes.disableDone'), duration: 2400 })
    await toast.present()
  } catch (e) {
    const toast = await toastController.create({
      message: apiErrorMessage(e, 'myCodes.disableFailed'),
      duration: 3200,
      color: 'danger',
    })
    await toast.present()
    // code_not_active 代表列表是舊的，重抓一次才看得到真正的狀態。
    await load()
  } finally {
    disablingID.value = ''
  }
}

// 日期格式跟著介面語言走，不然日文介面會看到中文的月／日排法。
// null 是永久有效的碼，這格位置只夠一個符號，用 ∞ 代替日期。
function formatDate(iso: string | null) {
  if (iso === null) return '∞'
  return new Date(iso).toLocaleDateString(locale.value, { month: 'numeric', day: 'numeric' })
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonTitle>{{ $t('myCodes.title') }}</IonTitle>
        <IonButtons slot="end">
          <!-- 審核結果是後台改的，app 不會收到通知。下拉刷新找得到的人有限，
               所以這裡再給一個看得見的入口。 -->
          <IonButton :disabled="loading" :aria-label="$t('myCodes.refresh')" @click="manualRefresh">
            <IonSpinner v-if="loading" slot="icon-only" name="crescent" />
            <IonIcon v-else slot="icon-only" :icon="refreshOutline" />
          </IonButton>
          <IonButton router-link="/add-code">
            <IonIcon slot="icon-only" :icon="addOutline" />
          </IonButton>
        </IonButtons>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <IonRefresher slot="fixed" @ion-refresh="refresh">
        <IonRefresherContent />
      </IonRefresher>

      <SkeletonList v-if="loading && codes.length === 0" :count="3" :lines="3" />

      <EmptyState
        v-else-if="errorMessage"
        :icon="alertCircleOutline"
        tone="danger"
        :title="$t('common.loadFailed')"
        :description="errorMessage"
      />

      <EmptyState
        v-else-if="codes.length === 0"
        :icon="pricetagsOutline"
        :title="$t('myCodes.emptyTitle')"
        :description="$t('myCodes.emptyDesc')"
      >
        <IonButton router-link="/add-code" class="wide">{{ $t('myCodes.addOne') }}</IonButton>
      </EmptyState>

      <template v-else>
        <!-- 六個籤放不進一行，所以橫向捲 —— 跟 ExploreTab 的排序列同一套視覺與行為。
             放在內容區而不是 toolbar 裡：塞進 header 的話這排會把整個版面撐寬。 -->
        <div class="filters">
          <button
            v-for="f in FILTERS"
            :key="f"
            class="filter"
            :class="{ on: filter === f }"
            @click="filter = f"
          >
            {{ filterLabel(f) }} {{ counts[f] }}
          </button>
        </div>

        <!-- 有碼但這個籤是空的：不要跟「一組碼都沒有」共用同一句話，
             那會讓人以為資料不見了。 -->
        <EmptyState
          v-if="visible.length === 0"
          :icon="funnelOutline"
          :title="$t('myCodes.filterEmptyTitle', { status: filterLabel(filter) })"
          :description="$t('myCodes.filterEmptyDesc')"
        >
          <IonButton fill="clear" @click="filter = 'all'">
            {{ $t('myCodes.filterShowAll') }}
          </IonButton>
        </EmptyState>

        <div v-else class="stack page-pad list">
          <div v-for="c in visible" :key="c.id" class="app-card card">
            <div class="head">
              <div class="who">
                <div class="logo">
                  <img v-if="c.merchant_logo_url" :src="c.merchant_logo_url" :alt="c.merchant_name" />
                  <span v-else>{{ c.merchant_name.trim().charAt(0) }}</span>
                </div>
                <div>
                  <h2>{{ c.merchant_name }}</h2>
                  <p class="mono tiny muted">{{ c.code }}</p>
                  <!-- 同一家可以同時上架推薦碼與折扣碼，不標的話兩張卡看起來一模一樣。 -->
                  <p class="tiny muted type">{{ $t(`codeType.${c.code_type}`) }}</p>
                </div>
              </div>
              <span class="pill" :class="statusTone[c.status]">{{ statusLabel(c.status) }}</span>
            </div>

            <!-- 上架者唯一在意的是「有沒有人看到」，所以數字要比碼本身大。 -->
            <div class="stats">
              <div>
                <strong>{{ c.impressions }}</strong>
                <span class="tiny muted">{{ $t('myCodes.impressions') }}</span>
              </div>
              <div>
                <strong>{{ c.quality_score }}</strong>
                <span class="tiny muted">{{ $t('myCodes.qualityScore') }}</span>
              </div>
              <div>
                <strong>{{ formatDate(c.expires_at) }}</strong>
                <span class="tiny muted">{{ $t('myCodes.expiresAt') }}</span>
              </div>
            </div>

            <!-- 被拒的碼沒有下架按鈕，理由剛好補在那個位置 —— 上架者最需要知道的
                 是「為什麼」，光看到紅色的「已拒絕」不知道要改什麼。 -->
            <div v-if="c.status === 'rejected' && c.reject_reason" class="reason">
              <span class="tiny">{{ $t('myCodes.rejectReason') }}</span>
              <p>{{ c.reject_reason }}</p>
            </div>

            <IonButton
              v-if="canDisable(c.status)"
              fill="clear"
              size="small"
              color="danger"
              class="disable"
              :disabled="disablingID === c.id"
              @click="disable(c)"
            >
              <IonIcon slot="start" :icon="arrowDownCircleOutline" />
              {{ $t('myCodes.disable') }}
            </IonButton>
          </div>
        </div>
      </template>
    </IonContent>
  </IonPage>
</template>

<style scoped>
/* 篩選籤。跟 ExploreTab 的排序列同一套視覺（膠囊、白底、深一階的框線，
   選中換主色），使用者不用學第二種可點的小東西長什麼樣。
   左右 padding 對齊 .page-pad，第一顆才不會比下面的卡片凸出來。 */
.filters {
  display: flex;
  gap: 8px;
  padding: 10px 16px 4px;
  overflow-x: auto;
  scrollbar-width: none;
}

.filters::-webkit-scrollbar {
  display: none;
}

/* flex: none 是這排不撐寬版面的關鍵：少了它，六顆籤會把整個頁面推到螢幕外。 */
.filter {
  flex: none;
  padding: 6px 13px;
  border: 1px solid var(--app-line-strong);
  border-radius: var(--app-radius-full);
  background: var(--app-surface);
  color: var(--app-muted);
  font-family: inherit;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
  cursor: pointer;
}

.filter.on {
  border-color: transparent;
  background: var(--ion-color-primary);
  color: var(--ion-color-primary-contrast);
}

.list {
  padding-top: 8px;
  padding-bottom: 24px;
}

/* flex column + 底下的 .stats margin-top:auto，是雙欄時卡片等高的另一半：
   grid 把矮的那張拉高之後，內容要嘛全部擠在上面、底下開一個洞，要嘛像現在這樣
   把數據那排壓到底邊，兩張卡的數字就對齊了。單欄時每張卡本來就只有自己的高度，
   margin-top:auto 沒有多餘空間可以吃，不影響。 */
.card {
  display: flex;
  flex-direction: column;
  padding: 16px;
}

.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  /* 原本這 16px 掛在 .stats 的 margin-top 上，那格現在要留給 auto 撐高度。
     移到這裡，單欄時的間距不變（16 + .stats 的 padding-top 14），
     雙欄被拉高時 auto 疊在這 16px 之上，不會貼死。 */
  margin-bottom: 16px;
}

.who {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.logo {
  display: grid;
  place-items: center;
  flex: none;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  overflow: hidden;
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 17px;
  font-weight: 700;
}

.logo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.head h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
}

.head p {
  margin: 2px 0 0;
  letter-spacing: 0.03em;
  /* 推薦碼是一長串沒有空白的字元，預設不會斷行 —— 遇到特別長的碼會把整張卡
     連同版面一起撐出螢幕外。 */
  overflow-wrap: anywhere;
}

.stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  /* auto 而不是固定值：見 .card 的說明，這是把數據排壓到卡片底邊的那一半。
     min-height 由 padding-top 撐著，不會因為 auto 而貼上標題。 */
  margin-top: auto;
  padding-top: 14px;
  border-top: 1px solid var(--app-line);
}

.stats div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stats strong {
  font-size: 19px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.type {
  margin-top: 2px;
}

/* 拒絕理由。淡紅底照 style.css 的 .pill.danger 那套配色，但這裡是一段會換行的
   文字不是標籤，所以走方框而不是膠囊。 */
.reason {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(var(--ion-color-danger-rgb), 0.1);
  color: var(--ion-color-danger-shade);
}

.reason span {
  font-weight: 700;
  opacity: 0.85;
}

.reason p {
  margin: 2px 0 0;
  font-size: 13px;
  line-height: 1.5;
  /* 審核理由是人工填的，長度沒有上限，不斷行會把卡片撐爆。 */
  overflow-wrap: anywhere;
}

@media (prefers-color-scheme: dark) {
  /* 深色底下 shade 比底色還暗，讀不到 —— 跟 .pill.danger 同一個處理。 */
  .reason {
    color: var(--ion-color-danger-tint);
  }
}

/* 破壞性動作靠右且不搶眼，主要動線還是看數據。 */
.disable {
  display: block;
  margin: 10px -8px -6px auto;
  --padding-start: 8px;
  --padding-end: 8px;
}

/* ── 平板 ───────────────────────────────────────────── */

@media (min-width: 768px) {
  /* 不設 align-items：預設的 stretch 讓同一列的卡片等高。原本是 start，
     卡片各自照內容長高，被拒的碼少了下架按鈕就比隔壁矮一截，看起來像沒對齊。 */
  .list {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}
</style>
