<script setup lang="ts">
import {
  IonBackButton,
  IonButton,
  IonButtons,
  IonContent,
  IonHeader,
  IonItem,
  IonLabel,
  IonList,
  IonPage,
  IonTitle,
  IonToolbar,
  onIonViewWillEnter,
  toastController,
} from '@ionic/vue'
import { personRemoveOutline } from 'ionicons/icons'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { api } from '../api/client'
import type { BlockedUser } from '../api/types'
import EmptyState from '../components/EmptyState.vue'
import SkeletonList from '../components/SkeletonList.vue'
import { apiErrorMessage } from '../i18n'

const { t } = useI18n()

const blocks = ref<BlockedUser[]>([])
const loading = ref(true)
const errorMessage = ref('')

// 這一頁存在的唯一理由是「解除封鎖」。封鎖之後對方的碼就從目錄消失了，
// 沒有這份清單，誤封的人再也找不到那個人 —— 那等於封鎖是不可逆的。
async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    blocks.value = (await api.listMyBlocks()).blocks
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'common.loadFailed')
  } finally {
    loading.value = false
  }
}

// 不用 onMounted：Ionic 會把離開的頁面留在記憶體裡，從服務商頁封鎖了人再走回來
// 不會重跑（同 MyCodesTab）。
onIonViewWillEnter(load)

async function unblock(b: BlockedUser) {
  try {
    await api.unblockUser(b.blocked_id)
    blocks.value = blocks.value.filter((x) => x.blocked_id !== b.blocked_id)
    const toast = await toastController.create({
      message: t('blocks.unblocked', { name: b.display_name }),
      duration: 2000,
    })
    await toast.present()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'blocks.unblockFailed')
  }
}
</script>

<template>
  <IonPage>
    <IonHeader>
      <IonToolbar>
        <IonButtons slot="start">
          <IonBackButton default-href="/tabs/account" text="" />
        </IonButtons>
        <IonTitle>{{ $t('blocks.title') }}</IonTitle>
      </IonToolbar>
    </IonHeader>

    <IonContent>
      <SkeletonList v-if="loading" :count="3" :lines="1" />

      <EmptyState
        v-else-if="blocks.length === 0"
        :icon="personRemoveOutline"
        :title="$t('blocks.emptyTitle')"
        :description="$t('blocks.emptyDesc')"
      />

      <div v-else class="page-pad">
        <p class="tiny muted lead">{{ $t('blocks.lead') }}</p>
        <IonList class="app-form" lines="full">
          <IonItem v-for="b in blocks" :key="b.blocked_id">
            <IonLabel>{{ b.display_name }}</IonLabel>
            <IonButton slot="end" size="small" fill="clear" @click="unblock(b)">
              {{ $t('blocks.unblock') }}
            </IonButton>
          </IonItem>
        </IonList>

        <p v-if="errorMessage" class="tiny error-text">{{ errorMessage }}</p>
      </div>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.lead {
  margin: 12px 2px;
}

.error-text {
  margin-top: 12px;
  color: var(--ion-color-danger-shade);
}

@media (prefers-color-scheme: dark) {
  .error-text {
    color: var(--ion-color-danger-tint);
  }
}
</style>
