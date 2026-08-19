<script setup lang="ts">
import {
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonIcon,
  IonPage,
  IonRefresher,
  IonRefresherContent,
  IonTitle,
  IonToolbar,
} from '@ionic/vue'
import type { RefresherCustomEvent } from '@ionic/vue'
import { addOutline, alertCircleOutline, appsOutline, chevronForward } from 'ionicons/icons'
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { api } from '../api/client'
import type { Category } from '../api/types'
import { categoryIcon } from '../categories'
import EmptyState from '../components/EmptyState.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { apiErrorMessage } from '../i18n'

const categories = ref<Category[]>([])
const loading = ref(true)
const errorMessage = ref('')
const { locale } = useI18n()

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    categories.value = (await api.listCategories()).categories
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'common.connectionFailed')
  } finally {
    loading.value = false
  }
}

onMounted(load)

// 分類名是後端依 ?lang= 回的。Ionic 把分頁留在記憶體裡，從「帳號」切語言
// 再切回來不會重跑 onMounted，所以要自己盯著 locale（跟探索頁同一個道理）。
watch(locale, load)

async function refresh(event: RefresherCustomEvent) {
  await load()
  event.target.complete()
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonTitle>{{ $t('tabs.category') }}</IonTitle>
        <IonButtons slot="end">
          <!-- 上架入口在每個瀏覽頁都有：想上架的念頭多半是在看別人的碼時冒出來的，
               不該逼使用者先切回「我的碼」分頁才找得到。 -->
          <IonButton router-link="/add-code">
            <IonIcon slot="icon-only" :icon="addOutline" />
          </IonButton>
        </IonButtons>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <IonRefresher slot="fixed" @ion-refresh="refresh">
        <IonRefresherContent />
      </IonRefresher>

      <div class="page-heading">
        <h1>{{ $t('categoryTab.heading') }}</h1>
        <p>{{ $t('categoryTab.lead') }}</p>
      </div>

      <SkeletonList v-if="loading" :count="6" :lines="1" />

      <EmptyState
        v-else-if="errorMessage"
        :icon="alertCircleOutline"
        tone="danger"
        :title="$t('common.loadFailed')"
        :description="errorMessage"
      />

      <EmptyState
        v-else-if="categories.length === 0"
        :icon="appsOutline"
        :title="$t('categoryTab.emptyTitle')"
        :description="$t('categoryTab.emptyDesc')"
      />

      <!-- 一格一列而不是磁磚格：這裡是專門用來挑分類的頁面，橫向的整列
           比探索頁那種四欄小磁磚好按，分類名也不必截字。 -->
      <div v-else class="stack page-pad list">
        <div
          v-for="(c, i) in categories"
          :key="c.id"
          class="app-card tappable row"
          @click="$router.push(`/category/${c.id}`)"
        >
          <span class="ico" :class="`a${(i % 4) + 1}`">
            <IonIcon :icon="categoryIcon(c.name)" />
          </span>
          <span class="name">{{ c.name }}</span>
          <IonIcon :icon="chevronForward" class="chevron" />
        </div>
      </div>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.list {
  padding-bottom: 24px;
}

.row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px;
}

.ico {
  display: grid;
  place-items: center;
  flex: none;
  width: 44px;
  height: 44px;
  border-radius: var(--app-radius);
  font-size: 21px;
}

.ico.a1 {
  background: var(--app-accent-1);
  color: var(--app-accent-1-ink);
}

.ico.a2 {
  background: var(--app-accent-2);
  color: var(--app-accent-2-ink);
}

.ico.a3 {
  background: var(--app-accent-3);
  color: var(--app-accent-3-ink);
}

.ico.a4 {
  background: var(--app-accent-4);
  color: var(--app-accent-4-ink);
}

.name {
  flex: 1;
  min-width: 0;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.chevron {
  flex: none;
  color: var(--app-muted);
  font-size: 16px;
  opacity: 0.6;
}

/* ── 平板 ───────────────────────────────────────────── */

@media (min-width: 768px) {
  .list {
    display: grid;
    grid-template-columns: 1fr 1fr;
    align-items: start;
  }
}
</style>
