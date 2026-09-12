import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 同源部署：Go 服务默认 serve web/dist，开发期把 API 与 OAuth 入口代理到本地后端。
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/auth': 'http://127.0.0.1:8080',
    },
  },
  build: { outDir: 'dist', chunkSizeWarningLimit: 800 },
})
