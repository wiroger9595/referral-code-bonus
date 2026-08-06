<script setup lang="ts">
import { marked } from 'marked'

import privacyEn from '~/content/legal/privacy-policy.en.md?raw'
import privacyZhTW from '~/content/legal/privacy-policy.zh-TW.md?raw'

const { t, locale } = useI18n()
const config = useRuntimeConfig()

useSeoMeta({
  title: () => t('legal.privacySeoTitle'),
  description: () => t('legal.privacySeoDescription'),
})

// 官網網域還沒定案時 siteUrl 是 localhost，這裡先把文件裡的佔位符換成
// 現在設定的網域，之後 NUXT_PUBLIC_SITE_URL 改成正式網域會自動跟著換。
const domain = config.public.siteUrl.replace(/^https?:\/\//, '')

// 日文版還沒有翻譯，先退回中文版並提示——不要假裝有日文版。
const source = computed(() => (locale.value === 'en' ? privacyEn : privacyZhTW))
const html = computed(() =>
  marked.parse(source.value.replaceAll('{{官網網域}}', domain), { async: false }) as string,
)
</script>

<template>
  <article class="legal">
    <p v-if="locale === 'ja'" class="notice">{{ $t('legal.jaFallbackNotice') }}</p>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="prose" v-html="html" />
  </article>
</template>

<style scoped>
.notice {
  margin-bottom: 20px;
  padding: 10px 14px;
  border-radius: 8px;
  background: var(--color-brand-soft);
  color: var(--color-brand-ink);
  font-size: 13px;
}

.prose :deep(h1) {
  font-size: 1.6rem;
  font-weight: 600;
  margin-bottom: 4px;
}

.prose :deep(h2) {
  font-size: 1.2rem;
  font-weight: 600;
  margin-top: 2rem;
  margin-bottom: 0.75rem;
}

.prose :deep(h3) {
  font-size: 1.05rem;
  font-weight: 600;
  margin-top: 1.25rem;
  margin-bottom: 0.5rem;
}

.prose :deep(p) {
  margin-bottom: 0.75rem;
  line-height: 1.7;
  color: var(--color-ink);
}

.prose :deep(ul),
.prose :deep(ol) {
  margin: 0.5rem 0 1rem 1.25rem;
  line-height: 1.7;
}

.prose :deep(li) {
  margin-bottom: 0.25rem;
}

.prose :deep(strong) {
  font-weight: 600;
}

.prose :deep(hr) {
  margin: 2rem 0;
  border: none;
  border-top: 1px solid var(--color-page);
}

.prose :deep(blockquote) {
  margin: 0.75rem 0;
  padding: 8px 14px;
  border-left: 3px solid var(--color-brand);
  background: var(--color-page);
  font-size: 13px;
  color: var(--color-muted, #6b7280);
}

.prose :deep(table) {
  width: 100%;
  margin: 0.75rem 0 1.25rem;
  border-collapse: collapse;
  font-size: 14px;
}

.prose :deep(th),
.prose :deep(td) {
  padding: 8px 10px;
  border: 1px solid var(--color-page);
  text-align: left;
  vertical-align: top;
}

.prose :deep(th) {
  background: var(--color-page);
  font-weight: 600;
}

.prose :deep(a) {
  color: var(--color-brand-ink);
  text-decoration: underline;
}

.prose :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: var(--color-page);
  padding: 1px 5px;
  border-radius: 4px;
}
</style>
