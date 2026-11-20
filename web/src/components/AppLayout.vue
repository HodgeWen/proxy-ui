<!-- 应用布局：左侧整列 nav 表面侧栏（渐变 logo 品牌区 / 导航 / 用户底部区），
     右侧顶栏（标题 + 副标题 / 内核状态胶囊 / 主题循环按钮）与内容区。
     顶栏位于滚动容器外天然吸顶；颜色/圆角/阴影全部走 --u-* 主题令牌，12 预设与明暗切换直接生效 -->
<script setup lang="ts">
import { UButton, UNav } from '@veltra/desktop'
import type { NavItem } from '@veltra/desktop'
import { AiChat, Download, Filter, Monitor, Moon, Secured, Setting, Share, Sun, Terminal, Upload, UserCircle } from '@veltra/icons/normal'
import { computed, onBeforeUnmount, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { fetchKernelStatus } from '../api/kernel'
import type { KernelStatus } from '../api/kernel'
import { useAuthStore } from '../stores/auth'
import { setThemeMode, useTheme } from '../theme'
import type { ThemeMode } from '../theme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { mode } = useTheme()

// ---------- 顶栏页面标题与内核状态 ----------

const pageTitle = computed(() => route.meta.title)
const pageSubtitle = computed(() => route.meta.subtitle)

const kernelStatus = shallowRef<KernelStatus | null>(null)

async function refreshKernelStatus() {
  try {
    kernelStatus.value = await fetchKernelStatus()
  } catch {
    // 静默：下轮轮询重试；内核状态仅作顶栏呈现，不阻塞导航
  }
}

/** 内核状态 → 胶囊 chip 的语义色（success 呼吸点，warning / info 静态点） */
const kernelChip = computed(() => {
  const status = kernelStatus.value
  if (!status) return undefined
  if (!status.installed) return { status: 'warning' as const, text: '内核未安装' }
  return status.running
    ? { status: 'success' as const, text: '内核运行中' }
    : { status: 'info' as const, text: '内核已停止' }
})

let kernelTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  void refreshKernelStatus()
  kernelTimer = setInterval(() => void refreshKernelStatus(), 10000)
})

onBeforeUnmount(() => {
  if (kernelTimer) clearInterval(kernelTimer)
})

// ---------- 顶栏主题循环按钮：light → dark → system，图标与提示随当前模式 ----------

const themeCycle: Record<ThemeMode, { icon: typeof Sun; label: string; next: ThemeMode }> = {
  light: { icon: Sun, label: '亮色', next: 'dark' },
  dark: { icon: Moon, label: '暗色', next: 'system' },
  system: { icon: Monitor, label: '跟随系统', next: 'light' },
}

const themeToggle = computed(() => themeCycle[mode.value])

function toggleThemeMode(): void {
  setThemeMode(themeToggle.value.next)
}

const menus: NavItem[] = [
  { title: '仪表盘', icon: Monitor, path: '/dashboard' },
  { title: '入站', icon: Download, path: '/inbounds' },
  { title: '出站', icon: Upload, path: '/outbounds' },
  { title: '证书', icon: Secured, path: '/certs' },
  { title: '规则', icon: Filter, path: '/rules' },
  { title: '订阅', icon: Share, path: '/subscription' },
  { title: 'AI 助手', icon: AiChat, path: '/ai' },
  { title: '设置', icon: Setting, path: '/settings' },
]

const currentPath = computed(() => route.path)

function onItemClick(item: NavItem) {
  router.push(item.path)
}

async function handleLogout() {
  await auth.doLogout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="app-shell">
    <aside class="app-aside" data-nav-variant="dark">
      <div class="app-brand">
        <span class="app-brand-mark">
          <Terminal class="app-brand-icon" />
        </span>
        <span class="app-brand-name">proxy-ui</span>
      </div>
      <u-nav class="app-nav" :menus="menus" :current-path="currentPath" @item-click="onItemClick" />
      <div class="app-aside-footer">
        <div class="app-user">
          <span class="app-user-avatar">
            <UserCircle class="app-user-icon" />
          </span>
          <span class="app-user-name">{{ auth.user?.username }}</span>
        </div>
        <u-button size="small" @click="handleLogout">退出</u-button>
      </div>
    </aside>
    <div class="app-main">
      <header class="app-header">
        <div class="app-heading">
          <h1 class="app-page-title">{{ pageTitle }}</h1>
          <p v-if="pageSubtitle" class="app-page-subtitle">{{ pageSubtitle }}</p>
        </div>
        <div class="app-header-side">
          <span v-if="kernelChip" class="app-kernel-chip" :data-status="kernelChip.status">
            <span class="app-kernel-dot" />
            {{ kernelChip.text }}
          </span>
          <span v-if="auth.version" class="app-version">v{{ auth.version }}</span>
          <u-button
            class="app-theme-toggle"
            circle
            text
            :icon="themeToggle.icon"
            :icon-size="16"
            :title="`当前：${themeToggle.label}，点击切换`"
            :aria-label="`当前：${themeToggle.label}，点击切换`"
            @click="toggleThemeMode"
          />
        </div>
      </header>
      <main class="app-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  height: 100vh;
}

/* 侧栏整列即 nav 表面：底色 / 叠层渐变 / 右缘发丝线全取 nav 令牌，data-nav-variant 驱动深浅联动 */
.app-aside {
  display: flex;
  flex-direction: column;
  width: 232px;
  flex-shrink: 0;
  color: var(--u-nav-color);
  background-color: var(--u-nav-bg-color);
  background-image: var(--u-nav-surface-overlay), var(--u-nav-bg-image);
  background-repeat: no-repeat;
  border-right: 1px solid var(--u-nav-surface-hairline);
}

/* 品牌区与顶栏同高（64px）保证左右视线齐平 */
.app-brand {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  flex-shrink: 0;
}

/* 渐变色 logo 方块：主色同 hue 浅→深，所有预设自动取各自主色 */
.app-brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  border-radius: var(--u-radius-default);
  background: linear-gradient(135deg, var(--u-color-primary-light-1), var(--u-color-primary-dark-3));
  box-shadow: var(--u-shadow-sm);
}

.app-brand-icon {
  width: 16px;
  height: 16px;
  color: var(--u-text-color-white);
}

.app-brand-name {
  font-size: var(--u-font-size-title-default);
  font-weight: 700;
  letter-spacing: 0.2px;
  color: var(--u-nav-strong-color);
}

.app-nav {
  flex: 1;
  min-height: 0;
}

/* UNav 自带浮起卡片外观（边框 / 大圆角 / 阴影 / 底色），侧栏内铺平与容器同面 */
.app-aside :deep(.u-nav) {
  background-color: transparent;
  background-image: none;
  border: none;
  border-radius: 0;
  box-shadow: none;
  backdrop-filter: none;
}

.app-aside-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: 10px;
  padding: 8px 10px;
  flex-shrink: 0;
  background: var(--u-nav-surface-chip-bg);
  border: 1px solid var(--u-nav-surface-chip-border);
  border-radius: var(--u-radius-default);
}

.app-user {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.app-user-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--u-nav-surface-chip-hover-bg);
}

.app-user-icon {
  width: 14px;
  height: 14px;
  color: var(--u-nav-second-color);
}

.app-user-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--u-font-size-main-small);
  color: var(--u-nav-color);
}

.app-main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

/* 顶栏在滚动容器外天然吸顶；右上角主色 radial 微光为唯一装饰，语义克制 */
.app-header {
  min-height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 24px;
  flex-shrink: 0;
  background-color: var(--u-bg-color-top);
  background-image: radial-gradient(85% 120% at 100% 0%, var(--u-color-primary-a-8), transparent 60%);
  background-repeat: no-repeat;
  border-bottom: 1px solid var(--u-border-color-a-22, var(--u-border-color));
}

.app-heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.app-page-title {
  margin: 0;
  font-size: var(--u-font-size-title-large);
  font-weight: 700;
  line-height: 1.2;
  color: var(--u-text-color-title);
  white-space: nowrap;
}

/* 无副标题的路由优雅降级：只显标题，标题列在 64px 顶栏内垂直居中 */
.app-page-subtitle {
  margin: 0;
  font-size: var(--u-font-size-assist-default);
  line-height: 1.3;
  color: var(--u-text-color-second);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-header-side {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
  margin-left: auto;
}

/* 内核状态胶囊：24px 高 + radius-large 接近半径 12px，拼出胶囊外形；
   语义色 tint 底 + 同色文字，success 点呼吸、warning / info 点静态 */
.app-kernel-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 10px;
  border-radius: var(--u-radius-large);
  font-size: var(--u-font-size-assist-default);
}

.app-kernel-chip[data-status='success'] {
  color: var(--u-color-success);
  background: var(--u-color-success-a-10);
  box-shadow: inset 0 0 0 1px var(--u-color-success-a-28);
}

.app-kernel-chip[data-status='warning'] {
  color: var(--u-color-warning);
  background: var(--u-color-warning-a-10);
  box-shadow: inset 0 0 0 1px var(--u-color-warning-a-28);
}

.app-kernel-chip[data-status='info'] {
  color: var(--u-color-info);
  background: var(--u-color-info-a-10);
  box-shadow: inset 0 0 0 1px var(--u-color-info-a-28);
}

.app-kernel-dot {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: currentColor;
}

.app-kernel-chip[data-status='success'] .app-kernel-dot {
  animation: app-dot-breathe 2.4s ease-in-out infinite;
}

@keyframes app-dot-breathe {
  0%,
  100% {
    box-shadow: 0 0 0 0 var(--u-color-success-a-40);
  }

  70% {
    box-shadow: 0 0 0 5px var(--u-color-success-a-4);
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-kernel-chip[data-status='success'] .app-kernel-dot {
    animation: none;
  }
}

.app-version {
  font-size: var(--u-font-size-assist-small);
  color: var(--u-text-color-second);
}

.app-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 24px;
  background-image: radial-gradient(70% 40% at 100% 0%, var(--u-color-primary-a-5), transparent 62%);
  background-repeat: no-repeat;
  background-color: var(--u-bg-color-middle);
}
</style>
