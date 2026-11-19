import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  server: {
    // 开发模式下 /api 代理到本机 Go 后端
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
