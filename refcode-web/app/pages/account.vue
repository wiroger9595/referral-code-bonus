<script setup lang="ts">
// 官網目前只有大頭照能改。顯示名稱與所在地還是在 app 裡改 ——
// 後端的 PATCH /v1/me 是整份覆寫，兩邊都做等於兩個地方各自維護同一份表單。
const { t } = useI18n()
const localePath = useLocalePath()
const { user, isLoggedIn, uploadAvatar } = useAuth()
const apiErrorMessage = useApiError()

// 個人頁沒有給搜尋引擎看的價值，而且網址底下是使用者自己的資料。
useSeoMeta({
  title: () => t('account.title'),
  robots: 'noindex, nofollow',
})

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const errorMessage = ref('')

function initial() {
  const name = user.value?.display_name || user.value?.email || ''
  return name.trim().charAt(0).toUpperCase()
}

async function onPicked(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // 選完就清空，不然連續選同一個檔案不會再觸發 change。
  input.value = ''
  if (!file) return

  errorMessage.value = ''
  uploading.value = true
  try {
    await uploadAvatar(await toAvatarBlob(file))
  } catch (e) {
    errorMessage.value = apiErrorMessage(e)
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <article class="space-y-6">
    <h1 class="text-2xl font-semibold">{{ $t('account.title') }}</h1>

    <section v-if="!isLoggedIn" class="space-y-3">
      <p class="text-muted">{{ $t('account.signInFirst') }}</p>
      <NuxtLink :to="{ path: localePath('/login'), query: { redirect: localePath('/account') } }" class="btn btn-primary">
        {{ $t('nav.login') }}
      </NuxtLink>
    </section>

    <template v-else>
      <section class="flex items-center gap-4">
        <button
          type="button"
          :aria-label="$t('account.avatarChange')"
          :disabled="uploading"
          class="relative size-20 shrink-0 overflow-hidden rounded-full bg-brand-soft text-2xl font-bold text-brand-ink disabled:opacity-60"
          @click="fileInput?.click()"
        >
          <img v-if="user?.avatar_url" :src="user.avatar_url" alt="" class="size-full object-cover" />
          <span v-else>{{ initial() }}</span>
          <span class="absolute inset-x-0 bottom-0 bg-black/45 py-0.5 text-[11px] font-medium text-white">
            {{ $t('account.avatarEdit') }}
          </span>
        </button>

        <div class="min-w-0">
          <p class="truncate font-semibold">{{ user?.display_name }}</p>
          <p class="truncate text-sm text-muted">{{ user?.email }}</p>
          <p v-if="uploading" class="text-sm text-muted">{{ $t('account.avatarUploading') }}</p>
          <p v-else-if="errorMessage" class="text-sm text-alert-ink">{{ errorMessage }}</p>
        </div>
      </section>

      <!-- accept 交給系統決定要開相機還是檔案選取。 -->
      <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onPicked" />

      <p class="text-sm text-muted">{{ $t('account.avatarHint') }}</p>

      <!-- 站頭的那個連結在窄螢幕是收起來的，這裡是手機上唯一的入口。 -->
      <section class="border-t border-line pt-6">
        <NuxtLink :to="localePath('/my-codes')" class="btn btn-outline">
          {{ $t('myCodes.title') }}
        </NuxtLink>
      </section>

      <section class="space-y-2 border-t border-line pt-6">
        <h2 class="font-medium">{{ $t('account.otherTitle') }}</h2>
        <p class="text-sm text-muted">{{ $t('account.otherBody') }}</p>
        <NuxtLink :to="localePath('/delete-account')" class="inline-block text-sm underline">
          {{ $t('account.deleteLink') }}
        </NuxtLink>
      </section>
    </template>
  </article>
</template>
