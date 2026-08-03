<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NInput,
  NModal,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { computed, onMounted, ref } from 'vue'

import { ApiError, api } from '../api/client'
import type { PendingCode } from '../api/types'

const message = useMessage()

const codes = ref<PendingCode[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')

const rejecting = ref<PendingCode | null>(null)
const rejectReason = ref('')
const submitting = ref(new Set<string>())

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.listPendingCodes()
    codes.value = res.codes
    total.value = res.total
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

onMounted(load)

// 後端上架時就擋過格式了，這裡再顯示一次是給審核者當判斷依據
// ——規則可能在碼上架之後才被改過。
function matchesFormat(code: PendingCode) {
  if (!code.code_format_regex) return null
  try {
    return new RegExp(code.code_format_regex).test(code.code)
  } catch {
    return null
  }
}

function daysUntil(iso: string) {
  return Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
}

async function approve(code: PendingCode) {
  submitting.value.add(code.id)
  try {
    await api.reviewCode(code.id, 'approve', '')
    codes.value = codes.value.filter((c) => c.id !== code.id)
    total.value -= 1
    message.success(`已核准 ${code.code}`)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
  } finally {
    submitting.value.delete(code.id)
  }
}

function closeReject(show: boolean) {
  if (!show) {
    rejecting.value = null
    rejectReason.value = ''
  }
}

// 回傳 false 會讓 NModal 保持開啟 —— 理由沒填或送出失敗時不該把輸入清掉。
async function confirmReject() {
  const code = rejecting.value
  if (!code) return false
  if (!rejectReason.value.trim()) {
    message.warning('請填寫拒絕原因')
    return false
  }

  submitting.value.add(code.id)
  try {
    await api.reviewCode(code.id, 'reject', rejectReason.value.trim())
    codes.value = codes.value.filter((c) => c.id !== code.id)
    total.value -= 1
    message.success(`已拒絕 ${code.code}`)
    rejecting.value = null
    rejectReason.value = ''
    return true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
    return false
  } finally {
    submitting.value.delete(code.id)
  }
}

const isEmpty = computed(() => !loading.value && codes.value.length === 0)
</script>

<template>
  <div>
    <NSpace align="center" justify="space-between" style="margin-bottom: 16px">
      <h2 style="margin: 0">審核佇列（{{ total }}）</h2>
      <NButton size="small" :loading="loading" @click="load">重新整理</NButton>
    </NSpace>

    <NAlert v-if="loadError" type="error" style="margin-bottom: 16px">{{ loadError }}</NAlert>

    <NSpin :show="loading">
      <NEmpty v-if="isEmpty" description="沒有待審的推薦碼" style="padding: 60px 0" />

      <NSpace v-else vertical :size="12">
        <NCard v-for="code in codes" :key="code.id" size="small">
          <NSpace align="center" justify="space-between">
            <div>
              <NSpace align="center" :size="8">
                <strong style="font-size: 16px">{{ code.merchant_name }}</strong>
                <NTag size="small" :bordered="false">{{ code.merchant_slug }}</NTag>
              </NSpace>

              <div style="margin: 8px 0">
                <code class="code">{{ code.code }}</code>
                <NTag
                  v-if="matchesFormat(code) === false"
                  type="warning"
                  size="small"
                  style="margin-left: 8px"
                >
                  不符格式規則 {{ code.code_format_regex }}
                </NTag>
              </div>

              <NText depth="3" style="font-size: 13px">
                {{ code.owner_name }}（{{ code.owner_email }}）
                ・到期 {{ new Date(code.expires_at).toLocaleDateString('zh-TW') }}
                <NTag v-if="daysUntil(code.expires_at) < 7" type="warning" size="small">
                  只剩 {{ daysUntil(code.expires_at) }} 天
                </NTag>
              </NText>

              <div v-if="code.note" style="margin-top: 6px">
                <NText depth="2" style="font-size: 13px">備註：{{ code.note }}</NText>
              </div>
            </div>

            <NSpace>
              <NButton
                type="primary"
                size="small"
                :loading="submitting.has(code.id)"
                @click="approve(code)"
              >
                核准
              </NButton>
              <NButton size="small" @click="rejecting = code">拒絕</NButton>
            </NSpace>
          </NSpace>
        </NCard>
      </NSpace>
    </NSpin>

    <NModal
      :show="rejecting !== null"
      preset="dialog"
      title="拒絕這個推薦碼"
      positive-text="確認拒絕"
      negative-text="取消"
      @update:show="closeReject"
      @positive-click="confirmReject"
    >
      <p>
        <code class="code">{{ rejecting?.code }}</code>
        — {{ rejecting?.merchant_name }}
      </p>
      <!-- 原因會寫進 code_reviews，使用者申訴時要拿得出來，所以必填。 -->
      <NInput
        v-model:value="rejectReason"
        type="textarea"
        placeholder="拒絕原因（會留下紀錄）"
        :rows="3"
      />
    </NModal>
  </div>
</template>

<style scoped>
.code {
  font-family: ui-monospace, monospace;
  font-size: 15px;
  background: rgba(128, 128, 128, 0.12);
  padding: 2px 8px;
  border-radius: 4px;
}
</style>
