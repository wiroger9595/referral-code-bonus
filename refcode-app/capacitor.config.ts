import type { CapacitorConfig } from '@capacitor/cli'

const config: CapacitorConfig = {
  appId: 'tw.refcode.app',
  appName: 'Referra',
  webDir: 'dist',
  server: {
    // 本機用瀏覽器開發時不需要這段；要在實機上連本機 API 時把
    // url 指到電腦的區網 IP（模擬器連 localhost 會連到裝置自己）。
    androidScheme: 'https',
  },
  // WebView 頁面走 https://localhost（見上面 androidScheme），開發時 API 還是
  // 明文的 http://區網IP，屬於 mixed content，Chromium 預設直接擋掉 fetch，
  // 跟 AndroidManifest 那份 network_security_config（管的是 OS 層級的明文流量）
  // 是兩層獨立的擋法，兩個都要放行才連得到本機 API。正式環境 API 一律是 https，
  // 不會真的觸發 mixed content，這個開關留著不影響上架。
  android: {
    allowMixedContent: true,
  },
  plugins: {
    // social-login 預設把四家 provider 的原生 SDK 全部打包進去。只開有在用的
    // Google，其餘關掉，原生層就不會多帶不用的 SDK 進去
    // （Facebook SDK 會把 AD_ID 權限、install referrer 與 com.facebook.katana
    // 的 queries 塞進合併後的 manifest，逼 Play 的資料安全性表單申報廣告 ID，
    // 也跟隱私權政策「沒有 Meta SDK」那句話衝突）。
    //
    // Apple 明知故犯地關掉：iOS 上同時提供 Google 登入時，App Store 4.8 規定
    // 必須也提供 Apple 登入，只關 Apple、留著 Google 送審會被退件。這裡是
    // 刻意接受這個風險（見 src/api/social.ts 的 appleReady），要嘛之後把
    // Apple 加回來，要嘛把 Google 也一起關掉再上架。
    SocialLogin: {
      providers: {
        google: true,
        apple: false,
        facebook: false,
        twitter: false,
      },
    },
  },
}

export default config
