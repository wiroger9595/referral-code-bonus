<script setup lang="ts">
import { NAlert, NButton, NCard, NForm, NFormItem, NInput } from 'naive-ui'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const email = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    const redirect = route.query.redirect
    router.push(typeof redirect === 'string' ? redirect : { name: 'review' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '連線失敗，請確認 API 是否啟動'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="wrap">
    <NCard title="後台登入" style="width: 380px">
      <NAlert v-if="error" type="error" style="margin-bottom: 16px">{{ error }}</NAlert>

      <NForm @submit.prevent="submit">
        <NFormItem label="Email">
          <NInput v-model:value="email" placeholder="admin@example.com" />
        </NFormItem>
        <NFormItem label="密碼">
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            @keyup.enter="submit"
          />
        </NFormItem>
        <NButton type="primary" block :loading="loading" @click="submit">登入</NButton>
      </NForm>
    </NCard>
  </div>
</template>

<style scoped>
.wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
}
</style>
