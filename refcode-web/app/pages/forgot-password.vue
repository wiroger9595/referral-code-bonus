<script setup lang="ts">
const auth = useAuth()
const route = useRoute()
const router = useRouter()
// 信件語言跟著使用者當下瀏覽的語言版本（/ja/... 就是日文）。
const { t, locale } = useI18n()
const localePath = useLocalePath()
const apiErrorMessage = useApiError()

definePageMeta({
  // 已經登入的人沒有理由走重設流程；要換密碼是帳號設定的事。
  middleware: [() => (useAuth().isLoggedIn.value ? navigateTo(useLocalePath()('/')) : undefined)],
})

// 跟 /login 一樣沒有給搜尋引擎看的價值。
useSeoMeta({
  title: () => t('auth.forgotTitle'),
  robots: 'noindex, nofollow',
})

// 兩步在同一頁：寄出驗證碼之後換掉表單，網址不變 —— 重新整理就等於重來一次，
// 那是對的行為，驗證碼本來就在信箱裡而不在網址上。
const step = ref<'email' | 'code'>('email')
const email = ref('')
const code = ref('')
const password = ref('')
const error = ref('')
const notice = ref('')
const pending = ref(false)

// 只收站內的絕對路徑，不然這個參數就成了轉址跳板（跟 AuthPanel 同一個理由）。
const redirectTo = computed(() => {
  const to = route.query.redirect
  return typeof to === 'string' && to.startsWith('/') && !to.startsWith('//')
    ? to
    : localePath('/')
})

async function run(action: () => Promise<void>) {
  if (pending.value) return
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    await action()
  } catch (e) {
    error.value = apiErrorMessage(e)
  } finally {
    pending.value = false
  }
}

function sendCode() {
  return run(async () => {
    await auth.forgotPassword(email.value.trim(), locale.value)
    step.value = 'code'
  })
}

function resend() {
  return run(async () => {
    await auth.forgotPassword(email.value.trim(), locale.value)
    // 舊碼在後端已經被新的覆蓋掉了，留在欄位裡只會讓人拿去試。
    code.value = ''
    notice.value = t('auth.forgotResent')
  })
}

function submit() {
  // 後端擋得住，但先講一句省一次往返，也省掉一組被浪費的驗證碼。
  if (password.value.length < 8) {
    error.value = t('auth.passwordTooShort')
    return
  }

  return run(async () => {
    await auth.resetPassword(email.value.trim(), code.value.trim(), password.value)
    await router.replace(redirectTo.value)
  })
}
</script>

<template>
  <div class="mx-auto max-w-sm">
    <h1 class="text-2xl font-semibold">{{ $t('auth.forgotTitle') }}</h1>
    <p class="mt-2 text-sm text-muted">
      {{
        step === 'email'
          ? $t('auth.forgotLeadEmail')
          : $t('auth.forgotLeadCode', { email: email.trim() })
      }}
    </p>

    <p
      v-if="error"
      class="mt-6 rounded-card bg-alert-soft p-3 text-sm text-alert-ink"
    >
      {{ error }}
    </p>
    <p
      v-else-if="notice"
      class="mt-6 rounded-md bg-green-50 p-3 text-sm text-green-700 dark:bg-green-950 dark:text-green-200"
    >
      {{ notice }}
    </p>

    <form v-if="step === 'email'" class="mt-6 space-y-4" @submit.prevent="sendCode">
      <div>
        <label for="email" class="block text-sm font-medium">{{ $t('auth.email') }}</label>
        <input
          id="email"
          v-model="email"
          type="email"
          required
          autocomplete="email"
          class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none focus:border-brand"
        />
      </div>

      <button
        type="submit"
        :disabled="pending"
        class="btn btn-primary w-full disabled:opacity-50"
      >
        {{ pending ? $t('auth.pending') : $t('auth.forgotSendCode') }}
      </button>
    </form>

    <form v-else class="mt-6 space-y-4" @submit.prevent="submit">
      <div>
        <label for="code" class="block text-sm font-medium">{{ $t('auth.forgotCode') }}</label>
        <input
          id="code"
          v-model="code"
          type="text"
          required
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          :placeholder="$t('auth.forgotCodePlaceholder')"
          class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm tracking-widest outline-none placeholder:tracking-normal placeholder:text-muted focus:border-brand"
        />
      </div>

      <div>
        <label for="password" class="block text-sm font-medium">
          {{ $t('auth.forgotNewPassword') }}
        </label>
        <input
          id="password"
          v-model="password"
          type="password"
          required
          autocomplete="new-password"
          class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none focus:border-brand"
        />
        <p class="mt-1 text-xs text-muted">{{ $t('auth.passwordHint') }}</p>
      </div>

      <button
        type="submit"
        :disabled="pending"
        class="btn btn-primary w-full disabled:opacity-50"
      >
        {{ pending ? $t('auth.pending') : $t('auth.forgotSubmit') }}
      </button>

      <button
        type="button"
        :disabled="pending"
        class="w-full text-center text-sm text-muted hover:underline disabled:opacity-50"
        @click="resend"
      >
        {{ $t('auth.forgotResend') }}
      </button>
    </form>

    <p class="mt-8 text-center text-sm text-muted">
      <NuxtLink
        :to="localePath('/login')"
        class="font-semibold text-brand hover:underline"
      >
        {{ $t('auth.backToLogin') }}
      </NuxtLink>
    </p>
  </div>
</template>
