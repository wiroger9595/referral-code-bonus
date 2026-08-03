<script setup lang="ts">
// 切語言要換的是網址（/ja/... ↔ /en/...），不是只改當下顯示的字，
// 否則使用者分享出去的連結會跟他看到的語言不一致。
const { locale, locales } = useI18n()
const switchLocalePath = useSwitchLocalePath()
const router = useRouter()

const options = computed(() =>
  (locales.value as { code: string; name?: string }[]).map((l) => ({
    code: l.code,
    name: l.name ?? l.code,
  })),
)

function change(event: Event) {
  const code = (event.target as HTMLSelectElement).value
  const path = switchLocalePath(code)
  if (path) router.push(path)
}
</script>

<template>
  <label class="relative">
    <span class="sr-only">{{ $t('lang.label') }}</span>
    <select
      :value="locale"
      class="cursor-pointer rounded-md border border-neutral-300 bg-transparent py-1.5 pr-7 pl-2 text-sm outline-none dark:border-neutral-700"
      @change="change"
    >
      <option v-for="o in options" :key="o.code" :value="o.code">{{ o.name }}</option>
    </select>
  </label>
</template>
