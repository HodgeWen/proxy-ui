<!-- 应用布局：左侧品牌 + 侧边导航，右侧顶栏（版本 / 用户 / 登出）与内容区 -->
<script setup lang="ts">
import { UButton, UNav, USegment } from '@veltra/desktop'
import type { NavItem } from '@veltra/desktop'
import { AiChat, Download, Filter, Monitor, Setting, Share, Upload } from '@veltra/icons/normal'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '../stores/auth'
import { setThemeMode, useTheme } from '../theme'
import type { ThemeMode } from '../theme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { mode } = useTheme()

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
    <aside class="app-aside">
      <div class="app-brand">proxy-ui</div>
      <u-nav
        :menus="menus"
        :current-path="currentPath"
        style="flex: 1; min-height: 0"
        @item-click="onItemClick"
      />
    </aside>
    <div class="app-main">
      <header class="app-header">
        <span v-if="auth.version" class="app-version">v{{ auth.version }}</span>
        <u-segment
          class="app-theme-switch"
          :model-value="mode"
          :items="themeModeItems"
          size="small"
          @update:model-value="setThemeMode"
        />
        <span class="app-user">{{ auth.user?.username }}</span>
        <u-button size="small" @click="handleLogout">退出登录</u-button>
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

.app-aside {
  display: flex;
  flex-direction: column;
  width: 232px;
  flex-shrink: 0;
  border-right: 1px solid var(--u-border-color, #e4e4e7);
}

.app-brand {
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  font-size: 18px;
  font-weight: 600;
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
  border-bottom: 1px solid var(--u-border-color, #e4e4e7);
}

.app-version {
  color: var(--u-text-color-second, #71717a);
  font-size: 13px;
}

.app-header .app-theme-switch {
  margin-left: auto;
}

.app-header .app-user {
  font-size: 14px;
}

.app-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 20px;
  background: var(--u-bg-color-middle, #fafafa);
}
</style>
