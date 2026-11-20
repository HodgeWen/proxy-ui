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
import { ChartPie, CloudDownload, Link } from '@veltra/icons/normal'
import { computed, onBeforeUnmount, onMounted, shallowRef } from 'vue'
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

// 「重启内核」禁用时的原因说明（disabled 条件保持 !status?.running 不变）
const restartDisabledReason = computed(() => {
  if (!status.value) return ''
  if (!status.value.installed) return '内核未安装，请先前往设置页安装'
  if (!status.value.running) return '内核未运行，请先启动内核'
  return ''
})

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
    const users = await getSubscription()
    token = users.find((u) => !u.revoked)?.token
  } catch {
    return // 错误已由请求封装统一提示
  }
  if (!token) {
    message.warn('暂无启用中的订阅用户，请先在订阅页创建')
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
// 入站/协议两列同为弹性列分摊剩余宽度，避免空表时首列独占整行失衡
const trafficColumns = defineTableColumns([
  { key: 'name', name: '入站', minWidth: 140 },
  { key: 'protocol', name: '协议', minWidth: 100 },
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
// 9 列列宽收敛（固定列合计约 800px）：1440px 视口内完整放下，更窄视口由 UTable 内置横向滚动承接
const connColumns = defineTableColumns([
  { key: 'inbound', name: '入站', width: 110 },
  { key: 'network', name: '网络', width: 64, align: 'center' },
  { key: 'source', name: '客户端', width: 130 },
  { key: 'destination', name: '目标', minWidth: 180 },
  { key: 'rule', name: '规则', width: 110 },
  { key: 'chain', name: '出站链', width: 110 },
  { key: 'upload', name: '上传', width: 88, align: 'right' },
  { key: 'download', name: '下载', width: 88, align: 'right' },
  { key: 'start', name: '建立时间', width: 100, align: 'center' },
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

// 内核未安装时统计接口无意义，概览数值统一显示 N/A 而非「0 B」
const statsUnavailable = computed(() => status.value !== null && !status.value.installed)

function statValue(v: number | undefined, fmt: (n: number) => string): string {
  return v === undefined || statsUnavailable.value ? 'N/A' : fmt(v)
}

function formatBytes(n?: number): string {
  if (n === undefined) return 'N/A'
  if (n <= 0) return '0 B'
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
  return n === undefined ? 'N/A' : `${formatBytes(n)}/s`
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
            <u-button
              :disabled="!status?.running"
              :title="restartDisabledReason"
              :loading="acting === 'restart'"
              @click="handleKernelAction('restart')"
            >
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
          :icon="CloudDownload"
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
                type="danger"
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
          <div class="stat-item stat-item--primary">
            <span class="stat-label">上传速率</span>
            <span class="stat-value is-up">{{ statValue(realtime?.up, formatRate) }}</span>
          </div>
          <div class="stat-item stat-item--primary">
            <span class="stat-label">下载速率</span>
            <span class="stat-value is-down">{{ statValue(realtime?.down, formatRate) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">活动连接</span>
            <span class="stat-value">{{ statValue(realtime?.activeConnections, String) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">内核内存</span>
            <span class="stat-value">{{ statValue(realtime?.memory, formatBytes) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">累计上传</span>
            <span class="stat-value is-up">{{ statValue(realtime?.uploadTotal, formatBytes) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">累计下载</span>
            <span class="stat-value is-down">{{ statValue(realtime?.downloadTotal, formatBytes) }}</span>
          </div>
        </div>
        <p v-if="status && !status.installed" class="hint">内核未安装，统计值暂不可用；安装后此处展示实时速率与连接。</p>
        <p v-else-if="status?.installed && !status.running" class="hint">内核未运行，启动后展示实时速率与连接。</p>
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
        <u-table :columns="trafficColumns" :data="trafficRows" row-key="inboundId" border stripe>
          <template #column:enabled="{ rowData }">
            <u-tag :type="rowData.enabled ? 'success' : 'info'">{{ rowData.enabled ? '启用' : '停用' }}</u-tag>
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
              title="暂无入站流量"
              description="尚未创建入站；创建入站并产生流量后，此处按入站累计上传与下载"
              cta-text="前往入站页"
              cta-plain
              @cta="router.push({ name: 'inbounds' })"
            />
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
        <u-table
          :columns="connColumns"
          :data="connections"
          row-key="id"
          border
          stripe
          text-ellipsis
        >
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
          <template #empty>
            <EmptyState
              :icon="Link"
              title="暂无活动连接"
              description="内核运行产生流量后，此处实时展示连接明细"
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

.quick-actions {
  display: flex;
  gap: var(--u-gap-default);
  flex-wrap: wrap;
}

.kernel-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  font-size: var(--u-font-size-main-default);
}

.kernel-actions {
  display: flex;
  gap: var(--u-gap-default);
  margin-left: auto;
}

/* 主指标（实时速率）与次指标分级行：主指标占两列、抬面放大，次指标单列缩小 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
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
  grid-column: span 2;
  background: var(--u-bg-color-top);
}

.stat-label {
  font-size: var(--u-font-size-assist-default);
  color: var(--u-text-color-second);
}

.stat-item--primary .stat-label {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-main);
}

.stat-value {
  font-size: var(--u-font-size-main-large);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.stat-item--primary .stat-value {
  font-size: var(--u-font-size-title-large);
}

/* 960px 即 --u-breakpoint-md（媒体查询内不能引用 CSS 变量，写等值 px） */
@media (max-width: 960px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .stat-item--primary {
    grid-column: span 2;
  }
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
