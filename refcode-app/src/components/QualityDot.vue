<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ score: number }>()

// 分數的級距要跟後端一致：新碼沒有回報資料時初始值是 60，
// 所以 60 必須落在「中性」而不是「不好」，否則每個新上架的碼一出生就被標成紅的。
const tone = computed(() => {
  if (props.score >= 80) return 'success'
  if (props.score >= 60) return ''
  if (props.score >= 40) return 'warning'
  return 'danger'
})
</script>

<template>
  <span class="pill" :class="tone">
    <span class="dot" />
    {{ $t('common.quality', { score }) }}
  </span>
</template>

<style scoped>
.dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
}
</style>
