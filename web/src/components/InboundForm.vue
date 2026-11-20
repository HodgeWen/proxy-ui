<!-- 入站声明式表单引擎：按协议 schema 渲染分组卡片（基础 / TLS 证书 / 高级），
     UFormItem tips 直出 schema description，弱配置警示内联展示并支持一键跳转 TLS 区块 -->
<script setup lang="ts">
import type { Certificate } from '../api/cert'
import { listCerts } from '../api/cert'
import { createInbound, updateInbound } from '../api/inbound'
import type { Inbound, InboundPayload } from '../api/inbound'
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
  message,
} from '@veltra/desktop'
import type { Component, PropType } from 'vue'
import { o } from '@cat-kit/core'
import { computed, defineComponent, h, nextTick, reactive, ref, shallowRef, useTemplateRef } from 'vue'

import FormGroupCard from './FormGroupCard.vue'
import {
  PROTOCOL_DEFS,
  TLS_ADVANCED,
  TLS_FIELDS,
  certOptions,
  computeInboundWarnings,
  defaultInboundConfig,
  protocolDef,
} from '../inbound-schema'
import type { FieldDef, FieldType, ProtocolDef } from '../inbound-schema'

const emit = defineEmits<{ saved: [inbound: Inbound]; 'manage-certs': [] }>()

/** 控件类型映射 */
const CONTROLS: Record<FieldType, Component> = {
  input: UInput,
  password: UPasswordInput,
  number: UNumberInput,
  select: USelect,
  multiselect: UMultiSelect,
  switch: USwitch,
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
const tlsCard = useTemplateRef<InstanceType<typeof FormGroupCard>>('tlsCard')

const dialogTitle = computed(() =>
  editing.value ? `编辑入站 · ${editing.value.name}` : `新建${def.value.name} 入站`,
)
/** 保存前实时计算的弱配置警示 */
const warnings = computed(() => computeInboundWarnings(def.value.protocol, form))
const advancedFields = computed(() => [...def.value.advanced, ...(def.value.tls ? TLS_ADVANCED : [])])

function resetForm(): void {
  Object.keys(form).forEach((key) => delete form[key])
  tlsCollapsed.value = false
  advancedCollapsed.value = true
  formRef.value?.clearValidate()
}

/** 新建：按协议预填默认值（随机凭据本地生成） */
function openCreate(protocol: string): void {
  const target = protocolDef(protocol)
  if (!target) return
  def.value = target
  editing.value = null
  resetForm()
  Object.assign(form, { name: '', port: target.defaultPort }, defaultInboundConfig(protocol))
  if (target.tls) void reloadCerts()
  visible.value = true
}

/** 编辑：回填现有配置 */
function openEdit(row: Inbound): void {
  const target = protocolDef(row.protocol)
  if (!target) return
  def.value = target
  editing.value = row
  resetForm()
  const cfg = structuredClone(row.config ?? {})
  if (target.tls && !cfg.tls) cfg.tls = { enabled: false }
  Object.assign(form, { name: row.name, port: row.listenPort }, cfg)
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

/** TLS 前置依赖快捷跳转：展开 TLS 分组卡片并滚动到位 */
async function jumpToTls(): Promise<void> {
  tlsCollapsed.value = false
  await nextTick()
  tlsCard.value?.$el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const { name, port, ...config } = JSON.parse(JSON.stringify(form))
    const payload: InboundPayload = {
      name: name ?? '',
      protocol: def.value.protocol,
      listenPort: Number(port),
      config,
    }
    const saved = editing.value ? await updateInbound(editing.value.id, payload) : await createInbound(payload)
    visible.value = false
    // 保存前后端口径一致的弱配置警示：保存时明确提示但不阻断
    if (saved.warnings?.length) message.warn(saved.warnings.join('；'))
    else message.success(editing.value ? '入站已更新' : '入站已创建')
    emit('saved', saved)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

defineExpose({ openCreate, openEdit, reloadCerts })
</script>

<template>
  <u-dialog v-model="visible" :title="dialogTitle" style="width: 1120px; max-width: 96vw">
    <div class="dialog-scroll">
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
            tips="入站备注名，展示于列表与统计；留空自动按 协议-端口 生成"
          >
            <u-input v-model="form.name" placeholder="留空自动按 协议-端口 生成" clearable />
          </u-form-item>
          <u-form-item
            label="监听端口"
            field="port"
            :rules="{ required: '请输入监听端口' }"
            tips="内核监听的端口，客户端按此端口连接；需未被其它服务占用"
          >
            <u-number-input v-model="form.port" :min="1" :max="65535" />
          </u-form-item>
          <SchemaField v-for="d in def.basic" :key="d.field" :def="d" :model="form" :certs="certs" />
        </FormGroupCard>

        <FormGroupCard v-if="def.tls" ref="tlsCard" v-model:collapsed="tlsCollapsed" title="TLS / 证书">
          <p v-if="!certs.length" class="tls-empty">
            暂无面板存储证书，可引用服务器本地路径，或
            <u-button text type="primary" @click="emit('manage-certs')">打开证书管理</u-button>
            上传/粘贴证书。
          </p>
          <SchemaField v-for="d in TLS_FIELDS" :key="d.field" :def="d" :model="form" :certs="certs" />
        </FormGroupCard>

        <FormGroupCard
          v-if="advancedFields.length"
          v-model:collapsed="advancedCollapsed"
          title="高级选项"
        >
          <SchemaField v-for="d in advancedFields" :key="d.field" :def="d" :model="form" :certs="certs" />
        </FormGroupCard>
      </u-form>
    </div>

    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.dialog-scroll {
  max-height: calc(90vh - 160px);
  overflow-y: auto;
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

.tls-empty {
  grid-column: 1 / -1;
  margin: 0 0 10px;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}
</style>
