<!-- 入站用户管理：列出该入站下所有认证身份（共享入站端口），支持新建/启停/删除；
     手动创建的用户没有订阅链接，仅供个人/临时使用，订阅用户请到订阅页创建 -->
<script setup lang="ts">
import {
  UButton,
  UDialog,
  UInput,
  UTable,
  UTag,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import { computed, ref, shallowRef } from 'vue'

import type { Inbound, InboundUser } from '../api/inbound'
import { createUser, deleteUser, setUserEnabled } from '../api/inbound'
import { formatBytes } from '../api/subscription'
import { dialogStyle } from '../dialog-style'
import { listenLabel } from '../inbound-schema'

const visible = defineModel<boolean>('visible', { default: false })
const props = defineProps<{ inbound: Inbound | null }>()
const emit = defineEmits<{ changed: [] }>()

const newName = ref('')
const creating = shallowRef(false)
const togglingId = shallowRef(0)

const users = computed<InboundUser[]>(() => props.inbound?.users ?? [])
const title = computed(() => `管理用户 · ${props.inbound?.name ?? '入站'}`)
const endpoint = computed(() =>
  props.inbound ? listenLabel(props.inbound.listen, props.inbound.listenPort) : '-',
)

const columns = defineTableColumns([
  { key: 'name', name: '用户名', minWidth: 160 },
  { key: 'tokenName', name: '归属订阅用户', minWidth: 130 },
  { key: 'usage', name: '累计用量', width: 120, align: 'right' },
  { key: 'state', name: '状态', width: 90, align: 'center' },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 90, align: 'center', fixed: 'right' },
])

/** 新建用户：名称留空时由后端按 协议-序号 自动生成 */
async function handleCreate(): Promise<void> {
  if (!props.inbound) return
  creating.value = true
  try {
    const user = await createUser(props.inbound.id, newName.value.trim())
    newName.value = ''
    message.success(`用户 ${user.name} 已创建`)
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    creating.value = false
  }
}

async function handleToggle(row: InboundUser, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  try {
    await setUserEnabled(row.id, enabled)
    row.enabled = enabled
    message.success(enabled ? '用户已启用并已生效' : '用户已停用，该身份已不再可用')
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    togglingId.value = 0
  }
}

/** 删除用户会一并删除其累计流量与订阅绑定，需二次确认 */
async function handleDelete(row: InboundUser): Promise<void> {
  const bind = row.tokenId ? `，其订阅绑定（${row.tokenName}）也会解除` : ''
  const action = await messageConfirm({
    title: '删除用户',
    message: `确认删除用户「${row.name}」吗？其累计流量记录${bind}，删除后不可恢复。`,
    confirmButtonText: '删除',
    confirmButtonType: 'danger',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteUser(row.id)
    message.success('用户已删除')
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}
</script>

<template>
  <u-dialog v-model="visible" :title="title" :style="dialogStyle(880)">
    <div class="users-intro">
      本入站监听 <code class="endpoint">{{ endpoint }}</code>，下列用户共享这一端口，各自持有独立凭据；
      用户名即客户端订阅里的节点名，也是流量统计的归属维度。
    </div>

    <div class="create-row">
      <u-input v-model="newName" placeholder="新用户名，留空自动按 协议-序号 生成" clearable />
      <u-button type="primary" :loading="creating" @click="handleCreate">新建用户</u-button>
    </div>
    <p class="create-hint">
      这里创建的用户没有订阅链接，只适合手动配置的个人/临时设备使用；需要订阅链接的用户请在订阅页创建，
      勾选本入站时会自动生成同名用户。
    </p>

    <u-table :columns="columns" :data="users" row-key="id" size="small">
      <template #empty>
        <span class="cell-dim">该入站下还没有用户，新建一个即可用独立凭据连接</span>
      </template>
      <template #column:tokenName="{ rowData }">
        <span v-if="rowData.tokenId">{{ rowData.tokenName || '已绑定' }}</span>
        <span v-else class="cell-dim">手动创建</span>
      </template>
      <template #column:usage="{ rowData }">
        {{ formatBytes(rowData.uploadBytes + rowData.downloadBytes) }}
      </template>
      <template #column:state="{ rowData }">
        <u-tag v-if="rowData.suspended" type="warning">已暂停</u-tag>
        <u-tag v-else-if="rowData.disabled" type="info">已停用</u-tag>
        <u-tag v-else-if="rowData.enabled" type="success">可用</u-tag>
        <u-tag v-else type="info">已停用</u-tag>
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
        <u-button text size="small" type="danger" @click="handleDelete(rowData)">删除</u-button>
      </template>
    </u-table>

    <template #footer="{ close }">
      <u-button type="primary" @click="close()">关闭</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.users-intro {
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-hover);
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.endpoint {
  color: var(--u-text-color-main);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.create-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.create-hint {
  margin: 8px 0 14px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.cell-dim {
  color: var(--u-text-color-assist);
}
</style>
