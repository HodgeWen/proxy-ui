// 前端入口：主题初始化（normalize + loadTheme 硬规则，必须在首次渲染前执行）。
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import '@veltra/styles/normalize'
import { loadTheme } from '@veltra/styles/theme'

import App from './App.vue'
import { router } from './router'

loadTheme()

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
