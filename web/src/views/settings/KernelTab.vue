<!-- 设置页「内核」页签：内置 sing-box 只读状态 + 日志（进入页签拉取并 5s 轮询，离开即停） -->
<script setup lang="ts">
import { UButton, UCard, UCardContent, UCardHeader, UTag, message, vLoading } from '@veltra/desktop'
import { nextTick, onActivated, onDeactivated, onUnmounted, shallowRef, useTemplateRef } from 'vue'

import EmptyState from '../../components/EmptyState.vue'
import { fetchKernelLogs, fetchKernelStatus } from '../../api/kernel'
import type { KernelLogLine, KernelStatus } from '../../api/kernel'
import { formatDateTime, formatUptime } from '../../utils/format'

/** 内置内核的只读运行状态与日志（内核编进面板随面板自动运行，不可启停或换版本） */
const status = shallowRef<KernelStatus | null>(null)
const logs = shallowRef<KernelLogLine[]>([])
const logsLoading = shallowRef(true)
const logBox = useTemplateRef<HTMLElement>('logBox')

let pollTimer: ReturnType<typeof setInterval> | undefined

async function refreshStatus(silent = true) {
  try {
    status.value = await fetchKernelStatus()
  } catch {
    if (!silent) message.error('查询内核状态失败')
  }
}

async function refreshLogs(silent = true) {
  if (!silent || !logs.value.length) logsLoading.value = true
  try {
    logs.value = await fetchKernelLogs(500)
    await nextTick()
    if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
  } catch {
    if (!silent) message.error('查询内核日志失败')
  } finally {
    logsLoading.value = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    void refreshStatus()
    void refreshLogs()
  }, 5000)
}

function stopPolling() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = undefined
}

// 进入页签（含首次挂载）拉取并轮询；切走 / 离开设置页即停
onActivated(() => {
  void refreshStatus(false)
  void refreshLogs()
  startPolling()
})

onDeactivated(stopPolling)
onUnmounted(stopPolling)
</script>

<template>
  <u-card class="settings-card">
    <u-card-header>
      <div class="card-header-row">
        <span>sing-box 内核</span>
      </div>
    </u-card-header>
    <u-card-content>
      <div class="info-meta">
        <u-tag :type="status ? (status.running ? 'success' : 'info') : 'info'">
          {{ status ? (status.running ? '运行中' : '已停止') : '加载中…' }}
        </u-tag>
        <span>版本：<strong>{{ status?.version || '-' }}</strong></span>
        <span>运行时长：{{ formatUptime(status?.uptimeSeconds) }}</span>
      </div>
      <p class="hint">
        内核随面板启动自动运行；任一配置保存后立即按新配置重建，起不来时自动回滚并提示原因；版本随面板一同升级。
      </p>
    </u-card-content>
  </u-card>

  <u-card class="settings-card">
    <u-card-header>
      <div class="card-header-row">
        <span>内核日志</span>
        <u-button :loading="logsLoading" @click="refreshLogs(false)">刷新日志</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <div v-loading="logsLoading && !logs.length" class="kernel-log-wrap">
        <EmptyState v-if="!logsLoading && !logs.length" title="暂无内核日志" />
        <div v-else-if="logs.length" ref="logBox" class="code-block kernel-log">
          <div v-for="(item, index) in logs" :key="index" class="kernel-log-line">
            <span class="kernel-log-time">{{ formatDateTime(item.time) }}</span>
            <span :class="['kernel-log-stream', `is-${item.stream}`]">{{ item.stream }}</span>
            <span class="kernel-log-text">{{ item.line }}</span>
          </div>
        </div>
      </div>
    </u-card-content>
  </u-card>
</template>

<style scoped>
/* settings-card / hint / card-header-row / info-meta 为 App.vue 全局类 */

/* 日志区：chrome 取全局 code-block，这里只定高度 */
.kernel-log-wrap {
  min-height: 120px;
}

.kernel-log {
  height: 320px;
}

.kernel-log-line {
  display: flex;
  gap: 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

.kernel-log-time {
  color: var(--u-text-color-second);
  flex-shrink: 0;
}

.kernel-log-stream {
  flex-shrink: 0;
  color: var(--u-color-primary);
}

.kernel-log-stream.is-stderr {
  color: var(--u-color-danger);
}

.kernel-log-stream.is-panel {
  color: var(--u-color-warning);
}
</style>
