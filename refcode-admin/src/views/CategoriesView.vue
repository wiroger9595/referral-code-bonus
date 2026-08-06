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
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
// name_en / name_ja 留空就存 NULL，公開 API 那邊會退回中文（見 refcode-api 的 localized）。
const form = ref({
  name: '',
  name_en: '',
  name_ja: '',
  sort_order: 0,
  image_url: null as string | null,
})

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
  form.value = { name: '', name_en: '', name_ja: '', sort_order: 0, image_url: null }
  showForm.value = true
}

function openEdit(c: Category) {
  editing.value = c
  form.value = {
    name: c.name,
    name_en: c.name_en ?? '',
    name_ja: c.name_ja ?? '',
    sort_order: c.sort_order,
    image_url: c.image_url,
  }
  showForm.value = true
}

function pickImage() {
  fileInput.value?.click()
}

async function onImageSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  uploading.value = true
  try {
    form.value.image_url = await api.uploadImage(file, 'categories')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '上傳失敗')
  } finally {
    uploading.value = false
    // 清空 value，不然選同一個檔案第二次不會觸發 change。
    input.value = ''
  }
}

async function submit() {
  submitting.value = true
  try {
    if (editing.value) {
      await api.updateCategory(editing.value.id, {
        name: form.value.name,
        name_en: form.value.name_en,
        name_ja: form.value.name_ja,
        sort_order: form.value.sort_order,
        image_url: form.value.image_url,
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
  {
    title: '圖片',
    key: 'image_url',
    width: 60,
    render: (row) =>
      row.image_url
        ? h('img', {
            src: row.image_url,
            style: 'width: 32px; height: 32px; object-fit: cover; border-radius: 4px',
          })
        : h('span', { style: 'opacity: 0.3' }, '—'),
  },
  { title: '排序', key: 'sort_order', width: 80 },
  { title: '名稱', key: 'name' },
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
        <NFormItem label="名稱（中文）">
          <NInput v-model:value="form.name" placeholder="銀行信用卡" />
        </NFormItem>
        <NFormItem label="名稱（English）">
          <NInput v-model:value="form.name_en" placeholder="留空的話英文站顯示中文" />
        </NFormItem>
        <NFormItem label="名稱（日本語）">
          <NInput v-model:value="form.name_ja" placeholder="留空的話日文站顯示中文" />
        </NFormItem>
        <NFormItem label="排序">
          <NInputNumber v-model:value="form.sort_order" />
        </NFormItem>
        <NFormItem label="圖片">
          <NSpace align="center">
            <img
              v-if="form.image_url"
              :src="form.image_url"
              style="width: 56px; height: 56px; object-fit: cover; border-radius: 6px"
            />
            <NButton size="small" :loading="uploading" @click="pickImage">
              {{ form.image_url ? '更換圖片' : '上傳圖片' }}
            </NButton>
            <NButton v-if="form.image_url" size="small" quaternary @click="form.image_url = null">
              移除
            </NButton>
            <input
              ref="fileInput"
              type="file"
              accept="image/*"
              style="display: none"
              @change="onImageSelected"
            />
          </NSpace>
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
