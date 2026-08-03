<script setup lang="ts">
import {
  NAlert,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSpace,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { h, onMounted, ref } from 'vue'

import { ApiError, api } from '../api/client'
import type { Category } from '../api/types'

const message = useMessage()

const categories = ref<Category[]>([])
const loading = ref(false)
const loadError = ref('')

const showForm = ref(false)
const editing = ref<Category | null>(null)
const submitting = ref(false)
const form = ref({ slug: '', name: '', sort_order: 0 })

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    categories.value = (await api.listCategories()).categories
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = { slug: '', name: '', sort_order: 0 }
  showForm.value = true
}

function openEdit(c: Category) {
  editing.value = c
  form.value = { slug: c.slug, name: c.name, sort_order: c.sort_order }
  showForm.value = true
}

async function submit() {
  submitting.value = true
  try {
    if (editing.value) {
      await api.updateCategory(editing.value.id, {
        slug: form.value.slug,
        name: form.value.name,
        sort_order: form.value.sort_order,
      })
    } else {
      await api.createCategory(form.value)
    }
    message.success('已儲存')
    showForm.value = false
    await load()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '儲存失敗')
  } finally {
    submitting.value = false
  }
}

async function remove(c: Category) {
  try {
    await api.deleteCategory(c.id)
    message.success('已刪除')
    await load()
  } catch (e) {
    // 後端會擋還有服務商掛著的分類（category_in_use），訊息比較長，
    // 拉長顯示時間，不要讓人來不及看完就消失。
    message.error(e instanceof ApiError ? e.message : '刪除失敗', { duration: 6000 })
  }
}

const columns: DataTableColumns<Category> = [
  { title: '排序', key: 'sort_order', width: 80 },
  { title: '名稱', key: 'name' },
  { title: 'slug', key: 'slug' },
  {
    title: '',
    key: 'actions',
    width: 140,
    render: (row) =>
      h(NSpace, {}, () => [
        h(NButton, { size: 'small', onClick: () => openEdit(row) }, () => '編輯'),
        h(
          NPopconfirm,
          { onPositiveClick: () => remove(row) },
          {
            trigger: () => h(NButton, { size: 'small', type: 'error' }, () => '刪除'),
            default: () => `刪除分類「${row.name}」？還有服務商掛著的話會失敗。`,
          },
        ),
      ]),
  },
]
</script>

<template>
  <div>
    <NSpace align="center" justify="space-between" style="margin-bottom: 16px">
      <h2 style="margin: 0">分類</h2>
      <NButton size="small" type="primary" @click="openCreate">新增分類</NButton>
    </NSpace>

    <NAlert v-if="loadError" type="error" style="margin-bottom: 16px">{{ loadError }}</NAlert>

    <NDataTable
      :columns="columns"
      :data="categories"
      :loading="loading"
      :row-key="(row: Category) => row.id"
      size="small"
    />

    <NModal
      v-model:show="showForm"
      preset="card"
      :title="editing ? `編輯 ${editing.name}` : '新增分類'"
      style="width: 460px"
    >
      <NForm>
        <NFormItem label="slug">
          <NInput v-model:value="form.slug" placeholder="bank" />
          <template #feedback>
            出現在 /category/{slug} 的網址上。改掉之後舊網址會自動 301 轉到新的。
          </template>
        </NFormItem>
        <NFormItem label="名稱">
          <NInput v-model:value="form.name" placeholder="銀行信用卡" />
        </NFormItem>
        <NFormItem label="排序">
          <NInputNumber v-model:value="form.sort_order" />
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
