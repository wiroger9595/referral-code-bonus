<script setup lang="ts">
import {
  IonBackButton,
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonIcon,
  IonInput,
  IonItem,
  IonList,
  IonPage,
  IonToolbar,
} from '@ionic/vue'
import { alertCircle, checkmarkCircle, lockClosedOutline } from 'ionicons/icons'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { api } from '../api/client'
import { apiErrorMessage } from '../i18n'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
// 信件語言跟著使用者當下選的介面語言，不是裝置語言 —— 他手動選過就是他要看的那個。
const { locale } = useI18n()

// 兩步走同一頁：寄碼之後只是換掉表單內容，回上一頁仍然是離開整個重設流程。
const step = ref<'email' | 'code'>('email')
const email = ref('')
const code = ref('')
const password = ref('')
const loading = ref(false)
const errorMessage = ref('')
const notice = ref('')

async function sendCode() {
  errorMessage.value = ''
  notice.value = ''
  loading.value = true
  try {
    await api.forgotPassword(email.value.trim(), locale.value)
    // 後端不透露這個 email 有沒有註冊過，所以這裡一律說「寄出去了」。
    step.value = 'code'
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'login.connectionFailed')
  } finally {
    loading.value = false
  }
}

async function resend() {
  errorMessage.value = ''
  notice.value = ''
  loading.value = true
  try {
    await api.forgotPassword(email.value.trim(), locale.value)
    // 舊碼在後端已經被覆蓋掉了，留在輸入框裡只會讓人拿它去試。
    code.value = ''
    notice.value = 'resent'
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'login.connectionFailed')
  } finally {
    loading.value = false
  }
}

async function submit() {
  errorMessage.value = ''
  notice.value = ''
  loading.value = true
  try {
    await auth.resetPassword(email.value.trim(), code.value.trim(), password.value)
    const redirect = route.query.redirect
    router.replace(typeof redirect === 'string' ? redirect : '/tabs/my-codes')
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'login.connectionFailed')
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
          <IonBackButton default-href="/login" text="" />
        </IonButtons>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <div class="wrap page-pad">
        <div class="brand">
          <div class="mark"><IonIcon :icon="lockClosedOutline" /></div>
          <h1>{{ $t('forgot.title') }}</h1>
          <p class="muted">
            {{ step === 'email' ? $t('forgot.leadEmail') : $t('forgot.leadCode', { email: email.trim() }) }}
          </p>
        </div>

        <IonList v-if="step === 'email'" class="app-form" lines="full">
          <IonItem>
            <IonInput
              v-model="email"
              label="Email"
              label-placement="stacked"
              type="email"
              autocomplete="email"
              inputmode="email"
            />
          </IonItem>
        </IonList>

        <IonList v-else class="app-form" lines="full">
          <IonItem>
            <IonInput
              v-model="code"
              :label="$t('forgot.code')"
              label-placement="stacked"
              inputmode="numeric"
              autocomplete="one-time-code"
              :maxlength="6"
              :placeholder="$t('forgot.codePlaceholder')"
            />
          </IonItem>
          <IonItem>
            <IonInput
              v-model="password"
              :label="$t('forgot.newPassword')"
              label-placement="stacked"
              type="password"
              autocomplete="new-password"
            />
          </IonItem>
        </IonList>

        <div v-if="errorMessage" class="error">
          <IonIcon :icon="alertCircle" />
          <span>{{ errorMessage }}</span>
        </div>
        <div v-else-if="notice" class="notice">
          <IonIcon :icon="checkmarkCircle" />
          <span>{{ $t('forgot.resent') }}</span>
        </div>

        <IonButton
          v-if="step === 'email'"
          expand="block"
          class="wide"
          :disabled="loading || !email.trim()"
          @click="sendCode"
        >
          {{ $t('forgot.sendCode') }}
        </IonButton>

        <template v-else>
          <IonButton
            expand="block"
            class="wide"
            :disabled="loading || !code.trim() || !password"
            @click="submit"
          >
            {{ $t('forgot.submit') }}
          </IonButton>
          <IonButton expand="block" fill="clear" size="small" :disabled="loading" @click="resend">
            {{ $t('forgot.resend') }}
          </IonButton>
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
  padding-bottom: 40px;
}

.brand {
  padding: 8px 0 8px;
}

.mark {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  margin-bottom: 18px;
  border-radius: var(--app-radius-lg);
  background: var(--ion-color-primary);
  color: var(--ion-color-primary-contrast);
  font-size: 25px;
}

.brand h1 {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.brand p {
  margin: 8px 0 0;
  font-size: 14px;
  line-height: 1.6;
}

.error,
.notice {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 14px;
  border-radius: var(--app-radius-sm);
  font-size: 13px;
  line-height: 1.5;
}

.error {
  background: rgba(var(--ion-color-danger-rgb), 0.1);
  color: var(--ion-color-danger-shade);
}

.notice {
  background: rgba(var(--ion-color-success-rgb), 0.1);
  color: var(--ion-color-success-shade);
}

@media (prefers-color-scheme: dark) {
  .error {
    color: var(--ion-color-danger-tint);
  }

  .notice {
    color: var(--ion-color-success-tint);
  }
}
/* 單欄的頁不跟著平板的內容欄一起放寬 —— 表單拉到 940 只會變成很長的一行，
   眼睛從欄位標題找到輸入框要橫跨整個螢幕。手機寬度小於這個值，不受影響。 */
ion-content {
  --app-content-max: 600px;
}
</style>
