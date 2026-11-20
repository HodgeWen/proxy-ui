<!-- 入站管理：协议分组新建入口（推荐组排前）、列表增删改启停、证书管理与应用配置入口 -->
<script setup lang="ts">
import { deleteInbound, listInbounds, setInboundEnabled } from '../api/inbound'
import type { Inbound } from '../api/inbound'
import { applyKernelConfig } from '../api/kernel'
import CertManager from '../components/CertManager.vue'
import InboundForm from '../components/InboundForm.vue'
import {
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
  messageConfirm,
} from '@veltra/desktop'
import { computed, onMounted, shallowRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import { PROTOCOL_DEFS, protocolDef } from '../inbound-schema'

const inbounds = shallowRef<Inbound[]>([])
const loading = shallowRef(false)
const pickerVisible = shallowRef(false)
const applying = shallowRef(false)
const togglingId = shallowRef(0)

const inboundForm = shallowRef<InstanceType<typeof InboundForm>>()
const certManager = shallowRef<InstanceType<typeof CertManager>>()

const recommended = computed(() => PROTOCOL_DEFS.filter((p) => p.recommended))
const classic = computed(() => PROTOCOL_DEFS.filter((p) => !p.recommended))

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 150 },
  { key: 'protocol', name: '协议', width: 160 },
  { key: 'listenPort', name: '端口', width: 90, align: 'center' },
  { key: 'warnings', name: '警示', width: 90, align: 'center' },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 120, align: 'center' },
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

async function handleToggle(row: Inbound, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  try {
    await setInboundEnabled(row.id, enabled)
    row.enabled = enabled
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    togglingId.value = 0
  }
}

async function handleDelete(row: Inbound): Promise<void> {
  const action = await messageConfirm({
    title: '删除入站',
    message: `确认删除入站「${row.name}」吗？`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteInbound(row.id)
    message.success('入站已删除')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  }
}

async function handleApply(): Promise<void> {
  applying.value = true
  try {
    const { applied, status } = await applyKernelConfig()
    message.success(
      applied
        ? `配置已应用并重启内核（${status.version ?? '未知版本'}）`
        : `配置已生成并通过校验，内核${status.running ? `运行中（${status.version ?? ''}）` : '未运行，将在下次启动时生效'}`,
    )
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    applying.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>入站管理</span>
        <div class="page-ops">
          <u-button :loading="applying" @click="handleApply">应用配置</u-button>
          <u-button @click="certManager?.open()">证书管理</u-button>
          <u-button type="primary" @click="pickerVisible = true">新建入站</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <u-table :columns="columns" :data="inbounds" row-key="id" border :loading="loading">
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无入站"
            description="点击右上角「新建入站」创建第一个入站，配置完成后需应用配置生效"
          />
        </template>
        <template #column:protocol="{ rowData }">
          <u-tag :type="protocolDef(rowData.protocol)?.recommended ? 'success' : 'info'">
            {{ protocolDef(rowData.protocol)?.name ?? rowData.protocol }}
          </u-tag>
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
            :model-value="rowData.enabled"
            :loading="togglingId === rowData.id"
            @update:model-value="(v: boolean) => handleToggle(rowData, v)"
          />
        </template>
        <template #column:actions="{ rowData }">
          <u-button text size="small" type="primary" @click="inboundForm?.openEdit(rowData)">编辑</u-button>
          <u-button text size="small" type="error" @click="handleDelete(rowData)">删除</u-button>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <!-- 新建入口：推荐组排前分组展示 -->
  <u-dialog v-model="pickerVisible" title="新建入站" style="width: 640px; max-width: 96vw">
    <div class="proto-group">
      <p class="proto-group-title">推荐</p>
      <div class="proto-grid">
        <button v-for="p in recommended" :key="p.protocol" class="proto-card" type="button" @click="startCreate(p.protocol)">
          <span class="proto-name">{{ p.name }}<u-tag type="success" size="small">推荐</u-tag></span>
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

  <InboundForm ref="inboundForm" @saved="refresh" @manage-certs="certManager?.open()" />
  <CertManager ref="certManager" @changed="inboundForm?.reloadCerts()" />
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
}

.page-ops {
  display: flex;
  gap: 8px;
}

.cell-dim {
  color: var(--u-text-color-assist);
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
