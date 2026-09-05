<script setup lang="ts">
import { Browser } from '@capacitor/browser'
import { Clipboard } from '@capacitor/clipboard'
import { Share } from '@capacitor/share'
import {
  IonBackButton,
  IonButton,
  IonButtons,
  IonContent,
  IonFooter,
  IonHeader,
  IonIcon,
  IonPage,
  IonTitle,
  IonToolbar,
  actionSheetController,
  alertController,
  toastController,
} from '@ionic/vue'
import {
  addOutline,
  ellipsisHorizontal,
  alertCircleOutline,
  checkmarkOutline,
  copyOutline,
  lockClosedOutline,
  openOutline,
  shareOutline,
  shieldCheckmarkOutline,
  ticketOutline,
} from 'ionicons/icons'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { api } from '../api/client'
import type { CodeItem, MerchantDetail, ReportResult } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import QualityDot from '../components/QualityDot.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { thumb } from '../images'
import { apiErrorMessage, expiryLabel, rewardText } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useRecordStore } from '../stores/record'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const record = useRecordStore()
const { t, locale } = useI18n()
const detail = ref<MerchantDetail | null>(null)
const loading = ref(true)
const errorMessage = ref('')

// 複製過的碼才問「能不能用」——沒複製的人根本沒試過，問了只會得到雜訊。
const copiedId = ref<string | null>(null)
const reportedIds = ref(new Set<string>())

async function load() {
  loading.value = true
  try {
    detail.value = await api.getMerchant(String(route.params.slug))
    // 進得來就算看過。紀錄頁靠這個列「看過的服務商」，寫失敗不該影響這一頁。
    const m = detail.value.merchant
    record
      .addMerchant({ slug: m.slug, name: m.name, logo: m.logo_url })
      .catch(() => {})
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'common.loadFailed')
  } finally {
    loading.value = false
  }
}

// 這三個任一個變了都要重抓，而且 Ionic 會把這頁留在導覽堆疊裡，走回來時
// 不會重跑 onMounted，所以一律靠 watch 而不是掛在 mount 上：
//   slug        —— 換一家服務商時這頁是被重用的，不是重新 mount。
//   locale      —— 獎勵說明與分類名是後端依 ?lang= 回的。
//   isLoggedIn  —— masked 是後端依登入狀態算的，登入完 redirect 回來要重抓。
//
// slug 的守衛不能省。從這頁跳去登入時，isLoggedIn 會在 router.replace 把路由
// 換回來之前就先變成 true，那個當下 route 還停在 /login，params.slug 是
// undefined —— 沒擋的話就會去打 /v1/merchants/undefined 拿一個 404，然後整頁
// 停在「找不到這個服務商」，而使用者剛剛才為了看這一頁的碼去登入。
watch(
  [() => route.params.slug, locale, () => auth.isLoggedIn],
  () => {
    if (route.params.slug) load()
  },
  { immediate: true },
)

async function toast(message: string) {
  const t = await toastController.create({ message, duration: 1800, position: 'bottom' })
  await t.present()
}

async function copyCode(code: CodeItem) {
  if (code.masked || code.code === null) return // 按鈕在遮碼狀態下不會出現，這只是保險

  // 寫剪貼簿會失敗（權限被拒、非 secure context 的 WebView）。不擋的話這行一拋，
  // 下面的紀錄與 toast 全部不會跑 —— 使用者按了複製，畫面上什麼都沒發生。
  try {
    await Clipboard.write({ string: code.code })
  } catch {
    toast(t('merchant.copyFailed'))
    return
  }

  copiedId.value = code.id
  api.track(code.id, 'copy').catch(() => {})

  // 複製完多半會離開 app 去服務商那邊註冊，回來時碼已經不在剪貼簿了。
  // 存一份到紀錄頁，讓他找得回來。
  if (detail.value) {
    record
      .addCode({
        codeId: code.id,
        code: code.code,
        merchantSlug: detail.value.merchant.slug,
        merchantName: detail.value.merchant.name,
        merchantLogo: detail.value.merchant.logo_url,
        expiresAt: code.expires_at,
      })
      .catch(() => {})
  }

  toast(t('merchant.copiedToast'))
}

// 要註冊才能拿到推薦碼：帶著 redirect 讓登入完直接回到這頁，不用重新找一次服務商。
function goLoginToReveal() {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

async function openSignup(code: CodeItem) {
  if (!detail.value) return
  api.track(code.id, 'click').catch(() => {})
  try {
    await Browser.open({ url: detail.value.merchant.signup_url })
  } catch {
    // 開不起來（網址壞掉、系統沒有可以處理的瀏覽器）時至少講一句，
    // 不然按鈕看起來就是壞的。
    toast(t('merchant.openFailed'))
  }
}

async function sendReport(code: CodeItem, result: ReportResult) {
  reportedIds.value = new Set(reportedIds.value).add(code.id)
  try {
    await api.report(code.id, result)
    toast(t('merchant.thanksToast'))
  } catch {
    // 重複回報後端本來就會去重，失敗不用打擾使用者。
  }
}

// UGC 政策要求「檢舉不當內容」與「封鎖濫用者」兩件事都要做得到
// （見 refcode-api 的 00012_user_blocks.sql）。放在每張卡的「⋯」裡而不是
// 常駐按鈕：絕大多數人是來拿碼的，檢舉是例外路徑，不該跟複製搶注意力。
async function moreActions(code: CodeItem) {
  const sheet = await actionSheetController.create({
    header: t('merchant.moreHeader', { name: code.owner_name }),
    buttons: [
      {
        text: t('merchant.reportObjectionable'),
        role: 'destructive',
        handler: () => void sendReport(code, 'objectionable'),
      },
      {
        text: t('merchant.blockOwner'),
        role: 'destructive',
        handler: () => void confirmBlock(code),
      },
      { text: t('common.cancel'), role: 'cancel' },
    ],
  })
  await sheet.present()
}

// 封鎖要先確認：它會讓對方所有的碼從這個人眼前消失，而解除封鎖藏在帳號頁，
// 誤觸的人不會知道東西去哪了。
async function confirmBlock(code: CodeItem) {
  if (!auth.isLoggedIn) {
    goLoginToReveal()
    return
  }
  const alert = await alertController.create({
    header: t('merchant.blockConfirmTitle'),
    message: t('merchant.blockConfirmBody', { name: code.owner_name }),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      {
        text: t('merchant.blockConfirm'),
        role: 'destructive',
        handler: () => void doBlock(code),
      },
    ],
  })
  await alert.present()
}

async function doBlock(code: CodeItem) {
  try {
    await api.blockCodeOwner(code.id)
    toast(t('merchant.blockDone', { name: code.owner_name }))
    // 重載讓對方的碼馬上消失 —— 封鎖之後還看得到他，等於沒封鎖。
    await load()
  } catch (e) {
    toast(apiErrorMessage(e, 'merchant.blockFailed'))
  }
}

// 底部的固定操作列拿的是清單第一個碼 —— 後端已經照品質排過，第一個就是最推薦的那組。
// 捲到第三張卡以後上面的按鈕就看不到了，這條列是那時候唯一的出口。
const topCode = computed(() => detail.value?.codes[0] ?? null)

// 上架頁靠這個 query 把服務商預先選好。detail 還沒載完就先不給連結，
// 免得帶出一個沒有 merchant 的網址讓使用者到了那邊還要自己選。
const addCodeLink = computed(() =>
  detail.value ? `/add-code?merchant=${detail.value.merchant.id}` : '',
)

async function shareMerchant() {
  if (!detail.value) return
  // 使用者在分享面板上按取消時這支會 reject，那不是錯誤，不能讓它變成
  // unhandled rejection。桌面瀏覽器沒有 Web Share API 時也是走這裡。
  try {
    await Share.share({
      title: t('merchant.titleSuffix', { name: detail.value.merchant.name }),
      text: rewardText(detail.value.merchant.reward_desc),
      url: detail.value.merchant.signup_url,
    })
  } catch {
    // 取消跟「這個環境不支援」在 plugin 這層分不出來，兩種都安靜收掉：
    // 前者本來就不該有提示，後者跳一句錯誤也幫不上他。
  }
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonButtons slot="start">
          <IonBackButton default-href="/tabs/explore" text="" />
        </IonButtons>
        <IonTitle>{{ detail?.merchant.name ?? '' }}</IonTitle>
        <IonButtons slot="end">
          <!-- 上架入口要常駐在這裡：底下那顆大按鈕只在「這家一組碼都沒有」時出現，
               已經有碼的服務商反而找不到地方上架自己的。 -->
          <IonButton :disabled="!detail" :router-link="addCodeLink">
            <IonIcon slot="icon-only" :icon="addOutline" />
          </IonButton>
          <IonButton :disabled="!detail" @click="shareMerchant">
            <IonIcon slot="icon-only" :icon="shareOutline" />
          </IonButton>
        </IonButtons>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <SkeletonList v-if="loading" :count="3" :lines="3" />

      <EmptyState
        v-else-if="errorMessage"
        :icon="alertCircleOutline"
        tone="danger"
        :title="$t('common.loadFailed')"
        :description="errorMessage"
      />

      <template v-else-if="detail">
        <!-- 獎勵內容是使用者往下滑之前唯一想知道的事，放在最上面用最大的字。
             整塊做成一張卡（商品頁的品牌頭），下面的碼才讀得出是隸屬於它的。 -->
        <div class="page-pad hero-wrap">
          <div class="app-card hero">
            <div class="logo">
              <img
                v-if="detail.merchant.logo_url"
                :src="thumb(detail.merchant.logo_url, 54)"
                :alt="detail.merchant.name"
              />
              <span v-else>{{ detail.merchant.name.trim().charAt(0) }}</span>
            </div>
            <p class="brand">{{ detail.merchant.name }}</p>
            <!-- 還沒補獎勵說明的服務商（多半是剛匯入的）這行會是替代文案，
                 樣式跟著降階，不然「還沒有資訊」會長得像一個賣點。 -->
            <h1 :class="{ pending: !detail.merchant.reward_desc }">
              {{ rewardText(detail.merchant.reward_desc) }}
            </h1>
            <div class="hero-meta">
              <span class="pill neutral">{{ detail.merchant.category_name }}</span>
              <span class="pill" :class="detail.total > 0 ? 'success' : 'neutral'">
                {{ $t('merchant.activeCodes', { count: detail.total }, detail.total) }}
              </span>
            </div>

            <p class="trust">
              <IonIcon :icon="shieldCheckmarkOutline" />
              {{ $t('merchant.reviewed') }}
            </p>
          </div>
        </div>

        <div class="stack page-pad list">
          <div v-for="code in detail.codes" :key="code.id" class="app-card card">
            <div class="head">
              <QualityDot :score="code.quality_score" />
              <!-- 兩種碼混在同一份清單，第一眼要分得出這是誰的推薦碼還是一組折扣碼。
                   折扣碼的優惠內容在下面的備註那行，上架時強制要填。 -->
              <span class="pill neutral">{{ $t(`codeType.${code.code_type}`) }}</span>
              <span v-if="code.worked_count > 0" class="pill success">
                {{ $t('merchant.workedReports', { count: code.worked_count }, code.worked_count) }}
              </span>
              <span v-else class="pill neutral">{{ $t('merchant.noReports') }}</span>
              <button class="more" :aria-label="$t('merchant.moreLabel')" @click="moreActions(code)">
                <IonIcon :icon="ellipsisHorizontal" />
              </button>
            </div>

            <p v-if="code.masked" class="code-text masked">
              <IonIcon :icon="lockClosedOutline" />
              {{ $t('merchant.maskedPlaceholder') }}
            </p>
            <p v-else class="code-text">{{ code.code }}</p>

            <p class="tiny muted owner">
              {{ $t('merchant.sharedBy', { name: code.owner_name }) }}・{{ expiryLabel(code.expires_at) }}
            </p>

            <p v-if="code.note" class="note">{{ code.note }}</p>

            <!-- 要註冊才能拿到推薦碼：遮碼狀態下只給一顆「登入查看」，不給複製或前往
                 註冊——沒有碼就跑去註冊服務商，使用者到了那邊也不知道要填什麼。 -->
            <div v-if="code.masked" class="actions">
              <IonButton expand="block" class="wide grow" @click="goLoginToReveal">
                <IonIcon slot="start" :icon="lockClosedOutline" />
                {{ $t('merchant.loginToReveal') }}
              </IonButton>
            </div>
            <div v-else class="actions">
              <IonButton
                class="wide"
                fill="outline"
                :color="copiedId === code.id ? 'success' : 'primary'"
                @click="copyCode(code)"
              >
                <IonIcon slot="start" :icon="copiedId === code.id ? checkmarkOutline : copyOutline" />
                {{ copiedId === code.id ? $t('merchant.copied') : $t('merchant.copy') }}
              </IonButton>
              <IonButton class="wide grow" @click="openSignup(code)">
                {{ $t('merchant.goSignup') }}
                <IonIcon slot="end" :icon="openOutline" />
              </IonButton>
            </div>

            <div v-if="copiedId === code.id" class="report">
              <template v-if="!reportedIds.has(code.id)">
                <span class="tiny">{{ $t('merchant.askWorks') }}</span>
                <div class="report-actions">
                  <IonButton size="small" fill="clear" color="success" @click="sendReport(code, 'worked')">
                    {{ $t('merchant.worked') }}
                  </IonButton>
                  <IonButton size="small" fill="clear" color="danger" @click="sendReport(code, 'failed')">
                    {{ $t('merchant.failed') }}
                  </IonButton>
                </div>
              </template>
              <span v-else class="tiny muted">{{ $t('merchant.thanksInline') }}</span>
            </div>
          </div>
        </div>

        <EmptyState
          v-if="detail.codes.length === 0"
          :icon="ticketOutline"
          :title="$t('merchant.emptyTitle')"
          :description="$t('merchant.emptyDesc')"
        >
          <IonButton :router-link="addCodeLink" class="wide">{{ $t('merchant.addMine') }}</IonButton>
        </EmptyState>
      </template>
    </IonContent>

    <!-- 固定在底部的轉換列。沒有碼的時候不出現 —— 那時候按下去也沒有東西可以帶走。 -->
    <IonFooter v-if="topCode" class="cta">
      <div class="cta-inner">
        <template v-if="topCode.masked">
          <IonButton expand="block" class="wide" @click="goLoginToReveal">
            <IonIcon slot="start" :icon="lockClosedOutline" />
            {{ $t('merchant.loginToReveal') }}
          </IonButton>
        </template>
        <template v-else>
          <p class="cta-hint tiny muted">{{ $t('merchant.ctaBest') }}</p>
          <div class="cta-actions">
            <IonButton
              class="wide"
              fill="outline"
              :color="copiedId === topCode.id ? 'success' : 'primary'"
              @click="copyCode(topCode)"
            >
              <IonIcon slot="start" :icon="copiedId === topCode.id ? checkmarkOutline : copyOutline" />
              {{ copiedId === topCode.id ? $t('merchant.copied') : $t('merchant.copy') }}
            </IonButton>
            <IonButton class="wide grow" @click="openSignup(topCode)">
              {{ $t('merchant.goSignup') }}
              <IonIcon slot="end" :icon="openOutline" />
            </IonButton>
          </div>
        </template>
      </div>
    </IonFooter>
  </IonPage>
</template>

<style scoped>
.hero-wrap {
  padding-top: 4px;
  padding-bottom: 18px;
}

.hero {
  padding: 18px;
}

.brand {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-muted);
}

.logo {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  margin-bottom: 14px;
  border-radius: var(--app-radius-lg);
  overflow: hidden;
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 22px;
  font-weight: 700;
}

.logo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.hero h1 {
  margin: 3px 0 0;
  font-size: 25px;
  font-weight: 700;
  line-height: 1.28;
  letter-spacing: -0.02em;
}

/* 還沒補獎勵說明時這行是替代文案，不該用跟真獎勵一樣的份量喊出來。 */
.hero h1.pending {
  font-size: 19px;
  font-weight: 600;
  color: var(--app-muted);
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}

.trust {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 14px 0 0;
  padding-top: 12px;
  border-top: 1px solid var(--app-line);
  font-size: 12px;
  font-weight: 600;
  color: var(--app-muted);
}

.trust ion-icon {
  color: var(--ion-color-success);
  font-size: 15px;
}

/* 底部有固定的操作列，清單要多留一段，最後一張卡才不會被蓋住。 */
.list {
  padding-bottom: 20px;
}

.card {
  padding: 16px;
}

.head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

/* 檢舉／封鎖的入口。推到最右邊，跟左邊的品質與回報標籤分開 —— 那些是資訊，
   這個是動作。做得小而不顯眼：需要的人找得到就好。 */
.more {
  margin-left: auto;
  padding: 2px 4px;
  border: 0;
  background: none;
  color: var(--app-muted);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

.code-text {
  margin: 0;
}

.code-text.masked {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-style: italic;
}

.owner {
  margin: 8px 0 0;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--app-line);
}

.note {
  margin: 12px 0 0;
  font-size: 14px;
  line-height: 1.55;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 14px;
}

/* 「前往註冊」是這頁的主要轉換動作，讓它吃掉剩下的寬度。 */
.grow {
  flex: 1;
}

.report {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--app-line);
}

.report-actions {
  display: flex;
  gap: 2px;
}

/* ── 底部操作列 ─────────────────────────────────────── */

/* ion-footer 預設會畫一條上框線，這裡改用陰影往上打，跟卡片式版面比較搭。
   ion-padding 的 safe area 只有 ion-content 有，底部的 home indicator 要自己讓。 */
.cta {
  background: var(--app-surface);
  box-shadow: 0 -2px 12px rgba(64, 72, 90, 0.1);
}

.cta::before {
  display: none;
}

.cta-inner {
  padding: 10px 16px calc(10px + env(safe-area-inset-bottom));
}

.cta-hint {
  margin: 0 0 6px;
}

.cta-actions {
  display: flex;
  gap: 8px;
}

@media (prefers-color-scheme: dark) {
  .cta {
    border-top: 1px solid var(--app-line);
    box-shadow: none;
  }
}

/* 詳情頁只講一家服務商，不跟著平板的內容欄放到 940 —— 一家通常也就
   一到三組碼，排成雙欄反而讓人以為是兩家。底部的 CTA 一起收窄，
   不然會變成一顆橫跨整個螢幕的按鈕（footer 不在 content 裡，要各自設）。 */
ion-content,
.cta {
  --app-content-max: 680px;
}
</style>
