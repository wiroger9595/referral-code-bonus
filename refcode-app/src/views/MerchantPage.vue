<script setup lang="ts">
import { Browser } from '@capacitor/browser'
import { Clipboard } from '@capacitor/clipboard'
import { Share } from '@capacitor/share'
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
  toastController,
} from '@ionic/vue'
import {
  alertCircleOutline,
  checkmarkOutline,
  copyOutline,
  lockClosedOutline,
  openOutline,
  shareOutline,
  ticketOutline,
} from 'ionicons/icons'
import { onMounted, ref } from 'vue'
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
        <!-- 獎勵內容是使用者往下滑之前唯一想知道的事，放在最上面用最大的字。 -->
        <div class="hero page-pad">
          <div class="logo">
            <img
              v-if="detail.merchant.logo_url"
              :src="detail.merchant.logo_url"
              :alt="detail.merchant.name"
            />
            <span v-else>{{ detail.merchant.name.trim().charAt(0) }}</span>
          </div>
          <h1>{{ detail.merchant.reward_desc }}</h1>
          <div class="hero-meta">
            <span class="pill neutral">{{ detail.merchant.category_name }}</span>
            <span class="pill" :class="detail.total > 0 ? 'success' : 'neutral'">
              {{ $t('merchant.activeCodes', { count: detail.total }, detail.total) }}
            </span>
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
  </IonPage>
</template>

<style scoped>
.hero {
  padding-top: 4px;
  padding-bottom: 22px;
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
  margin: 0;
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

.list {
  padding-bottom: 28px;
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
</style>
