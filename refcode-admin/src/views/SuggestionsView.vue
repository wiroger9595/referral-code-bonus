<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NFormItem,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NSpin,
  NText,
  useMessage,
} from 'naive-ui'
import { computed, onMounted, ref } from 'vue'

import { ApiError, api } from '../api/client'
import type { Category, MerchantSuggestion } from '../api/types'

const message = useMessage()

const suggestions = ref<MerchantSuggestion[]>([])
const categories = ref<Category[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')

const approving = ref<MerchantSuggestion | null>(null)
const approveSlug = ref('')
const approveCategory = ref<string | null>(null)

const rejecting = ref<MerchantSuggestion | null>(null)
const rejectReason = ref('')

const submitting = ref(new Set<string>())

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    // 分類是通過時的必填欄位，跟清單一起拿，不要等 modal 開了才去撈。
    const [res, cats] = await Promise.all([api.listMerchantSuggestions(), api.listCategories()])
    suggestions.value = res.suggestions
    total.value = res.total
    categories.value = cats.categories
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

onMounted(load)

const categoryOptions = () => categories.value.map((c) => ({ label: c.name, value: c.id }))

// slug 會直接出現在網址上，後端只收小寫英數與連字號。英文品牌名多半直接轉得出來，
// 中文名（「台新銀行」）整串會被濾掉，那時就留空讓 admin 自己命名。
function suggestSlug(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function openApprove(s: MerchantSuggestion) {
  approving.value = s
  approveSlug.value = suggestSlug(s.name)
  approveCategory.value = null
}

function closeApprove(show: boolean) {
  if (!show) approving.value = null
}

function remove(id: string) {
  suggestions.value = suggestions.value.filter((s) => s.id !== id)
  total.value -= 1
}

// 回傳 false 讓 NModal 保持開啟——欄位沒填或送出失敗時不該把輸入清掉。
async function confirmApprove() {
  const s = approving.value
  if (!s) return false
  if (!approveSlug.value.trim()) {
    message.warning('請填寫 slug')
    return false
  }
  if (!approveCategory.value) {
    message.warning('請選擇分類')
    return false
  }

  submitting.value.add(s.id)
  try {
    await api.reviewMerchantSuggestion(s.id, {
      action: 'approve',
      slug: approveSlug.value.trim(),
      category_id: approveCategory.value,
    })
    remove(s.id)
    // 建出來的是停用的草稿，不講清楚會以為按完就上架了。
    message.success(`已建立 ${s.name}，請到「服務商」補獎勵說明後啟用`)
    approving.value = null
    return true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
    return false
  } finally {
    submitting.value.delete(s.id)
  }
}

function closeReject(show: boolean) {
  if (!show) {
    rejecting.value = null
    rejectReason.value = ''
  }
}

async function confirmReject() {
  const s = rejecting.value
  if (!s) return false
  if (!rejectReason.value.trim()) {
    message.warning('請填寫拒絕原因')
    return false
  }

  submitting.value.add(s.id)
  try {
    await api.reviewMerchantSuggestion(s.id, {
      action: 'reject',
      reason: rejectReason.value.trim(),
    })
    remove(s.id)
    message.success(`已拒絕 ${s.name}`)
    rejecting.value = null
    rejectReason.value = ''
    return true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
    return false
  } finally {
    submitting.value.delete(s.id)
  }
}

const isEmpty = computed(() => !loading.value && suggestions.value.length === 0)
</script>

<template>
  <div>
    <NSpace align="center" justify="space-between" style="margin-bottom: 16px">
      <h2 style="margin: 0">平台建議（{{ total }}）</h2>
      <NButton size="small" :loading="loading" @click="load">重新整理</NButton>
    </NSpace>

    <NAlert v-if="loadError" type="error" style="margin-bottom: 16px">{{ loadError }}</NAlert>

    <NSpin :show="loading">
      <NEmpty v-if="isEmpty" description="沒有待審的平台建議" style="padding: 60px 0" />

      <NSpace v-else vertical :size="12">
        <NCard v-for="s in suggestions" :key="s.id" size="small">
          <NSpace align="center" justify="space-between">
            <div>
              <strong style="font-size: 16px">{{ s.name }}</strong>

              <div style="margin: 8px 0">
                <!-- 審核的第一件事就是打開這個連結確認是哪一家，擺在最顯眼的位置。 -->
                <a :href="s.signup_url" target="_blank" rel="noopener noreferrer">
                  {{ s.signup_url }}
                </a>
              </div>

              <NText depth="3" style="font-size: 13px">
                {{ s.owner_name }}（{{ s.owner_email }}）
                ・{{ new Date(s.created_at).toLocaleDateString('zh-TW') }}
              </NText>

              <div v-if="s.note" style="margin-top: 6px">
                <NText depth="2" style="font-size: 13px">備註：{{ s.note }}</NText>
              </div>
            </div>

            <NSpace>
              <NButton
                type="primary"
                size="small"
                :loading="submitting.has(s.id)"
                @click="openApprove(s)"
              >
                通過並建立
              </NButton>
              <NButton size="small" @click="rejecting = s">拒絕</NButton>
            </NSpace>
          </NSpace>
        </NCard>
      </NSpace>
    </NSpin>

    <NModal
      :show="approving !== null"
      preset="dialog"
      title="通過並建立服務商"
      positive-text="建立"
      negative-text="取消"
      @update:show="closeApprove"
      @positive-click="confirmApprove"
    >
      <NText depth="3" style="font-size: 13px">
        會建成停用的草稿（名稱與註冊連結沿用建議單），獎勵說明、logo 與適用國家
        到「服務商」補完之後再啟用。
      </NText>

      <NFormItem label="名稱" style="margin-top: 12px">
        <NInput :value="approving?.name" disabled />
      </NFormItem>
      <!-- slug 是公開網址的一部分，建立之後改了舊連結就是死的，所以在這裡確認一次。 -->
      <NFormItem label="slug">
        <NInput v-model:value="approveSlug" placeholder="小寫英數字與連字號" />
      </NFormItem>
      <NFormItem label="分類">
        <NSelect
          v-model:value="approveCategory"
          :options="categoryOptions()"
          placeholder="選一個分類"
        />
      </NFormItem>
    </NModal>

    <NModal
      :show="rejecting !== null"
      preset="dialog"
      title="拒絕這筆平台建議"
      positive-text="確認拒絕"
      negative-text="取消"
      @update:show="closeReject"
      @positive-click="confirmReject"
    >
      <p>{{ rejecting?.name }}</p>
      <!-- 原因會存進建議單，使用者問「為什麼沒上」時要拿得出來，所以必填。 -->
      <NInput
        v-model:value="rejectReason"
        type="textarea"
        placeholder="拒絕原因（會留下紀錄）"
        :rows="3"
      />
    </NModal>
  </div>
</template>
