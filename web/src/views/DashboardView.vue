<!-- 仪表盘：内核状态与启停、实时速率/内存/连接概览、按入站累计流量、当前连接列表与快捷操作 -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UTable,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import { Download } from '@veltra/icons/normal'
import { onBeforeUnmount, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { fetchKernelStatus, kernelAction } from '../api/kernel'
import EmptyState from '../components/EmptyState.vue'
import type { KernelStatus } from '../api/kernel'
import { getSubscription, subscriptionLink } from '../api/subscription'
import { fetchConnections, fetchInboundTraffic, fetchRealtimeStats } from '../api/stats'
import type { InboundTrafficRow, RealtimeStats, StatsConnection } from '../api/stats'

const router = useRouter()

// ---------- 内核状态 ----------

const status = shallowRef<KernelStatus | null>(null)
const acting = shallowRef<'start' | 'stop' | 'restart' | ''>('')

async function refreshStatus() {
  try {
    status.value = await fetchKernelStatus()
  } catch {
    // 静默：下轮轮询重试
  }
}

async function handleKernelAction(action: 'start' | 'stop' | 'restart') {
  acting.value = action
  try {
    await kernelAction(action)
    message.success(action === 'start' ? '内核已启动' : action === 'stop' ? '内核已停止' : '内核已重启')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    acting.value = ''
    await refreshStatus()
  }
}

// ---------- 快捷操作 ----------

async function copySubscriptionLink() {
  let token: string | undefined
  try {
    const info = await getSubscription()
    token = info.tokens.find((t) => !t.revoked)?.token
  } catch {
    return // 错误已由请求封装统一提示
  }
  if (!token) {
    message.warn('暂无可用订阅令牌，请先在订阅页生成')
    return
  }
  try {
    await clipboard.copy(subscriptionLink(token))
    message.success('订阅链接已复制')
  } catch {
    message.error('复制失败，请手动复制')
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

// ---------- 按入站累计流量 ----------

const trafficRows = shallowRef<InboundTrafficRow[]>([])
const trafficColumns = defineTableColumns([
  { key: 'name', name: '入站', minWidth: 140 },
  { key: 'protocol', name: '协议', width: 120 },
  { key: 'enabled', name: '状态', width: 80, align: 'center' },
  { key: 'uploadBytes', name: '累计上传', width: 130, align: 'right' },
  { key: 'downloadBytes', name: '累计下载', width: 130, align: 'right' },
])

async function refreshTraffic() {
  try {
    trafficRows.value = await fetchInboundTraffic()
  } catch {
    // 错误已由请求封装统一提示
  }
}

// ---------- 当前连接 ----------

const connections = shallowRef<StatsConnection[]>([])
const connColumns = defineTableColumns([
  { key: 'inbound', name: '入站', width: 130 },
  { key: 'network', name: '网络', width: 70, align: 'center' },
  { key: 'source', name: '客户端', width: 170 },
  { key: 'destination', name: '目标', minWidth: 200 },
  { key: 'rule', name: '规则', width: 150 },
  { key: 'chain', name: '出站链', width: 140 },
  { key: 'upload', name: '上传', width: 100, align: 'right' },
  { key: 'download', name: '下载', width: 100, align: 'right' },
  { key: 'start', name: '建立时间', width: 90, align: 'center' },
])

async function refreshConnections() {
  try {
    connections.value = (await fetchConnections()).connections
  } catch {
    // 静默：下轮轮询重试
  }
}

// ---------- 轮询 ----------

const timers: ReturnType<typeof setInterval>[] = []

onMounted(() => {
  void refreshStatus()
  void refreshRealtime()
  void refreshTraffic()
  void refreshConnections()
  timers.push(
    setInterval(() => void refreshStatus(), 2000),
    setInterval(() => void refreshRealtime(), 1000),
    setInterval(() => void refreshConnections(), 2000),
    setInterval(() => void refreshTraffic(), 15000),
  )
})

onBeforeUnmount(() => timers.forEach(clearInterval))

// ---------- 格式化 ----------

function formatBytes(n?: number): string {
  if (!n || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = n
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${unit === 0 ? value : value.toFixed(2)} ${units[unit]}`
}

function formatRate(n?: number): string {
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
      <u-card-header>
        <div class="card-header-row">
          <span>内核状态</span>
          <div class="quick-actions">
            <u-button :disabled="!status?.running" :loading="acting === 'restart'" @click="handleKernelAction('restart')">
              重启内核
            </u-button>
            <u-button @click="copySubscriptionLink">复制订阅链接</u-button>
            <u-button @click="router.push({ name: 'subscription' })">前往订阅页</u-button>
          </div>
        </div>
      </u-card-header>
      <u-card-content>
        <EmptyState
          v-if="status && !status.installed"
          :icon="Download"
          title="尚未安装 sing-box 内核"
          description="内核未安装时面板仅能管理配置，前往设置页下载安装后即可启动"
          cta-text="前往设置安装"
          @cta="router.push('/settings')"
        />
        <template v-else>
          <div class="kernel-meta">
            <u-tag :type="status?.running ? 'success' : 'info'">
              {{ status?.running ? '运行中' : '已停止' }}
            </u-tag>
            <span>版本：<strong>{{ status?.version || '未知' }}</strong></span>
            <span v-if="status?.running">PID：{{ status.pid }}</span>
            <span v-if="status?.running">运行时长：{{ formatUptime(status.uptimeSeconds) }}</span>
            <div class="kernel-actions">
              <u-button
                type="primary"
                :disabled="status?.running"
                :loading="acting === 'start'"
                @click="handleKernelAction('start')"
              >
                启动
              </u-button>
              <u-button
                type="error"
                :disabled="!status?.running"
                :loading="acting === 'stop'"
                @click="handleKernelAction('stop')"
              >
                停止
              </u-button>
              <u-button
                :disabled="!status?.running"
                :loading="acting === 'restart'"
                @click="handleKernelAction('restart')"
              >
                重启
              </u-button>
            </div>
          </div>
        </template>
      </u-card-content>
    </u-card>

    <u-card class="dash-card">
      <u-card-header>流量概览</u-card-header>
      <u-card-content>
        <div class="stat-grid">
          <div class="stat-item">
            <span class="stat-label">上传速率</span>
            <span class="stat-value is-up">{{ formatRate(realtime?.up) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">下载速率</span>
            <span class="stat-value is-down">{{ formatRate(realtime?.down) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">内核内存</span>
            <span class="stat-value">{{ formatBytes(realtime?.memory) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">活动连接</span>
            <span class="stat-value">{{ realtime?.activeConnections ?? 0 }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">累计上传</span>
            <span class="stat-value is-up">{{ formatBytes(realtime?.uploadTotal) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">累计下载</span>
            <span class="stat-value is-down">{{ formatBytes(realtime?.downloadTotal) }}</span>
          </div>
        </div>
        <p v-if="status?.installed && !status.running" class="hint">内核未运行，启动后展示实时速率与连接。</p>
      </u-card-content>
    </u-card>

    <u-card class="dash-card">
      <u-card-header>
        <div class="card-header-row">
          <span>按入站累计流量</span>
          <u-button @click="refreshTraffic">刷新</u-button>
        </div>
      </u-card-header>
      <u-card-content>
        <u-table :columns="trafficColumns" :data="trafficRows" row-key="inboundId" border>
          <template #column:enabled="{ rowData }">
            <u-tag :type="rowData.enabled ? 'success' : 'info'">{{ rowData.enabled ? '启用' : '停用' }}</u-tag>
          </template>
          <template #column:uploadBytes="{ rowData }">
            {{ formatBytes(rowData.uploadBytes) }}
          </template>
          <template #column:downloadBytes="{ rowData }">
            {{ formatBytes(rowData.downloadBytes) }}
          </template>
        </u-table>
      </u-card-content>
    </u-card>

    <u-card class="dash-card">
      <u-card-header>
        <div class="card-header-row">
          <span>当前连接</span>
          <u-button @click="refreshConnections">刷新</u-button>
        </div>
      </u-card-header>
      <u-card-content>
        <u-table :columns="connColumns" :data="connections" row-key="id" border>
          <template #column:network="{ rowData }">
            <u-tag :type="rowData.network === 'tcp' ? 'primary' : 'warning'">{{ rowData.network }}</u-tag>
          </template>
          <template #column:chain="{ rowData }">
            {{ (rowData.chain ?? []).join(' → ') || '-' }}
          </template>
          <template #column:upload="{ rowData }">
            {{ formatBytes(rowData.upload) }}
          </template>
          <template #column:download="{ rowData }">
            {{ formatBytes(rowData.download) }}
          </template>
          <template #column:start="{ rowData }">
            {{ new Date(rowData.start).toLocaleTimeString() }}
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
  gap: 12px;
}

.quick-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.kernel-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  font-size: 14px;
}

.kernel-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-middle);
}

.stat-label {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

.stat-value {
  font-size: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.stat-value.is-up {
  color: var(--u-color-success);
}

.stat-value.is-down {
  color: var(--u-color-primary);
}

.hint {
  margin: 10px 0 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
