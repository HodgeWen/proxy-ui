<!-- 订阅用户新建 / 编辑弹窗：名称、流量配额（数值 + 单位换算）、到期时间（永久 / 日期 + 时刻）、勾选入站；
     打开时按 token 回填并拉取入站列表，保存走创建 / 更新，成功后 emit saved 由父页刷新列表 -->
<script setup lang="ts">
import {
  UDatePicker,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UMultiSelect,
  UNumberInput,
  USelect,
  USwitch,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, reactive, shallowRef, watch } from 'vue'

import { listInbounds } from '../api/inbound'
import type { Inbound } from '../api/inbound'
import { createSubscriptionToken, updateSubscriptionToken } from '../api/subscription'
import type { SubscriptionPayload, SubscriptionToken } from '../api/subscription'
import { dialogStyle } from '../dialog-style'
import { protocolDef } from '../inbound-schema'
import DialogFooter from './DialogFooter.vue'

const visible = defineModel<boolean>('visible', { default: false })
const { token } = defineProps<{ token: SubscriptionToken | null }>()
const emit = defineEmits<{ saved: [editing: boolean] }>()

/** 配额单位；'none' 表示不限 */
type QuotaUnit = 'none' | 'MB' | 'GB' | 'TB'

const QUOTA_UNITS: { label: string; value: QuotaUnit }[] = [
  { label: '不限', value: 'none' },
  { label: 'MB', value: 'MB' },
  { label: 'GB', value: 'GB' },
  { label: 'TB', value: 'TB' },
]

const UNIT_BYTES: Record<Exclude<QuotaUnit, 'none'>, number> = {
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
}

const saving = shallowRef(false)
const formRef = shallowRef<FormExposed>()
const inbounds = shallowRef<Inbound[]>([])

const form = reactive({
  name: '',
  quotaUnit: 'none' as QuotaUnit,
  quotaValue: undefined as number | undefined,
  /** 永久有效（关闭时用 expiry 拼到期时刻） */
  permanent: true,
  expiry: { date: undefined as string | undefined, time: undefined as string | undefined },
  nodeInbounds: [] as number[],
})

/** 可勾选的入站：UMultiSelect 不支持逐项禁用，停用的入站不进选项；
     但该用户已勾选的（编辑回显）仍列出并标注「已停用」，否则管理员取消不掉这个节点 */
const inboundOptions = computed(() =>
  inbounds.value
    .filter((t) => t.enabled || form.nodeInbounds.includes(t.id))
    .map((t) => ({
      label: `${t.name}（${protocolDef(t.protocol)?.name ?? t.protocol}，端口 ${t.listenPort}）${t.enabled ? '' : ' · 已停用'}`,
      value: t.id,
    })),
)
/** 未勾选且已停用的入站：不可勾选，列出来让管理员知道为什么没有它 */
const stoppedInbounds = computed(() =>
  inbounds.value.filter((t) => !t.enabled && !form.nodeInbounds.includes(t.id)),
)
const hasEnabledInbound = computed(() => inbounds.value.some((t) => t.enabled))

/** 切到「不限」时清掉数值，避免残留数字被误读成仍在限制用量 */
function handleQuotaUnitChange(unit: QuotaUnit): void {
  if (unit === 'none') form.quotaValue = undefined
}

const quotaRules = {
  validator: (value: number | undefined) =>
    form.quotaUnit !== 'none' && !value ? '请输入配额数值，或把单位切回「不限」' : '',
}

const expiryRules = {
  validator: (value: string | undefined) =>
    form.permanent || value ? '' : '请选择到期日期，或切换为「永久有效」',
}

/** 字节数拆回「数值 + 单位」，便于编辑回显 */
function splitQuota(bytes: number): { value: number | undefined; unit: QuotaUnit } {
  if (!bytes) return { value: undefined, unit: 'none' }
  if (bytes % UNIT_BYTES.TB === 0) return { value: bytes / UNIT_BYTES.TB, unit: 'TB' }
  if (bytes % UNIT_BYTES.GB === 0) return { value: bytes / UNIT_BYTES.GB, unit: 'GB' }
  // 不足 1 GB 的零头按 MB 回显；不足 1 MB 时保留两位小数，避免回显成 0 后校验不过
  return { value: Math.max(0.01, Math.round((bytes / UNIT_BYTES.MB) * 100) / 100), unit: 'MB' }
}

/** 表单到期时刻拆成日期 + 时间（本地时区） */
function splitDateTime(iso: string): { date: string; time: string } {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return {
    date: `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`,
    time: `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`,
  }
}

function quotaBytes(): number {
  if (form.quotaUnit === 'none') return 0
  return Math.round((form.quotaValue ?? 0) * UNIT_BYTES[form.quotaUnit])
}

/** 到期时刻转 RFC3339；未选日期时为 null */
function expiryISO(): string | null {
  const { date, time } = form.expiry
  return date ? new Date(`${date}T${time || '00:00:00'}`).toISOString() : null
}

// 打开即回填（新建给默认值），随后拉取入站列表供勾选
watch(visible, async (open) => {
  if (!open) return
  const quota = splitQuota(token?.trafficLimitBytes ?? 0)
  const expiry = token?.expiresAt ? splitDateTime(token.expiresAt) : undefined
  form.name = token?.name ?? ''
  form.quotaUnit = quota.unit
  form.quotaValue = quota.value
  form.permanent = !expiry
  form.expiry = { date: expiry?.date, time: expiry?.time }
  // 已勾选的入站按节点的 inboundId 回显
  form.nodeInbounds = token ? token.nodes.map((n) => n.inboundId) : []
  try {
    inbounds.value = await listInbounds()
  } catch {
    // 错误已由请求封装统一提示，入站列表为空时弹窗内会提示先去创建
  }
})

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const payload: SubscriptionPayload = {
      name: form.name.trim(),
      trafficLimitBytes: quotaBytes(),
      expiresAt: form.permanent ? null : expiryISO(),
      // 永久有效时显式清空到期时间（后端按 clearExpiry 优先处理）
      clearExpiry: form.permanent,
      nodeInbounds: form.nodeInbounds,
    }
    const editing = !!token
    if (token) await updateSubscriptionToken(token.id, payload)
    else await createSubscriptionToken(payload)
    visible.value = false
    emit('saved', editing)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <!-- 新建 / 编辑：名称、配额、到期、勾选的入站 -->
  <u-dialog
    v-model="visible"
    :title="token ? '编辑订阅用户' : '新建订阅用户'"
    :style="dialogStyle(620)"
  >
    <u-form ref="formRef" :model="form" :cols="1" label-width="88px">
      <u-input
        label="名称"
        field="name"
        placeholder="如：我的手机"
        tips="用于辨识设备或使用者；入站里的用户名由「名称-入站名」生成"
        :rules="{ required: '请输入名称' }"
      />
      <u-form-item
        label="流量配额"
        field="quotaValue"
        :rules="quotaRules"
        tips="上+下行累计用量达到配额后，该用户全部节点自动暂停；清零流量后恢复"
      >
        <!-- USelect / UDatePicker / UTimePicker 是多根组件，scoped class 落不到根元素上，
             控制宽度的 class 必须放在原生包裹 div 上，否则弹窗里会被下拉框挤变形 -->
        <div class="field-row">
          <div class="field-grow">
            <u-number-input
              v-model="form.quotaValue"
              :min="0"
              :step="1"
              :disabled="form.quotaUnit === 'none'"
              placeholder="不限"
            />
          </div>
          <div class="field-unit">
            <u-select v-model="form.quotaUnit" :options="QUOTA_UNITS" @update:model-value="handleQuotaUnitChange" />
          </div>
        </div>
      </u-form-item>
      <u-form-item
        label="到期时间"
        field="expiry.date"
        :rules="expiryRules"
        tips="到达该时刻后该用户全部节点自动暂停，续期改这里即可恢复；默认「永久有效」不限制"
      >
        <div class="field-row">
          <u-switch v-model="form.permanent" active-text="永久有效" />
          <template v-if="!form.permanent">
            <div class="field-grow">
              <u-date-picker v-model="form.expiry.date" placeholder="到期日期" />
            </div>
            <div class="field-time">
              <u-time-picker v-model="form.expiry.time" placeholder="时刻" />
            </div>
          </template>
        </div>
      </u-form-item>
      <u-multi-select
        label="勾选入站"
        field="nodeInbounds"
        :options="inboundOptions"
        placeholder="选择该用户可用的入站"
        tips="勾选的每个入站都会为该用户生成一个节点（独立凭据、共用入站端口），订阅里只有这些节点；默认不勾，保存后可随时补勾"
        filterable
      />
      <p class="form-hint">
        勾选或取消入站，会在对应入站里创建 / 删除该用户的身份，节点随之增减；保存即生效，内核自动按新配置重建，无需任何额外操作。
        只改配额或到期时间同样保存即生效，只暂停 / 恢复节点，不影响链接与端口。
      </p>
      <p v-if="!hasEnabledInbound" class="form-hint is-warning">
        暂无启用的入站，请先到「入站」页创建并启用入站，再回来为该用户勾选节点。
      </p>
      <p v-if="stoppedInbounds.length" class="form-hint">
        已停用、不可勾选的入站：{{ stoppedInbounds.map((t) => t.name).join('、') }}
      </p>
    </u-form>

    <template #footer="{ close }">
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>

<style scoped>
.field-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.field-grow {
  flex: 1;
  min-width: 0;
}

.field-unit {
  width: 96px;
  flex-shrink: 0;
}

.field-time {
  width: 120px;
  flex-shrink: 0;
}

.form-hint {
  margin: 0 0 8px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.form-hint.is-warning {
  color: var(--u-color-warning);
}
</style>
