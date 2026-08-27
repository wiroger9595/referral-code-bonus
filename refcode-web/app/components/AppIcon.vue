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
  game: [
    'M8 9h8a4.5 4.5 0 0 1 0 9H8a4.5 4.5 0 0 1 0-9z',
    'M10.3 12v3',
    'M8.8 13.5h3',
    'M14.6 12.6h.01',
    'M16.1 14.4h.01',
  ],
  cloud: ['M17.5 18.5H9a6.5 6.5 0 1 1 6.3-8.2h2.2a4.1 4.1 0 0 1 0 8.2z'],
  car: [
    'M4.5 15.5v-3.4l2.1-4.1h10.8l2.1 4.1v3.4',
    'M4.5 12.1h15',
    'M4.5 15.5h1.6',
    'M9.6 15.5h4.8',
    'M17.9 15.5h1.6',
    'M9.5 15.5a1.7 1.7 0 1 1-3.4 0 1.7 1.7 0 0 1 3.4 0',
    'M17.9 15.5a1.7 1.7 0 1 1-3.4 0 1.7 1.7 0 0 1 3.4 0',
  ],
  fitness: ['M6.5 9.5v5', 'M4 10.8v2.4', 'M17.5 9.5v5', 'M20 10.8v2.4', 'M6.5 12h11'],
  school: [
    'M3 9.8l9-3.8 9 3.8-9 3.8z',
    'M7.5 11.7v3.9c0 1.2 2 2.2 4.5 2.2s4.5-1 4.5-2.2v-3.9',
  ],
  tool: [
    'M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.8-3.8a6 6 0 0 1-7.9 7.9l-6.9 6.9a2.1 2.1 0 0 1-3-3l6.9-6.9a6 6 0 0 1 7.9-7.9l-3.8 3.8z',
  ],
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
