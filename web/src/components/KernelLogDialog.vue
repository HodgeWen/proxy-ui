<!-- 内核日志弹框：仅打开期间拉取并 5s 轮询（页面不可见或上次请求未返回时跳过），关闭即停；重开时先清空再拉取。
     渲染性能：内容未变跳过更新；用户上翻查看时不强制滚到底；日志区 contain 隔离重排 -->
<script setup lang="ts">
import { UButton, UDialog, vLoading } from '@veltra/desktop'
import { nextTick, onUnmounted, shallowRef, useTemplateRef, watch } from 'vue'

import { fetchKernelLogs } from '../api/kernel'
import type { KernelLogLine } from '../api/kernel'
import { dialogStyle } from '../dialog-style'
import { formatDateTime } from '../utils/format'
import EmptyState from './EmptyState.vue'

const visible = defineModel<boolean>('visible', { default: false })

const logs = shallowRef<KernelLogLine[]>([])
const loading = shallowRef(false)
const logBox = useTemplateRef<HTMLElement>('logBox')

let timer: ReturnType<typeof setInterval> | undefined
let inflight = false

function sameAsCurrent(next: KernelLogLine[]): boolean {
  const cur = logs.value
  if (cur.length !== next.length) return false
  const same = (a?: KernelLogLine, b?: KernelLogLine) => a?.time === b?.time && a?.line === b?.line
  return same(cur[0], next[0]) && same(cur.at(-1), next.at(-1))
}

async function refresh(manual = false) {
  if (inflight) return
  inflight = true
  if (manual) loading.value = true
  try {
    const next = await fetchKernelLogs(500)
    if (!visible.value || sameAsCurrent(next)) return
    const box = logBox.value
    // 贴底（或首次）才跟随滚动，用户上翻查看历史时保持位置
    const stick = !box || box.scrollHeight - box.scrollTop - box.clientHeight < 24
    logs.value = next
    if (stick) {
      await nextTick()
      if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
    }
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    inflight = false
    loading.value = false
  }
}

function stop() {
  if (timer) clearInterval(timer)
  timer = undefined
}

watch(visible, (open) => {
  stop()
  if (!open) return
  logs.value = []
  loading.value = true
  void refresh()
  timer = setInterval(() => {
    if (!document.hidden) void refresh()
  }, 5000)
})

onUnmounted(stop)
</script>

<template>
  <u-dialog v-model="visible" title="内核日志" :style="dialogStyle(960, 94)">
    <template #default="{ maximized }">
      <div class="log-panel" :style="{ height: maximized ? '100%' : 'calc(90vh - 150px)' }">
        <div class="toolbar">
          <span class="note">最近 500 行，打开期间每 5 秒自动刷新</span>
          <u-button size="small" :loading="loading" @click="refresh(true)">刷新</u-button>
        </div>
        <div v-loading="loading && !logs.length" class="log-wrap">
          <EmptyState v-if="!loading && !logs.length" title="暂无内核日志" />
          <div v-else-if="logs.length" ref="logBox" class="code-block kernel-log">
            <div v-for="(item, index) in logs" :key="index" class="kernel-log-line">
              <span class="kernel-log-time">{{ formatDateTime(item.time) }}</span>
              <span :class="['kernel-log-stream', `is-${item.stream}`]">{{ item.stream }}</span>
              <span>{{ item.line }}</span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </u-dialog>
</template>

<style scoped>
.log-panel {
  display: flex;
  flex-direction: column;
  gap: var(--u-gap-default);
  min-height: 240px;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.note {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

.log-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* chrome 取全局 code-block；contain 把每次刷新的重排限制在日志区内 */
.kernel-log {
  flex: 1;
  min-height: 0;
  contain: strict;
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
