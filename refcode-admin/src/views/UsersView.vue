<script setup lang="ts">
import {
  NAlert,
  NButton,
  NDataTable,
  NDatePicker,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { h, onMounted, ref } from 'vue'

import { ApiError, api } from '../api/client'
import type { AdminUserItem } from '../api/types'

const message = useMessage()

const users = ref<AdminUserItem[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const query = ref('')

const granting = ref<AdminUserItem | null>(null)
const grantExpiresAt = ref<number | null>(null)
const submitting = ref(new Set<string>())

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.listUsers(query.value.trim())
    users.value = res.users
    total.value = res.total
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

onMounted(load)

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
    width: 160,
    render: (row) =>
      h(NSpace, {}, () =>
        row.is_pro
          ? [
              h(
                NPopconfirm,
                { onPositiveClick: () => revoke(row) },
                {
                  trigger: () =>
                    h(
                      NButton,
                      { size: 'small', type: 'error', quaternary: true, loading: submitting.value.has(row.id) },
                      () => '撤銷 Pro',
                    ),
                  default: () => `撤銷 ${row.email} 的 Pro？商店訂閱下次同步時仍會蓋過這個狀態。`,
                },
              ),
            ]
          : [
              h(
                NButton,
                { size: 'small', onClick: () => openGrant(row) },
                () => '補發 Pro',
              ),
            ],
      ),
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
          @keyup.enter="load"
        />
        <NButton size="small" :loading="loading" @click="load">搜尋</NButton>
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
  </div>
</template>
