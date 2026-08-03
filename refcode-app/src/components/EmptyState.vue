<script setup lang="ts">
import { IonIcon } from '@ionic/vue'

defineProps<{
  icon: string
  title: string
  description?: string
  tone?: 'danger'
}>()
</script>

<template>
  <div class="empty">
    <div class="icon" :class="tone">
      <IonIcon :icon="icon" />
    </div>
    <h3>{{ title }}</h3>
    <p v-if="description">{{ description }}</p>
    <!-- 有動作才給 slot，空狀態下沒有下一步的話留白比塞按鈕好。 -->
    <slot />
  </div>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 56px 32px;
  text-align: center;
}

.icon {
  display: grid;
  place-items: center;
  width: 56px;
  height: 56px;
  margin-bottom: 6px;
  border-radius: 999px;
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 26px;
}

.icon.danger {
  background: rgba(var(--ion-color-danger-rgb), 0.12);
  color: var(--ion-color-danger);
}

h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
}

p {
  margin: 0;
  max-width: 30ch;
  font-size: 14px;
  line-height: 1.6;
  color: var(--app-muted);
}

.empty :slotted(ion-button) {
  margin-top: 14px;
}
</style>
