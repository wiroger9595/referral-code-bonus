<script setup lang="ts">
// 提報「希望上架的平台」。目錄只由 admin 維護，所以搜不到某一家的人原本
// 在站上是完全沒有出口的 —— 他關掉分頁，我們也不知道少了哪幾家。
import type { MerchantSuggestion } from '~/types/api'

const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const { isLoggedIn, authedFetch } = useAuth()
const apiErrorMessage = useApiError()

// 這一頁沒有值得被收錄的內容（一份表單），而且會跟服務商頁搶「推薦碼」這類字。
useSeoMeta({
  title: () => t('suggest.title'),
  robots: 'noindex, follow',
})

// 從搜尋結果頁過來時把剛剛沒搜到的字帶進來當預設名稱 —— 那就是他要提報的那一家，
// 再叫他打一次很沒必要。
const name = ref(String(route.query.name ?? '').trim())
const signupUrl = ref('')
const note = ref('')

const pending = ref(false)
const errorMessage = ref('')
const done = ref(false)

const loginTo = computed(() => ({
  path: localePath('/login'),
  query: { redirect: route.fullPath },
}))

async function submit() {
  if (pending.value) return

  errorMessage.value = ''
  if (!name.value.trim()) {
    errorMessage.value = t('suggest.nameRequired')
    return
  }
  if (!signupUrl.value.trim()) {
    errorMessage.value = t('suggest.urlRequired')
    return
  }

  pending.value = true
  try {
    await authedFetch<MerchantSuggestion>('/v1/merchant-suggestions', {
      method: 'POST',
      body: {
        name: name.value.trim(),
        signup_url: signupUrl.value.trim(),
        note: note.value.trim(),
      },
    })
    done.value = true
  } catch (e) {
    errorMessage.value = apiErrorMessage(e)
  } finally {
    pending.value = false
  }
}

// 送出後讓他能接著提下一家，不必回上一頁再點一次。
function again() {
  name.value = ''
  signupUrl.value = ''
  note.value = ''
  done.value = false
}
</script>

<template>
  <article class="mx-auto max-w-sm">
    <h1 class="text-2xl font-bold tracking-tight">{{ $t('suggest.title') }}</h1>
    <p class="mt-2 text-sm/relaxed text-muted">{{ $t('suggest.lead') }}</p>

    <!-- 建議單會進人工審核佇列，匿名放行等於開一個沒有成本的洗版管道，
         所以這一頁要登入。 -->
    <section v-if="!isLoggedIn" class="mt-6 space-y-3">
      <p class="text-muted">{{ $t('suggest.signInFirst') }}</p>
      <NuxtLink :to="loginTo" class="btn btn-primary">{{ $t('nav.login') }}</NuxtLink>
    </section>

    <section v-else-if="done" class="app-card mt-6 space-y-3 p-5">
      <h2 class="font-semibold">{{ $t('suggest.doneTitle') }}</h2>
      <p class="text-sm/relaxed text-muted">{{ $t('suggest.doneDesc') }}</p>
      <button type="button" class="btn btn-outline" @click="again">
        {{ $t('suggest.again') }}
      </button>
    </section>

    <template v-else>
      <p
        v-if="errorMessage"
        role="alert"
        class="mt-6 rounded-card bg-alert-soft p-3 text-sm text-alert-ink"
      >
        {{ errorMessage }}
      </p>

      <form class="app-card mt-6 space-y-4 p-5" @submit.prevent="submit">
        <div>
          <label for="suggest-name" class="block text-sm font-medium">
            {{ $t('suggest.name') }}
          </label>
          <input
            id="suggest-name"
            v-model="name"
            type="text"
            required
            :placeholder="$t('suggest.namePlaceholder')"
            class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none placeholder:text-muted focus:border-brand"
          />
        </div>

        <div>
          <label for="suggest-url" class="block text-sm font-medium">
            {{ $t('suggest.url') }}
          </label>
          <input
            id="suggest-url"
            v-model="signupUrl"
            type="url"
            inputmode="url"
            required
            placeholder="https://"
            class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none placeholder:text-muted focus:border-brand"
          />
          <p class="mt-1 text-xs text-muted">{{ $t('suggest.urlHint') }}</p>
        </div>

        <div>
          <label for="suggest-note" class="block text-sm font-medium">
            {{ $t('suggest.note') }}
          </label>
          <textarea
            id="suggest-note"
            v-model="note"
            rows="3"
            :placeholder="$t('suggest.notePlaceholder')"
            class="mt-1 w-full rounded-card border border-line-strong bg-surface px-3 py-2 text-sm outline-none placeholder:text-muted focus:border-brand"
          />
        </div>

        <button type="submit" :disabled="pending" class="btn btn-primary w-full disabled:opacity-50">
          {{ pending ? $t('suggest.pending') : $t('suggest.submit') }}
        </button>

        <p class="text-xs/relaxed text-muted">{{ $t('suggest.reviewNotice') }}</p>
      </form>
    </template>
  </article>
</template>
