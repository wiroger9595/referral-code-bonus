<script setup lang="ts">
// 官網版的「我的推薦碼」只做得到看，上架與下架還是在 app 裡 ——
// 那兩件事都要先跑過審核流程與到期日的表單，搬過來等於兩邊各維護一份。
import type { CodeStatus, MyCode } from '~/types/api'

const { t, locale } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const { isLoggedIn, authedFetch } = useAuth()
const apiErrorMessage = useApiError()
const { isUrgent } = useExpiry()

// 網址底下是使用者自己的資料，跟 /account 一樣不給索引。
useSeoMeta({
  title: () => t('myCodes.title'),
  robots: 'noindex, nofollow',
})

// 'all' 之後照使用者實際會找的順序排：還在架上的擺前面，
// 已經沒救的（拒絕、到期）擺最後。跟 app 的 MyCodesTab 同一套順序。
const FILTERS = ['all', 'active', 'pending', 'disabled', 'expired', 'rejected'] as const
type Filter = (typeof FILTERS)[number]

const filter = ref<Filter>('all')
const errorMessage = ref('')

// 個人頁不給爬蟲看，就沒有 SSR 的理由（同 search.vue）。
const { data, status } = await useAsyncData(
  'my-codes',
  async () => {
    if (!isLoggedIn.value) return null
    errorMessage.value = ''
    try {
      return await authedFetch<{ codes: MyCode[] }>('/v1/me/codes')
    } catch (e) {
      errorMessage.value = apiErrorMessage(e)
      return null
    }
  },
  { watch: [isLoggedIn], server: false },
)

const codes = computed(() => data.value?.codes ?? [])
const loading = computed(() => status.value === 'pending')

// 清單一頁就抓完（免費 3 個、Pro 也不會多到哪去），篩選純在前端做 ——
// 切籤不用重打 API，換來的體感差很多。
const visible = computed(() =>
  filter.value === 'all' ? codes.value : codes.value.filter((c) => c.status === filter.value),
)

// 籤上直接標數量，才不用逐一點進去才知道哪個籤是空的。
const counts = computed(() => {
  const map = { all: codes.value.length } as Record<Filter, number>
  for (const f of FILTERS) {
    if (f !== 'all') map[f] = codes.value.filter((c) => c.status === f).length
  }
  return map
})

// .pill 只有 ok / alert 兩個變體，審核中借用品牌色（橘），
// 到期就用 .pill 自己的灰 —— 那不是壞事，只是過去了。
const statusPill: Record<CodeStatus, string> = {
  pending: 'bg-brand-soft text-brand-ink',
  active: 'pill-ok',
  rejected: 'pill-alert',
  expired: '',
  disabled: 'pill-alert',
}

function statusLabel(s: CodeStatus) {
  return t(`myCodes.status.${s}`)
}

function filterLabel(f: Filter) {
  return f === 'all' ? t('myCodes.filterAll') : statusLabel(f)
}

// 日期格式跟著介面語言走，不然日文介面會看到中文的月／日排法。
// null 是沒有到期日的碼，這一格改顯示「無期限」而不是日期。
function formatDate(iso: string | null) {
  if (iso === null) return t('common.noExpiry')
  return new Date(iso).toLocaleDateString(locale.value, { year: 'numeric', month: 'numeric', day: 'numeric' })
}

// 只有還在架上的碼才需要提醒快到期了；已下架、已到期的標紅只是雜訊。
function urgent(c: MyCode) {
  return c.status === 'active' && isUrgent(c.expires_at)
}

const loginTo = computed(() => ({
  path: localePath('/login'),
  query: { redirect: route.fullPath },
}))
</script>

<template>
  <article class="space-y-6">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h1 class="text-2xl font-semibold">{{ $t('myCodes.title') }}</h1>
      <p v-if="isLoggedIn && codes.length" class="text-xs text-muted">
        {{ $t('myCodes.total', { count: codes.length }, codes.length) }}
      </p>
    </div>

    <section v-if="!isLoggedIn" class="space-y-3">
      <p class="text-muted">{{ $t('myCodes.signInFirst') }}</p>
      <NuxtLink :to="loginTo" class="btn btn-primary">{{ $t('nav.login') }}</NuxtLink>
    </section>

    <template v-else>
      <p v-if="loading" class="py-12 text-center text-muted">{{ $t('myCodes.loading') }}</p>

      <p v-else-if="errorMessage" class="py-12 text-center text-alert-ink">{{ errorMessage }}</p>

      <div v-else-if="codes.length === 0" class="py-12 text-center">
        <h2 class="font-semibold">{{ $t('myCodes.emptyTitle') }}</h2>
        <p class="mt-2 text-sm text-muted">{{ $t('myCodes.emptyDesc') }}</p>
      </div>

      <template v-else>
        <!-- 六個籤在窄螢幕放不下，橫向捲比擠成兩行好認。 -->
        <div class="-mx-4 overflow-x-auto px-4">
          <div class="flex w-max gap-2">
            <button
              v-for="f in FILTERS"
              :key="f"
              type="button"
              class="pill"
              :class="filter === f ? 'bg-brand text-on-brand' : 'hover:text-ink'"
              @click="filter = f"
            >
              {{ filterLabel(f) }} {{ counts[f] }}
            </button>
          </div>
        </div>

        <!-- 有碼但這個籤是空的：不要跟「一組碼都沒有」共用同一句話，
             那會讓人以為資料不見了。 -->
        <div v-if="visible.length === 0" class="py-12 text-center">
          <h2 class="font-semibold">
            {{ $t('myCodes.filterEmptyTitle', { status: filterLabel(filter) }) }}
          </h2>
          <button
            type="button"
            class="mt-3 text-sm font-semibold text-muted underline hover:text-ink"
            @click="filter = 'all'"
          >
            {{ $t('myCodes.filterShowAll') }}
          </button>
        </div>

        <ul v-else class="grid gap-3 sm:grid-cols-2">
          <li v-for="c in visible" :key="c.id" class="rounded-2xl border border-line bg-surface p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="flex min-w-0 items-center gap-3">
                <span
                  class="grid size-10 shrink-0 place-items-center overflow-hidden rounded-xl bg-brand-soft font-bold text-brand-ink"
                >
                  <img
                    v-if="c.merchant_logo_url"
                    :src="c.merchant_logo_url"
                    :alt="c.merchant_name"
                    class="size-full object-cover"
                  />
                  <template v-else>{{ c.merchant_name.trim().charAt(0) }}</template>
                </span>
                <div class="min-w-0">
                  <!-- 連到公開頁，讓上架者看得到自己的碼在別人眼裡長什麼樣。 -->
                  <NuxtLink
                    :to="localePath(`/referral/${c.merchant_slug}`)"
                    class="block truncate font-semibold hover:underline"
                  >
                    {{ c.merchant_name }}
                  </NuxtLink>
                  <p class="truncate font-mono text-xs tracking-wider text-muted">{{ c.code }}</p>
                </div>
              </div>
              <span class="pill shrink-0" :class="statusPill[c.status]">
                {{ statusLabel(c.status) }}
              </span>
            </div>

            <!-- 上架者唯一在意的是「有沒有人看到」，所以數字要比碼本身大。 -->
            <dl class="mt-4 grid grid-cols-3 gap-2 border-t border-line pt-3">
              <div>
                <dd class="text-lg font-bold">{{ c.impressions }}</dd>
                <dt class="text-xs text-muted">{{ $t('myCodes.impressions') }}</dt>
              </div>
              <div>
                <dd class="text-lg font-bold">{{ c.quality_score }}</dd>
                <dt class="text-xs text-muted">{{ $t('myCodes.qualityScore') }}</dt>
              </div>
              <div>
                <dd class="text-lg font-bold" :class="urgent(c) && 'text-alert-ink'">
                  {{ formatDate(c.expires_at) }}
                </dd>
                <dt class="text-xs text-muted">{{ $t('myCodes.expiresAt') }}</dt>
              </div>
            </dl>
          </li>
        </ul>

        <p class="text-sm text-muted">{{ $t('myCodes.manageInApp') }}</p>
      </template>
    </template>
  </article>
</template>
