import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  // 後端的 CORS_ORIGINS 預設就列了這個 port，改的話兩邊要一起改。
  server: {
    port: 5174,
    // ios/、android/ 是原生專案的建置產物（例如 ios/DerivedData），
    // 檔案量上千且會一直變動，沒排除掉會讓 dev server 狂觸發不相關的 reload。
    watch: { ignored: ['**/ios/**', '**/android/**'] },
  },
})
