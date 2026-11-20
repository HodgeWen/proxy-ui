import vue from '@vitejs/plugin-vue'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => ({
  plugins: [vue()],
  server: {
    // 开发模式下 /api 代理到本机 Go 后端；API_PROXY 环境变量可覆盖（make dev 按 DEV_ADDR 联动传入）
    proxy: {
      '/api': loadEnv(mode, '.', '')['API_PROXY'] || 'http://127.0.0.1:8080',
    },
  },
}))
