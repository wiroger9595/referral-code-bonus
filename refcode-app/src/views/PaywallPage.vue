<script setup lang="ts">
import type { PurchasesPackage } from '@revenuecat/purchases-capacitor'
import {
  IonBackButton,
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonIcon,
  IonPage,
  IonSpinner,
  IonTitle,
  IonToolbar,
  toastController,
} from '@ionic/vue'
import {
  alertCircle,
  barChartOutline,
  checkmarkCircle,
  flashOutline,
  infiniteOutline,
  sparklesOutline,
  timeOutline,
} from 'ionicons/icons'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { PurchaseCancelled } from '../api/purchases'
import { useAuthStore } from '../stores/auth'
import { NotIdentifiedError, useSubscriptionStore } from '../stores/subscription'

const subs = useSubscriptionStore()
const auth = useAuthStore()
const router = useRouter()
const { t } = useI18n()

// Apple 要求 paywall 上要看得到服務條款與隱私權政策的連結，不是只在帳號頁有。
const siteUrl = import.meta.env.VITE_SITE_URL ?? ''

const selected = ref<string | null>(null)
const buying = ref(false)
const errorMessage = ref('')

const perks = [
  { icon: infiniteOutline, key: 'unlimited' },
  { icon: timeOutline, key: 'ranking' },
  { icon: barChartOutline, key: 'stats' },
  { icon: flashOutline, key: 'priority' },
] as const

onMounted(async () => {
  try {
    await subs.loadPackages()
    // 預設選中間那個方案；只有一個就選它。年繳通常擺中間，也是我們想推的。
    const list = subs.packages
    selected.value = list[Math.floor(list.length / 2)]?.identifier ?? null
  } catch {
    errorMessage.value = t('pro.loadFailed')
  }
})

function priceOf(pkg: PurchasesPackage) {
  // 一律用商店回傳的當地貨幣字串，不要自己組 —— 幣別、稅、格式各國都不一樣。
  return pkg.product.priceString
}

// 免費試用。introPrice 同時代表「優惠價」與「免費試用」兩種促銷，
// price === 0 才是真的免費試用 —— 折扣首期（例如首月半價）不能講成免費。
//
// 天數也用商店回傳的，不要寫死 7 天：試用長度是在 Play Console / App Store
// Connect 設定的，寫死的話那邊改了這裡就會騙人。
function trialOf(pkg: PurchasesPackage) {
  const intro = pkg.product.introPrice
  return intro && intro.price === 0 ? intro : null
}

function trialLabel(pkg: PurchasesPackage): string | null {
  const intro = trialOf(pkg)
  if (!intro) return null

  const count = intro.periodNumberOfUnits
  switch (intro.periodUnit) {
    case 'DAY':
      return t('pro.trialDay', { count })
    case 'WEEK':
      return t('pro.trialWeek', { count })
    case 'MONTH':
      return t('pro.trialMonth', { count })
    default:
      return null
  }
}

// 有試用期就要多一句揭露（兩家商店都要求講清楚「什麼時候開始收錢」）。
// 只看目前選中的那個方案 —— 月費有試用、年費沒有的情況下，
// 不分方案一律顯示會變成對其中一個說謊。
const selectedHasTrial = computed(() => {
  const pkg = subs.packages.find((p) => p.identifier === selected.value)
  return pkg ? trialOf(pkg) !== null : false
})

async function purchase() {
  const pkg = subs.packages.find((p) => p.identifier === selected.value)
  if (!pkg) return

  errorMessage.value = ''
  buying.value = true
  try {
    // 沒有 user id 就不該走到這頁（route 有 requiresAuth），防禦性再擋一次。
    const userID = auth.user?.id
    if (!userID) return

    await subs.purchase(pkg, userID)
    const toast = await toastController.create({ message: t('pro.thanks'), duration: 2200 })
    await toast.present()
    router.back()
  } catch (e) {
    if (e instanceof PurchaseCancelled) return
    // 綁定不上就不會走到扣款，訊息要講「稍後再試」而不是「購買失敗」——
    // 後者會讓人以為錢已經扣了。
    errorMessage.value =
      e instanceof NotIdentifiedError ? t('pro.notLinked') : t('pro.purchaseFailed')
  } finally {
    buying.value = false
  }
}

async function restore() {
  errorMessage.value = ''
  buying.value = true
  try {
    const ok = await subs.restorePurchases()
    const toast = await toastController.create({
      message: ok ? t('pro.restored') : t('pro.nothingToRestore'),
      duration: 2200,
    })
    await toast.present()
    if (ok) router.back()
  } catch {
    errorMessage.value = t('pro.restoreFailed')
  } finally {
    buying.value = false
  }
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonButtons slot="start">
          <IonBackButton default-href="/tabs/account" text="" />
        </IonButtons>
        <IonTitle>{{ $t('pro.title') }}</IonTitle>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <div class="page-pad wrap">
        <div class="hero">
          <div class="mark"><IonIcon :icon="sparklesOutline" /></div>
          <h1>{{ $t('pro.heroTitle') }}</h1>
          <p class="muted">{{ $t('pro.heroDesc') }}</p>
        </div>

        <div class="app-card perks">
          <div v-for="p in perks" :key="p.key" class="perk">
            <IonIcon :icon="p.icon" />
            <div>
              <h3>{{ $t(`pro.perks.${p.key}.title`) }}</h3>
              <p class="tiny muted">{{ $t(`pro.perks.${p.key}.desc`) }}</p>
            </div>
          </div>
        </div>

        <!-- 已經是 Pro 就不要再賣一次，兩家商店都會退件。 -->
        <div v-if="subs.isPro" class="app-card active-state">
          <IonIcon :icon="checkmarkCircle" color="success" />
          <div>
            <h3>{{ $t('pro.alreadyPro') }}</h3>
            <p v-if="subs.expiresAt" class="tiny muted">
              {{ $t('pro.renewsOn', { date: new Date(subs.expiresAt).toLocaleDateString() }) }}
            </p>
          </div>
        </div>

        <!-- 瀏覽器沒有 RevenueCat SDK，購買只能在 app 裡做。 -->
        <p v-else-if="!subs.available" class="tiny muted notice">{{ $t('pro.nativeOnly') }}</p>

        <template v-else>
          <div v-if="subs.loading" class="center"><IonSpinner /></div>

          <div v-else-if="subs.packages.length" class="stack">
            <button
              v-for="pkg in subs.packages"
              :key="pkg.identifier"
              class="app-card plan"
              :class="{ on: selected === pkg.identifier }"
              @click="selected = pkg.identifier"
            >
              <div class="plan-body">
                <h3>{{ pkg.product.title }}</h3>
                <p v-if="trialLabel(pkg)" class="tiny trial">{{ trialLabel(pkg) }}</p>
                <p class="tiny muted">{{ pkg.product.description }}</p>
              </div>
              <strong>{{ priceOf(pkg) }}</strong>
            </button>
          </div>

          <p v-else class="tiny muted notice">{{ $t('pro.noPackages') }}</p>

          <div v-if="errorMessage" class="error">
            <IonIcon :icon="alertCircle" />
            <span>{{ errorMessage }}</span>
          </div>

          <IonButton
            expand="block"
            class="wide"
            :disabled="buying || !selected"
            @click="purchase"
          >
            {{ $t('pro.subscribe') }}
          </IonButton>

          <!-- 兩家商店都要求 app 內有「恢復購買」，沒有會被退件。 -->
          <IonButton expand="block" fill="clear" size="small" :disabled="buying" @click="restore">
            {{ $t('pro.restore') }}
          </IonButton>

          <!-- 自動續訂的揭露是硬性要求：期間、價格、何時扣款、怎麼取消。
               有試用期時還要額外講「試用結束才開始收費」，少了這句會被退件。 -->
          <p v-if="selectedHasTrial" class="tiny muted terms">{{ $t('pro.trialDisclosure') }}</p>
          <p class="tiny muted terms">{{ $t('pro.disclosure') }}</p>

          <p v-if="siteUrl" class="tiny muted terms links">
            <a :href="`${siteUrl}/terms`" target="_blank">{{ $t('account.terms') }}</a>
            <span aria-hidden="true">·</span>
            <a :href="`${siteUrl}/privacy`" target="_blank">{{ $t('account.privacy') }}</a>
          </p>
        </template>
      </div>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.wrap {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 8px;
  padding-bottom: 40px;
}

.hero {
  padding: 8px 0;
}

.mark {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  margin-bottom: 16px;
  border-radius: var(--app-radius);
  background: var(--ion-color-primary);
  color: var(--ion-color-primary-contrast);
  font-size: 25px;
}

.hero h1 {
  margin: 0;
  font-size: 27px;
  font-weight: 750;
  letter-spacing: -0.02em;
  line-height: 1.2;
}

.hero p {
  margin: 8px 0 0;
  font-size: 14px;
  line-height: 1.6;
}

.perks {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 18px 16px;
}

.perk {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.perk ion-icon {
  flex: none;
  margin-top: 2px;
  color: var(--ion-color-primary);
  font-size: 20px;
}

.perk h3,
.active-state h3 {
  margin: 0 0 2px;
  font-size: 15px;
  font-weight: 700;
}

.perk p,
.active-state p {
  margin: 0;
}

.active-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
}

.active-state ion-icon {
  flex: none;
  font-size: 24px;
}

.plan {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px;
  width: 100%;
  text-align: start;
  font-family: inherit;
  color: inherit;
  cursor: pointer;
}

/* 選中的方案用主色描邊，不要換底色 —— 換底色會讓兩張卡看起來是不同東西。 */
.plan.on {
  border-color: var(--ion-color-primary);
  box-shadow: 0 0 0 1px var(--ion-color-primary) inset, var(--app-shadow);
}

.plan-body {
  min-width: 0;
}

.plan strong {
  flex: none;
  font-size: 17px;
  font-weight: 750;
}

.links {
  display: flex;
  justify-content: center;
  gap: 10px;
}

.links a {
  color: var(--app-muted);
  text-decoration: underline;
}

.notice,
.terms {
  margin: 0;
  text-align: center;
  line-height: 1.6;
}

/* 試用期是這頁最重要的一句話，要跟灰色的說明文字分開。 */
.trial {
  margin: 2px 0 0;
  color: var(--ion-color-success, #2dd36f);
  font-weight: 600;
}

.error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 14px;
  border-radius: var(--app-radius-sm);
  background: rgba(var(--ion-color-danger-rgb), 0.1);
  color: var(--ion-color-danger-shade);
  font-size: 13px;
  line-height: 1.5;
}

@media (prefers-color-scheme: dark) {
  .error {
    color: var(--ion-color-danger-tint);
  }
}
/* 單欄的頁不跟著平板的內容欄一起放寬 —— 表單拉到 940 只會變成很長的一行，
   眼睛從欄位標題找到輸入框要橫跨整個螢幕。手機寬度小於這個值，不受影響。 */
ion-content {
  --app-content-max: 600px;
}
</style>
