<!-- 仪表盘：内核只读状态（运行状态 / 版本 / 运行时长）、实时速率概览、按用户累计流量 -->
<script setup lang="ts">
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UTable,
  UTag,
  defineTableColumns,
} from '@veltra/desktop'
import { ChartPie } from '@veltra/icons/normal'
import { onBeforeUnmount, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { fetchKernelStatus } from '../api/kernel'
import type { KernelStatus } from '../api/kernel'
import { formatBytes } from '../api/subscription'
import { fetchRealtimeStats, fetchUserTraffic } from '../api/stats'
import type { RealtimeStats, UserTrafficRow } from '../api/stats'
import EmptyState from '../components/EmptyState.vue'
import { protocolDef } from '../inbound-schema'

const router = useRouter()

// ---------- 内核状态（只读） ----------

const status = shallowRef<KernelStatus | null>(null)

async function refreshStatus() {
  try {
    status.value = await fetchKernelStatus()
  } catch {
    // 静默：下轮轮询重试
  }
}

// ---------- 实时统计 ----------

const realtime = shallowRef<RealtimeStats | null>(null)

async function refreshRealtime() {
  try {
    realtime.value = await fetchRealtimeStats()
  } catch {
    // 静默：下轮轮询重试
  }
}

// ---------- 按用户累计流量 ----------

const trafficRows = shallowRef<UserTrafficRow[]>([])
// 用户/订阅用户两列同为弹性列分摊剩余宽度，避免空表时首列独占整行失衡
const trafficColumns = defineTableColumns([
  { key: 'tag', name: '用户', minWidth: 160 },
  { key: 'subscriber', name: '订阅用户', minWidth: 120 },
  { key: 'inboundName', name: '入站', minWidth: 140 },
  { key: 'protocol', name: '协议', width: 150 },
  { key: 'uploadBytes', name: '累计上传', width: 130, align: 'right' },
  { key: 'downloadBytes', name: '累计下载', width: 130, align: 'right' },
])

async function refreshTraffic() {
  try {
    trafficRows.value = await fetchUserTraffic()
  } catch {
    // 错误已由请求封装统一提示
  }
}

// ---------- 轮询 ----------

const timers: ReturnType<typeof setInterval>[] = []

onMounted(() => {
  void refreshStatus()
  void refreshRealtime()
  void refreshTraffic()
  timers.push(
    setInterval(() => void refreshStatus(), 2000),
    setInterval(() => void refreshRealtime(), 1000),
    setInterval(() => void refreshTraffic(), 15000),
  )
})

onBeforeUnmount(() => timers.forEach(clearInterval))

// ---------- 格式化 ----------

function formatRate(n: number): string {
  return `${formatBytes(n)}/s`
}

function formatUptime(seconds?: number): string {
  if (seconds === undefined) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  return [h, m, s].map((n) => String(n).padStart(2, '0')).join(':')
}
</script>

<template>
  <div class="dashboard">
    <u-card class="dash-card">
      <u-card-header>内核状态</u-card-header>
      <u-card-content>
        <div class="kernel-meta">
          <u-tag :type="status?.running ? 'success' : 'info'">
            {{ status?.running ? '运行中' : '已停止' }}
          </u-tag>
          <u-tag type="info">内置内核</u-tag>
          <span>版本：<strong>{{ status?.version || '内置' }}</strong></span>
          <span>运行时长：{{ formatUptime(status?.uptimeSeconds) }}</span>
        </div>
      </u-card-content>
    </u-card>

    <u-card class="dash-card">
      <u-card-header>流量概览</u-card-header>
      <u-card-content>
        <div class="stat-grid">
          <div class="stat-item stat-item--primary">
            <span class="stat-label">上传速率</span>
            <span class="stat-value is-up">{{ realtime ? formatRate(realtime.up) : 'N/A' }}</span>
          </div>
          <div class="stat-item stat-item--primary">
            <span class="stat-label">下载速率</span>
            <span class="stat-value is-down">{{ realtime ? formatRate(realtime.down) : 'N/A' }}</span>
          </div>
        </div>
      </u-card-content>
    </u-card>

    <u-card class="dash-card">
      <u-card-header>
        <div class="card-header-row">
          <span>按用户累计流量</span>
          <u-button @click="refreshTraffic">刷新</u-button>
        </div>
      </u-card-header>
      <u-card-content>
        <u-table :columns="trafficColumns" :data="trafficRows" row-key="tag" stripe>
          <template #column:tag="{ rowData }">
            <span class="mono">{{ rowData.tag }}</span>
          </template>
          <template #column:subscriber="{ rowData }">
            {{ rowData.subscriber || '—' }}
          </template>
          <template #column:protocol="{ rowData }">
            {{ protocolDef(rowData.protocol)?.name ?? rowData.protocol }}
          </template>
          <template #column:uploadBytes="{ rowData }">
            {{ formatBytes(rowData.uploadBytes) }}
          </template>
          <template #column:downloadBytes="{ rowData }">
            {{ formatBytes(rowData.downloadBytes) }}
          </template>
          <template #empty>
            <EmptyState
              :icon="ChartPie"
              title="暂无用户流量"
              description="入站下还没有用户，或用户尚未产生流量；创建用户后此处按用户累计上传与下载"
              cta-text="前往入站页"
              cta-plain
              @cta="router.push({ name: 'inbounds' })"
            />
          </template>
        </u-table>
      </u-card-content>
    </u-card>
  </div>
</template>

<style scoped>
.dash-card {
  margin-bottom: 16px;
}

.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
  gap: var(--u-gap-large);
}

.kernel-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  font-size: var(--u-font-size-main-default);
}

/* 主指标（实时速率）占满一行两列并抬面放大 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--u-gap-default);
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: var(--u-gap-small);
  padding: 12px 14px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-middle);
}

.stat-item--primary {
  background: var(--u-bg-color-top);
}

.stat-label {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-main);
}

.stat-value {
  font-size: var(--u-font-size-title-large);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.stat-value.is-up {
  color: var(--u-color-success);
}

.stat-value.is-down {
  color: var(--u-color-primary);
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
}

.hint {
  margin: 10px 0 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
