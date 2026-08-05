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
  toastController,
} from '@ionic/vue'
import {
  alertCircleOutline,
  checkmarkOutline,
  copyOutline,
  lockClosedOutline,
  openOutline,
  shareOutline,
  shieldCheckmarkOutline,
  ticketOutline,
} from 'ionicons/icons'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { api } from '../api/client'
import type { CodeItem, MerchantDetail, ReportResult } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import QualityDot from '../components/QualityDot.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { apiErrorMessage, expiryLabel } from '../i18n'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
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
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'common.loadFailed')
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function toast(message: string) {
  const t = await toastController.create({ message, duration: 1800, position: 'bottom' })
  await t.present()
}

async function copyCode(code: CodeItem) {
  if (code.masked || code.code === null) return // 按鈕在遮碼狀態下不會出現，這只是保險
  await Clipboard.write({ string: code.code })
  copiedId.value = code.id
  api.track(code.id, 'copy').catch(() => {})
  toast(t('merchant.copiedToast'))
}

// 要註冊才能拿到推薦碼：帶著 redirect 讓登入完直接回到這頁，不用重新找一次服務商。
function goLoginToReveal() {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

async function openSignup(code: CodeItem) {
  if (!detail.value) return
  api.track(code.id, 'click').catch(() => {})
  await Browser.open({ url: detail.value.merchant.signup_url })
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

// 底部的固定操作列拿的是清單第一個碼 —— 後端已經照品質排過，第一個就是最推薦的那組。
// 捲到第三張卡以後上面的按鈕就看不到了，這條列是那時候唯一的出口。
const topCode = computed(() => detail.value?.codes[0] ?? null)

async function shareMerchant() {
  if (!detail.value) return
  await Share.share({
    title: t('merchant.titleSuffix', { name: detail.value.merchant.name }),
    text: detail.value.merchant.reward_desc,
    url: detail.value.merchant.signup_url,
  })
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
                :src="detail.merchant.logo_url"
                :alt="detail.merchant.name"
              />
              <span v-else>{{ detail.merchant.name.trim().charAt(0) }}</span>
            </div>
            <p class="brand">{{ detail.merchant.name }}</p>
            <h1>{{ detail.merchant.reward_desc }}</h1>
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
              <span v-if="code.worked_count > 0" class="pill success">
                {{ $t('merchant.workedReports', { count: code.worked_count }, code.worked_count) }}
              </span>
              <span v-else class="pill neutral">{{ $t('merchant.noReports') }}</span>
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
          <IonButton router-link="/add-code" class="wide">{{ $t('merchant.addMine') }}</IonButton>
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
</style>
