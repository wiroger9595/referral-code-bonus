<script setup lang="ts">
// 站上用到的圖示就這幾個，為了它們裝一整包 icon library 不划算 ——
// 全部是同一組 24 格線、同一個線寬畫出來的路徑。
const PATHS: Record<string, string[]> = {
  card: ['M3 6.5h18v11H3z', 'M3 10.5h18'],
  invest: ['M3 16.5l6-6 4 4 8-8', 'M15 6.5h6v6'],
  food: ['M4 11.5h16a8 8 0 0 1-16 0z', 'M12 4v3.5', 'M3 20h18'],
  play: ['M12 3.5a8.5 8.5 0 1 0 0 17 8.5 8.5 0 0 0 0-17z', 'M10.5 8.5l5 3.5-5 3.5z'],
  mobile: ['M7.5 3.5h9v17h-9z', 'M11 17.5h2'],
  travel: ['M21 3.5L3 10.5l7 3 3 7z'],
  bag: ['M5.5 8h13l-1 12.5h-11z', 'M9 8a3 3 0 0 1 6 0'],
  grid: ['M4 4.5h6v6H4z', 'M14 4.5h6v6h-6z', 'M4 14.5h6v6H4z', 'M14 14.5h6v6h-6z'],
  shield: ['M12 3.5l7 2.5v5c0 4-3 7.5-7 9-4-1.5-7-5-7-9v-5z', 'M9 12l2 2 4-4'],
  clock: ['M12 3.5a8.5 8.5 0 1 0 0 17 8.5 8.5 0 0 0 0-17z', 'M12 7.5v5l3.5 2'],
  flame: ['M12 3s5 4 5 8.5a5 5 0 0 1-10 0C7 9 9 7 12 3z', 'M12 20a2.5 2.5 0 0 0 2.5-2.5c0-1.5-2.5-3.5-2.5-3.5s-2.5 2-2.5 3.5A2.5 2.5 0 0 0 12 20z'],
  arrow: ['M5 12h14', 'M13 6l6 6-6 6'],
  lock: ['M6.5 10.5h11v10h-11z', 'M9 10.5V8a3 3 0 0 1 6 0v2.5'],
  check: ['M5 12.5l4.5 4.5L19 7.5'],
  copy: ['M9 9h11v11H9z', 'M15 9V4H4v11h5'],
  external: ['M14 4h6v6', 'M20 4l-8 8', 'M18 14v6H4V6h6'],
  search: ['M17.5 11a6.5 6.5 0 1 1-13 0 6.5 6.5 0 0 1 13 0z', 'M16 16l4.5 4.5'],
}

defineProps<{ name: keyof typeof PATHS | string }>()
</script>

<template>
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    class="size-[1em] shrink-0"
  >
    <path v-for="(d, i) in PATHS[name] ?? PATHS.grid" :key="i" :d="d" />
  </svg>
</template>
