<script setup lang="ts">
import {
  NAlert,
  NButton,
  NDataTable,
  NDatePicker,
  NInput,
  NModal,
  NPagination,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { h, onMounted, ref, watch } from 'vue'

import { ApiError, api } from '../api/client'
import type { AdminUserItem } from '../api/types'

const message = useMessage()

const PAGE_SIZE = 50

const users = ref<AdminUserItem[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const loadError = ref('')
const query = ref('')

const granting = ref<AdminUserItem | null>(null)
const grantExpiresAt = ref<number | null>(null)
const suspending = ref<AdminUserItem | null>(null)
const suspendUntil = ref<number | null>(null)
const submitting = ref(new Set<string>())

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.listUsers(query.value.trim(), PAGE_SIZE, (page.value - 1) * PAGE_SIZE)
    users.value = res.users
    total.value = res.total
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

watch(page, load)
onMounted(load)

// 換關鍵字一律回到第一頁。停在第 3 頁搜一個新名字，結果不足三頁時畫面會是空的，
// 看起來就像查無此人 —— 而實際上人就在第一頁。
function search() {
  if (page.value !== 1) {
    page.value = 1 // watch(page) 會接著 load
    return
  }
  load()
}

function openGrant(u: AdminUserItem) {
  granting.value = u
  grantExpiresAt.value = null
}

// 回傳 false 會讓 NModal 保持開啟。
async function confirmGrant() {
  const u = granting.value
  if (!u) return false

  submitting.value.add(u.id)
  try {
    const expiresAt = grantExpiresAt.value ? new Date(grantExpiresAt.value).toISOString() : null
    await api.grantPro(u.id, expiresAt)
    message.success(`已補發 Pro 給 ${u.email}`)
    granting.value = null
    await load()
    return true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
    return false
  } finally {
    submitting.value.delete(u.id)
  }
}

async function revoke(u: AdminUserItem) {
  submitting.value.add(u.id)
  try {
    await api.revokePro(u.id)
    message.success(`已撤銷 ${u.email} 的 Pro`)
    await load()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
  } finally {
    submitting.value.delete(u.id)
  }
}

function openSuspend(u: AdminUserItem) {
  suspending.value = u
  suspendUntil.value = null
}

// 回傳 false 會讓 NModal 保持開啟。
async function confirmSuspend() {
  const u = suspending.value
  if (!u) return false

  submitting.value.add(u.id)
  try {
    // NDatePicker 給的是當天 00:00，停權「到 9/18」的語意是那天結束為止，
    // 所以送出前補到隔天零點 —— 不補的話選今天等於立刻期滿。
    const until = suspendUntil.value
      ? new Date(suspendUntil.value + 24 * 60 * 60 * 1000).toISOString()
      : null
    const res = await api.suspendUser(u.id, until)
    message.success(
      res.disabled_codes > 0
        ? `已停權 ${u.email}，連帶下架 ${res.disabled_codes} 個碼`
        : `已停權 ${u.email}`,
    )
    suspending.value = null
    await load()
    return true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
    return false
  } finally {
    submitting.value.delete(u.id)
  }
}

async function reinstate(u: AdminUserItem) {
  submitting.value.add(u.id)
  try {
    const res = await api.reinstateUser(u.id)
    message.success(
      res.restored_codes > 0
        ? `已解除 ${u.email} 的停權，放回 ${res.restored_codes} 個碼`
        : `已解除 ${u.email} 的停權`,
    )
    await load()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失敗')
  } finally {
    submitting.value.delete(u.id)
  }
}

const columns: DataTableColumns<AdminUserItem> = [
  {
    title: '使用者',
    key: 'email',
    render: (row) =>
      h('div', [
        h('div', { style: 'font-weight: 500' }, row.email),
        h('div', { style: 'font-size: 12px; opacity: 0.6' }, row.display_name || '（未設暱稱）'),
      ]),
  },
  {
    title: '註冊時間',
    key: 'created_at',
    width: 120,
    render: (row) => new Date(row.created_at).toLocaleDateString('zh-TW'),
  },
  {
    title: '狀態',
    key: 'status',
    width: 140,
    render: (row) => {
      if (row.status !== 'suspended') {
        return h('span', { style: 'opacity: 0.4' }, '正常')
      }
      // 無限期與有期限要一眼分得出來：前者要有人記得回來解除，後者排程會自己放。
      const until = row.suspended_until
        ? `至 ${new Date(row.suspended_until).toLocaleDateString('zh-TW')}`
        : '無限期'
      return h(NSpace, { vertical: true, size: 2 }, () => [
        h(NTag, { type: 'error', size: 'small', bordered: false }, () => '已停權'),
        h('span', { style: 'font-size: 12px; opacity: 0.6' }, until),
      ])
    },
  },
  {
    title: 'Pro',
    key: 'is_pro',
    width: 90,
    render: (row) =>
      h(
        NTag,
        { type: row.is_pro ? 'success' : 'default', size: 'small', bordered: false },
        () => (row.is_pro ? 'Pro' : '免費'),
      ),
  },
  {
    title: '到期 / 來源',
    key: 'pro_expires_at',
    render: (row) => {
      if (!row.is_pro) return h('span', { style: 'opacity: 0.4' }, '—')
      const expiry = row.pro_expires_at
        ? new Date(row.pro_expires_at).toLocaleDateString('zh-TW')
        : '永久'
      return h('span', { style: 'font-size: 13px' }, `${expiry}（${row.pro_store ?? '—'}）`)
    },
  },
  {
    title: '',
    key: 'actions',
    width: 250,
    render: (row) => {
      const loading = submitting.value.has(row.id)
      const pro = row.is_pro
        ? h(
            NPopconfirm,
            { onPositiveClick: () => revoke(row) },
            {
              trigger: () =>
                h(
                  NButton,
                  { size: 'small', type: 'error', quaternary: true, loading },
                  () => '撤銷 Pro',
                ),
              default: () => `撤銷 ${row.email} 的 Pro？商店訂閱下次同步時仍會蓋過這個狀態。`,
            },
          )
        : h(NButton, { size: 'small', onClick: () => openGrant(row) }, () => '補發 Pro')

      // 停權會連帶下架他架上的碼，解除只還「因停權下架」的那批，兩邊都要
      // 二次確認 —— 誤按的代價是一個人的碼整批消失。
      const suspend =
        row.status === 'suspended'
          ? h(
              NPopconfirm,
              { onPositiveClick: () => reinstate(row) },
              {
                trigger: () => h(NButton, { size: 'small', loading }, () => '解除停權'),
                default: () =>
                  `解除 ${row.email} 的停權？當初因停權被下架的碼會放回架上。`,
              },
            )
          : h(NButton, { size: 'small', type: 'error', onClick: () => openSuspend(row) }, () => '停權')

      return h(NSpace, {}, () => [pro, suspend])
    },
  },
]
</script>

<template>
  <div>
    <NSpace align="center" justify="space-between" style="margin-bottom: 16px">
      <h2 style="margin: 0">使用者（{{ total }}）</h2>
      <NSpace>
        <NInput
          v-model:value="query"
          placeholder="搜尋 email"
          clearable
          style="width: 240px"
          @keyup.enter="search"
        />
        <NButton size="small" :loading="loading" @click="search">搜尋</NButton>
      </NSpace>
    </NSpace>

    <NText depth="3" style="font-size: 13px; display: block; margin-bottom: 12px">
      訂閱狀態的真相在 RevenueCat（見 webhook）；這裡的補發/撤銷是客服用的手動覆蓋，
      商店那邊真的送事件過來時一樣會蓋過去。
    </NText>

    <NAlert v-if="loadError" type="error" style="margin-bottom: 16px">{{ loadError }}</NAlert>

    <NDataTable
      :columns="columns"
      :data="users"
      :loading="loading"
      :row-key="(row: AdminUserItem) => row.id"
      size="small"
    />

    <!-- 標題那個數字是全站總人數，但清單一次只給 50 筆 —— 沒有這個，
         第 51 個使用者起就查不到，而畫面上完全看不出少了東西。 -->
    <NPagination
      v-if="total > PAGE_SIZE"
      v-model:page="page"
      :page-size="PAGE_SIZE"
      :item-count="total"
      style="margin-top: 16px; justify-content: flex-end"
    />

    <NModal
      :show="granting !== null"
      preset="dialog"
      title="補發 Pro"
      positive-text="確認補發"
      negative-text="取消"
      @update:show="(show: boolean) => { if (!show) granting = null }"
      @positive-click="confirmGrant"
    >
      <p>
        <strong>{{ granting?.email }}</strong>
      </p>
      <NSpace vertical>
        <NText depth="3" style="font-size: 13px">到期日期，留空代表永久授權</NText>
        <NDatePicker v-model:value="grantExpiresAt" type="date" clearable style="width: 100%" />
      </NSpace>
    </NModal>

    <NModal
      :show="suspending !== null"
      preset="dialog"
      title="停權使用者"
      positive-text="確認停權"
      negative-text="取消"
      @update:show="(show: boolean) => { if (!show) suspending = null }"
      @positive-click="confirmSuspend"
    >
      <p>
        <strong>{{ suspending?.email }}</strong>
      </p>
      <NSpace vertical>
        <NText depth="3" style="font-size: 13px">
          停權會立刻把他架上的碼全部下架、並讓他無法登入與上架。解除時只會還回
          「因為這次停權才被下架」的那些。
        </NText>
        <NText depth="3" style="font-size: 13px">停權到哪一天（含當天），留空代表無限期</NText>
        <NDatePicker v-model:value="suspendUntil" type="date" clearable style="width: 100%" />
      </NSpace>
    </NModal>
  </div>
</template>
