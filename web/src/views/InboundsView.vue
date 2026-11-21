<!-- 入站管理：协议分组新建入口（推荐组排前）与真实监听入站列表（端口/用户数/警示）；
     「证书管理」跳转独立证书页（/certs）。
     一个入站 = 一个监听端口；用户统一在「订阅用户」页管理（勾选入站即在该端口上生成独立凭据的节点），
     这里只展示各入站下的用户数。所有写操作落库后由后端直接热应用内核，因此页面没有「应用配置」入口。 -->
<script setup lang="ts">
import { deleteInbound, listInbounds, setInboundEnabled } from '../api/inbound'
import type { Inbound } from '../api/inbound'
import InboundForm from '../components/InboundForm.vue'
import {
  UAction,
  UActionGroup,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  USwitch,
  UTable,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import { onMounted, shallowRef, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import EmptyState from '../components/EmptyState.vue'
import { useEnabledOverride } from '../composables/useEnabledOverride'
import { confirmAction } from '../confirm-action'
import { dialogStyle } from '../dialog-style'
import { PROTOCOL_DEFS, listenLabel, protocolDef } from '../inbound-schema'

const recommended = PROTOCOL_DEFS.filter((p) => p.recommended)
const classic = PROTOCOL_DEFS.filter((p) => !p.recommended)

const router = useRouter()

const inbounds = shallowRef<Inbound[]>([])
const loading = shallowRef(false)
const pickerVisible = shallowRef(false)

const inboundForm = useTemplateRef<InstanceType<typeof InboundForm>>('inboundForm')

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 160 },
  { key: 'protocol', name: '协议', width: 150 },
  { key: 'listen', name: '监听', width: 170 },
  { key: 'users', name: '用户数', width: 90, align: 'center' },
  { key: 'warnings', name: '警示', width: 90, align: 'center' },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  // 通用操作列固定在最右侧（两项操作单行内容约 130px + 单元格左右 padding 16px，再留少量余量）
  { key: 'actions', name: '操作', width: 160, align: 'center', fixed: 'right' },
])

async function refresh(): Promise<void> {
  loading.value = true
  try {
    inbounds.value = await listInbounds()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

function startCreate(protocol: string): void {
  pickerVisible.value = false
  inboundForm.value?.openCreate(protocol)
}

const { rowEnabled, togglingId, toggle: handleToggle } = useEnabledOverride(
  inbounds,
  async (row, enabled) => {
    await setInboundEnabled(row.id, enabled)
    inbounds.value = inbounds.value.map((item) => (item.id === row.id ? { ...item, enabled } : item))
    message.success(enabled ? '入站已启用并已生效' : '入站已停用，其下全部用户已随之下线')
  },
)

/** 危险操作：删除入站会连带删除其全部用户、流量记录与订阅绑定，二次确认 */
async function handleDelete(row: Inbound): Promise<void> {
  const extra = row.userCount
    ? `，其下 ${row.userCount} 个用户、累计流量记录以及这些用户在订阅里的绑定也会一起删除`
    : ''
  const ok = await confirmAction(
    '删除入站',
    `确认删除「${row.name}」（${listenLabel(row.listen, row.listenPort)}）吗${extra}？删除后不可恢复。`,
    true,
    '删除',
  )
  if (!ok) return
  try {
    await deleteInbound(row.id)
    message.success('入站已删除')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  }
}

onMounted(refresh)
</script>

<template>
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>入站</span>
        <div class="page-ops">
          <u-button @click="router.push('/certs')">证书管理</u-button>
          <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
          <u-button v-if="inbounds.length" type="primary" @click="pickerVisible = true">新建入站</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <div class="card-desc">
        一个入站就是一个<b>真实监听端口</b>（协议 + 监听地址/端口 + TLS 等连接层配置）。
        用户在「订阅用户」页统一管理：勾选入站即在该端口上生成独立凭据的节点，流量按用户单独累计，
        这里只展示各入站下的用户数。任何改动保存后立即生效，内核随即按新配置重建。
      </div>
      <u-table :columns="columns" :data="inbounds" row-key="id" :loading="loading">
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无入站"
            description="先创建一个入站（协议 + 监听地址/端口 + TLS 等），再到「订阅用户」页勾选它，为使用者生成节点"
            cta-text="新建入站"
            @cta="pickerVisible = true"
          />
        </template>
        <template #column:protocol="{ rowData }">
          <u-tag :type="protocolDef(rowData.protocol)?.recommended ? 'success' : 'info'">
            {{ protocolDef(rowData.protocol)?.name ?? rowData.protocol }}
          </u-tag>
        </template>
        <template #column:listen="{ rowData }">
          <span class="mono">{{ listenLabel(rowData.listen, rowData.listenPort) }}</span>
        </template>
        <template #column:users="{ rowData }">
          <span :class="{ 'cell-dim': !rowData.userCount }">{{ rowData.userCount }} 个</span>
        </template>
        <template #column:warnings="{ rowData }">
          <u-tag v-if="rowData.warnings?.length" type="warning" :title="rowData.warnings.join('\n')">
            ⚠ {{ rowData.warnings.length }}
          </u-tag>
          <span v-else class="cell-dim">—</span>
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
          <u-action-group>
            <u-action @run="inboundForm?.openEdit(rowData)">编辑</u-action>
            <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
          </u-action-group>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <!-- 新建入口：推荐组排前分组展示 -->
  <u-dialog v-model="pickerVisible" title="新建入站" :style="dialogStyle(640)">
    <p class="picker-tip">选择协议后填写监听地址、端口与连接层参数；端口留空会自动分配一个随机空闲端口。</p>
    <div class="proto-group">
      <p class="proto-group-title">推荐</p>
      <div class="proto-grid">
        <button v-for="p in recommended" :key="p.protocol" class="proto-card" type="button" @click="startCreate(p.protocol)">
          <span class="proto-name">{{ p.name }}</span>
          <span class="proto-desc">{{ p.desc }}</span>
        </button>
      </div>
    </div>
    <div class="proto-group">
      <p class="proto-group-title">经典</p>
      <div class="proto-grid">
        <button v-for="p in classic" :key="p.protocol" class="proto-card" type="button" @click="startCreate(p.protocol)">
          <span class="proto-name">{{ p.name }}</span>
          <span class="proto-desc">{{ p.desc }}</span>
        </button>
      </div>
    </div>
  </u-dialog>

  <InboundForm ref="inboundForm" @saved="refresh" />
</template>

<style scoped>
/* page-header / page-ops / cell-dim / mono 为 App.vue 全局类 */

.card-desc {
  margin-bottom: 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.picker-tip {
  margin: 0 0 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.proto-group {
  margin-bottom: 8px;
}

.proto-group-title {
  margin: 0 0 8px;
  font-size: var(--u-font-size-main-small);
  font-weight: 600;
  color: var(--u-text-color-second);
}

.proto-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.proto-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 12px 14px;
  text-align: left;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-top);
  cursor: pointer;
  transition: border-color var(--u-transition-fast) var(--u-transition-ease-out);
}

.proto-card:hover {
  border-color: var(--u-color-primary);
}

.proto-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--u-font-size-main-default);
  font-weight: 600;
}

.proto-desc {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
