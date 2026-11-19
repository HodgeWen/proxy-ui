// 路由与守卫：未初始化强制进向导，未登录访问管理页重定向登录页，已初始化后向导页不可达。
import { createRouter, createWebHistory } from 'vue-router'

import { setUnauthorizedHandler } from '../api/http'
import { useAuthStore } from '../stores/auth'

// 路由 meta 类型：title 用于页面标题
declare module 'vue-router' {
  interface RouteMeta {
    title?: string
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('../components/AppLayout.vue'),
      children: [
        { path: '', redirect: '/dashboard' },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('../views/DashboardView.vue'),
          meta: { title: '仪表盘' },
        },
        {
          path: 'inbounds',
          name: 'inbounds',
          component: () => import('../views/InboundsView.vue'),
          meta: { title: '入站' },
        },
        {
          path: 'outbounds',
          name: 'outbounds',
          component: () => import('../views/OutboundsView.vue'),
          meta: { title: '出站' },
        },
        {
          path: 'rules',
          name: 'rules',
          component: () => import('../views/RulesView.vue'),
          meta: { title: '规则' },
        },
        {
          path: 'subscription',
          name: 'subscription',
          component: () => import('../views/SubscriptionView.vue'),
          meta: { title: '订阅' },
        },
        {
          path: 'ai',
          name: 'ai',
          component: () => import('../views/AiChatView.vue'),
          meta: { title: 'AI 助手' },
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('../views/SettingsView.vue'),
          meta: { title: '设置' },
        },
      ],
    },
    {
      path: '/setup',
      name: 'setup',
      component: () => import('../views/SetupView.vue'),
      meta: { title: '初始化设置' },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { title: '登录' },
    },
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
})

// 会话过期（任意接口 401）时清空本地登录态并跳登录页
setUnauthorizedHandler(() => {
  const auth = useAuthStore()
  auth.user = null
  if (router.currentRoute.value.name !== 'login') {
    router.push({ name: 'login' })
  }
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  try {
    await auth.ensureHealth()
  } catch {
    // 后端不可达时放行导航，由页面内具体请求暴露错误
  }

  // 未初始化：一切页面都进首次设置向导
  if (!auth.initialized) {
    return to.name === 'setup' ? true : { name: 'setup' }
  }
  // 已初始化：向导页不可达
  if (to.name === 'setup') {
    return auth.user ? { name: 'dashboard' } : { name: 'login' }
  }
  // 刷新后首次导航时校验既有会话
  if (!auth.user) {
    await auth.ensureSession()
  }
  // 未登录：管理页重定向登录页并带回跳地址
  if (!auth.user) {
    return to.name === 'login' ? true : { name: 'login', query: { redirect: to.fullPath } }
  }
  // 已登录访问登录页：进仪表盘
  return to.name === 'login' ? { name: 'dashboard' } : true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · proxy-ui` : 'proxy-ui'
})
