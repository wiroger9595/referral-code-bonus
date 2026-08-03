<script setup lang="ts">
import {
  IonIcon,
  IonLabel,
  IonPage,
  IonRouterOutlet,
  IonTabBar,
  IonTabButton,
  IonTabs,
} from '@ionic/vue'
import { compassOutline, personOutline, pricetagsOutline } from 'ionicons/icons'
</script>

<template>
  <IonPage>
    <IonTabs>
      <IonRouterOutlet />

      <IonTabBar slot="bottom">
        <IonTabButton tab="explore" href="/tabs/explore">
          <IonIcon :icon="compassOutline" />
          <IonLabel>{{ $t('tabs.explore') }}</IonLabel>
        </IonTabButton>

        <IonTabButton tab="my-codes" href="/tabs/my-codes">
          <IonIcon :icon="pricetagsOutline" />
          <IonLabel>{{ $t('tabs.myCodes') }}</IonLabel>
        </IonTabButton>

        <IonTabButton tab="account" href="/tabs/account">
          <IonIcon :icon="personOutline" />
          <IonLabel>{{ $t('tabs.account') }}</IonLabel>
        </IonTabButton>
      </IonTabBar>
    </IonTabs>
  </IonPage>
</template>
