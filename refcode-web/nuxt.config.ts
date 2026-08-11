import tailwindcss from '@tailwindcss/vite'

const siteUrl = process.env.NUXT_PUBLIC_SITE_URL || 'http://localhost:3000'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  // 圖片跨 web/app/admin 共用，統一放在 monorepo 根目錄的 public/，
  // 不要各自留一份 —— 這裡指過去，否則 Nuxt 預設抓自己資料夾底下的 public/。
  dir: { public: '../public' },

  modules: ['@nuxtjs/i18n'],

  css: ['~/assets/css/main.css'],
  vite: {
    plugins: [tailwindcss()],
  },

  i18n: {
    defaultLocale: 'zh-TW',
    // 中文維持原本的網址（/referral/xxx），英文才加前綴。既有的中文頁面
    // 已經被索引了，加前綴等於整站換網址。
    strategy: 'prefix_except_default',
    // 日文先停用（付費服務還沒做日本市場的功能／文案），翻譯檔留著沒刪
    // （i18n/locales/ja.json），之後要重開直接把這行加回來就好。
    locales: [
      { code: 'zh-TW', language: 'zh-Hant-TW', name: '繁體中文', file: 'zh-TW.json' },
      { code: 'en', language: 'en-US', name: 'English', file: 'en.json' },
    ],
    // hreflang 要絕對網址，跟 sitemap 共用同一個來源。
    baseUrl: siteUrl,
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'refcode_lang',
      // 只在首頁依瀏覽器語言轉址。深層頁跟著轉的話，爬蟲拿到的內容會跟
      // 它請求的網址對不上，hreflang 就白做了。
      redirectOn: 'root',
    },
  },

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:7802',
      siteUrl,
      // 沒填就不顯示 Google 登入按鈕（見 useGoogleSignIn）。
      googleClientId: process.env.NUXT_PUBLIC_GOOGLE_CLIENT_ID || '',
      // 聯絡信箱。footer 的「聯絡我們」與刪除帳號頁的求助那一行都用它，
      // 沒填兩處都不顯示 —— 但送審前一定要填，那是站上唯一的對外聯絡管道。
      supportEmail: process.env.NUXT_PUBLIC_SUPPORT_EMAIL || '',
    },
  },

  app: {
    head: {
      // lang 由 i18n 依當下語言決定（見 app.vue 的 useLocaleHead）。
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      ],
      // favicon 移進 public/images/ 之後不再有 /favicon.ico 這條路徑，
      // 瀏覽器預設的隱性請求會 404，所以要明確宣告。
      // SVG 版擺前面：它靠內嵌的 prefers-color-scheme 在深色分頁列上換色，
      // ico 是給不吃 SVG favicon 的環境的退路。
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/images/favicon.svg' },
        { rel: 'icon', type: 'image/x-icon', href: '/images/favicon.ico' },
      ],
    },
  },

  // 這個站靠自然搜尋吃流量，服務商頁一定要 SSR，不能退回 SPA。
  ssr: true,
})
