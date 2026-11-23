// 路由与守卫：未初始化强制进向导，未登录访问管理页重定向登录页，已初始化后向导页不可达。
import { createRouter, createWebHistory } from 'vue-router'

import { setUnauthorizedHandler } from '../api/http'
import AppLayout from '../components/AppLayout.vue'
import { useAuthStore } from '../stores/auth'
import CertsView from '../views/CertsView.vue'
import DashboardView from '../views/DashboardView.vue'
import InboundsView from '../views/InboundsView.vue'
import OutboundsView from '../views/OutboundsView.vue'
import RulesView from '../views/RulesView.vue'
import SettingsView from '../views/SettingsView.vue'
import SubscriptionView from '../views/SubscriptionView.vue'

// 路由 meta 类型：title 用于页面标题；subtitle 为页面一句话副标题（供顶栏消费）
declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    subtitle?: string
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', redirect: '/dashboard' },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: DashboardView,
          meta: { title: '仪表盘', subtitle: '内核运行状态、实时速率与按用户流量' },
        },
        {
          path: 'inbounds',
          name: 'inbounds',
          component: InboundsView,
          meta: { title: '入站', subtitle: '服务端协议监听，客户端订阅里的节点来源' },
        },
        {
          path: 'outbounds',
          name: 'outbounds',
          component: OutboundsView,
          meta: { title: '出站', subtitle: '流量的出口与路由规则的最终去向，未命中规则的流量也走这里的兜底出站' },
        },
        {
          path: 'certs',
          name: 'certs',
          component: CertsView,
          meta: { title: '证书', subtitle: 'TLS 证书管理：一键自签、上传/粘贴 PEM、引用服务器路径与到期倒计时' },
        },
        {
          path: 'rules',
          name: 'rules',
          component: RulesView,
          meta: { title: '规则', subtitle: '决定什么流量走哪个出站：自上而下逐条匹配，未命中走兜底出站' },
        },
        {
          path: 'subscription',
          name: 'subscription',
          component: SubscriptionView,
          meta: { title: '订阅用户', subtitle: '为每台设备创建订阅用户，导出客户端订阅链接' },
        },
        {
          path: 'settings',
          name: 'settings',
          component: SettingsView,
          meta: { title: '设置', subtitle: '外观、账号、面板更新、AI 供应商、DNS' },
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
  // 首次导航并发检查健康态与登录态，减少一次串行往返
  await Promise.allSettled([auth.ensureHealth(), auth.user ? Promise.resolve() : auth.ensureSession()])

  // 未初始化：一切页面都进首次设置向导
  if (!auth.initialized) {
    return to.name === 'setup' ? true : { name: 'setup' }
  }
  // 已初始化：向导页不可达
  if (to.name === 'setup') {
    return auth.user ? { name: 'dashboard' } : { name: 'login' }
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
