<script setup lang="ts">
import { Clipboard } from '@capacitor/clipboard'
import {
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonIcon,
  IonPage,
  IonTitle,
  IonToolbar,
  alertController,
  toastController,
} from '@ionic/vue'
import { addOutline, checkmarkOutline, copyOutline, timeOutline } from 'ionicons/icons'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import EmptyState from '../components/EmptyState.vue'
import { daysUntilExpiry, expiryLabel } from '../i18n'
import type { CopiedCode } from '../stores/record'
import { useRecordStore } from '../stores/record'

const record = useRecordStore()
const { t, locale } = useI18n()

onMounted(() => record.load())

const isEmpty = computed(() => record.codes.length === 0 && record.merchants.length === 0)

// 沒有到期日的碼永遠不算過期。
function isExpired(iso: string | null) {
  return iso !== null && daysUntilExpiry(iso) <= 0
}

// 日期格式跟著介面語言走，不然日文介面會看到中文的月／日排法。
function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(locale.value, { month: 'numeric', day: 'numeric' })
}

const justCopied = ref('')

async function recopy(c: CopiedCode) {
  await Clipboard.write({ string: c.code })
  justCopied.value = c.codeId
  const toast = await toastController.create({
    message: t('record.copiedToast'),
    duration: 1800,
    position: 'bottom',
  })
  await toast.present()
}

// 清除是不可逆的，而且紀錄是使用者唯一能找回剛剛那個碼的地方，先問一次。
async function confirmClear(which: 'codes' | 'merchants') {
  const alert = await alertController.create({
    header: t(which === 'codes' ? 'record.clearCodesTitle' : 'record.clearMerchantsTitle'),
    message: t('record.clearWarning'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      { text: t('record.clearConfirm'), role: 'destructive' },
    ],
  })
  await alert.present()
  const { role } = await alert.onDidDismiss()
  if (role !== 'destructive') return

  if (which === 'codes') await record.clearCodes()
  else await record.clearMerchants()
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonTitle>{{ $t('tabs.record') }}</IonTitle>
        <IonButtons slot="end">
          <!-- 上架入口在每個瀏覽頁都有：想上架的念頭多半是在看別人的碼時冒出來的，
               不該逼使用者先切回「我的碼」分頁才找得到。 -->
          <IonButton router-link="/add-code">
            <IonIcon slot="icon-only" :icon="addOutline" />
          </IonButton>
        </IonButtons>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <div class="page-heading">
        <h1>{{ $t('record.heading') }}</h1>
        <p>{{ $t('record.lead') }}</p>
      </div>

      <EmptyState
        v-if="isEmpty"
        :icon="timeOutline"
        :title="$t('record.emptyTitle')"
        :description="$t('record.emptyDesc')"
      >
        <IonButton router-link="/tabs/explore" class="wide">{{ $t('record.goExplore') }}</IonButton>
      </EmptyState>

      <template v-else>
        <!-- 複製過的碼放上面：複製完跑去註冊、回來想再貼一次，是這一頁最主要的用途。 -->
        <section v-if="record.codes.length" class="section">
          <div class="sec-head page-pad">
            <h2>{{ $t('record.sectionCodes') }}</h2>
            <button class="clear" @click="confirmClear('codes')">{{ $t('record.clear') }}</button>
          </div>

          <div class="stack page-pad">
            <div v-for="c in record.codes" :key="c.codeId" class="app-card card">
              <div class="row" @click="$router.push(`/merchant/${c.merchantSlug}`)">
                <div class="logo">
                  <img v-if="c.merchantLogo" :src="c.merchantLogo" :alt="c.merchantName" />
                  <span v-else>{{ c.merchantName.trim().charAt(0) }}</span>
                </div>
                <div class="body">
                  <p class="brand">{{ c.merchantName }}</p>
                  <p class="code-text sm">{{ c.code }}</p>
                </div>
              </div>

              <div class="foot">
                <!-- 到期的碼留著不刪：使用者可能想知道「我那時候用的是哪一組」，
                     但要標清楚它已經沒用了，免得又貼一次去撞牆。 -->
                <span v-if="isExpired(c.expiresAt)" class="pill danger">
                  {{ $t('record.expired') }}
                </span>
                <span v-else class="tiny muted">{{ expiryLabel(c.expiresAt) }}</span>

                <span class="tiny muted when">{{ $t('record.copiedOn', { date: formatDate(c.copiedAt) }) }}</span>

                <IonButton
                  size="small"
                  fill="outline"
                  :color="justCopied === c.codeId ? 'success' : 'primary'"
                  @click="recopy(c)"
                >
                  <IonIcon slot="start" :icon="justCopied === c.codeId ? checkmarkOutline : copyOutline" />
                  {{ justCopied === c.codeId ? $t('merchant.copied') : $t('merchant.copy') }}
                </IonButton>
              </div>
            </div>
          </div>
        </section>

        <section v-if="record.merchants.length" class="section">
          <div class="sec-head page-pad">
            <h2>{{ $t('record.sectionMerchants') }}</h2>
            <button class="clear" @click="confirmClear('merchants')">{{ $t('record.clear') }}</button>
          </div>

          <div class="stack page-pad list">
            <div
              v-for="m in record.merchants"
              :key="m.slug"
              class="app-card tappable row pad"
              @click="$router.push(`/merchant/${m.slug}`)"
            >
              <div class="logo">
                <img v-if="m.logo" :src="m.logo" :alt="m.name" />
                <span v-else>{{ m.name.trim().charAt(0) }}</span>
              </div>
              <div class="body">
                <p class="name">{{ m.name }}</p>
                <p class="tiny muted">{{ $t('record.viewedOn', { date: formatDate(m.viewedAt) }) }}</p>
              </div>
            </div>
          </div>
        </section>
      </template>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.section {
  margin-bottom: 24px;
}

.sec-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.sec-head h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.clear {
  border: 0;
  background: none;
  color: var(--app-muted);
  font-family: inherit;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.card {
  padding: 14px;
}

.row {
  display: flex;
  align-items: center;
  gap: 14px;
  cursor: pointer;
}

.row.pad {
  padding: 14px;
}

.logo {
  display: grid;
  place-items: center;
  flex: none;
  width: 44px;
  height: 44px;
  border-radius: var(--app-radius);
  overflow: hidden;
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 18px;
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

/* 卡片裡的碼比服務商頁小一階：這裡是回顧，不是要一個字一個字核對的主角。 */
.code-text.sm {
  margin: 2px 0 0;
  font-size: 16px;
}

.name {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.foot {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--app-line);
}

/* 複製鈕靠右，前面的到期與時間各自佔原本的位置。 */
.foot ion-button {
  margin-left: auto;
}

.when {
  white-space: nowrap;
}

.list {
  padding-bottom: 8px;
}

/* ── 平板 ───────────────────────────────────────────── */

/* 兩個區塊（複製過的碼、看過的服務商）用的都是全域的 .stack，
   在這裡只覆寫它在平板下的排列方向，其他頁不受影響。 */
@media (min-width: 768px) {
  .stack {
    display: grid;
    grid-template-columns: 1fr 1fr;
    align-items: start;
  }
}
</style>
