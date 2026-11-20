<!-- 应用布局：左侧整列 nav 表面侧栏（品牌 / 导航 / 用户底部区），右侧顶栏（页面标题 / 版本 / 内核状态 / 主题切换）与内容区 -->
<script setup lang="ts">
import { UButton, UNav, USegment, UTag } from '@veltra/desktop'
import type { NavItem } from '@veltra/desktop'
import { AiChat, Download, Filter, Monitor, Setting, Share, Terminal, Upload, UserCircle } from '@veltra/icons/normal'
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

const kernelStatus = shallowRef<KernelStatus | null>(null)

async function refreshKernelStatus() {
  try {
    kernelStatus.value = await fetchKernelStatus()
  } catch {
    // 静默：下轮轮询重试；内核状态仅作顶栏呈现，不阻塞导航
  }
}

const kernelTag = computed(() => {
  const status = kernelStatus.value
  if (!status) return undefined
  if (!status.installed) return { type: 'warning' as const, text: '内核未安装' }
  return status.running
    ? { type: 'success' as const, text: '内核运行中' }
    : { type: 'info' as const, text: '内核已停止' }
})

let kernelTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  void refreshKernelStatus()
  kernelTimer = setInterval(() => void refreshKernelStatus(), 10000)
})

onBeforeUnmount(() => {
  if (kernelTimer) clearInterval(kernelTimer)
})

const themeModeItems: { label: string; value: ThemeMode }[] = [
  { label: '亮色', value: 'light' },
  { label: '暗色', value: 'dark' },
  { label: '跟随系统', value: 'system' },
]

const menus: NavItem[] = [
  { title: '仪表盘', icon: Monitor, path: '/dashboard' },
  { title: '入站', icon: Download, path: '/inbounds' },
  { title: '出站', icon: Upload, path: '/outbounds' },
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
          <UserCircle class="app-user-icon" />
          <span class="app-user-name">{{ auth.user?.username }}</span>
        </div>
        <u-button @click="handleLogout">退出</u-button>
      </div>
    </aside>
    <div class="app-main">
      <header class="app-header">
        <h1 class="app-page-title">{{ pageTitle }}</h1>
        <span v-if="auth.version" class="app-version">v{{ auth.version }}</span>
        <u-tag v-if="kernelTag" class="app-kernel-tag" :type="kernelTag.type" size="small">
          {{ kernelTag.text }}
        </u-tag>
        <u-segment
          class="app-theme-switch"
          :model-value="mode"
          :items="themeModeItems"
          size="small"
          @update:model-value="setThemeMode"
        />
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

.app-brand {
  height: 56px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  flex-shrink: 0;
}

.app-brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--u-nav-surface-mark-ring);
  border-radius: var(--u-radius-default);
}

.app-brand-icon {
  width: 16px;
  height: 16px;
  color: var(--u-nav-strong-color);
}

.app-brand-name {
  font-size: var(--u-font-size-title-default);
  font-weight: 600;
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
  margin: 8px;
  padding: 8px 10px;
  flex-shrink: 0;
  background: var(--u-nav-surface-chip-bg);
  border: 1px solid var(--u-nav-surface-chip-border);
  border-radius: var(--u-radius-default);
}

.app-user {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.app-user-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: var(--u-nav-second-color);
}

.app-user-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--u-font-size-main-default);
}

.app-main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.app-header {
  height: 56px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  background-color: var(--u-bg-color-top);
  border-bottom: var(--u-border);
}

.app-page-title {
  margin: 0;
  font-size: var(--u-font-size-title-default);
  font-weight: 600;
  color: var(--u-text-color-title);
}

.app-version {
  color: var(--u-text-color-assist);
  font-size: var(--u-font-size-assist-default);
}

.app-kernel-tag {
  flex-shrink: 0;
}

.app-header .app-theme-switch {
  margin-left: auto;
}

.app-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 20px;
  background: var(--u-bg-color-middle);
}
</style>
