<script setup lang="ts">
// 登入與註冊只差一個欄位跟幾句文案，共用同一塊；/login 與 /register 各自只放 SEO meta。
const props = defineProps<{ mode: 'login' | 'register' }>()

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const localePath = useLocalePath()
const apiErrorMessage = useApiError()
const { options: countryOptions, defaultCountry } = useCountries()

const isRegister = computed(() => props.mode === 'register')

const email = ref('')
const password = ref('')
const displayName = ref('')
// 預設跟著介面語言猜，使用者可以改成「不指定」或別的地方。
const country = ref(defaultCountry.value)
const error = ref('')
const pending = ref(false)

// 登入前想去的頁面。只收站內的絕對路徑，不然這個參數就成了轉址跳板。
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
  try {
    await action()
    await router.replace(redirectTo.value)
  } catch (e) {
    error.value = apiErrorMessage(e)
  } finally {
    pending.value = false
  }
}

function submit() {
  // 後端擋得住，但在送出前就講比較不浪費一次往返。
  if (isRegister.value && password.value.length < 8) {
    error.value = t('auth.passwordTooShort')
    return
  }

  return run(() =>
    isRegister.value
      ? auth.register(email.value.trim(), password.value, displayName.value.trim(), country.value)
      : auth.login(email.value.trim(), password.value),
  )
}

const {
  target: googleTarget,
  enabled: googleEnabled,
  failed: googleFailed,
} = useGoogleSignIn((idToken) =>
  // 註冊頁上選的所在地一起送 —— 這顆按鈕對還沒有帳號的人來說就是註冊。
  run(() => auth.loginWithProvider('google', idToken, isRegister.value ? country.value : '')),
)
</script>

<template>
  <div class="mx-auto max-w-sm">
    <h1 class="text-2xl font-bold tracking-tight">
      {{ isRegister ? $t('auth.registerTitle') : $t('auth.loginTitle') }}
    </h1>
    <p class="mt-2 text-sm/relaxed text-muted">
      {{ isRegister ? $t('auth.registerLead') : $t('auth.loginLead') }}
    </p>

    <!-- role="alert" 不能省：這段是送出失敗後才插進 DOM 的，沒有它螢幕閱讀器
         不會唸出來，使用者只會覺得按了送出沒有任何反應。 -->
    <p v-if="error" role="alert" class="mt-6 rounded-card bg-alert-soft p-3 text-sm text-alert-ink">
      {{ error }}
    </p>

    <!-- 表單放進卡片，跟目錄頁的卡片式版面同一套語言。 -->
    <form class="app-card mt-6 space-y-4 p-5" @submit.prevent="submit">
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

      <div v-if="isRegister">
        <label for="display-name" class="block text-sm font-medium">
          {{ $t('auth.displayName') }}
        </label>
        <input
          id="display-name"
          v-model="displayName"
          type="text"
          autocomplete="nickname"
          :placeholder="$t('auth.displayNamePlaceholder')"
          class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none placeholder:text-muted focus:border-brand"
        />
        <p class="mt-1 text-xs text-muted">{{ $t('auth.displayNameHint') }}</p>
      </div>

      <div v-if="isRegister">
        <label for="country" class="block text-sm font-medium">{{ $t('auth.country') }}</label>
        <select
          id="country"
          v-model="country"
          autocomplete="country"
          class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none focus:border-brand"
        >
          <option value="">{{ $t('auth.countryUnset') }}</option>
          <option v-for="c in countryOptions" :key="c.code" :value="c.code">{{ c.label }}</option>
        </select>
        <p class="mt-1 text-xs text-muted">{{ $t('auth.countryHint') }}</p>
      </div>

      <div>
        <label for="password" class="block text-sm font-medium">{{ $t('auth.password') }}</label>
        <input
          id="password"
          v-model="password"
          type="password"
          required
          :autocomplete="isRegister ? 'new-password' : 'current-password'"
          class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none focus:border-brand"
        />
        <p v-if="isRegister" class="mt-1 text-xs text-muted">{{ $t('auth.passwordHint') }}</p>
        <p v-else class="mt-1 text-right text-xs">
          <NuxtLink
            :to="localePath('/forgot-password')"
            class="text-muted hover:underline"
          >
            {{ $t('auth.forgotLink') }}
          </NuxtLink>
        </p>
      </div>

      <button
        type="submit"
        :disabled="pending"
        class="btn btn-primary w-full disabled:opacity-50"
      >
        {{
          pending
            ? $t('auth.pending')
            : isRegister
              ? $t('auth.submitRegister')
              : $t('auth.submitLogin')
        }}
      </button>
    </form>

    <div v-if="googleEnabled" class="mt-6">
      <div class="mb-4 flex items-center gap-3 text-xs text-muted">
        <span class="h-px flex-1 bg-line" />
        {{ $t('auth.or') }}
        <span class="h-px flex-1 bg-line" />
      </div>

      <div ref="googleTarget" class="flex justify-center" />

      <p v-if="googleFailed" class="mt-2 text-center text-xs text-muted">
        {{ $t('auth.googleFailed') }}
      </p>
    </div>

    <p class="mt-8 text-center text-sm text-muted">
      <template v-if="isRegister">
        {{ $t('auth.haveAccount') }}
        <NuxtLink
          :to="localePath('/login')"
          class="font-semibold text-brand hover:underline"
        >
          {{ $t('auth.toLogin') }}
        </NuxtLink>
      </template>
      <template v-else>
        {{ $t('auth.noAccount') }}
        <NuxtLink
          :to="localePath('/register')"
          class="font-semibold text-brand hover:underline"
        >
          {{ $t('auth.toRegister') }}
        </NuxtLink>
      </template>
    </p>
  </div>
</template>
