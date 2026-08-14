<script setup lang="ts">
import { IonButton, IonContent, IonIcon, IonPage } from '@ionic/vue'
import {
  searchOutline,
  shieldCheckmarkOutline,
  sparklesOutline,
  ticketOutline,
} from 'ionicons/icons'
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { useOnboardingStore } from '../stores/onboarding'

const router = useRouter()
const onboarding = useOnboardingStore()

// key 對到 i18n 的 onboarding.slides.*，順序就是滑動順序：
// 這是什麼 → 怎麼找 → 為什麼碼是活的 → 你也可以上架。
const slides = [
  { key: 'what', icon: ticketOutline },
  { key: 'find', icon: searchOutline },
  { key: 'fresh', icon: shieldCheckmarkOutline },
  { key: 'share', icon: sparklesOutline },
] as const

const index = ref(0)
const track = ref<HTMLElement | null>(null)

// 用捲動位置回推目前第幾頁 —— 手指滑動與按「下一步」都會經過這裡，
// 兩條路徑共用同一個狀態，不會出現圓點跟畫面對不上的情況。
function onScroll() {
  const el = track.value
  if (!el) return
  index.value = Math.round(el.scrollLeft / el.clientWidth)
}

function goTo(i: number) {
  track.value?.scrollTo({ left: i * (track.value?.clientWidth ?? 0), behavior: 'smooth' })
}

async function finish() {
  await onboarding.markSeen()
  router.replace('/tabs/explore')
}

function next() {
  if (index.value >= slides.length - 1) {
    finish()
    return
  }
  goTo(index.value + 1)
}
</script>

<template>
  <IonPage>
    <IonContent :scroll-y="false">
      <div class="wrap">
        <!-- 跳過永遠在：強迫看完不會讓人更想用，只會讓人更想解除安裝。 -->
        <div class="top">
          <IonButton fill="clear" size="small" color="medium" @click="finish">
            {{ $t('onboarding.skip') }}
          </IonButton>
        </div>

        <div ref="track" class="track" @scroll.passive="onScroll">
          <section v-for="s in slides" :key="s.key" class="slide">
            <div class="mark"><IonIcon :icon="s.icon" /></div>
            <h1>{{ $t(`onboarding.slides.${s.key}.title`) }}</h1>
            <p>{{ $t(`onboarding.slides.${s.key}.desc`) }}</p>
          </section>
        </div>

        <div class="bottom">
          <div class="dots">
            <button
              v-for="(s, i) in slides"
              :key="s.key"
              type="button"
              class="dot"
              :class="{ on: i === index }"
              :aria-label="`${i + 1} / ${slides.length}`"
              @click="goTo(i)"
            />
          </div>

          <IonButton expand="block" class="wide" @click="next">
            {{ index === slides.length - 1 ? $t('onboarding.start') : $t('onboarding.next') }}
          </IonButton>
        </div>
      </div>
    </IonContent>
  </IonPage>
</template>

<style scoped>
.wrap {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.top {
  display: flex;
  justify-content: flex-end;
  padding: 4px 8px 0;
}

/* 一頁一格的水平捲動。scroll-snap 讓它有「翻頁」的手感，
   不用為了這個裝一套 carousel 套件。 */
.track {
  flex: 1;
  display: flex;
  overflow-x: auto;
  overflow-y: hidden;
  scroll-snap-type: x mandatory;
  scrollbar-width: none;
}

.track::-webkit-scrollbar {
  display: none;
}

.slide {
  flex: 0 0 100%;
  scroll-snap-align: start;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding: 0 32px;
  text-align: center;
}

.mark {
  display: grid;
  place-items: center;
  width: 88px;
  height: 88px;
  margin-bottom: 22px;
  border-radius: var(--app-radius-lg);
  background: var(--app-tint);
  color: var(--app-tint-ink);
  font-size: 42px;
}

.slide h1 {
  margin: 0;
  font-size: 25px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.35;
}

.slide p {
  margin: 6px 0 0;
  max-width: 22em;
  color: var(--app-muted);
  font-size: 15px;
  line-height: 1.65;
}

.bottom {
  padding: 0 24px calc(20px + var(--ion-safe-area-bottom, 0px));
}

.dots {
  display: flex;
  justify-content: center;
  gap: 7px;
  margin-bottom: 20px;
}

.dot {
  width: 7px;
  height: 7px;
  padding: 0;
  border: none;
  border-radius: 999px;
  background: var(--app-line-strong);
  transition: width 0.2s, background-color 0.2s;
}

.dot.on {
  width: 20px;
  background: var(--ion-color-primary);
}
</style>
