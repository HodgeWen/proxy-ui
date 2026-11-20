<!-- 仪表盘：内核状态与启停、实时速率概览、按用户累计流量与快捷操作 -->
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
import { ChartPie } from '@veltra/icons/normal'
import { onBeforeUnmount, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { applyKernelConfig, fetchKernelStatus, kernelAction } from '../api/kernel'
import type { KernelStatus } from '../api/kernel'
import { formatBytes, getSubscription, subscriptionLink } from '../api/subscription'
import { fetchRealtimeStats, fetchUserTraffic } from '../api/stats'
import type { RealtimeStats, UserTrafficRow } from '../api/stats'
import EmptyState from '../components/EmptyState.vue'
import { protocolDef } from '../inbound-schema'

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

const applying = shallowRef(false)

/** 应用配置：重新聚合入站/用户/出站并生成内核配置，内核运行中则按新配置重建 */
async function handleApplyConfig() {
  applying.value = true
  try {
    const { applied, status: next } = await applyKernelConfig()
    status.value = next
    message.success(applied ? '配置已应用并重启内核' : '配置已生成并通过校验，内核未运行时不重启')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    applying.value = false
  }
}

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
      <u-card-header>
        <div class="card-header-row">
          <span>内核状态</span>
          <div class="quick-actions">
            <u-button :loading="applying" @click="handleApplyConfig">应用配置</u-button>
            <u-button @click="copySubscriptionLink">复制订阅链接</u-button>
            <u-button @click="router.push({ name: 'subscription' })">前往订阅页</u-button>
          </div>
        </div>
      </u-card-header>
      <u-card-content>
        <div class="kernel-meta">
          <u-tag :type="status?.running ? 'success' : 'info'">
            {{ status?.running ? '运行中' : '已停止' }}
          </u-tag>
          <u-tag type="info">内置内核</u-tag>
          <span>版本：<strong>{{ status?.version || '内置' }}</strong></span>
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
        <p v-if="status && !status.running" class="hint">内核未运行，启动后展示实时速率。</p>
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
        <u-table :columns="trafficColumns" :data="trafficRows" row-key="tag" border stripe>
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
              description="入站下还没有用户，或用户尚未产生流量；创建用户并应用配置后，此处按用户累计上传与下载"
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
