import type { CapacitorConfig } from '@capacitor/cli'

const config: CapacitorConfig = {
  appId: 'tw.refcode.app',
  appName: '推薦碼交流站',
  webDir: 'dist',
  server: {
    // 本機用瀏覽器開發時不需要這段；要在實機上連本機 API 時把
    // url 指到電腦的區網 IP（模擬器連 localhost 會連到裝置自己）。
    androidScheme: 'https',
  },
  plugins: {
    // social-login 預設把四家 provider 的原生 SDK 全部打包進去。我們只用 Google
    // 與 Apple —— 不關掉的話 Facebook SDK 會進 APK，連帶把 AD_ID 權限、
    // install referrer 與 com.facebook.katana 的 queries 塞進合併後的 manifest。
    // 那會逼 Play 的資料安全性表單必須申報使用廣告 ID，也跟隱私權政策裡
    // 「沒有 Meta SDK」那句話直接衝突。改了要重跑 npx cap sync 才會生效。
    SocialLogin: {
      providers: {
        google: true,
        apple: true,
        facebook: false,
        twitter: false,
      },
    },
  },
}

export default config
