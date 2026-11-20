<!-- 出站管理：页顶出站说明卡、direct 出站说明与添加、自定义出站表单（高级项与类型速查折叠、附加字段样例）增删改；改动保存即热应用内核 -->
<script setup lang="ts">
import { createOutbound, deleteOutbound, listOutbounds, setOutboundEnabled, updateOutbound } from '../api/outbound'
import type { Outbound, OutboundPayload } from '../api/outbound'
import {
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
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, onMounted, reactive, ref, shallowRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import FormGroupCard from '../components/FormGroupCard.vue'
import { dialogStyle } from '../dialog-style'

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

/** 常见出站类型；可输入创建其它类型 */
const typeOptions = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'socks', 'http', 'wireguard', 'hysteria', 'ssh'].map(
  (t) => ({ label: t, value: t }),
)

/** 新建出站的默认类型：最常见通用的推荐项（direct 是内置直连，不进自定义表单） */
const DEFAULT_TYPE = 'shadowsocks'

/**
 * 常见出站类型的适用场景与附加配置要点；字段名与 sing-box 1.14 出站选项一致。
 * required 为「无」表示只填服务器地址与端口即可用。
 */
const TYPE_NOTES: Record<string, { scene: string; required: string; sample?: Record<string, unknown> }> = {
  shadowsocks: {
    scene: '最通用简单的选择，客户端支持最广；用 2022 系列加密时性能与抗探测都不错，拿不准就选它',
    required: 'method（加密方式）、password',
    sample: { method: '2022-blake3-aes-128-gcm', password: '<随机生成的密钥>' },
  },
  vless: {
    scene: '轻量现代协议，无额外加密开销；配合 Reality / Vision 抗封锁能力最强，新建节点首选',
    required: 'uuid',
    sample: { uuid: '<服务端分配的 UUID>', flow: 'xtls-rprx-vision' },
  },
  vmess: {
    scene: 'V2Ray 经典协议，老牌生态兼容好；新部署更建议 vless',
    required: 'uuid',
    sample: { uuid: '<服务端分配的 UUID>' },
  },
  trojan: {
    scene: '伪装成普通 HTTPS 的老牌协议，配置简单，依赖服务端的 TLS 证书',
    required: 'password',
    sample: { password: '<服务端密码>' },
  },
  hysteria2: {
    scene: '基于 QUIC 的高速协议，在丢包多、线路差的场景下速度优势明显',
    required: 'password',
    sample: { password: '<服务端密码>' },
  },
  tuic: {
    scene: '基于 QUIC 的低延迟协议，握手快，适合手机等弱网设备',
    required: 'uuid、password',
    sample: { uuid: '<服务端 UUID>', password: '<服务端密码>', congestion_control: 'bbr', udp_relay_mode: 'native' },
  },
  anytls: {
    scene: '专为代理设计的新协议（2025），流量特征规整、连接复用高效',
    required: 'password',
    sample: { password: '<服务端密码>' },
  },
  socks: {
    scene: '通用 SOCKS5 代理，常用于对接本机或内网的其它代理服务',
    required: '无（账号密码按服务端配置）',
    sample: { version: '5' },
  },
  http: {
    scene: 'HTTP 明文代理，仅建议在可信内网使用',
    required: '无',
  },
  wireguard: {
    scene: 'WireGuard 隧道；对端地址与端口写在 peers 里，上方「服务器 / 服务器端口」可留空',
    required: 'private_key、address、peers（含 public_key）',
    sample: {
      private_key: '<本机私钥>',
      address: ['172.16.0.2/30'],
      peers: [{ address: '<对端地址>', port: 51820, public_key: '<对端公钥>' }],
    },
  },
  hysteria: {
    scene: 'hysteria2 的上一代协议，官方已停止演进，新建建议改用 hysteria2',
    required: 'auth_str、up_mbps、down_mbps',
  },
  ssh: {
    scene: 'SSH 隧道出站，直接复用已有服务器的 SSH 服务转发流量，无需另外部署代理',
    required: 'user',
    sample: { user: 'root' },
  },
}

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
    const action = await messageConfirm({
      title: '填入样例',
      message: '附加配置里已有内容，填入样例会覆盖当前内容，确认继续吗？',
      confirmButtonText: '覆盖填入',
      cancelButtonText: '取消',
    }).onClosed
    if (action !== 'confirm') return
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
  const action = await messageConfirm({
    title: '删除出站',
    message: `确认删除出站「${row.name}」吗？被规则引用时内核将无法重建，你会看到具体原因。`,
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
          <template v-if="rowData.type !== 'direct'">
            <u-button text size="small" type="primary" @click="openEdit(rowData)">编辑</u-button>
            <u-button text size="small" type="error" @click="handleDelete(rowData)">删除</u-button>
          </template>
          <template v-else>
            <u-button text size="small" type="error" @click="handleDelete(rowData)">删除</u-button>
          </template>
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
          <pre class="extra-sample-code">{{ typeSampleText }}</pre>
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
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.intro-card {
  margin-bottom: 16px;
}

.intro-list {
  margin: 8px 0;
  padding-left: 18px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.intro-note {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

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

.extra-sample-code {
  margin: 0;
  padding: 10px 12px;
  overflow-x: auto;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-middle);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
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
