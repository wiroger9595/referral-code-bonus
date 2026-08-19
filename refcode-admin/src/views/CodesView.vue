<script setup lang="ts">
import {
  NAlert,
  NButton,
  NDataTable,
  NInput,
  NModal,
  NPagination,
  NSelect,
  NSpace,
  NTabPane,
  NTabs,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { computed, h, onMounted, ref, watch } from 'vue'

import { ApiError, api } from '../api/client'
import type { AdminCodeItem, AdminCodeStatus, ReviewAction } from '../api/types'
import { CODE_TYPE_LABELS } from '../codeType'

const message = useMessage()

const PAGE_SIZE = 50

const tab = ref<'auto' | 'all'>('auto')

// 兩個分頁各自持有資料與頁碼：切回來時不用重抓，也不會互相蓋掉頁碼。
const autoCodes = ref<AdminCodeItem[]>([])
const autoTotal = ref(0)
const autoPage = ref(1)

const codes = ref<AdminCodeItem[]>([])
const total = ref(0)
const page = ref(1)
const query = ref('')
const status = ref<AdminCodeStatus | null>(null)

const loading = ref(false)
const loadError = ref('')
const submitting = ref(new Set<string>())

// 「全部狀態」不是一個選項，是清空選擇（NSelect 的 value 不吃 null 選項）。
const statusOptions = [
  { label: '上架中', value: 'active' },
  { label: '已下架', value: 'disabled' },
  { label: '已到期', value: 'expired' },
  { label: '已拒絕', value: 'rejected' },
]

async function loadAuto() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.listAutoDisabledCodes(PAGE_SIZE, (autoPage.value - 1) * PAGE_SIZE)
    autoCodes.value = res.codes
    autoTotal.value = res.total
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.listCodes({
      status: status.value,
      q: query.value.trim(),
      limit: PAGE_SIZE,
      offset: (page.value - 1) * PAGE_SIZE,
    })
    codes.value = res.codes
    total.value = res.total
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

function load() {
  return tab.value === 'auto' ? loadAuto() : loadAll()
}

// 待複核的筆數不管在哪個分頁都要是對的，所以進頁面就先抓一次。
onMounted(loadAuto)

watch(tab, (t) => {
  if (t === 'all' && codes.value.length === 0) loadAll()
})
watch(autoPage, loadAuto)
watch([page, status], loadAll)

// 換篩選條件要回到第一頁，否則篩完停在第 3 頁會看到空白。
watch([status, query], () => {
  page.value = 1
})

// 審核動作要填的理由。理由會寫進 code_reviews，上架者申訴時要拿得出來。
const reviewing = ref<{ code: AdminCodeItem; action: ReviewAction } | null>(null)
const reviewReason = ref('')

const reviewTitle = computed(() =>
  reviewing.value?.action === 'restore' ? '恢復上架' : '維持下架',
)

function openReview(code: AdminCodeItem, action: ReviewAction) {
  reviewing.value = { code, action }
  reviewReason.value = ''
}

// 回傳 false 會讓 NModal 保持開啟 —— 理由沒填或送出失敗時不該把輸入清掉。
async function confirmReview() {
  const target = reviewing.value
  if (!target) return false
  if (!reviewReason.value.trim()) {
    message.warning('請填寫原因')
    return false
  }

  submitting.value.add(target.code.id)
  try {
    await api.reviewCode(target.code.id, target.action, reviewReason.value.trim())
    message.success(`${reviewTitle.value}：${target.code.code}`)
    reviewing.value = null
    reviewReason.value = ''
    // 複核完這個碼就離開待複核清單了（最後一筆審核紀錄不再是 auto_disable），
    // 所以重抓而不是在本地改狀態。
    await load()
    return true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
    return false
  } finally {
    submitting.value.delete(target.code.id)
  }
}

const STATUS_META: Record<string, { label: string; type: 'success' | 'error' | 'warning' | 'default' }> = {
  active: { label: '上架中', type: 'success' },
  disabled: { label: '已下架', type: 'error' },
  expired: { label: '已到期', type: 'default' },
  rejected: { label: '已拒絕', type: 'warning' },
}

// 回報欄。只顯示有數字的那幾種——四種全列出來會讓整欄都是 0，
// 掃過去看不出哪個碼有問題。
const REPORT_KINDS = [
  { key: 'report_worked', label: '成功', type: 'success' as const },
  { key: 'report_failed', label: '沒拿到', type: 'warning' as const },
  { key: 'report_invalid_code', label: '碼無效', type: 'error' as const },
  { key: 'report_merchant_closed', label: '活動結束', type: 'default' as const },
]

function renderReports(row: AdminCodeItem) {
  if (row.report_total === 0) return h('span', { style: 'opacity: 0.4' }, '尚無回報')

  const tags = REPORT_KINDS.filter((k) => (row[k.key as keyof AdminCodeItem] as number) > 0).map(
    (k) =>
      h(
        NTag,
        { size: 'small', type: k.type, bordered: false },
        () => `${k.label} ${row[k.key as keyof AdminCodeItem]}`,
      ),
  )
  return h(NSpace, { size: 4 }, () => tags)
}

function formatDateTime(iso: string | null | undefined) {
  return iso ? new Date(iso).toLocaleString('zh-TW', { hour12: false }) : '—'
}

// 兩個分頁共用的欄位；自動下架清單再自己補「下架時間」。
function baseColumns(): DataTableColumns<AdminCodeItem> {
  return [
    {
      title: '服務商 / 碼',
      key: 'code',
      render: (row) =>
        h('div', [
          h('div', { style: 'font-weight: 500' }, row.merchant_name),
          h('code', { class: 'code' }, row.code),
          h('div', { style: 'font-size: 12px; opacity: 0.6' }, CODE_TYPE_LABELS[row.code_type]),
        ]),
    },
    {
      title: '上架者',
      key: 'owner_email',
      width: 200,
      render: (row) =>
        h('div', [
          h('div', { style: 'font-size: 13px' }, row.owner_email),
          h('div', { style: 'font-size: 12px; opacity: 0.6' }, row.owner_name || '（未設暱稱）'),
        ]),
    },
    {
      title: '回報',
      key: 'reports',
      width: 260,
      render: renderReports,
    },
    {
      title: '最近回報',
      key: 'last_reported_at',
      width: 160,
      render: (row) =>
        h('span', { style: 'font-size: 13px' }, formatDateTime(row.last_reported_at)),
    },
    {
      // 排序權重的第一個因子，回報一進來就會被重算（見 internal/ranking）。
      title: '品質',
      key: 'quality_score',
      width: 70,
      render: (row) =>
        h(
          'span',
          { style: row.quality_score < 40 ? 'color: #d03050; font-weight: 500' : '' },
          row.quality_score,
        ),
    },
  ]
}

const autoColumns: DataTableColumns<AdminCodeItem> = [
  ...baseColumns(),
  {
    title: '下架時間',
    key: 'disabled_at',
    width: 160,
    render: (row) => h('span', { style: 'font-size: 13px' }, formatDateTime(row.disabled_at)),
  },
  {
    title: '',
    key: 'actions',
    width: 180,
    render: (row) =>
      h(NSpace, { size: 8 }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            loading: submitting.value.has(row.id),
            onClick: () => openReview(row, 'restore'),
          },
          () => '恢復上架',
        ),
        h(
          NButton,
          { size: 'small', onClick: () => openReview(row, 'disable') },
          () => '維持下架',
        ),
      ]),
  },
]

const allColumns: DataTableColumns<AdminCodeItem> = [
  ...baseColumns(),
  {
    title: '狀態',
    key: 'status',
    width: 90,
    render: (row) => {
      const meta = STATUS_META[row.status] ?? { label: row.status, type: 'default' as const }
      return h(NTag, { size: 'small', type: meta.type, bordered: false }, () => meta.label)
    },
  },
  {
    title: '',
    key: 'actions',
    width: 110,
    render: (row) => {
      if (row.status === 'active') {
        return h(
          NButton,
          {
            size: 'small',
            type: 'error',
            quaternary: true,
            loading: submitting.value.has(row.id),
            onClick: () => openReview(row, 'disable'),
          },
          () => '下架',
        )
      }
      // 已到期的碼不給恢復：狀態會變回 active，但到期排程下一輪就再打掉一次。
      // 要救那些碼得先讓上架者改到期日。
      if (row.status === 'disabled' || row.status === 'rejected') {
        return h(
          NButton,
          {
            size: 'small',
            loading: submitting.value.has(row.id),
            onClick: () => openReview(row, 'restore'),
          },
          () => '恢復上架',
        )
      }
      return h('span', { style: 'opacity: 0.4' }, '—')
    },
  },
]
</script>

<template>
  <div>
    <NSpace align="center" justify="space-between" style="margin-bottom: 16px">
      <h2 style="margin: 0">上架的碼</h2>
      <NButton size="small" :loading="loading" @click="load">重新整理</NButton>
    </NSpace>

    <NAlert v-if="loadError" type="error" style="margin-bottom: 16px">{{ loadError }}</NAlert>

    <NTabs v-model:value="tab" type="line" animated>
      <NTabPane name="auto" :tab="`自動下架待複核（${autoTotal}）`">
        <NText depth="3" style="font-size: 13px; display: block; margin-bottom: 12px">
          系統依最近 10 筆回報自動打掉的碼。少數幾筆惡意回報就湊得出門檻，
          誤判要在這裡救回來；複核過（恢復或維持下架）就會離開這份清單。
        </NText>

        <NDataTable
          :columns="autoColumns"
          :data="autoCodes"
          :loading="loading"
          :row-key="(row: AdminCodeItem) => row.id"
          size="small"
        />

        <NPagination
          v-if="autoTotal > PAGE_SIZE"
          v-model:page="autoPage"
          :page-size="PAGE_SIZE"
          :item-count="autoTotal"
          style="margin-top: 16px; justify-content: flex-end"
        />
      </NTabPane>

      <NTabPane name="all" :tab="`全部已上架（${total}）`">
        <NSpace style="margin-bottom: 12px">
          <NInput
            v-model:value="query"
            placeholder="搜尋碼 / 服務商 / 上架者 email"
            clearable
            style="width: 280px"
            @keyup.enter="loadAll"
          />
          <NSelect
            v-model:value="status"
            :options="statusOptions"
            clearable
            style="width: 140px"
            placeholder="全部狀態"
          />
          <NButton size="small" :loading="loading" @click="loadAll">搜尋</NButton>
        </NSpace>

        <NText depth="3" style="font-size: 13px; display: block; margin-bottom: 12px">
          被回報成用不了的排在最前面，不必翻頁找。待審的碼不在這裡，在審核佇列。
        </NText>

        <NDataTable
          :columns="allColumns"
          :data="codes"
          :loading="loading"
          :row-key="(row: AdminCodeItem) => row.id"
          size="small"
        />

        <NPagination
          v-if="total > PAGE_SIZE"
          v-model:page="page"
          :page-size="PAGE_SIZE"
          :item-count="total"
          style="margin-top: 16px; justify-content: flex-end"
        />
      </NTabPane>
    </NTabs>

    <NModal
      :show="reviewing !== null"
      preset="dialog"
      :title="reviewTitle"
      positive-text="確認"
      negative-text="取消"
      @update:show="(show: boolean) => { if (!show) reviewing = null }"
      @positive-click="confirmReview"
    >
      <p>
        <code class="code">{{ reviewing?.code.code }}</code>
        — {{ reviewing?.code.merchant_name }}
      </p>
      <!-- 原因會寫進 code_reviews，上架者申訴時要拿得出來，所以必填。 -->
      <NInput
        v-model:value="reviewReason"
        type="textarea"
        placeholder="原因（會留下紀錄）"
        :rows="3"
      />
    </NModal>
  </div>
</template>

<style scoped>
.code {
  font-family: ui-monospace, monospace;
  font-size: 14px;
  background: rgba(128, 128, 128, 0.12);
  padding: 1px 6px;
  border-radius: 4px;
}
</style>
