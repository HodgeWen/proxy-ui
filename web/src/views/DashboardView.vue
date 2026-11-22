<!-- 仪表盘：内核只读状态（运行状态 / 版本 / 运行时长）、实时速率概览、按用户累计流量 -->
<script setup lang="ts">
import {
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UTable,
  UTag,
  defineTableColumns,
  vLoading,
} from '@veltra/desktop'
import { ChartPie } from '@veltra/icons/normal'
import { onBeforeUnmount, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { fetchKernelStatus } from '../api/kernel'
import type { KernelStatus } from '../api/kernel'
import { fetchRealtimeStats, fetchUserTraffic } from '../api/stats'
import type { RealtimeStats, UserTrafficRow } from '../api/stats'
import EmptyState from '../components/EmptyState.vue'
import { protocolDef } from '../inbound-schema'
import { formatBytes, formatUptime } from '../utils/format'

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
const trafficLoading = shallowRef(true)
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
  trafficLoading.value = true
  try {
    trafficRows.value = await fetchUserTraffic()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    trafficLoading.value = false
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
</script>

<template>
  <div class="dashboard">
    <!-- pendingChanges：热应用失败后内核一直跑回滚配置，必须常驻提示（而非只在保存时弹一次） -->
    <u-alert v-if="status?.pendingChanges" type="warning" class="dash-alert">
      有配置未生效：内核热应用失败，正在回滚的旧配置上运行，重新保存相关配置可重试。
    </u-alert>

    <u-card class="dash-card">
      <u-card-header>内核状态</u-card-header>
      <u-card-content>
        <div class="kernel-meta">
          <u-tag :type="status ? (status.running ? 'success' : 'info') : 'info'">
            {{ status ? (status.running ? '运行中' : '已停止') : '加载中…' }}
          </u-tag>
          <span>版本：<strong>{{ status?.version || '-' }}</strong></span>
          <span>运行时长：{{ formatUptime(status?.uptimeSeconds) }}</span>
          <!-- 失败原因合入行内单行省略展示，避免 2s 轮询状态切换时挤动下方卡片 -->
          <span
            v-if="status && !status.running && status.error"
            class="kernel-error"
            :title="`启动失败：${status.error}。修正配置后重新保存任一配置，内核会自动重建。`"
          >
            启动失败：{{ status.error }}（修正配置后重新保存可重建）
          </span>
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
          <u-button :loading="trafficLoading" @click="refreshTraffic">刷新</u-button>
        </div>
      </u-card-header>
      <u-card-content>
        <div v-loading="trafficLoading && !trafficRows.length" class="table-wrap">
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
                v-if="!trafficLoading"
                :icon="ChartPie"
                title="暂无用户流量"
                description="入站下还没有用户，或用户尚未产生流量；创建用户后此处按用户累计上传与下载"
                cta-text="前往入站页"
                cta-plain
                @cta="router.push({ name: 'inbounds' })"
              />
            </template>
          </u-table>
        </div>
      </u-card-content>
    </u-card>
  </div>
</template>

<style scoped>
.dash-card {
  margin-bottom: 16px;
}

.dash-alert {
  margin-bottom: 16px;
}

/* 布局取全局 card-header-row，这里只补本页的宽松间距 */
.card-header-row {
  gap: var(--u-gap-large);
}

.kernel-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  font-size: var(--u-font-size-main-default);
}

.kernel-meta > :not(.kernel-error) {
  flex-shrink: 0;
  white-space: nowrap;
}

/* 内核停止原因：行内单行省略呈现，不撑高卡片 */
.kernel-error {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--u-color-warning);
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
</style>
