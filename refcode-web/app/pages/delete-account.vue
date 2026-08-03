<script setup lang="ts">
// Google Play 要求提供一個「不必安裝 app 就能送出刪除請求」的網址，
// 而且那個網址要能真的完成刪除，不能只是說明頁。這頁就是填進 Play Console
// 「資料刪除」欄位的那一個，所以它必須自己走完整個流程。
const { t } = useI18n()
const localePath = useLocalePath()
const { public: cfg } = useRuntimeConfig()
const { user, isLoggedIn, deleteAccount } = useAuth()
const apiErrorMessage = useApiError()

// 這頁不該被索引，但**必須是公開可存取的** —— Play 會實際去抓，
// 擋爬蟲或要求登入才看得到頁面都會被判定不合格。這裡的做法是：頁面本身公開，
// 真正的刪除動作才需要登入。
useSeoMeta({
  title: () => t('deleteAccount.title'),
  robots: 'noindex, follow',
})

const confirmText = ref('')
const submitting = ref(false)
const errorMessage = ref('')
const done = ref(false)

async function submit() {
  const email = user.value?.email ?? ''
  if (confirmText.value.trim().toLowerCase() !== email.toLowerCase()) {
    errorMessage.value = t('deleteAccount.confirmMismatch')
    return
  }

  errorMessage.value = ''
  submitting.value = true
  try {
    await deleteAccount(confirmText.value.trim())
    done.value = true
  } catch (e) {
    errorMessage.value = apiErrorMessage(e)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <article class="space-y-6">
    <h1 class="text-2xl font-semibold">{{ $t('deleteAccount.title') }}</h1>

    <!-- 刪完之後 user 已經被清掉，不要再 render 下面的表單。 -->
    <template v-if="done">
      <div class="rounded-lg border border-emerald-600/30 bg-emerald-50 p-4 dark:bg-emerald-950/30">
        <h2 class="font-medium">{{ $t('deleteAccount.doneTitle') }}</h2>
        <p class="mt-1 text-neutral-600 dark:text-neutral-400">{{ $t('deleteAccount.doneBody') }}</p>
      </div>
      <NuxtLink :to="localePath('/')" class="inline-block underline">
        {{ $t('deleteAccount.backHome') }}
      </NuxtLink>
    </template>

    <template v-else>
      <p class="text-neutral-600 dark:text-neutral-400">{{ $t('deleteAccount.lead') }}</p>

      <section class="space-y-2">
        <h2 class="font-medium">{{ $t('deleteAccount.whatHappensTitle') }}</h2>
        <ul class="list-disc space-y-1 pl-5 text-neutral-600 dark:text-neutral-400">
          <li v-for="(item, i) in ($tm('deleteAccount.whatHappens') as unknown[])" :key="i">
            {{ $rt(item as string) }}
          </li>
        </ul>
      </section>

      <section class="space-y-2">
        <h2 class="font-medium">{{ $t('deleteAccount.keptTitle') }}</h2>
        <p class="text-neutral-600 dark:text-neutral-400">{{ $t('deleteAccount.kept') }}</p>
      </section>

      <!-- 訂閱不會因為刪帳號而停止扣款，這件事一定要在按下去之前講清楚。 -->
      <section class="rounded-lg border border-amber-500/40 bg-amber-50 p-4 dark:bg-amber-950/30">
        <h2 class="font-medium">{{ $t('deleteAccount.subscriptionTitle') }}</h2>
        <p class="mt-1 text-neutral-700 dark:text-neutral-300">
          {{ $t('deleteAccount.subscriptionNote') }}
        </p>
      </section>

      <section v-if="!isLoggedIn" class="space-y-3">
        <p class="text-neutral-600 dark:text-neutral-400">{{ $t('deleteAccount.signInFirst') }}</p>
        <NuxtLink
          :to="localePath('/login')"
          class="inline-block rounded-lg bg-neutral-900 px-4 py-2 text-white dark:bg-white dark:text-neutral-900"
        >
          {{ $t('deleteAccount.signIn') }}
        </NuxtLink>
      </section>

      <section v-else class="space-y-3 rounded-lg border border-red-500/40 p-4">
        <label class="block space-y-1">
          <span class="font-medium">{{ $t('deleteAccount.confirmLabel') }}</span>
          <input
            v-model="confirmText"
            type="email"
            autocapitalize="off"
            autocorrect="off"
            :placeholder="user?.email"
            class="w-full rounded-lg border border-neutral-300 px-3 py-2 dark:border-neutral-700 dark:bg-neutral-900"
          />
        </label>

        <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>

        <p class="text-sm text-neutral-600 dark:text-neutral-400">
          {{ $t('deleteAccount.irreversible') }}
        </p>

        <button
          type="button"
          :disabled="submitting || !confirmText"
          class="rounded-lg bg-red-600 px-4 py-2 text-white disabled:opacity-50"
          @click="submit"
        >
          {{ $t('deleteAccount.submit') }}
        </button>
      </section>

      <p v-if="cfg.supportEmail" class="text-sm text-neutral-500">
        {{ $t('deleteAccount.contactNote', { email: cfg.supportEmail }) }}
      </p>
    </template>
  </article>
</template>
