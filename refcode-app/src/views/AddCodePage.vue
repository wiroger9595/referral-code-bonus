<script setup lang="ts">
import {
  IonBackButton,
  IonButton,
  IonButtons,
  IonContent,
  IonDatetime,
  IonHeader,
  IonIcon,
  IonInput,
  IonItem,
  IonList,
  IonPage,
  IonSelect,
  IonSelectOption,
  IonTitle,
  IonToolbar,
  toastController,
} from '@ionic/vue'
import { alertCircle, giftOutline, shieldCheckmarkOutline } from 'ionicons/icons'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { ApiError, api } from '../api/client'
import type { MerchantSummary } from '../api/types'
import { apiErrorMessage } from '../i18n'

const router = useRouter()
const { t } = useI18n()

const merchants = ref<MerchantSummary[]>([])
const merchantId = ref('')
const code = ref('')
const note = ref('')
const loading = ref(false)
const errorMessage = ref('')

// 後端限制最長一年，預設先給三個月 —— 大部分活動不會撐更久。
const defaultExpiry = new Date(Date.now() + 90 * 86400000).toISOString()
const expiresAt = ref(defaultExpiry)
const maxExpiry = new Date(Date.now() + 364 * 86400000).toISOString()

onMounted(async () => {
  try {
    merchants.value = (await api.listMerchants()).merchants
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'addCode.loadMerchantsFailed')
  }
})

const selected = computed(() => merchants.value.find((m) => m.id === merchantId.value))

async function submit() {
  errorMessage.value = ''
  if (!merchantId.value) {
    errorMessage.value = t('addCode.chooseMerchant')
    return
  }

  loading.value = true
  try {
    await api.createCode({
      merchant_id: merchantId.value,
      code: code.value.trim(),
      note: note.value.trim(),
      expires_at: new Date(expiresAt.value).toISOString(),
    })
    const toast = await toastController.create({
      message: t('addCode.submitted'),
      duration: 2200,
    })
    await toast.present()
    router.replace('/tabs/my-codes')
  } catch (e) {
    // 撞到免費方案上限時後端回 pro_required，這是要導去 paywall 而不是
    // 顯示一行紅字 —— 使用者在這裡才最有動機升級。
    if (e instanceof ApiError && e.code === 'pro_required') {
      router.push('/pro')
      return
    }
    errorMessage.value = apiErrorMessage(e, 'addCode.submitFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonButtons slot="start">
          <IonBackButton default-href="/tabs/my-codes" text="" />
        </IonButtons>
        <IonTitle>{{ $t('addCode.title') }}</IonTitle>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <div class="page-pad form">
        <section>
          <h2 class="label">{{ $t('addCode.sectionCode') }}</h2>
          <IonList class="app-form" lines="full">
            <IonItem>
              <IonSelect
                v-model="merchantId"
                :label="$t('addCode.merchant')"
                label-placement="stacked"
                :placeholder="$t('addCode.merchantPlaceholder')"
                interface="action-sheet"
              >
                <IonSelectOption v-for="m in merchants" :key="m.id" :value="m.id">
                  {{ m.name }}
                </IonSelectOption>
              </IonSelect>
            </IonItem>

            <IonItem>
              <IonInput
                v-model="code"
                :label="$t('addCode.code')"
                label-placement="stacked"
                :placeholder="$t('addCode.codePlaceholder')"
                autocapitalize="off"
                autocorrect="off"
                class="mono"
              />
            </IonItem>

            <IonItem>
              <IonInput
                v-model="note"
                :label="$t('addCode.note')"
                label-placement="stacked"
                :placeholder="$t('addCode.notePlaceholder')"
              />
            </IonItem>
          </IonList>

          <!-- 選了服務商就把獎勵秀出來，確認選對家了。 -->
          <div v-if="selected?.reward_desc" class="callout">
            <IonIcon :icon="giftOutline" />
            <span>{{ selected.reward_desc }}</span>
          </div>
        </section>

        <section>
          <h2 class="label">{{ $t('addCode.sectionExpiry') }}</h2>
          <p class="tiny muted sub">{{ $t('addCode.expiryHint') }}</p>
          <div class="app-card picker">
            <IonDatetime
              v-model="expiresAt"
              presentation="date"
              :min="new Date().toISOString()"
              :max="maxExpiry"
            />
          </div>
        </section>

        <div v-if="errorMessage" class="error">
          <IonIcon :icon="alertCircle" />
          <span>{{ errorMessage }}</span>
        </div>

        <div class="callout muted">
          <IonIcon :icon="shieldCheckmarkOutline" />
          <span>{{ $t('addCode.reviewNotice') }}</span>
        </div>

        <IonButton expand="block" class="wide" :disabled="loading" @click="submit">{{ $t('addCode.submit') }}</IonButton>
      </div>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 26px;
  padding-top: 8px;
  padding-bottom: 40px;
}

.label {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: var(--app-muted);
}

.sub {
  margin: -6px 0 10px;
}

.picker {
  padding: 4px;
  overflow: hidden;
}

.picker ion-datetime {
  --background: transparent;
  width: 100%;
}

.callout {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
  padding: 12px 14px;
  border-radius: var(--app-radius-sm);
  background: var(--app-tint);
  color: var(--ion-color-primary);
  font-size: 13px;
  line-height: 1.5;
}

.callout.muted {
  margin-top: 0;
  background: rgba(var(--ion-color-medium-rgb), 0.1);
  color: var(--app-muted);
}

.callout ion-icon {
  flex: none;
  font-size: 17px;
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
</style>
