import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  // 圖片跨 web/app/admin 共用，統一放在 monorepo 根目錄的 public/，不要各自留一份。
  publicDir: '../public',
  // 後端的 CORS_ORIGINS 預設就列了這個 port，改的話兩邊要一起改。
  server: { port: 5173 },
})
