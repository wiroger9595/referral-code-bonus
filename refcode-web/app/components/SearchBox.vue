<script setup lang="ts">
// 送出一律導到 /search?q=，不在原地篩：搜尋結果要能分享、能用瀏覽器上一頁退回，
// 兩件事都需要一個自己的網址。
const props = withDefaults(
  defineProps<{
    // 目前的查詢字串。搜尋頁把網址上的 q 傳進來，讓輸入框跟網址對得起來。
    initial?: string
    // 搜尋頁的主要輸入框用大的，站頭那個用小的。
    size?: 'sm' | 'lg'
    autofocus?: boolean
  }>(),
  { initial: '', size: 'sm', autofocus: false },
)

const localePath = useLocalePath()
const text = ref(props.initial)

// 從 /search?q=a 點到 /search?q=b（例如點了熱門關鍵字）時網址變了但元件沒重建，
// 不同步的話輸入框會一直停在舊的字。
watch(
  () => props.initial,
  (v) => {
    text.value = v
  },
)

function submit() {
  const q = text.value.trim()
  if (!q) return
  navigateTo({ path: localePath('/search'), query: { q } })
}
</script>

<template>
  <form role="search" class="relative" @submit.prevent="submit">
    <AppIcon
      name="search"
      class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted"
    />
    <input
      v-model="text"
      type="search"
      :autofocus="autofocus"
      :placeholder="$t('search.placeholder')"
      :aria-label="$t('search.placeholder')"
      class="w-full rounded-full border border-line-strong bg-surface pr-3 pl-9 outline-none focus:border-brand"
      :class="size === 'lg' ? 'py-3 text-base' : 'py-2 text-sm'"
    />
  </form>
</template>
