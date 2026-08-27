import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  // web 與 admin 指到 monorepo 根目錄那份共用 public/，app 刻意不跟 ——
  // 那裡面幾乎都是官網與 Play 商店用的大圖（光背景圖就 6MB、商店宣傳圖也在裡面），
  // 而 app 一張都沒引用（商家 logo 全是遠端 URL）。Vite 的 publicDir 是整包複製、
  // 沒有排除機制，指過去等於白白把 10MB 打進 APK，直接踩 Google Play 的
  // 安裝體積與點陣圖用量門檻。這裡只留 app 真的需要的，目前就一個 favicon。
  publicDir: 'public',
  // 後端的 CORS_ORIGINS 預設就列了這個 port，改的話兩邊要一起改。
  server: {
    port: 5174,
    // 監聽所有介面而不是只有 localhost。`npx cap run ios -l` 不給 --host 時會自己
    // 抓這台機器的區網 IP 寫進原生設定，只綁 localhost 的話那個位址沒人在聽，
    // app 一啟動就是白頁；換了 WiFi、IP 變了也一樣。綁 0.0.0.0 之後不管當下
    // 的 IP 是什麼都連得到，實機測試也不必再為了 IP 改設定。
    host: true,
    // ios/、android/ 是原生專案的建置產物（例如 ios/DerivedData），
    // 檔案量上千且會一直變動，沒排除掉會讓 dev server 狂觸發不相關的 reload。
    watch: { ignored: ['**/ios/**', '**/android/**'] },
  },
})
