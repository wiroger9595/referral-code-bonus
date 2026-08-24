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
  IonModal,
  IonPage,
  IonSegment,
  IonSegmentButton,
  IonSelect,
  IonSelectOption,
  IonTitle,
  IonToggle,
  IonToolbar,
  onIonViewWillEnter,
  toastController,
} from '@ionic/vue'
import { addCircleOutline, alertCircle, giftOutline, shieldCheckmarkOutline } from 'ionicons/icons'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { ApiError, api } from '../api/client'
import type { CodeType, MerchantSummary } from '../api/types'
import { apiErrorMessage } from '../i18n'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const merchants = ref<MerchantSummary[]>([])
// 從服務商頁按「上架」進來時網址會帶著那一家，直接選好 —— 使用者剛剛才在
// 那個頁面上，再讓他從整份清單裡把同一家找出來一次很蠢。
const merchantId = ref(typeof route.query.merchant === 'string' ? route.query.merchant : '')
const code = ref('')
const note = ref('')
const loading = ref(false)
const errorMessage = ref('')

// 上架的是自己的推薦碼，還是手上的折扣碼。哪幾種能選是服務商的設定
// （沒有推薦計畫的服務商只收折扣碼），所以要等選完服務商才知道。
const codeType = ref<CodeType>('referral')

// IonDatetime 在 presentation="date" 下只比對日期那一段，這裡不能用 toISOString()
// —— 那是 UTC，台灣凌晨那幾個小時會算成前一天，今天就又選得到了。
function localDate(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// 最早只能選明天：今天到期的碼上架當下就已經沒用了，撿到的人也兌不到。
const minExpiry = localDate(new Date(Date.now() + 86400000))

// 預設不設期限。多數服務商的推薦計畫是常駐的，逼使用者填一個猜出來的日期
// 只會讓碼在還能用的時候被排程下架。有活動檔期的自己打開開關去選。
const hasExpiry = ref(false)

// 開關打開時才用得到，預設先擺三個月 —— 大部分活動不會撐更久。
const expiresAt = ref(localDate(new Date(Date.now() + 90 * 86400000)))

// 不能用 onMounted：Ionic 把離開的頁面留在 DOM 裡重用（同 MyCodesTab），
// setup 只跑一次，從第二家服務商按上架進來時表單會還停在上一家。
onIonViewWillEnter(async () => {
  // 帶了服務商就照它設；沒帶（從分頁或帳號頁進來）就保留使用者原本選的，
  // 免得去 paywall 繞一圈回來，剛選好的服務商被清掉。
  const fromQuery = route.query.merchant
  if (typeof fromQuery === 'string' && fromQuery) {
    merchantId.value = fromQuery
  }

  if (merchants.value.length > 0) {
    return
  }
  try {
    merchants.value = (await api.listMerchants()).merchants
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'addCode.loadMerchantsFailed')
  }
})

const selected = computed(() => merchants.value.find((m) => m.id === merchantId.value))

// 「我要上架的是哪一種」是使用者一進來就想做的選擇，不能等到選完服務商才出現
// —— 藏在後面等於整個折扣碼功能看不見。還沒選服務商時兩種都給點，
// 選完之後才收斂成那家實際收的類型。
const ALL_CODE_TYPES: CodeType[] = ['referral', 'discount']
const allowedTypes = computed<CodeType[]>(
  () => selected.value?.allowed_code_types ?? ALL_CODE_TYPES,
)

// 換服務商時把類型拉回這家收的第一種：上一家能選推薦碼、下一家只收折扣碼的話，
// 留著舊的選擇會讓使用者送出後才被後端擋下來。
watch(selected, (m) => {
  if (m && !m.allowed_code_types.includes(codeType.value)) {
    codeType.value = m.allowed_code_types[0] ?? 'referral'
  }
})

const isDiscount = computed(() => codeType.value === 'discount')

// merchant.name 是從 App Store 上架名稱整個匯進來的，例如
// 「Klook - 全球旅遊＆玩樂體驗預訂平台」，選單裡只需要品牌名那一段。
function shortName(name: string) {
  return name.split(/\s*[-|｜]\s*/)[0]
}

// 目錄裡沒有那一家的時候的出口。沒有這個，手上有碼但服務商還沒上架的人
// 在這一頁就是完全走不下去 —— 他只能放棄，我們也不知道少了哪幾家。
//
// 用 modal 而不是另開一頁：他正填到一半，跳頁回來表單就沒了。
const suggestOpen = ref(false)
const suggestName = ref('')
const suggestUrl = ref('')
const suggestNote = ref('')
const suggestLoading = ref(false)
const suggestError = ref('')

function openSuggest() {
  suggestError.value = ''
  suggestOpen.value = true
}

async function submitSuggestion() {
  suggestError.value = ''
  if (!suggestName.value.trim()) {
    suggestError.value = t('suggestMerchant.nameRequired')
    return
  }
  if (!suggestUrl.value.trim()) {
    suggestError.value = t('suggestMerchant.urlRequired')
    return
  }

  suggestLoading.value = true
  try {
    await api.suggestMerchant({
      name: suggestName.value.trim(),
      signup_url: suggestUrl.value.trim(),
      note: suggestNote.value.trim(),
    })
    suggestOpen.value = false
    suggestName.value = ''
    suggestUrl.value = ''
    suggestNote.value = ''

    const toast = await toastController.create({
      message: t('suggestMerchant.submitted'),
      duration: 2600,
    })
    await toast.present()
  } catch (e) {
    suggestError.value = apiErrorMessage(e, 'suggestMerchant.submitFailed')
  } finally {
    suggestLoading.value = false
  }
}

// 使用者選的是「有效到這一天」，所以送當地時區那天的最後一刻。
// 不能直接 new Date('2026-08-18')：那會被當成 UTC 午夜，台灣的使用者
// 選了 8/18，碼會在 8/18 早上八點就過期。
function endOfLocalDay(value: string) {
  const [y, m, d] = value.slice(0, 10).split('-').map(Number)
  return new Date(y, m - 1, d, 23, 59, 59, 999).toISOString()
}

async function submit() {
  errorMessage.value = ''
  if (!merchantId.value) {
    errorMessage.value = t('addCode.chooseMerchant')
    return
  }

  // 折扣碼的優惠內容只有備註講得出來，空著送出去撿到的人不知道能換到什麼。
  // 後端也擋（discount_note_required），這裡先擋是免得白跑一趟。
  if (isDiscount.value && !note.value.trim()) {
    errorMessage.value = t('addCode.discountNoteRequired')
    return
  }

  loading.value = true
  try {
    await api.createCode({
      merchant_id: merchantId.value,
      code: code.value.trim(),
      note: note.value.trim(),
      expires_at: hasExpiry.value ? endOfLocalDay(expiresAt.value) : null,
      code_type: codeType.value,
    })
    const toast = await toastController.create({
      message: t('addCode.submitted'),
      duration: 2200,
    })
    await toast.present()
    // 這一輪結束了，把表單清乾淨 —— 頁面實例會被留著，不清的話下次進來
    // 還看得到剛剛送出去的那組碼。
    code.value = ''
    note.value = ''
    hasExpiry.value = false
    codeType.value = 'referral'
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
                  {{ shortName(m.name) }}
                </IonSelectOption>
              </IonSelect>
            </IonItem>

            <!-- 選單裡找不到的那些人在這裡才會發現，出口就要放在選單旁邊。 -->
            <IonItem lines="none" button :detail="false" @click="openSuggest">
              <IonIcon slot="start" :icon="addCircleOutline" class="suggest-icon" />
              <span class="suggest-link">{{ $t('suggestMerchant.entry') }}</span>
            </IonItem>

            <!-- 只收一種碼的服務商不給選，直接照那一種走；有選擇才值得佔一整列。 -->
            <IonItem v-if="allowedTypes.length > 1" lines="none">
              <div class="type-picker">
                <span class="type-label">{{ $t('addCode.codeTypeLabel') }}</span>
                <IonSegment v-model="codeType">
                  <IonSegmentButton v-for="ty in allowedTypes" :key="ty" :value="ty">
                    {{ $t(`codeType.${ty}`) }}
                  </IonSegmentButton>
                </IonSegment>
              </div>
            </IonItem>

            <IonItem>
              <IonInput
                v-model="code"
                :label="isDiscount ? $t('codeType.discount') : $t('codeType.referral')"
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
                :label="isDiscount ? $t('addCode.noteDiscount') : $t('addCode.note')"
                label-placement="stacked"
                :placeholder="isDiscount ? $t('addCode.noteDiscountPlaceholder') : $t('addCode.notePlaceholder')"
              />
            </IonItem>
          </IonList>

          <p class="tiny muted type-hint">
            {{ isDiscount ? $t('codeType.discountHint') : $t('codeType.referralHint') }}
          </p>

          <!-- 選了服務商就把獎勵秀出來，確認選對家了。獎勵說明講的是推薦計畫，
               上架折扣碼時把它秀出來只會誤導。 -->
          <div v-if="selected?.reward_desc && !isDiscount" class="callout">
            <IonIcon :icon="giftOutline" />
            <span>{{ selected.reward_desc }}</span>
          </div>
        </section>

        <section>
          <h2 class="label">{{ $t('addCode.sectionExpiry') }}</h2>
          <IonList class="app-form" lines="none">
            <IonItem>
              <IonToggle v-model="hasExpiry">{{ $t('addCode.hasExpiry') }}</IonToggle>
            </IonItem>
          </IonList>
          <p class="tiny muted toggle-hint">
            {{ hasExpiry ? $t('addCode.expiryHint') : $t('addCode.noExpiryHint') }}
          </p>
          <div v-if="hasExpiry" class="app-card picker">
            <IonDatetime
              v-model="expiresAt"
              presentation="date"
              :min="minExpiry"
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

      <IonModal :is-open="suggestOpen" @did-dismiss="suggestOpen = false">
        <IonHeader>
          <IonToolbar>
            <IonTitle>{{ $t('suggestMerchant.title') }}</IonTitle>
            <IonButtons slot="end">
              <IonButton @click="suggestOpen = false">{{ $t('common.cancel') }}</IonButton>
            </IonButtons>
          </IonToolbar>
        </IonHeader>

        <IonContent>
          <div class="page-pad form">
            <p class="tiny muted">{{ $t('suggestMerchant.intro') }}</p>

            <IonList class="app-form" lines="full">
              <IonItem>
                <IonInput
                  v-model="suggestName"
                  :label="$t('suggestMerchant.name')"
                  label-placement="stacked"
                  :placeholder="$t('suggestMerchant.namePlaceholder')"
                />
              </IonItem>

              <IonItem>
                <IonInput
                  v-model="suggestUrl"
                  :label="$t('suggestMerchant.url')"
                  label-placement="stacked"
                  :placeholder="$t('suggestMerchant.urlPlaceholder')"
                  type="url"
                  inputmode="url"
                  autocapitalize="off"
                  autocorrect="off"
                />
              </IonItem>

              <IonItem>
                <IonInput
                  v-model="suggestNote"
                  :label="$t('suggestMerchant.note')"
                  label-placement="stacked"
                  :placeholder="$t('suggestMerchant.notePlaceholder')"
                />
              </IonItem>
            </IonList>

            <div v-if="suggestError" class="error">
              <IonIcon :icon="alertCircle" />
              <span>{{ suggestError }}</span>
            </div>

            <div class="callout muted">
              <IonIcon :icon="shieldCheckmarkOutline" />
              <span>{{ $t('suggestMerchant.reviewNotice') }}</span>
            </div>

            <IonButton
              expand="block"
              class="wide"
              :disabled="suggestLoading"
              @click="submitSuggestion"
            >
              {{ $t('suggestMerchant.submit') }}
            </IonButton>
          </div>
        </IonContent>
      </IonModal>
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

/* 提報入口刻意做得比表單欄位輕：它是這一頁的支線，不該跟「選服務商」搶注意力。 */
.suggest-link {
  font-size: 14px;
  font-weight: 600;
  color: var(--ion-color-primary);
}

.suggest-icon {
  font-size: 18px;
  color: var(--ion-color-primary);
  margin-inline-end: 8px;
}

.type-picker {
  width: 100%;
  padding: 10px 0;
}

.type-label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--app-muted);
}

.type-hint {
  margin: 10px 2px 0;
}

/* .sub 的負 margin 是給緊接在 .label 後面用的，接在 IonList 後面會被卡片蓋掉。 */
.toggle-hint {
  margin: 10px 2px 12px;
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
/* 單欄的頁不跟著平板的內容欄一起放寬 —— 表單拉到 940 只會變成很長的一行，
   眼睛從欄位標題找到輸入框要橫跨整個螢幕。手機寬度小於這個值，不受影響。 */
ion-content {
  --app-content-max: 600px;
}
</style>
