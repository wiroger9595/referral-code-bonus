<script setup lang="ts">
import {
  NAlert,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { h, onMounted, ref } from 'vue'

import { ApiError, api } from '../api/client'
import type { AdminMerchant, Category, MerchantInput } from '../api/types'

const message = useMessage()

const merchants = ref<AdminMerchant[]>([])
const categories = ref<Category[]>([])
const loading = ref(false)
const loadError = ref('')

const showForm = ref(false)
const editing = ref<AdminMerchant | null>(null)
const submitting = ref(false)

function emptyForm(): MerchantInput {
  return {
    slug: '',
    name: '',
    category_id: '',
    logo_url: null,
    signup_url: 'https://',
    reward_desc: '',
    code_format_regex: null,
    is_active: true,
    countries: [],
  }
}

const form = ref<MerchantInput>(emptyForm())

// 常用選項而已。NSelect 開了 tag，其他 ISO 3166-1 alpha-2 代碼可以直接打進去，
// 後端只驗格式（refcode-api 的 internal/geo）。
const COUNTRY_CODES = ['TW', 'JP', 'HK', 'SG', 'MY', 'KR', 'CN', 'US', 'GB', 'AU', 'CA']
const countryNames = new Intl.DisplayNames(['zh-TW'], { type: 'region' })
const countryOptions = COUNTRY_CODES.map((code) => ({
  label: `${countryNames.of(code) ?? code}（${code}）`,
  value: code,
}))

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const [m, c] = await Promise.all([api.listMerchants(), api.listCategories()])
    merchants.value = m.merchants
    categories.value = c.categories
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = emptyForm()
  showForm.value = true
}

function openEdit(m: AdminMerchant) {
  editing.value = m
  form.value = {
    slug: m.slug,
    name: m.name,
    category_id: m.category_id,
    logo_url: m.logo_url,
    signup_url: m.signup_url,
    reward_desc: m.reward_desc,
    code_format_regex: m.code_format_regex,
    is_active: m.is_active,
    countries: m.countries,
  }
  showForm.value = true
}

async function submit() {
  // 空字串跟 null 對後端是兩回事：null 代表「不驗格式」。
  const payload: MerchantInput = {
    ...form.value,
    logo_url: form.value.logo_url?.trim() || null,
    code_format_regex: form.value.code_format_regex?.trim() || null,
  }

  submitting.value = true
  try {
    if (editing.value) {
      await api.updateMerchant(editing.value.id, payload)
      message.success('已更新')
    } else {
      await api.createMerchant(payload)
      message.success('已新增')
    }
    showForm.value = false
    await load()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '儲存失敗')
  } finally {
    submitting.value = false
  }
}

const columns: DataTableColumns<AdminMerchant> = [
  {
    title: '名稱',
    key: 'name',
    render: (row) =>
      h('div', [
        h('div', { style: 'font-weight: 500' }, row.name),
        h('div', { style: 'font-size: 12px; opacity: 0.6' }, row.slug),
      ]),
  },
  { title: '分類', key: 'category_name' },
  {
    title: '可用碼',
    key: 'active_code_count',
    width: 90,
    render: (row) => String(row.active_code_count),
  },
  {
    title: '適用國家',
    key: 'countries',
    render: (row) =>
      row.countries.length
        ? h('span', { style: 'font-size: 12px' }, row.countries.join('、'))
        : h('span', { style: 'opacity: 0.4' }, '不分地區'),
  },
  {
    title: '格式規則',
    key: 'code_format_regex',
    render: (row) =>
      row.code_format_regex
        ? h('code', { style: 'font-size: 12px' }, row.code_format_regex)
        : h('span', { style: 'opacity: 0.4' }, '不驗'),
  },
  {
    title: '狀態',
    key: 'is_active',
    width: 90,
    render: (row) =>
      h(
        NTag,
        { type: row.is_active ? 'success' : 'default', size: 'small', bordered: false },
        () => (row.is_active ? '上架中' : '已停用'),
      ),
  },
  {
    title: '',
    key: 'actions',
    width: 80,
    render: (row) =>
      h(NButton, { size: 'small', quaternary: true, onClick: () => openEdit(row) }, () => '編輯'),
  },
]

const categoryOptions = () =>
  categories.value.map((c) => ({ label: c.name, value: c.id }))
</script>

<template>
  <div>
    <NSpace align="center" justify="space-between" style="margin-bottom: 16px">
      <h2 style="margin: 0">服務商（{{ merchants.length }}）</h2>
      <NSpace>
        <NButton size="small" :loading="loading" @click="load">重新整理</NButton>
        <NButton size="small" type="primary" @click="openCreate">新增服務商</NButton>
      </NSpace>
    </NSpace>

    <NAlert v-if="loadError" type="error" style="margin-bottom: 16px">{{ loadError }}</NAlert>

    <NDataTable
      :columns="columns"
      :data="merchants"
      :loading="loading"
      :row-key="(row: AdminMerchant) => row.id"
      size="small"
    />

    <NModal
      v-model:show="showForm"
      preset="card"
      :title="editing ? `編輯 ${editing.name}` : '新增服務商'"
      style="width: 560px"
    >
      <NForm>
        <NFormItem label="slug">
          <NInput v-model:value="form.slug" placeholder="ooo-bank（小寫英數與連字號）" />
          <template #feedback>
            出現在 /referral/{slug} 的網址上。改掉之後舊網址會自動 301 轉到新的，
            但搜尋引擎重新收錄要一段時間，沒必要不要改。
          </template>
        </NFormItem>
        <NFormItem label="名稱">
          <NInput v-model:value="form.name" placeholder="OOO 銀行" />
        </NFormItem>
        <NFormItem label="分類">
          <NSelect v-model:value="form.category_id" :options="categoryOptions()" />
        </NFormItem>
        <NFormItem label="註冊連結">
          <NInput v-model:value="form.signup_url" placeholder="https://..." />
        </NFormItem>
        <NFormItem label="獎勵說明">
          <NInput v-model:value="form.reward_desc" placeholder="雙方各得 500 元" />
        </NFormItem>
        <NFormItem label="Logo URL">
          <NInput v-model:value="form.logo_url as string" placeholder="留空代表沒有" />
        </NFormItem>
        <NFormItem label="適用國家">
          <!-- 留空＝不分地區（串流、雲端這種）。使用者的所在地對得上就排前面，
               對不上排後面，但兩種都看得到。 -->
          <NSelect
            v-model:value="form.countries"
            multiple
            filterable
            tag
            :options="countryOptions"
            placeholder="留空代表不分地區"
          />
        </NFormItem>
        <NFormItem label="碼格式規則">
          <NInput
            v-model:value="form.code_format_regex as string"
            placeholder="^[A-Z0-9]{6,10}$（留空代表不驗）"
          />
        </NFormItem>
        <NFormItem v-if="editing" label="上架中">
          <NSwitch v-model:value="form.is_active as boolean" />
        </NFormItem>
      </NForm>

      <template #footer>
        <NSpace justify="end">
          <NButton @click="showForm = false">取消</NButton>
          <NButton type="primary" :loading="submitting" @click="submit">儲存</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>
