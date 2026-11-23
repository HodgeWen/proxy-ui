<!-- 出站管理：页顶出站说明卡、direct 出站说明与添加、出站列表增删改启停（编辑弹窗拆至 OutboundEditorDialog）；改动保存即热应用内核 -->
<script setup lang="ts">
import { createOutbound, deleteOutbound, listOutbounds, setOutboundEnabled } from '../api/outbound'
import type { Outbound } from '../api/outbound'
import {
  UAction,
  UActionGroup,
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  USwitch,
  UTable,
  UTag,
  defineTableColumns,
  message,
  vLoading,
} from '@veltra/desktop'
import { computed, onMounted, shallowRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import OutboundEditorDialog from '../components/OutboundEditorDialog.vue'
import { useEnabledOverride } from '../composables/useEnabledOverride'
import { confirmAction } from '../confirm-action'

const outbounds = shallowRef<Outbound[]>([])
const loading = shallowRef(true)
const addingDirect = shallowRef(false)
const deletingId = shallowRef(0)
const formVisible = shallowRef(false)
const editing = shallowRef<Outbound | null>(null)

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'type', name: '类型', width: 130, align: 'center' },
  { key: 'endpoint', name: '服务器', minWidth: 200 },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 130, align: 'center' },
])

const hasDirect = computed(() => outbounds.value.some((o) => o.type === 'direct'))

async function refresh(): Promise<void> {
  loading.value = true
  try {
    outbounds.value = await listOutbounds()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  formVisible.value = true
}

function openEdit(row: Outbound): void {
  editing.value = row
  formVisible.value = true
}

async function addDirect(): Promise<void> {
  addingDirect.value = true
  try {
    await createOutbound({ name: 'direct', type: 'direct', config: {} })
    message.success('direct 出站已添加')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    addingDirect.value = false
  }
}

const { rowEnabled, togglingId, toggle: handleToggle } = useEnabledOverride(
  outbounds,
  async (row, enabled) => {
    await setOutboundEnabled(row.id, enabled)
    outbounds.value = outbounds.value.map((o) => (o.id === row.id ? { ...o, enabled } : o))
  },
)

async function handleDelete(row: Outbound): Promise<void> {
  const ok = await confirmAction(
    '删除出站',
    `确认删除出站「${row.name}」吗？被路由规则、策略组成员、兜底出站或 DNS 服务器 detour 引用时无法删除，会提示先改掉这些引用。`,
    true,
    '删除',
  )
  if (!ok) return
  deletingId.value = row.id
  try {
    await deleteOutbound(row.id)
    message.success('出站已删除')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    deletingId.value = 0
  }
}

function endpointOf(row: Outbound): string {
  const cfg = (row.config ?? {}) as Record<string, unknown>
  if (row.type === 'direct') return '内置直连出站'
  if (typeof cfg.server === 'string') return `${cfg.server}${typeof cfg.server_port === 'number' ? ':' + cfg.server_port : ''}`
  return '—'
}

onMounted(refresh)
</script>

<template>
  <!-- 说明卡 size=small 紧凑呈现，不与 direct 缺失提醒、主卡抢视觉 -->
  <u-card size="small" class="intro-card">
    <u-card-header>出站是什么</u-card-header>
    <u-card-content>
      <div>出站是流量的出口、路由规则的最终去向：规则命中后从对应出站转发，未命中的流量也走兜底出站，因此这里至少要有一个可用出口。</div>
      <ul class="intro-list">
        <li><strong>direct</strong>：内置直连出口，不走代理，通常作为兜底默认保留。</li>
        <li><strong>自定义出站</strong>：服务器作为客户端连接上游代理服务器（链式转发），让本服务器的出海流量经上游代理出去；仅在服务端生效。</li>
      </ul>
      <div class="intro-note">客户端订阅里的节点来自「入站」页，与本页的自定义出站无关。</div>
    </u-card-content>
  </u-card>

  <!-- 标题不可交互故不走 UAlert 彩色标题，正文加粗引导；添加动作为次级 plain，主操作留给「新建自定义出站」 -->
  <u-alert v-if="!hasDirect && !loading" class="direct-alert" type="info">
    <div class="direct-row">
      <span><strong>direct 出站：</strong>sing-box 内置直连出站，作为未命中规则的默认出口，通常保持添加即可。</span>
      <u-button type="primary" plain :loading="addingDirect" @click="addDirect">添加 direct 出站</u-button>
    </div>
  </u-alert>

  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>出站管理</span>
        <div class="page-ops">
          <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
          <u-button v-if="outbounds.length" type="primary" @click="openCreate">新建自定义出站</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <div v-loading="loading" class="table-wrap">
        <u-table :columns="columns" :data="outbounds" row-key="id" stripe>
          <template #empty>
            <EmptyState
              v-if="!loading"
              title="暂无出站"
              description="可先添加 direct 内置直连出站作为兜底，再按需新建自定义出站"
              cta-text="新建自定义出站"
              @cta="openCreate"
            />
          </template>
          <template #column:type="{ rowData }">
            <u-tag :type="rowData.type === 'direct' ? 'success' : 'info'">{{ rowData.type }}</u-tag>
          </template>
          <template #column:endpoint="{ rowData }">
            {{ endpointOf(rowData) }}
          </template>
          <template #column:enabled="{ rowData }">
            <u-switch
              size="small"
              :model-value="rowEnabled(rowData)"
              :disabled="togglingId === rowData.id"
              @update:model-value="(v: boolean) => handleToggle(rowData, v)"
            />
          </template>
          <template #column:actions="{ rowData }">
            <u-action-group :loading="deletingId === rowData.id">
              <!-- direct 出站不可编辑，只能删除 -->
              <u-action v-if="rowData.type !== 'direct'" @run="openEdit(rowData)">编辑</u-action>
              <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
            </u-action-group>
          </template>
        </u-table>
      </div>
    </u-card-content>
  </u-card>

  <OutboundEditorDialog v-model:visible="formVisible" :editing="editing" @saved="refresh" />
</template>

<style scoped>
/* intro-card / intro-list / intro-note / page-header / page-ops 为 App.vue 全局类 */

.direct-alert {
  margin-bottom: 16px;
}

.direct-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
</style>
