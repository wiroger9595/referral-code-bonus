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
  IonTitle,
  IonToolbar,
} from '@ionic/vue'
import type { RefresherCustomEvent } from '@ionic/vue'
import { addOutline, alertCircleOutline, pricetagsOutline } from 'ionicons/icons'
import { onActivated, onMounted, ref } from 'vue'
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

onMounted(load)
// 從新增頁返回時 Ionic 會沿用快取的頁面，要重新抓才看得到剛上架的碼。
onActivated(load)

async function refresh(event: RefresherCustomEvent) {
  await load()
  event.target.complete()
}

// 日期格式跟著介面語言走，不然日文介面會看到中文的月／日排法。
function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(locale.value, { month: 'numeric', day: 'numeric' })
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonTitle>{{ $t('myCodes.title') }}</IonTitle>
        <IonButtons slot="end">
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

      <div v-else class="stack page-pad list">
        <div v-for="c in codes" :key="c.id" class="app-card card">
          <div class="head">
            <div class="who">
              <div class="logo">
                <img v-if="c.merchant_logo_url" :src="c.merchant_logo_url" :alt="c.merchant_name" />
                <span v-else>{{ c.merchant_name.trim().charAt(0) }}</span>
              </div>
              <div>
                <h2>{{ c.merchant_name }}</h2>
                <p class="mono tiny muted">{{ c.code }}</p>
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
        </div>
      </div>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.list {
  padding-top: 8px;
  padding-bottom: 24px;
}

.card {
  padding: 16px;
}

.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
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
}

.stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  margin-top: 16px;
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
</style>
