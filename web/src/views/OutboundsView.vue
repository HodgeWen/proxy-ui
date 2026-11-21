<!-- 出站管理：页顶出站说明卡、direct 出站说明与添加、自定义出站表单（高级项与类型速查折叠、附加字段样例）增删改；改动保存即热应用内核 -->
<script setup lang="ts">
import { createOutbound, deleteOutbound, listOutbounds, setOutboundEnabled, updateOutbound } from '../api/outbound'
import type { Outbound, OutboundPayload } from '../api/outbound'
import {
  UAction,
  UActionGroup,
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
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
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, onMounted, reactive, ref, shallowRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import DialogFooter from '../components/DialogFooter.vue'
import FormGroupCard from '../components/FormGroupCard.vue'
import { confirmAction } from '../confirm-action'
import { dialogStyle } from '../dialog-style'
import { DEFAULT_TYPE, TYPE_NOTES, typeOptions } from '../outbound-type-notes'

const outbounds = shallowRef<Outbound[]>([])
const loading = shallowRef(false)
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
/** 高级选项分组默认折叠（基础配置关键项默认展开） */
const advancedCollapsed = ref(true)
/** 类型速查默认折叠：只在需要横向比对类型时展开 */
const cheatsheetCollapsed = ref(true)
const form = reactive({
  name: '',
  type: '',
  server: '',
  serverPort: undefined as number | undefined,
  extraJson: '',
})

/** 当前类型的场景说明（未知类型回落到通用说明） */
const typeTips = computed(
  () => TYPE_NOTES[form.type]?.scene ?? '出站协议类型；下拉含常见类型，也可输入其它 sing-box 出站类型',
)

/** 当前类型的必填附加字段提示（无样例需求的类型说明「填服务器即可」） */
const requiredText = computed(() => {
  const note = TYPE_NOTES[form.type]
  if (!note) return ''
  if (note.required === '无') return `${form.type} 没有必填的附加字段，填好服务器与端口即可保存`
  return `${form.type} 还需在下方高级选项的附加配置里填：${note.required}`
})

/** 当前类型的附加配置样例文本（无样例的类型不展示） */
const typeSampleText = computed(() => {
  const sample = TYPE_NOTES[form.type]?.sample
  return sample ? JSON.stringify(sample, null, 2) : ''
})

/** 样例填入：已有手写内容时先确认，避免一键覆盖 */
async function fillSample(): Promise<void> {
  const text = typeSampleText.value
  if (!text) return
  if (form.extraJson.trim() && form.extraJson.trim() !== text) {
    const ok = await confirmAction('填入样例', '附加配置里已有内容，填入样例会覆盖当前内容，确认继续吗？', false, '覆盖填入')
    if (!ok) return
  }
  form.extraJson = text
}

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
  Object.assign(form, { name: '', type: DEFAULT_TYPE, server: '', serverPort: undefined, extraJson: '' })
  advancedCollapsed.value = true
  cheatsheetCollapsed.value = true
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
  advancedCollapsed.value = true
  cheatsheetCollapsed.value = true
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
  const ok = await confirmAction(
    '删除出站',
    `确认删除出站「${row.name}」吗？被规则引用时内核将无法重建，你会看到具体原因。`,
    true,
    '删除',
  )
  if (!ok) return
  try {
    await deleteOutbound(row.id)
    message.success('出站已删除')
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
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
      <u-button type="primary" plain @click="addDirect">添加 direct 出站</u-button>
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
      <u-table :columns="columns" :data="outbounds" row-key="id" stripe :loading="loading">
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
            :model-value="rowData.enabled"
            :loading="togglingId === rowData.id"
            @update:model-value="(v: boolean) => handleToggle(rowData, v)"
          />
        </template>
        <template #column:actions="{ rowData }">
          <u-action-group>
            <!-- direct 出站不可编辑，只能删除 -->
            <u-action v-if="rowData.type !== 'direct'" @run="openEdit(rowData)">编辑</u-action>
            <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
          </u-action-group>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog
    v-model="formVisible"
    :title="editing ? `编辑出站 · ${editing.name}` : '新建自定义出站'"
    :style="dialogStyle(1120)"
  >
    <!-- 内容超高时由 UDialog 自带滚动容器接管，header / footer 常驻 -->
    <u-form ref="formRef" :model="form" :cols="1" label-width="92px">
      <FormGroupCard title="基础配置">
        <u-form-item
          label="名称"
          field="name"
          :rules="{ required: '请输入名称' }"
          tips="出站 tag，供路由规则与策略组引用；需与其它出站、策略组名不重复"
        >
          <u-input v-model="form.name" placeholder="出站 tag，供规则引用" clearable />
        </u-form-item>
        <u-form-item label="类型" field="type" :rules="{ required: '请选择类型' }" :tips="typeTips">
          <u-select v-model="form.type" :options="typeOptions" placeholder="选择或输入出站类型" filterable creatable />
        </u-form-item>
        <!-- 当前类型的必填附加字段提示：让用户知道只填服务器还不够 -->
        <p v-if="requiredText" class="type-required">{{ requiredText }}</p>
        <u-form-item
          label="服务器"
          field="server"
          tips="远端代理服务器地址（域名或 IP）；wireguard 的对端地址在附加配置 peers 里，此处可留空"
        >
          <u-input v-model="form.server" placeholder="远端服务器地址" clearable />
        </u-form-item>
        <u-form-item label="服务器端口" field="serverPort" tips="远端代理服务器端口，1-65535；按协议常见值如 443 / 8388">
          <u-number-input v-model="form.serverPort" :min="1" :max="65535" />
        </u-form-item>
      </FormGroupCard>

      <FormGroupCard v-model:collapsed="advancedCollapsed" title="高级选项">
        <u-form-item
          label="附加配置"
          field="extraJson"
          span="full"
          tips="JSON 对象，与服务器地址一起合并进出站配置（如密码、UUID、传输层等协议字段），下方有当前类型的字段样例"
          :rules="{ validator: validateExtra }"
        >
          <u-textarea v-model="form.extraJson" :rows="6" placeholder='例如 { "password": "..." }' />
        </u-form-item>
        <!-- 当前类型的附加字段样例：可折叠预览、一键填入（已填内容先确认再覆盖） -->
        <div v-if="typeSampleText" class="extra-sample">
          <div class="extra-sample-head">
            <span>{{ form.type }} 附加字段样例（按实际服务端参数修改占位值）</span>
            <u-button size="small" @click="fillSample">填入样例</u-button>
          </div>
          <pre class="code-block">{{ typeSampleText }}</pre>
        </div>
      </FormGroupCard>

      <!-- 每种类型适合什么场景：默认折叠，选型时展开对照 -->
      <FormGroupCard v-model:collapsed="cheatsheetCollapsed" title="类型速查（每种类型适合什么）">
        <dl class="type-cheatsheet">
          <div v-for="opt in typeOptions" :key="opt.value" class="type-cheatsheet-row">
            <dt>{{ opt.label }}</dt>
            <dd>{{ TYPE_NOTES[opt.value]?.scene }}</dd>
          </div>
        </dl>
      </FormGroupCard>
    </u-form>
    <template #footer="{ close }">
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>

<style scoped>
/* intro-card / intro-list / intro-note / page-header / page-ops / code-block 为 App.vue 全局类 */

.direct-alert {
  margin-bottom: 16px;
}

.direct-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

/* 当前类型的必填附加字段提示：与表单 tips 同色态，整行展示 */
.type-required {
  grid-column: 1 / -1;
  margin: -4px 0 8px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 附加字段样例：标题行 + 等宽代码块，整行展示 */
.extra-sample {
  grid-column: 1 / -1;
}

.extra-sample-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 类型速查：dt 固定宽度，dd 与之同行；整组占满栅格行 */
.type-cheatsheet {
  grid-column: 1 / -1;
  margin: 0;
}

.type-cheatsheet-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 4px 0;
}

.type-cheatsheet-row + .type-cheatsheet-row {
  border-top: 1px dashed var(--u-border-color);
}

.type-cheatsheet-row dt {
  width: 110px;
  flex-shrink: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-weight: 600;
}

.type-cheatsheet-row dd {
  margin: 0;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}
</style>
