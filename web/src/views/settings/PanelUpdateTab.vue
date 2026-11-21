<!-- 设置页「面板更新」页签：检查可用版本、一键更新（下载替换 + 等待重启恢复） -->
<script setup lang="ts">
import { sleep } from '@cat-kit/core'
import { UButton, UCard, UCardContent, UCardHeader, UTag, message } from '@veltra/desktop'
import { computed, onActivated, shallowRef } from 'vue'

import { fetchHealth } from '../../api/auth'
import { fetchPanelVersions, updatePanel } from '../../api/panel'
import type { ReleaseVersion } from '../../api/panel'

/** 面板当前版本与可用版本 */
const panelVersion = shallowRef('')
const panelVersions = shallowRef<ReleaseVersion[]>([])
const panelChecking = shallowRef(false)
const panelUpdating = shallowRef(false)
/** 面板更新阶段提示（下载替换 / 等待重启恢复） */
const panelPhase = shallowRef('')

/** 最新稳定版 */
const latestPanel = computed(() => panelVersions.value.find((item) => !item.prerelease))

type UpdateState = 'unchecked' | 'latest' | 'available'

/** 更新按钮三态：未检查（禁用并给出原因）/ 已是最新（次级重装）/ 有新版（主操作） */
const panelState = computed<UpdateState>(() => {
  if (!latestPanel.value) return 'unchecked'
  return latestPanel.value.installed ? 'latest' : 'available'
})

async function checkPanelUpdate() {
  panelChecking.value = true
  try {
    const { currentVersion, versions: list } = await fetchPanelVersions()
    panelVersion.value = currentVersion
    panelVersions.value = list
  } catch {
    // 网络错误已由请求封装统一提示
  } finally {
    panelChecking.value = false
  }
}

async function handlePanelUpdate() {
  if (!latestPanel.value) return
  panelUpdating.value = true
  panelPhase.value = '正在下载并替换面板二进制…'
  try {
    const { version } = await updatePanel('')
    panelPhase.value = `已更新到 ${version}，面板正在重启，等待服务恢复…`
    if (await waitPanelOnline(version)) {
      message.success(`面板已更新到 ${version}`)
      window.location.reload()
    } else {
      message.warn('面板服务恢复超时，请稍后手动刷新查看')
      panelPhase.value = ''
    }
  } catch {
    panelPhase.value = ''
    // 下载失败错误已由请求封装统一提示
  } finally {
    panelUpdating.value = false
  }
}

/** 等待面板重启恢复且版本号变为目标版本；60 秒内未恢复返回 false */
async function waitPanelOnline(targetVersion: string): Promise<boolean> {
  for (let i = 0; i < 60; i++) {
    await sleep(1000)
    try {
      const health = await fetchHealth()
      if (health.version === targetVersion) return true
    } catch {
      // 服务重启中，继续等待
    }
  }
  return false
}

// 进入页签（含首次挂载）自动检查一次更新
onActivated(checkPanelUpdate)
</script>

<template>
  <u-card class="settings-card">
    <u-card-header>面板更新</u-card-header>
    <u-card-content>
      <div class="info-meta">
        <span>当前版本：<strong>{{ panelVersion || '未知' }}</strong></span>
        <span>最新稳定版：<strong>{{ latestPanel?.tag ?? '未检查' }}</strong></span>
        <u-tag v-if="panelState === 'latest'" type="success">已是最新版</u-tag>
      </div>
      <div class="ops-row">
        <u-button :loading="panelChecking" @click="checkPanelUpdate">检查更新</u-button>
        <u-button
          v-if="panelState === 'available'"
          type="primary"
          :loading="panelUpdating"
          @click="handlePanelUpdate"
        >
          一键更新
        </u-button>
        <u-button v-else-if="panelState === 'latest'" :loading="panelUpdating" @click="handlePanelUpdate">
          重装最新版
        </u-button>
        <u-button v-else disabled>一键更新</u-button>
      </div>
      <p v-if="panelState === 'unchecked'" class="hint">尚未获取可用版本，「一键更新」待「检查更新」完成后可用。</p>
      <p v-if="panelPhase" class="hint">{{ panelPhase }}</p>
      <p class="hint">更新会替换面板二进制并自动重启进程，数据与配置保留；内核随面板一同升级。</p>
    </u-card-content>
  </u-card>
</template>
<!-- settings-card / hint / info-meta / ops-row 为 App.vue 全局类，本组件无局部样式 -->
