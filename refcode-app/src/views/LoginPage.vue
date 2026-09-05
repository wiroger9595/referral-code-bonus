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
  IonSegment,
  IonSegmentButton,
  IonSelect,
  IonSelectOption,
  IonToolbar,
} from '@ionic/vue'
import { alertCircle, logoApple, logoGoogle, ticketOutline } from 'ionicons/icons'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { SocialLoginCancelled, availableProviders } from '../api/social'
import { countryOptions, defaultCountry } from '../countries'
import type { OAuthProvider } from '../api/types'
import { apiErrorMessage } from '../i18n'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const mode = ref<'login' | 'register'>('login')
const email = ref('')
const password = ref('')
const displayName = ref('')
// 預設跟著介面語言猜，使用者可以改成「不指定」或別的地方。
const country = ref(defaultCountry())
const countries = countryOptions()
const loading = ref(false)
const errorMessage = ref('')

// 沒設定 client id 的 provider 不顯示，按下去只會拿到原生層的錯誤。
const providers = availableProviders()
const providerLabels: Record<OAuthProvider, { key: string; icon: string }> = {
  google: { key: 'login.googleContinue', icon: logoGoogle },
  apple: { key: 'login.appleContinue', icon: logoApple },
}

// 切分頁時把上一則錯誤收掉。登入失敗的「email 或密碼錯誤」留在註冊表單上方，
// 看起來像是註冊這邊填錯了 —— 而他還一個字都沒填。
watch(mode, () => {
  errorMessage.value = ''
})

function goNext() {
  const redirect = route.query.redirect
  router.replace(typeof redirect === 'string' ? redirect : '/tabs/my-codes')
}

// 帶著 redirect 過去：重設完會直接登入，該回到的還是原本要去的那一頁。
function goForgot() {
  router.push({ path: '/forgot-password', query: route.query })
}

async function submit() {
  errorMessage.value = ''

  // 空欄位在前端擋掉。讓它送出去的話後端回的是「email 或密碼錯誤」——
  // 那句話會讓人以為自己填錯了，而其實他根本沒填。
  if (!email.value.trim() || !password.value) {
    errorMessage.value = t('login.fieldsRequired')
    return
  }
  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login(email.value.trim(), password.value)
    } else {
      await auth.register(
        email.value.trim(),
        password.value,
        displayName.value.trim(),
        country.value,
      )
    }
    goNext()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'login.connectionFailed')
  } finally {
    loading.value = false
  }
}

async function submitSocial(provider: OAuthProvider) {
  errorMessage.value = ''
  loading.value = true
  try {
    // 註冊分頁上選的所在地一起送 —— 這幾顆按鈕對還沒有帳號的人來說就是註冊。
    await auth.loginWithProvider(provider, mode.value === 'register' ? country.value : '')
    goNext()
  } catch (e) {
    // 使用者自己取消的不是錯誤，跳紅字只會讓人以為壞掉了。
    if (e instanceof SocialLoginCancelled) return
    errorMessage.value = apiErrorMessage(e, 'login.loginFailed')
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
          <IonBackButton default-href="/tabs/account" text="" />
        </IonButtons>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <div class="wrap page-pad">
        <div class="brand">
          <div class="mark"><IonIcon :icon="ticketOutline" /></div>
          <h1>{{ mode === 'login' ? $t('login.titleLogin') : $t('login.titleRegister') }}</h1>
          <p class="muted">{{ $t('login.lead') }}</p>
        </div>

        <IonSegment v-model="mode">
          <IonSegmentButton value="login">{{ $t('login.tabLogin') }}</IonSegmentButton>
          <IonSegmentButton value="register">{{ $t('login.tabRegister') }}</IonSegmentButton>
        </IonSegment>

        <IonList class="app-form" lines="full">
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
          <IonItem>
            <IonInput
              v-model="password"
              :label="$t('login.password')"
              label-placement="stacked"
              type="password"
              :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
            />
          </IonItem>
          <IonItem v-if="mode === 'register'">
            <IonInput
              v-model="displayName"
              :label="$t('login.displayName')"
              label-placement="stacked"
              :placeholder="$t('login.displayNamePlaceholder')"
            />
          </IonItem>
          <IonItem v-if="mode === 'register'">
            <IonSelect
              v-model="country"
              :label="$t('login.country')"
              label-placement="stacked"
              interface="action-sheet"
              :placeholder="$t('login.countryUnset')"
            >
              <IonSelectOption value="">{{ $t('login.countryUnset') }}</IonSelectOption>
              <IonSelectOption v-for="c in countries" :key="c.code" :value="c.code">
                {{ c.label }}
              </IonSelectOption>
            </IonSelect>
          </IonItem>
        </IonList>

        <div v-if="errorMessage" class="error">
          <IonIcon :icon="alertCircle" />
          <span>{{ errorMessage }}</span>
        </div>

        <IonButton expand="block" class="wide" :disabled="loading" @click="submit">
          {{ mode === 'login' ? $t('login.titleLogin') : $t('login.titleRegister') }}
        </IonButton>

        <IonButton
          v-if="mode === 'login'"
          expand="block"
          fill="clear"
          size="small"
          :disabled="loading"
          @click="goForgot"
        >
          {{ $t('login.forgotPassword') }}
        </IonButton>

        <template v-if="providers.length">
          <div class="divider"><span>{{ $t('login.or') }}</span></div>

          <IonButton
            v-for="provider in providers"
            :key="provider"
            expand="block"
            class="wide"
            fill="outline"
            color="dark"
            :disabled="loading"
            @click="submitSocial(provider)"
          >
            <IonIcon slot="start" :icon="providerLabels[provider].icon" />
            {{ $t(providerLabels[provider].key) }}
          </IonButton>
        </template>

        <!-- client id 沒設好、或（目前的 Apple）整個被關掉時，這區才會不出現。
             Apple 目前是刻意關掉的，見 src/api/social.ts 的 appleReady —— 上架
             App Store 前要處理掉那邊寫的 4.8 風險，不是這裡的事。 -->
        <p v-else class="tiny muted hint">
          {{ $t('login.emailOnlyNotice') }}
        </p>
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

.divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 4px 0;
  color: var(--app-muted);
  font-size: 12px;
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--app-line);
}

.hint {
  margin: 0;
  text-align: center;
}
/* 單欄的頁不跟著平板的內容欄一起放寬 —— 表單拉到 940 只會變成很長的一行，
   眼睛從欄位標題找到輸入框要橫跨整個螢幕。手機寬度小於這個值，不受影響。 */
ion-content {
  --app-content-max: 600px;
}
</style>
