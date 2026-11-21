<!-- 入站声明式表单引擎：按协议 schema 渲染分组卡片（基础 / TLS 证书 / 高级），
     UFormItem tips 直出 schema description，弱配置警示内联展示并支持一键跳转 TLS 区块。
     入站是真实监听：协议 + 监听地址/端口 + 连接层参数；用户凭据由后端为入站内每个用户生成 -->
<script setup lang="ts">
import type { Certificate } from '../api/cert'
import { listCerts } from '../api/cert'
import { createInbound, fetchInboundTemplate, updateInbound } from '../api/inbound'
import type { Inbound, InboundPayload, ListenAddress } from '../api/inbound'
import {
  UButton,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UMultiSelect,
  UNumberInput,
  UPasswordInput,
  USelect,
  USwitch,
  UTextarea,
  message,
} from '@veltra/desktop'
import type { Component, PropType } from 'vue'
import { copy, o } from '@cat-kit/core'
import { computed, defineComponent, h, nextTick, onBeforeUnmount, reactive, ref, shallowRef, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import DialogFooter from './DialogFooter.vue'
import FormGroupCard from './FormGroupCard.vue'
import { dialogStyle } from '../dialog-style'
import {
  LISTEN_OPTIONS,
  PROTOCOL_DEFS,
  TLS_ADVANCED,
  TLS_FIELDS,
  certOptions,
  computeInboundWarnings,
  protocolDef,
  protocolNotes,
} from '../inbound-schema'
import type { FieldDef, FieldType, ProtocolDef } from '../inbound-schema'

const emit = defineEmits<{ saved: [inbound: Inbound] }>()
const router = useRouter()

/** 控件类型映射 */
const CONTROLS: Record<FieldType, Component> = {
  input: UInput,
  password: UPasswordInput,
  number: UNumberInput,
  select: USelect,
  multiselect: UMultiSelect,
  switch: USwitch,
  textarea: UTextarea,
}

/** 声明式字段渲染：UFormItem 包裹控件（description → tips、span → 栅格占位），经 o() 按点号路径读写表单模型 */
const SchemaField = defineComponent({
  name: 'SchemaField',
  props: {
    def: { type: Object as PropType<FieldDef>, required: true },
    model: { type: Object as PropType<Record<string, any>>, required: true },
    certs: { type: Array as PropType<Certificate[]>, default: () => [] },
  },
  setup(props) {
    return () => {
      const d = props.def
      if (d.when && !d.when(props.model)) return null
      const current = o(props.model).get(d.field)
      const controlProps: Record<string, any> = {
        modelValue: d.type === 'multiselect' ? current ?? [] : current,
        'onUpdate:modelValue': (value: unknown) => {
          o(props.model).set(d.field, value)
          // 选定面板证书后自动带出服务器名称：SNI 需与证书域名匹配，直接取证书主域名
          if (d.field === 'tls.certId' && value) {
            const cert = props.certs.find((c) => c.id === value)
            const domain = cert?.commonName || cert?.domains?.[0]
            if (domain) o(props.model).set('tls.serverName', domain)
          }
        },
      }
      if (d.placeholder) controlProps.placeholder = d.placeholder
      if (d.type === 'select' || d.type === 'multiselect') {
        // tls.certId 的选项按面板证书列表动态生成
        controlProps.options = d.field === 'tls.certId' ? certOptions(props.certs) : d.options
        if (d.field === 'tls.certId') controlProps.clearable = true
      }
      if (d.type === 'number') {
        if (d.min !== undefined) controlProps.min = d.min
        if (d.max !== undefined) controlProps.max = d.max
      }
      if (d.type === 'textarea') controlProps.rows = 3
      return h(
        UFormItem,
        { label: d.label, tips: d.description, span: d.span },
        { default: () => [h(CONTROLS[d.type], controlProps)] },
      )
    }
  },
})

const visible = shallowRef(false)
const def = shallowRef<ProtocolDef>(PROTOCOL_DEFS[0])
const editing = shallowRef<Inbound | null>(null)
const form = reactive<Record<string, any>>({})
const formRef = shallowRef<{ validate: () => Promise<boolean>; clearValidate: () => void }>()
const certs = shallowRef<Certificate[]>([])
const saving = shallowRef(false)
/** TLS 分组默认展开（证书选择是关键配置），高级分组默认折叠 */
const tlsCollapsed = ref(false)
const advancedCollapsed = ref(true)
/** TLS 卡高亮脉冲：跳转后无条件可见的反馈（视口无滚动余量时滚动/展开均为 no-op） */
const tlsHighlighted = ref(false)
let tlsHighlightTimer: ReturnType<typeof setTimeout> | undefined
const tlsCard = useTemplateRef<InstanceType<typeof FormGroupCard>>('tlsCard')

const dialogTitle = computed(() =>
  editing.value ? `编辑入站 · ${editing.value.name}` : `新建${def.value.name} 入站`,
)
/** 保存前实时计算的弱配置警示 */
const warnings = computed(() => computeInboundWarnings(def.value.protocol, form))
const notes = computed(() => protocolNotes(def.value.protocol))
const advancedFields = computed(() => [...def.value.advanced, ...(def.value.tls ? TLS_ADVANCED : [])])
/** 协议要求必须启用 TLS 时不再渲染开关（始终开启），只保留其余 TLS 字段 */
const tlsFields = computed(() =>
  def.value.tlsRequired ? TLS_FIELDS.filter((f) => f.field !== 'tls.enabled') : TLS_FIELDS,
)

function resetForm(): void {
  Object.keys(form).forEach((key) => delete form[key])
  tlsCollapsed.value = false
  advancedCollapsed.value = true
  formRef.value?.clearValidate()
}

/** 后端模板经 omitempty 省略零值字段：补回零值外壳，select/switch 控件才能落在默认档
    （如 TUIC 心跳的「内核默认」、Shadowsocks 网络的「TCP + UDP」）；默认值本身仍来自后端模板 */
function templateShells(tpl: Record<string, any>): Record<string, any> {
  const shells: Record<string, any> = {
    tcpFastOpen: false,
    multiplex: { enabled: false, padding: false },
    heartbeat: '',
    network: '',
    ...tpl,
  }
  if ('masquerade' in tpl) shells.masquerade = { type: '', ...tpl.masquerade }
  return shells
}

/** 新建：拉后端模板端点回填连接层默认值（含预生成的 Reality 密钥），端口留空由后端分配随机空闲端口。
    模板拉取失败时兜底空配置打开弹框，缺省字段由后端保存时补齐（CompleteRealityKeys / SS PSK） */
async function openCreate(protocol: string): Promise<void> {
  const target = protocolDef(protocol)
  if (!target) return
  def.value = target
  editing.value = null
  resetForm()
  let tpl: Record<string, any> = {}
  try {
    tpl = await fetchInboundTemplate(protocol)
  } catch {
    // 兜底：模板拉不到也必须能打开弹框
  }
  Object.assign(form, { name: '', listen: '::', listenPort: undefined }, templateShells(tpl))
  if (target.tls) void reloadCerts()
  visible.value = true
}

/** 编辑：回填现有监听与配置 */
function openEdit(row: Inbound): void {
  const target = protocolDef(row.protocol)
  if (!target) return
  def.value = target
  editing.value = row
  resetForm()
  // row.config 是表格行里的响应式 Proxy，structuredClone 会抛 DataCloneError，用 copy 深拷贝
  const cfg = copy(row.config ?? {}) as Record<string, any>
  if (target.tls && !cfg.tls) cfg.tls = { enabled: false }
  // 协议要求必须 TLS 的旧记录（历史数据可能关着）：编辑时直接修正为开启
  if (target.tlsRequired) cfg.tls.enabled = true
  // 旧记录缺少新字段的嵌套结构：只补「功能关闭」的空壳，不套用新建默认值
  // （Hysteria2 混淆默认开只作用于新建，编辑已有入站必须保持原值）
  if (!cfg.multiplex) cfg.multiplex = { enabled: false, padding: false }
  if (cfg.tls && !cfg.tls.ech) cfg.tls.ech = { enabled: false }
  if (row.protocol === 'hysteria2' && !cfg.masquerade) cfg.masquerade = { type: '' }
  Object.assign(form, { name: row.name, listen: row.listen, listenPort: row.listenPort }, cfg)
  if (target.tls) void reloadCerts()
  visible.value = true
}

async function reloadCerts(): Promise<void> {
  try {
    certs.value = await listCerts()
  } catch {
    // 静默：证书列表加载失败时选择区为空，表单内已有引导提示
  }
}

/** TLS 前置依赖快捷跳转：展开分组卡、滚动到位并高亮脉冲（无滚动余量的视口下高亮是唯一可见反馈） */
async function jumpToTls(): Promise<void> {
  tlsCollapsed.value = false
  tlsHighlighted.value = true
  clearTimeout(tlsHighlightTimer)
  tlsHighlightTimer = setTimeout(() => (tlsHighlighted.value = false), 1600)
  await nextTick()
  const el = tlsCard.value?.$el as HTMLElement | undefined
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  el?.querySelector<HTMLElement>('.form-group-card__header')?.focus({ preventScroll: true })
}

// 高亮脉冲是一次性定时器，组件卸载时一并清掉
onBeforeUnmount(() => clearTimeout(tlsHighlightTimer))

/** 前往证书页：关闭当前弹框并跳转（证书上传 / 一键自签都在证书页完成） */
function goCerts(): void {
  visible.value = false
  void router.push('/certs')
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const { name, listen, listenPort, ...config } = JSON.parse(JSON.stringify(form))
    // 协议要求必须 TLS：开关不可关闭，保存口径也强制为开启（双保险）
    if (def.value.tlsRequired) config.tls = { ...(config.tls ?? {}), enabled: true }
    const payload: InboundPayload = {
      name: name ?? '',
      protocol: def.value.protocol,
      listen: (listen ?? '::') as ListenAddress,
      // 端口留空（undefined）时后端分配随机空闲端口
      listenPort: listenPort || undefined,
      config,
    }
    const saved = editing.value ? await updateInbound(editing.value.id, payload) : await createInbound(payload)
    visible.value = false
    // 保存前后端口径一致的弱配置警示：保存时明确提示但不阻断
    if (saved.warnings?.length) message.warn(saved.warnings.join('；'))
    else message.success(editing.value ? '入站已更新' : `入站已创建，监听 ${saved.listenPort} 端口`)
    emit('saved', saved)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

defineExpose({ openCreate, openEdit })
</script>

<template>
  <u-dialog v-model="visible" :title="dialogTitle" :style="dialogStyle(1120)">
    <!-- 语义说明：解释这里配的是什么 -->
    <div class="form-intro">
      这里配置一个<b>真实监听</b>：协议、监听地址与端口，以及连接层参数（TLS / Reality / 混淆 / 拥塞算法）。
      该端口由本入站下的所有用户共享；用户在「订阅用户」页勾选本入站时自动生成独立凭据，用户名即客户端订阅里的节点名。
    </div>

    <div v-if="warnings.length" class="form-warnings">
      <div v-for="w in warnings" :key="w.text" class="form-warning">
        <span class="form-warning-text">⚠ {{ w.text }}</span>
        <u-button v-if="w.jump === 'tls'" text type="primary" @click="jumpToTls">去配置 TLS</u-button>
      </div>
    </div>

    <u-form ref="formRef" :model="form" :cols="1" label-width="112px">
      <FormGroupCard title="基础配置">
        <u-form-item
          label="名称"
          field="name"
          tips="入站备注名，展示于列表与订阅页；留空自动按 协议-序号 生成。用户默认以此名拼接作为节点名"
        >
          <u-input v-model="form.name" placeholder="留空自动按 协议-序号 生成" clearable />
        </u-form-item>
        <u-form-item
          label="监听地址"
          field="listen"
          :rules="{ required: '请选择监听地址' }"
          tips="入站对外监听的网卡地址。前置了 Nginx 等反向代理且只需要本机转发时选「仅本机」；QUIC 类协议（Hysteria2/TUIC）无法经普通反代转发，需选双栈或仅 IPv4 直连"
        >
          <u-select v-model="form.listen" :options="LISTEN_OPTIONS" />
        </u-form-item>
        <u-form-item
          label="监听端口"
          field="listenPort"
          tips="本入站的监听端口，其下所有用户共用同一个端口。创建时留空由后端自动分配 10000-60000 之间的随机空闲端口；编辑时留空保持原端口"
        >
          <u-number-input v-model="form.listenPort" :min="1" :max="65535" placeholder="留空自动分配随机空闲端口" />
        </u-form-item>
        <SchemaField v-for="d in def.basic" :key="d.field" :def="d" :model="form" :certs="certs" />
      </FormGroupCard>

      <FormGroupCard
        v-if="def.tls"
        ref="tlsCard"
        v-model:collapsed="tlsCollapsed"
        title="TLS / 证书"
        :highlight="tlsHighlighted"
      >
        <!-- 证书引导：没有证书时给出完整引导；已有证书也保留「没有合适的证书」的跳转提示 -->
        <p v-if="!certs.length" class="tls-empty">
          暂无面板存储证书：可
          <u-button text type="primary" @click="goCerts">前往证书页</u-button>
          一键生成自签或上传/粘贴 PEM，也可把下方「证书引用」切换为服务器本地路径手动填写。
        </p>
        <p v-else class="tls-empty">
          没有合适的证书？可
          <u-button text type="primary" @click="goCerts">前往证书页</u-button>
          生成自签或上传；证书文件已在服务器上时，把「证书引用」切换为服务器本地路径即可。
        </p>
        <!-- 协议要求必须 TLS：开关锁死为开启，不给「能关掉再报错」的错觉 -->
        <u-form-item
          v-if="def.tlsRequired"
          label="启用 TLS"
          tips="该协议的传输层必须经 TLS 加密（协议要求，不可关闭）；选择面板证书后服务器名称自动带出"
        >
          <u-switch :model-value="true" disabled />
        </u-form-item>
        <SchemaField v-for="d in tlsFields" :key="d.field" :def="d" :model="form" :certs="certs" />
      </FormGroupCard>

      <FormGroupCard v-if="advancedFields.length" v-model:collapsed="advancedCollapsed" title="高级选项">
        <SchemaField v-for="d in advancedFields" :key="d.field" :def="d" :model="form" :certs="certs" />
      </FormGroupCard>

      <div v-if="notes.length" class="form-notes">
        <div v-for="n in notes" :key="n" class="form-note">· {{ n }}</div>
      </div>
    </u-form>

    <template #footer="{ close }">
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>

<style scoped>
.form-intro {
  margin-bottom: 14px;
  padding: 10px 12px;
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-hover);
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.form-warnings {
  display: grid;
  gap: 6px;
  margin-bottom: 14px;
  padding: 10px 12px;
  border: 1px solid var(--u-color-warning);
  border-radius: var(--u-radius-small);
  background: var(--u-color-warning-a-8);
}

.form-warning {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.form-warning-text {
  font-size: var(--u-font-size-main-small);
  line-height: 1.6;
  color: var(--u-color-warning);
}

.form-notes {
  margin-top: 4px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.tls-empty {
  grid-column: 1 / -1;
  margin: 0 0 10px;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
