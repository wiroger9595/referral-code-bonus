<script setup lang="ts">
import {
  IonButton,
  IonContent,
  IonHeader,
  IonIcon,
  IonItem,
  IonLabel,
  IonList,
  IonPage,
  IonInput,
  IonSelect,
  IonSelectOption,
  IonTitle,
  IonToolbar,
  alertController,
  toastController,
} from '@ionic/vue'
import {
  addCircleOutline,
  cameraOutline,
  chevronForward,
  earthOutline,
  documentTextOutline,
  languageOutline,
  mailOutline,
  personCircleOutline,
  personRemoveOutline,
  shieldCheckmarkOutline,
  sparklesOutline,
  trashOutline,
  warningOutline,
} from 'ionicons/icons'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import EmptyState from '../components/EmptyState.vue'
import { countryOptions } from '../countries'
import { toAvatarBlob } from '../images'
import { SUPPORTED, apiErrorMessage, setLocale, type LocaleCode } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useSubscriptionStore } from '../stores/subscription'

const auth = useAuthStore()
const subs = useSubscriptionStore()
const router = useRouter()
const { t, locale } = useI18n()
const countries = countryOptions()
const countryError = ref('')

// 刪除帳號。Apple 5.1.1(v) 與 Play 都要求 app 內能自己刪，而且不能只給一個連結。
const deleting = ref(false)
const deleteError = ref('')
const confirmText = ref('')
const showDelete = ref(false)

async function confirmDelete() {
  const email = auth.user?.email ?? ''
  // 後端也會比對一次 —— 這裡擋的是誤觸，不是身分驗證。
  if (confirmText.value.trim().toLowerCase() !== email.toLowerCase()) {
    deleteError.value = t('account.deleteConfirmHint', { email })
    return
  }

  // 最後一道確認用系統原生的 alert，跟畫面上的表單分開，避免一路點下去。
  const alert = await alertController.create({
    header: t('account.deleteTitle'),
    message: t('account.deleteWarning'),
    buttons: [
      { text: t('account.deleteCancel'), role: 'cancel' },
      { text: t('account.deleteConfirm'), role: 'destructive' },
    ],
  })
  await alert.present()
  const { role } = await alert.onDidDismiss()
  if (role !== 'destructive') return

  deleteError.value = ''
  deleting.value = true
  try {
    await auth.deleteAccount(confirmText.value.trim())
    const toast = await toastController.create({ message: t('account.deleteDone'), duration: 2400 })
    await toast.present()
    router.replace('/tabs/explore')
  } catch (e) {
    deleteError.value = apiErrorMessage(e, 'account.deleteFailed')
  } finally {
    deleting.value = false
  }
}

// 大頭照。用原生的 file input 而不是 Camera plugin —— WebView 本來就會叫出系統的
// 「相機／相簿」選單，裝 plugin 反而要多申報相機權限，資料安全性表單也得跟著改。
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarUploading = ref(false)
const avatarError = ref('')

function pickAvatar() {
  avatarInput.value?.click()
}

async function onAvatarPicked(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // 選完就清空，不然連續選同一個檔案不會再觸發 change。
  input.value = ''
  if (!file) return

  avatarError.value = ''
  avatarUploading.value = true
  try {
    await auth.setAvatar(await toAvatarBlob(file))
    const toast = await toastController.create({ message: t('account.avatarDone'), duration: 2000 })
    await toast.present()
  } catch (e) {
    avatarError.value = apiErrorMessage(e, 'account.avatarFailed')
  } finally {
    avatarUploading.value = false
  }
}

// UGC app 要有 app 內可見的聯絡方式與條款連結（Apple 審核指南 1.2、5.1.1）。
// 兩個都從 .env 讀，沒設定就不顯示 —— 送審前一定要設，見 store/checklist.md。
const supportEmail = import.meta.env.VITE_SUPPORT_EMAIL ?? ''
const siteUrl = import.meta.env.VITE_SITE_URL ?? ''

async function logout() {
  await auth.logout()
  router.replace('/tabs/explore')
}

// 改所在地會馬上打 API。失敗就顯示一行錯誤，並把選單拉回原本的值 ——
// 讓選單停在沒存進去的狀態，使用者會以為改成功了。
async function changeCountry(event: CustomEvent) {
  const next = (event.detail as { value: string }).value
  if (next === (auth.user?.country ?? '')) return

  countryError.value = ''
  try {
    await auth.setCountry(next)
  } catch (e) {
    countryError.value = apiErrorMessage(e, 'account.countrySaveFailed')
  }
}

function changeLocale(event: CustomEvent) {
  setLocale((event.detail as { value: LocaleCode }).value)
}

function initial() {
  const name = auth.user?.display_name || auth.user?.email || ''
  return name.trim().charAt(0).toUpperCase()
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonTitle>{{ $t('account.title') }}</IonTitle>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <template v-if="auth.isLoggedIn">
        <div class="profile page-pad">
          <button
            type="button"
            class="avatar"
            :aria-label="$t('account.avatarChange')"
            :disabled="avatarUploading"
            @click="pickAvatar"
          >
            <img v-if="auth.user?.avatar_url" :src="auth.user.avatar_url" alt="" />
            <span v-else>{{ initial() }}</span>
            <IonIcon :icon="cameraOutline" class="avatar-badge" />
          </button>
          <!-- accept 交給系統決定要開相機還是相簿，不另外裝 Camera plugin。 -->
          <input
            ref="avatarInput"
            type="file"
            accept="image/*"
            class="avatar-input"
            @change="onAvatarPicked"
          />
          <h1>{{ auth.user?.display_name }}</h1>
          <p class="muted">{{ auth.user?.email }}</p>
          <p v-if="avatarUploading" class="tiny muted">{{ $t('account.avatarUploading') }}</p>
          <p v-else-if="avatarError" class="tiny" style="color: var(--ion-color-danger)">
            {{ avatarError }}
          </p>
        </div>

        <div class="page-pad">
          <IonList class="app-form" lines="full">
            <IonItem button router-link="/pro" :detail="false">
              <IonIcon slot="start" :icon="sparklesOutline" color="primary" />
              <IonLabel>
                <h3>{{ subs.isPro ? $t('pro.badge') : $t('pro.upgrade') }}</h3>
                <p v-if="subs.isPro && subs.expiresAt">
                  {{ $t('pro.renewsOn', { date: new Date(subs.expiresAt).toLocaleDateString() }) }}
                </p>
                <p v-else-if="!subs.isPro">{{ $t('pro.heroDesc') }}</p>
              </IonLabel>
              <IonIcon slot="end" :icon="chevronForward" class="chev" />
            </IonItem>
            <IonItem button router-link="/add-code" :detail="false">
              <IonIcon slot="start" :icon="addCircleOutline" color="primary" />
              <IonLabel>{{ $t('account.addCode') }}</IonLabel>
              <IonIcon slot="end" :icon="chevronForward" class="chev" />
            </IonItem>
            <!-- 解除封鎖的唯一入口。封鎖之後對方的碼就消失了，沒有這條路
                 誤封的人救不回來（UGC 政策也要求封鎖是可逆的）。 -->
            <IonItem button router-link="/blocks" :detail="false">
              <IonIcon slot="start" :icon="personRemoveOutline" color="primary" />
              <IonLabel>{{ $t('blocks.title') }}</IonLabel>
              <IonIcon slot="end" :icon="chevronForward" class="chev" />
            </IonItem>
            <IonItem :detail="false">
              <IonIcon slot="start" :icon="earthOutline" color="primary" />
              <IonSelect
                :label="$t('account.country')"
                :value="auth.user?.country ?? ''"
                interface="action-sheet"
                @ion-change="changeCountry"
              >
                <IonSelectOption value="">{{ $t('account.countryUnset') }}</IonSelectOption>
                <IonSelectOption v-for="c in countries" :key="c.code" :value="c.code">
                  {{ c.label }}
                </IonSelectOption>
              </IonSelect>
            </IonItem>
            <IonItem v-if="countryError" :detail="false">
              <IonLabel color="danger" class="ion-text-wrap">{{ countryError }}</IonLabel>
            </IonItem>
            <IonItem v-if="supportEmail" button :href="`mailto:${supportEmail}`" target="_blank" :detail="false">
              <IonIcon slot="start" :icon="mailOutline" color="primary" />
              <IonLabel>
                <h3>{{ $t('account.contact') }}</h3>
                <p>{{ $t('account.contactDesc') }}</p>
              </IonLabel>
            </IonItem>
            <IonItem v-if="siteUrl" button :href="`${siteUrl}/terms`" target="_blank" :detail="false">
              <IonIcon slot="start" :icon="documentTextOutline" color="primary" />
              <IonLabel>{{ $t('account.terms') }}</IonLabel>
              <IonIcon slot="end" :icon="chevronForward" class="chev" />
            </IonItem>
            <IonItem v-if="siteUrl" button :href="`${siteUrl}/privacy`" target="_blank" :detail="false">
              <IonIcon slot="start" :icon="shieldCheckmarkOutline" color="primary" />
              <IonLabel>{{ $t('account.privacy') }}</IonLabel>
              <IonIcon slot="end" :icon="chevronForward" class="chev" />
            </IonItem>
            <IonItem :detail="false">
              <IonIcon slot="start" :icon="languageOutline" color="primary" />
              <IonSelect
                :label="$t('account.language')"
                :value="locale"
                interface="action-sheet"
                @ion-change="changeLocale"
              >
                <IonSelectOption v-for="l in SUPPORTED" :key="l.code" :value="l.code">
                  {{ l.name }}
                </IonSelectOption>
              </IonSelect>
            </IonItem>
          </IonList>
        </div>

        <div class="page-pad actions">
          <IonButton expand="block" fill="clear" color="medium" @click="logout">{{ $t('account.logout') }}</IonButton>

          <!-- 刪除帳號要點得到但不能誤觸：預設收起來，展開後還要打完整 email，
               最後再經過一次原生 alert。 -->
          <IonButton
            v-if="!showDelete"
            expand="block"
            fill="clear"
            size="small"
            color="danger"
            @click="showDelete = true"
          >
            <IonIcon slot="start" :icon="trashOutline" />
            {{ $t('account.deleteAccount') }}
          </IonButton>

          <div v-else class="app-card danger-zone">
            <h3>{{ $t('account.deleteTitle') }}</h3>
            <p class="tiny">{{ $t('account.deleteWarning') }}</p>

            <!-- 訂閱不會因為刪帳號而取消，這句不寫清楚會變成扣款客訴。 -->
            <div v-if="subs.isPro" class="warn">
              <IonIcon :icon="warningOutline" />
              <span>{{ $t('account.deleteSubscriptionNote') }}</span>
            </div>

            <IonInput
              v-model="confirmText"
              class="confirm"
              fill="outline"
              :label="$t('account.deleteConfirmLabel')"
              label-placement="stacked"
              type="email"
              autocapitalize="off"
              autocorrect="off"
              :placeholder="auth.user?.email"
            />

            <p v-if="deleteError" class="tiny err">{{ deleteError }}</p>

            <div class="danger-actions">
              <IonButton fill="outline" color="medium" :disabled="deleting" @click="showDelete = false">
                {{ $t('account.deleteCancel') }}
              </IonButton>
              <IonButton color="danger" :disabled="deleting || !confirmText" @click="confirmDelete">
                {{ $t('account.deleteConfirm') }}
              </IonButton>
            </div>
          </div>
        </div>
      </template>

      <EmptyState
        v-else
        :icon="personCircleOutline"
        :title="$t('account.guestTitle')"
        :description="$t('account.guestDesc')"
      >
        <IonButton router-link="/login" class="wide">{{ $t('account.signIn') }}</IonButton>
      </EmptyState>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.profile {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 12px;
  padding-bottom: 26px;
  text-align: center;
}

.avatar {
  position: relative;
  display: grid;
  place-items: center;
  width: 72px;
  height: 72px;
  margin-bottom: 14px;
  padding: 0;
  border: none;
  border-radius: 999px;
  overflow: hidden;
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 28px;
  font-weight: 700;
  cursor: pointer;
}

.avatar:disabled {
  opacity: 0.6;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* 沒有這顆相機圖示的話，看不出頭像是可以按的。 */
.avatar-badge {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 100%;
  padding: 3px 0 2px;
  background: rgb(0 0 0 / 45%);
  color: #fff;
  font-size: 13px;
}

.avatar-input {
  display: none;
}

.profile h1 {
  margin: 0;
  font-size: 21px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.profile p {
  margin: 4px 0 0;
  font-size: 14px;
}

.chev {
  color: var(--app-muted);
  font-size: 15px;
  opacity: 0.6;
}

.danger-zone {
  margin-top: 8px;
  padding: 16px;
  border-color: rgba(var(--ion-color-danger-rgb), 0.35);
}

.danger-zone h3 {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 700;
  color: var(--ion-color-danger);
}

.danger-zone p {
  margin: 0;
  color: var(--app-muted);
  line-height: 1.6;
}

.warn {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: var(--app-radius-sm);
  background: rgba(var(--ion-color-warning-rgb), 0.16);
  color: var(--ion-color-warning-shade);
  font-size: 12px;
  line-height: 1.55;
}

.warn ion-icon {
  flex: none;
  margin-top: 1px;
  font-size: 15px;
}

.confirm {
  margin-top: 14px;
}

.err {
  margin: 8px 0 0;
  color: var(--ion-color-danger);
}

.danger-actions {
  display: flex;
  gap: 8px;
  margin-top: 14px;
}

.danger-actions ion-button {
  flex: 1;
}

.actions {
  padding-top: 20px;
  padding-bottom: 24px;
}
/* 單欄的頁不跟著平板的內容欄一起放寬 —— 表單拉到 940 只會變成很長的一行，
   眼睛從欄位標題找到輸入框要橫跨整個螢幕。手機寬度小於這個值，不受影響。 */
ion-content {
  --app-content-max: 600px;
}
</style>
