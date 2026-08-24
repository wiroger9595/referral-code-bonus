<script setup lang="ts">
import type { MerchantListResponse, PopularTerm } from '~/types/api'

const route = useRoute()
const { public: cfg } = useRuntimeConfig()
const { t } = useI18n()
const localePath = useLocalePath()
const { authHeaders } = useAuth()
const lang = useApiLang()
const { regionQuery } = useRegionFilter()
// 攤平成頂層的 ref，template 才會自動解包（放在物件裡就得寫 .value）。
const {
  items: recentTerms,
  load: loadHistory,
  add: addHistory,
  remove: removeTerm,
  clear: clearHistory,
} = useSearchHistory()

const q = computed(() => String(route.query.q ?? '').trim())

// 搜尋結果頁一律 noindex：這種頁面內容單薄、彼此高度重複，收錄進去只會稀釋
// 服務商頁的排名，還會跟首頁搶「推薦碼」這類字。
// follow 留著 —— 結果裡的服務商連結本身值得被爬。
useSeoMeta({
  robots: 'noindex, follow',
  title: () => (q.value ? t('search.seoTitleQuery', { q: q.value }) : t('search.seoTitle')),
})

// 既然不給搜尋引擎看，就沒有 SSR 的理由。server: false 讓查詢只在瀏覽器發生，
// 每一次搜尋都不用佔一條 Nuxt server 的往返。
const { data, status } = await useAsyncData(
  'search-results',
  () => {
    // 空查詢等於整份目錄，那是首頁的事，不要在這裡多打一次。
    if (!q.value) return Promise.resolve(null)
    return $fetch<MerchantListResponse>('/v1/merchants', {
      baseURL: cfg.apiBase,
      // commit 讓後端把這個詞計進熱門榜。這一頁只會從真正的搜尋動作進來
      // （按 Enter、點建議、點歷史、開別人分享的連結），所以永遠帶。
      query: { q: q.value, limit: 50, lang: lang.value, commit: 1, region: regionQuery.value },
      headers: authHeaders(),
    })
  },
  { watch: [q, lang, regionQuery], server: false },
)

const { data: popular } = await useAsyncData(
  'search-popular',
  () =>
    $fetch<{ terms: PopularTerm[] }>('/v1/search/popular', {
      baseURL: cfg.apiBase,
      query: { lang: lang.value },
    }),
  { watch: [lang], server: false },
)

const merchants = computed(() => data.value?.merchants ?? [])
const total = computed(() => data.value?.total ?? 0)
const suggestions = computed(() => data.value?.suggestions ?? [])
const popularTerms = computed(() => popular.value?.terms ?? [])
const loading = computed(() => status.value === 'pending')

function termLink(term: string) {
  return { path: localePath('/search'), query: { q: term } }
}

// localStorage 讀不到 SSR，所以歷史一律等掛載後才碰，否則首次 render
// 會跟伺服器給的 HTML 對不起來。
onMounted(() => {
  loadHistory()
  if (q.value) addHistory(q.value)
})

watch(q, (v) => {
  if (v) addHistory(v)
})
</script>

<template>
  <div>
    <SearchBox :initial="q" size="lg" :autofocus="!q" class="mb-8" />

    <!-- 有查詢字串：結果、或搜不到 -->
    <template v-if="q">
      <p v-if="loading" class="py-12 text-center text-muted">{{ $t('search.searching') }}</p>

      <template v-else-if="total > 0">
        <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2">
          <h1 class="text-xl font-bold tracking-tight">{{ $t('search.heading', { q }) }}</h1>
          <div class="flex items-baseline gap-3">
            <RegionToggle />
            <p class="text-xs text-muted">
              {{ $t('search.resultCount', { count: total }, total) }}
            </p>
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <MerchantCard v-for="m in merchants" :key="m.id" :merchant="m" />
        </div>
      </template>

      <div v-else class="py-8 text-center">
        <h1 class="text-xl font-bold tracking-tight">{{ $t('search.emptyTitle', { q }) }}</h1>
        <p class="mt-2 text-sm text-muted">{{ $t('search.emptyHint') }}</p>

        <!-- 打錯字時的救援。後端拿名稱相似度找的，找不到就整段不出現。 -->
        <section v-if="suggestions.length" class="mt-8">
          <h2 class="mb-3 text-sm font-bold">{{ $t('search.suggestionsTitle') }}</h2>
          <div class="flex flex-wrap justify-center gap-2">
            <NuxtLink
              v-for="s in suggestions"
              :key="s.slug"
              :to="localePath(`/referral/${s.slug}`)"
              class="btn btn-outline"
            >
              {{ s.name }}
            </NuxtLink>
          </div>
        </section>

        <!-- 搜不到最常見的原因是目錄真的沒有這一家。提報的入口要放在這裡，
             因為使用者就是在這一刻發現的；名字直接帶過去，不用再打一次。 -->
        <section class="mt-8">
          <NuxtLink :to="{ path: localePath('/suggest'), query: { name: q } }" class="btn btn-outline">
            {{ $t('search.suggestCta') }}
          </NuxtLink>
        </section>

        <!-- 空結果頁一定要給一條出路，不然使用者只能按上一頁。 -->
        <section v-if="popularTerms.length" class="mt-8">
          <h2 class="mb-3 text-sm font-bold">{{ $t('search.popularTitle') }}</h2>
          <div class="flex flex-wrap justify-center gap-2">
            <NuxtLink
              v-for="p in popularTerms"
              :key="p.term"
              :to="termLink(p.term)"
              class="pill hover:text-ink"
            >
              {{ p.term }}
            </NuxtLink>
          </div>
        </section>
      </div>
    </template>

    <!-- 還沒搜任何東西：把上次搜過的與大家在搜的擺出來，比一個空白畫面有用 -->
    <template v-else>
      <section v-if="recentTerms.length" class="mb-10">
        <div class="mb-3 flex items-baseline justify-between gap-2">
          <h2 class="text-sm font-bold">{{ $t('search.historyTitle') }}</h2>
          <button class="text-xs font-semibold text-muted hover:text-ink" @click="clearHistory()">
            {{ $t('search.historyClear') }}
          </button>
        </div>

        <div class="flex flex-wrap gap-2">
          <span v-for="term in recentTerms" :key="term" class="pill gap-0 pr-1">
            <NuxtLink :to="termLink(term)" class="hover:text-ink">{{ term }}</NuxtLink>
            <button
              class="ml-1 grid size-4 place-items-center rounded-full hover:text-ink"
              :aria-label="$t('search.historyRemove', { term })"
              @click="removeTerm(term)"
            >
              ×
            </button>
          </span>
        </div>
      </section>

      <section v-if="popularTerms.length">
        <h2 class="mb-3 text-sm font-bold">{{ $t('search.popularTitle') }}</h2>
        <div class="flex flex-wrap gap-2">
          <NuxtLink
            v-for="p in popularTerms"
            :key="p.term"
            :to="termLink(p.term)"
            class="pill hover:text-ink"
          >
            {{ p.term }}
          </NuxtLink>
        </div>
      </section>

      <p v-if="!recentTerms.length && !popularTerms.length" class="py-12 text-center text-muted">
        {{ $t('search.startHint') }}
      </p>
    </template>
  </div>
</template>
