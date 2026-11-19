<!-- 出站管理：direct 出站说明与添加、自定义出站表单（高级项折叠）增删改、应用配置入口 -->
<script setup lang="ts">
import { createOutbound, deleteOutbound, listOutbounds, setOutboundEnabled, updateOutbound } from '../api/outbound'
import type { Outbound, OutboundPayload } from '../api/outbound'
import { applyKernelConfig } from '../api/kernel'
import {
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UCollapse,
  UCollapseItem,
  UDialog,
  UEmpty,
  UForm,
  UFormItem,
  UInput,
  UNumberInput,
  USelect,
  USwitch,
  UTable,
  UTag,
  UTextarea,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, onMounted, reactive, shallowRef } from 'vue'

const outbounds = shallowRef<Outbound[]>([])
const loading = shallowRef(false)
const applying = shallowRef(false)
const togglingId = shallowRef(0)

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

// ---------- 自定义出站表单 ----------

const formVisible = shallowRef(false)
const saving = shallowRef(false)
const editing = shallowRef<Outbound | null>(null)
const formRef = shallowRef<FormExposed>()
const expandedSections = shallowRef<string[]>([])
const form = reactive({
  name: '',
  type: '',
  server: '',
  serverPort: undefined as number | undefined,
  extraJson: '',
})

/** 常见出站类型；可输入创建其它类型 */
const typeOptions = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'socks', 'http', 'wireguard', 'hysteria', 'ssh'].map(
  (t) => ({ label: t, value: t }),
)

/** 附加配置须为 JSON 对象 */
function validateExtra(value: string): string {
  if (!value.trim()) return ''
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? '' : '附加配置须为 JSON 对象'
  } catch {
    return '附加配置 JSON 不合法'
  }
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', type: '', server: '', serverPort: undefined, extraJson: '' })
  expandedSections.value = []
  formVisible.value = true
}

function openEdit(row: Outbound): void {
  const cfg = (row.config ?? {}) as Record<string, unknown>
  const { server, server_port: serverPort, ...extra } = cfg
  editing.value = row
  Object.assign(form, {
    name: row.name,
    type: row.type,
    server: typeof server === 'string' ? server : '',
    serverPort: typeof serverPort === 'number' ? serverPort : undefined,
    extraJson: Object.keys(extra).length ? JSON.stringify(extra, null, 2) : '',
  })
  expandedSections.value = []
  formVisible.value = true
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const extra = form.extraJson.trim() ? (JSON.parse(form.extraJson) as Record<string, unknown>) : {}
    const config: Record<string, unknown> = { ...extra }
    if (form.server) config.server = form.server
    if (form.serverPort) config.server_port = form.serverPort
    const payload: OutboundPayload = { name: form.name, type: form.type, config }
    if (editing.value) await updateOutbound(editing.value.id, payload)
    else await createOutbound(payload)
    message.success(editing.value ? '出站已更新' : '出站已创建')
    formVisible.value = false
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

// ---------- 列表操作 ----------

async function addDirect(): Promise<void> {
  try {
    await createOutbound({ name: 'direct', type: 'direct', config: {} })
    message.success('direct 出站已添加')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  }
}

async function handleToggle(row: Outbound, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  try {
    await setOutboundEnabled(row.id, enabled)
    row.enabled = enabled
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    togglingId.value = 0
  }
}

async function handleDelete(row: Outbound): Promise<void> {
  const action = await messageConfirm({
    title: '删除出站',
    message: `确认删除出站「${row.name}」吗？被规则引用时应用配置将失败。`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteOutbound(row.id)
    message.success('出站已删除')
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

function endpointOf(row: Outbound): string {
  const cfg = (row.config ?? {}) as Record<string, unknown>
  if (row.type === 'direct') return '内置直连出站'
  if (typeof cfg.server === 'string') return `${cfg.server}${typeof cfg.server_port === 'number' ? ':' + cfg.server_port : ''}`
  return '—'
}

onMounted(refresh)
</script>

<template>
  <u-alert v-if="!hasDirect && !loading" class="direct-alert" type="info" title="direct 出站">
    <div class="direct-row">
      <span>direct 为 sing-box 内置直连出站，作为未命中规则的默认出口，通常保持添加即可。</span>
      <u-button size="small" type="primary" @click="addDirect">添加 direct 出站</u-button>
    </div>
  </u-alert>

  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>出站管理</span>
        <div class="page-ops">
          <u-button size="small" :loading="applying" @click="handleApply">应用配置</u-button>
          <u-button size="small" type="primary" @click="openCreate">新建自定义出站</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <u-empty v-if="!outbounds.length && !loading" text="暂无出站" block />
      <u-table v-else :columns="columns" :data="outbounds" row-key="id" border :loading="loading">
        <template #column:type="{ rowData }">
          <u-tag :type="rowData.type === 'direct' ? 'success' : 'info'">{{ rowData.type }}</u-tag>
        </template>
        <template #column:endpoint="{ rowData }">
          {{ endpointOf(rowData) }}
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
          <template v-if="rowData.type !== 'direct'">
            <u-button text type="primary" @click="openEdit(rowData)">编辑</u-button>
            <u-button text type="error" @click="handleDelete(rowData)">删除</u-button>
          </template>
          <template v-else>
            <u-button text type="error" @click="handleDelete(rowData)">删除</u-button>
          </template>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? `编辑出站 · ${editing.name}` : '新建自定义出站'" style="width: 620px; max-width: 96vw">
    <u-form ref="formRef" :model="form" :cols="1" label-width="92px">
      <u-input label="名称" field="name" placeholder="出站 tag，供规则引用" :rules="{ required: '请输入名称' }" />
      <u-select
        label="类型"
        field="type"
        :options="typeOptions"
        placeholder="选择或输入出站类型"
        filterable
        creatable
        :rules="{ required: '请选择类型' }"
      />
      <u-input label="服务器" field="server" placeholder="远端服务器地址" />
      <u-number-input label="服务器端口" field="serverPort" :min="1" :max="65535" />

      <u-collapse v-model="expandedSections" default-collapse-all>
        <u-collapse-item value="advanced" title="高级选项">
          <u-form-item
            label="附加配置"
            field="extraJson"
            tips="JSON 对象，与服务器地址一起合并进出站配置（如密码、传输层等协议字段）"
            :rules="{ validator: validateExtra }"
          >
            <u-textarea v-model="form.extraJson" :rows="6" placeholder='例如 { "password": "..." }' />
          </u-form-item>
        </u-collapse-item>
      </u-collapse>
    </u-form>
    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.direct-alert {
  margin-bottom: 16px;
}

.direct-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

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
</style>
