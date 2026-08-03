import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  // 後端的 CORS_ORIGINS 預設就列了這個 port，改的話兩邊要一起改。
  server: { port: 5174 },
})
