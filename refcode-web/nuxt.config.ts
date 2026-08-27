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
      // 分頁圖示直接用 app 的圖示（public/images/app-icon-1024.png 就是 iOS
      // AppIcon 那張），web 與 app 在使用者眼裡是同一個東西，不要兩套視覺。
      // 1024 當 favicon 太大，先切出 32 與 180 兩個尺寸。
      // 另外 iOS Safari 在沒有宣告的情況下會自己去試 /apple-touch-icon.png
      // 與 -precomposed 版，兩條都不存在，Nuxt 會把它們當成頁面路徑丟給
      // Vue Router，於是每次開站都洗出幾行 R0004 警告，所以要明確宣告。
      link: [
        { rel: 'icon', type: 'image/png', sizes: '32x32', href: '/images/app-icon-32.png' },
        { rel: 'icon', type: 'image/png', sizes: '180x180', href: '/images/app-icon-180.png' },
        { rel: 'apple-touch-icon', href: '/images/app-icon-180.png' },
      ],
    },
  },

  // 這個站靠自然搜尋吃流量，服務商頁一定要 SSR，不能退回 SPA。
  ssr: true,
})
