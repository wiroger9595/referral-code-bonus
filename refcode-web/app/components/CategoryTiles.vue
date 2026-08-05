<script setup lang="ts">
import type { Category } from '~/types/api'

defineProps<{ categories: Category[]; activeId?: string }>()

const localePath = useLocalePath()

// 分類在後端只有 id 與 name，沒有可以對應圖示的穩定欄位，所以只能拿名稱去比。
// 比不到就給通用圖示，不會壞掉，但新增分類時記得回來補一條 ——
// 正解是後端在分類上加一個 icon 欄位。
const ICONS: { match: RegExp; icon: string }[] = [
  { match: /銀行|信用卡|カード|bank|card/i, icon: 'card' },
  { match: /券商|投資|証券|invest|broker|stock/i, icon: 'invest' },
  { match: /外送|外食|デリバリー|delivery|food/i, icon: 'food' },
  { match: /影音|串流|動画|音楽|stream|video|music/i, icon: 'play' },
  { match: /電信|通訊|通信|携帯|telecom|mobile/i, icon: 'mobile' },
  { match: /旅遊|訂房|旅行|ホテル|travel|hotel|flight/i, icon: 'travel' },
  { match: /購物|電商|通販|shop|retail|commerce/i, icon: 'bag' },
]

function iconFor(name: string) {
  return ICONS.find((c) => c.match.test(name))?.icon ?? 'grid'
}

// 四組淡底照排序輪流套，避免一整片磁磚變成同一坨橘。
const TONES = [
  'bg-a1 text-a1-ink',
  'bg-a2 text-a2-ink',
  'bg-a3 text-a3-ink',
  'bg-a4 text-a4-ink',
]
</script>

<template>
  <!-- 分類做成磁磚而不是一排文字連結：分類數是個位數，一眼看得完，
       磁磚在手機上也比細長的文字連結好按。 -->
  <div class="grid grid-cols-4 gap-x-2 gap-y-4 sm:grid-cols-6 lg:grid-cols-8">
    <NuxtLink
      v-for="(c, i) in categories"
      :key="c.id"
      :to="localePath(`/category/${c.id}`)"
      class="group flex flex-col items-center gap-2"
    >
      <span
        class="grid size-14 place-items-center rounded-tile text-2xl transition group-hover:scale-105"
        :class="[
          TONES[i % TONES.length],
          activeId === c.id ? 'ring-2 ring-brand ring-offset-2 ring-offset-page' : '',
        ]"
      >
        <AppIcon :name="iconFor(c.name)" />
      </span>
      <span
        class="w-full truncate text-center text-xs font-semibold"
        :class="activeId === c.id ? 'text-brand' : ''"
      >
        {{ c.name }}
      </span>
    </NuxtLink>
  </div>
</template>
