<!-- 自定义出站新建 / 编辑弹窗：基础配置 + 附加配置 JSON 与样例填入 + 出站类型速查 -->
<script setup lang="ts">
import {
  UButton,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UNumberInput,
  USelect,
  UTextarea,
  message,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, nextTick, reactive, shallowRef, useTemplateRef, watch } from 'vue'

import { createOutbound, updateOutbound } from '../api/outbound'
import type { Outbound, OutboundPayload } from '../api/outbound'
import { confirmAction } from '../confirm-action'
import { dialogStyle } from '../dialog-style'
import { DEFAULT_TYPE, TYPE_NOTES, typeOptions } from '../outbound-type-notes'
import DialogFooter from './DialogFooter.vue'
import FormGroupCard from './FormGroupCard.vue'

const visible = defineModel<boolean>('visible', { default: false })
const { editing = null } = defineProps<{ editing?: Outbound | null }>()
const emit = defineEmits<{ saved: [] }>()

function defaultOutboundForm() {
  return {
    name: '',
    type: DEFAULT_TYPE,
    server: '',
    serverPort: undefined as number | undefined,
    extraJson: '',
  }
}

const saving = shallowRef(false)
const formRef = useTemplateRef<FormExposed>('formRef')
/** 高级选项分组默认折叠（基础配置关键项默认展开） */
const advancedCollapsed = shallowRef(true)
/** 类型速查默认折叠：只在需要横向比对类型时展开 */
const cheatsheetCollapsed = shallowRef(true)
const form = reactive(defaultOutboundForm())

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

watch(visible, (open) => {
  if (!open) return
  const cfg = (editing?.config ?? {}) as Record<string, unknown>
  const { server, server_port: serverPort, ...extra } = cfg
  Object.assign(
    form,
    defaultOutboundForm(),
    editing
      ? {
          name: editing.name,
          type: editing.type,
          server: typeof server === 'string' ? server : '',
          serverPort: typeof serverPort === 'number' ? serverPort : undefined,
          extraJson: Object.keys(extra).length ? JSON.stringify(extra, null, 2) : '',
        }
      : {},
  )
  advancedCollapsed.value = true
  cheatsheetCollapsed.value = true
  void nextTick(() => formRef.value?.clearValidate())
})

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const extra = form.extraJson.trim() ? (JSON.parse(form.extraJson) as Record<string, unknown>) : {}
    const config: Record<string, unknown> = { ...extra }
    if (form.server) config.server = form.server
    if (form.serverPort) config.server_port = form.serverPort
    const payload: OutboundPayload = { name: form.name, type: form.type, config }
    if (editing) await updateOutbound(editing.id, payload)
    else await createOutbound(payload)
    message.success(editing ? '出站已更新' : '出站已创建')
    visible.value = false
    emit('saved')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <u-dialog
    v-model="visible"
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
/* code-block 为 App.vue 全局类 */

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
