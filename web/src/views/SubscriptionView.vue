<!-- 订阅用户：一位使用者（或一台设备）一个订阅链接 + 独立配额/到期。
     勾选入站 = 在该入站里为这个人建一个用户身份（共用入站端口、各自凭据），流量按用户归属，
     配额用尽 / 到期 / 停用只暂停这个人的节点。链接与二维码收进弹窗，页面只留列表与配额管理 -->
<script setup lang="ts">
import {
  UAction,
  UActionGroup,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UTable,
  UTag,
  defineTableColumns,
  message,
  vLoading,
} from '@veltra/desktop'
import { UserGroup } from '@veltra/icons/normal'
import { onMounted, shallowRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import SubscriptionEditorDialog from '../components/SubscriptionEditorDialog.vue'
import SubscriptionLinksDialog from '../components/SubscriptionLinksDialog.vue'
import SubscriptionPreviewDialog from '../components/SubscriptionPreviewDialog.vue'
import {
  deleteSubscriptionToken,
  getSubscription,
  resetSubscriptionToken,
  resetSubscriptionTraffic,
  revokeSubscriptionToken,
} from '../api/subscription'
import type { SubscriptionToken } from '../api/subscription'
import { confirmAction } from '../confirm-action'
import { protocolDef } from '../inbound-schema'
import { formatBytes } from '../utils/format'

// ---------- 列表 ----------

const users = shallowRef<SubscriptionToken[]>([])
const loading = shallowRef(false)
const actingID = shallowRef(0)

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'status', name: '状态', width: 96, align: 'center' },
  { key: 'usage', name: '用量', minWidth: 200 },
  { key: 'nodes', name: '节点', minWidth: 200 },
  { key: 'expiresAt', name: '到期时间', minWidth: 150 },
  { key: 'lastUsedAt', name: '最近使用', minWidth: 150 },
  // 订阅链接与配置预览是订阅用户专属动作，单独成列（单行排布）；通用操作列固定在右侧
  { key: 'links', name: '订阅', width: 124, align: 'center' },
  { key: 'actions', name: '操作', width: 160, align: 'center', fixed: 'right' },
])

async function refresh(): Promise<void> {
  loading.value = true
  try {
    users.value = await getSubscription()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

/** 状态标签：停用优先于到期，到期优先于超额 */
function statusOf(user: SubscriptionToken): { label: string; type: 'info' | 'warning' | 'danger' | 'success' } {
  if (user.revoked) return { label: '已停用', type: 'info' }
  if (user.expired) return { label: '已到期', type: 'warning' }
  if (user.overQuota) return { label: '流量用尽', type: 'danger' }
  return { label: '启用', type: 'success' }
}

/** 配额使用百分比（配额为 0 表示不限） */
function usagePercent(user: SubscriptionToken): number {
  if (!user.trafficLimitBytes) return 0
  return Math.min(100, Math.round((user.usedBytes / user.trafficLimitBytes) * 100))
}

/** 节点概况：数量 + 去重后的协议名 */
function nodeSummary(user: SubscriptionToken): string {
  const protocols = [...new Set(user.nodes.map((n) => protocolShortName(n.protocol)))]
  return `${user.nodes.length} 个 · ${protocols.join('、')}`
}

function protocolName(protocol: string): string {
  return protocolDef(protocol)?.name ?? protocol
}

/** 协议族短名：节点列放不下「VLESS + Reality」「TUIC v5」「Shadowsocks-2022」，只留协议族 */
function protocolShortName(protocol: string): string {
  return protocolName(protocol)
    .replace(/-\d[\w.]*$/, '')
    .split(' ')[0]
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString()
}

// ---------- 订阅链接弹窗 ----------

const linksVisible = shallowRef(false)
const linksUser = shallowRef<SubscriptionToken | null>(null)

function openLinks(user: SubscriptionToken): void {
  linksUser.value = user
  linksVisible.value = true
}

// ---------- 配置预览弹窗 ----------

const previewVisible = shallowRef(false)
const previewUser = shallowRef<SubscriptionToken | null>(null)

function openPreview(user: SubscriptionToken): void {
  previewUser.value = user
  previewVisible.value = true
}

// ---------- 新建 / 编辑弹窗 ----------

const editorVisible = shallowRef(false)
const editorUser = shallowRef<SubscriptionToken | null>(null)

function openEditor(user: SubscriptionToken | null): void {
  editorUser.value = user
  editorVisible.value = true
}

/** 弹窗内完成保存与关闭；保存成功后这里刷新列表并提示（与原流程同序） */
async function handleEditorSaved(editing: boolean): Promise<void> {
  await refresh()
  message.success(editing ? '订阅用户已更新' : '订阅用户已创建，可在列表中查看其订阅链接')
}

// ---------- 列表操作 ----------

async function handleToggle(user: SubscriptionToken): Promise<void> {
  actingID.value = user.id
  try {
    await revokeSubscriptionToken(user.id, !user.revoked)
    await refresh()
    message.success(user.revoked ? '订阅用户已启用' : '订阅用户已停用，其客户端立即拉不到订阅')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

/** 重置链接会换令牌，旧链接立即失效，必须说清后果 */
async function handleReset(user: SubscriptionToken): Promise<void> {
  const ok = await confirmAction(
    '重置订阅链接',
    `重置将生成新链接，「${user.name || '未命名'}」的旧链接立即失效，需在客户端重新导入，确认继续吗？`,
  )
  if (!ok) return
  actingID.value = user.id
  try {
    await resetSubscriptionToken(user.id)
    await refresh()
    message.success('订阅链接已重置，请在客户端更新订阅')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleResetTraffic(user: SubscriptionToken): Promise<void> {
  actingID.value = user.id
  try {
    await resetSubscriptionTraffic(user.id)
    await refresh()
    message.success('该用户的流量统计已清零，因超额暂停的节点随之恢复')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

/** 删除会连带节点与累计流量，必须说清后果 */
async function handleDelete(user: SubscriptionToken): Promise<void> {
  const ok = await confirmAction(
    '删除订阅用户',
    `删除后「${user.name || '未命名'}」的订阅链接立即失效，其在各入站里的 ${user.nodes.length} 个用户身份与累计流量一并删除且不可恢复，确认删除吗？`,
    true,
  )
  if (!ok) return
  actingID.value = user.id
  try {
    await deleteSubscriptionToken(user.id)
    await refresh()
    message.success('订阅用户已删除')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

onMounted(refresh)
</script>

<template>
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>订阅用户</span>
        <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
        <u-button v-if="users.length" type="primary" @click="openEditor(null)">新建订阅用户</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <p class="card-desc">
        一位使用者（或一台设备）就是一个「订阅用户」，只有一条订阅链接，配额与到期时间都记在这条链接上。
        勾选入站即在该入站里为这个人创建一个用户身份：<b>同一入站下的节点共用该入站的端口，但凭据各自独立</b>。
        流量按用户归属统计，所以配额用尽、到期或停用时只暂停这个人的节点，同端口的其他人不受影响。
      </p>
      <div v-loading="loading" class="table-wrap">
        <u-table :columns="columns" :data="users" row-key="id">
          <template #empty>
            <EmptyState
              v-if="!loading"
              :icon="UserGroup"
              title="尚无订阅用户"
              description="为每位使用者或每台设备创建一个订阅用户，勾选入站后即在该入站里得到独立凭据的节点与订阅链接"
              cta-text="新建订阅用户"
              @cta="openEditor(null)"
            />
          </template>
          <template #column:name="{ rowData }">
            <span :class="{ 'cell-dim': rowData.revoked }">{{ rowData.name || '未命名' }}</span>
          </template>
          <template #column:status="{ rowData }">
            <u-tag :type="statusOf(rowData).type">{{ statusOf(rowData).label }}</u-tag>
          </template>
          <template #column:usage="{ rowData }">
            <div class="usage">
              <div class="usage-head">
                <span>
                  {{ formatBytes(rowData.usedBytes) }} /
                  {{ rowData.trafficLimitBytes ? formatBytes(rowData.trafficLimitBytes) : '不限' }}
                </span>
                <span v-if="rowData.trafficLimitBytes" class="cell-dim">{{ usagePercent(rowData) }}%</span>
              </div>
              <div v-if="rowData.trafficLimitBytes" class="usage-bar">
                <div
                  class="usage-bar-fill"
                  :class="{ 'is-over': rowData.overQuota }"
                  :style="{ width: `${usagePercent(rowData)}%` }"
                ></div>
              </div>
            </div>
          </template>
          <template #column:nodes="{ rowData }">
            <span v-if="rowData.nodes.length">{{ nodeSummary(rowData) }}</span>
            <span v-else class="cell-dim">未勾选入站</span>
          </template>
          <template #column:expiresAt="{ rowData }">
            {{ rowData.expiresAt ? formatTime(rowData.expiresAt) : '永久' }}
          </template>
          <template #column:lastUsedAt="{ rowData }">
            <span :class="{ 'cell-dim': !rowData.lastUsedAt }">
              {{ rowData.lastUsedAt ? formatTime(rowData.lastUsedAt) : '从未使用' }}
            </span>
          </template>
          <template #column:links="{ rowData }">
            <div class="links-cell">
              <u-button text type="primary" size="small" @click="openLinks(rowData)">查看订阅</u-button>
              <u-button text type="primary" size="small" @click="openPreview(rowData)">预览</u-button>
            </div>
          </template>
          <template #column:actions="{ rowData }">
            <!-- 5 个动作超出 max=3：编辑/启停内联，其余收进「更多」下拉 -->
            <u-action-group :max="3" :loading="actingID === rowData.id">
              <u-action @run="openEditor(rowData)">编辑</u-action>
              <!-- 停用会让客户端立刻断供：气泡确认即可；清零流量同样在气泡里确认 -->
              <u-action :need-confirm="!rowData.revoked" @run="handleToggle(rowData)">
                {{ rowData.revoked ? '启用' : '停用' }}
              </u-action>
              <!-- 重置链接会换令牌、删除会连节点与流量一起删，走弹窗说清后果 -->
              <u-action @run="handleReset(rowData)">重置链接</u-action>
              <u-action need-confirm @run="handleResetTraffic(rowData)">清零流量</u-action>
              <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
            </u-action-group>
          </template>
        </u-table>
      </div>
    </u-card-content>
  </u-card>

  <SubscriptionEditorDialog v-model:visible="editorVisible" :token="editorUser" @saved="handleEditorSaved" />

  <SubscriptionLinksDialog v-model:visible="linksVisible" :token="linksUser" />
  <SubscriptionPreviewDialog v-model:visible="previewVisible" :token="previewUser" />
</template>

<style scoped>
/* page-header / cell-dim 为 App.vue 全局类 */

.table-wrap {
  position: relative;
  min-height: 120px;
}

.card-desc {
  margin: 0 0 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.links-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
  white-space: nowrap;
}

.usage {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.usage-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: var(--u-font-size-main-small);
}

.usage-bar {
  height: 4px;
  border-radius: 2px;
  background-color: var(--u-bg-color-hover);
  overflow: hidden;
}

.usage-bar-fill {
  height: 100%;
  background-color: var(--u-color-primary);
  transition: width var(--u-transition-fast) var(--u-transition-ease-out);
}

.usage-bar-fill.is-over {
  background-color: var(--u-color-danger);
}
</style>
