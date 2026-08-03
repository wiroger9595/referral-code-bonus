<script setup lang="ts">
import { IonSkeletonText } from '@ionic/vue'

// 載入中用骨架而不是轉圈：這幾個頁面的內容都是等高的卡片列表，
// 骨架撐住版面，資料進來時畫面不會整個跳一次。
withDefaults(defineProps<{ count?: number; lines?: number }>(), { count: 4, lines: 2 })
</script>

<template>
  <div class="stack page-pad">
    <div v-for="i in count" :key="i" class="app-card skeleton">
      <IonSkeletonText :animated="true" class="line title" />
      <IonSkeletonText v-for="l in lines" :key="l" :animated="true" class="line" />
    </div>
  </div>
</template>

<style scoped>
.skeleton {
  padding: 16px;
}

.line {
  height: 13px;
  margin: 0 0 10px;
  border-radius: 6px;
}

.line:last-child {
  width: 55%;
  margin-bottom: 0;
}

.title {
  width: 42%;
  height: 17px;
  margin-bottom: 14px;
}
</style>
