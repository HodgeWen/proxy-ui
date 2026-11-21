<!-- 路由规则新建 / 编辑弹窗：匹配对象 × 匹配方式 + 匹配值（或规则集引用）+ 出站，
     底部附一句话实时预览；「去维护规则集」经 emit 跳到规则集内容项维护 -->
<script setup lang="ts">
import { createRule, updateRule } from '../api/rule'
import type { RuleMatchMethod, RuleMatchType, RulePayload, RuleRecord, RuleSet } from '../api/rule'
import { UButton, UDialog, UForm, UFormItem, UInput, USelect, message } from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, reactive, shallowRef, watch } from 'vue'

import { dialogStyle } from '../dialog-style'
import { matchMethodLabel } from '../rule-labels'
import DialogFooter from './DialogFooter.vue'

const visible = defineModel<boolean>('visible', { default: false })
const props = defineProps<{
  /** 正在编辑的规则；null 为新建 */
  editing: RuleRecord | null
  ruleSets: RuleSet[]
  outboundOptions: { label: string; value: string }[]
}>()
const emit = defineEmits<{
  changed: []
  /** 从规则弹窗跳到对应规则集的内容项维护 */
  maintainRuleSet: [ruleSet: RuleSet]
}>()

const saving = shallowRef(false)
const formRef = shallowRef<FormExposed>()
const form = reactive<{
  matchType: RuleMatchType
  matchMethod: RuleMatchMethod | ''
  value: string
  ruleSetId?: number
  outbound: string
}>({
  matchType: 'domain',
  matchMethod: 'suffix',
  value: '',
  ruleSetId: undefined,
  outbound: 'direct',
})

/** 匹配对象选项 */
const matchTypeOptions = [
  { label: '域名', value: 'domain' },
  { label: 'IP', value: 'ip' },
  { label: '规则集', value: 'rule_set' },
]

/** 域名匹配方式选项，附一句命中语义供下拉直接读懂 */
const domainMethodOptions = [
  { label: '精确（仅该域名本身）', value: 'exact' },
  { label: '后缀（该域名及其所有子域）', value: 'suffix' },
  { label: '关键字（域名含该词即命中）', value: 'keyword' },
  { label: '正则（按正则匹配域名）', value: 'regex' },
]

/** 规则集下拉选项：附启用中的来源备注，未配置来源时明示 */
const ruleSetOptions = computed(() =>
  props.ruleSets.map((set) => {
    const active = set.items.find((item) => item.enabled)
    return { label: `${set.name}（${active?.remark || '未配置来源'}）`, value: set.id }
  }),
)

/** 当前选中的规则集，用于「去维护规则集」跳转 */
const selectedRuleSet = computed(() => props.ruleSets.find((set) => set.id === form.ruleSetId) ?? null)

/** 规则集下拉兜底文案：引用已被删除的规则集时仍显示可读信息，而不是空下拉 */
const ruleSetText = computed(() => {
  if (!form.ruleSetId) return undefined
  return selectedRuleSet.value?.name ?? `已删除的规则集 #${form.ruleSetId}`
})

/** 出站选择 tips：说明三类去向，并给出「通常指向策略组」的主流用法 */
const outboundTips = computed(() => {
  const hasGroup = props.outboundOptions.some((o) => o.label.endsWith('（策略组）'))
  return hasGroup
    ? '命中后流量从哪出去：direct=直连不走代理；block=拒绝连接（广告拦截就是它）；其余为自定义出站或策略组。主流用法是指向策略组，客户端里可自行切换节点'
    : '命中后流量从哪出去：direct=直连不走代理；block=拒绝连接（广告拦截就是它）；其余为自定义出站或策略组'
})

/** 新建规则的默认出站：已有策略组时取第一个（主流做法是规则指向策略组），否则 direct */
const defaultOutbound = computed(
  () => props.outboundOptions.find((o) => o.label.endsWith('（策略组）'))?.value ?? 'direct',
)

const valuePlaceholder = computed(() => {
  if (form.matchType === 'ip') return '10.0.0.0/8；多个用英文逗号分隔'
  switch (form.matchMethod) {
    case 'exact':
    case 'suffix':
      return 'google.com'
    case 'keyword':
      return 'google'
    case 'regex':
      return '^ads?\\.'
    default:
      return '请输入匹配值'
  }
})

const valueTips = computed(() => {
  if (form.matchType === 'ip') {
    return '按网段匹配：填写 CIDR，如 10.0.0.0/8、192.168.0.0/16；多个网段用英文逗号分隔，命中任一即生效'
  }
  switch (form.matchMethod) {
    case 'exact':
      return '只命中这个域名本身，不含其子域：填 google.com 不会命中 mail.google.com'
    case 'suffix':
      return '命中该域名及其所有子域：填 google.com 同时命中 mail.google.com、www.google.com'
    case 'keyword':
      return '域名中只要出现该词就命中：填 google 会命中 google.com、googleapis.com 等'
    case 'regex':
      return '按正则表达式匹配完整域名：如 ^ads?\\. 命中 ad. 与 ads. 开头的域名'
    default:
      return '多个值用英文逗号分隔，命中任一即生效'
  }
})

/** 出站选择的直白描述，用于弹窗底部的实时预览 */
function outboundPreview(outbound: string): string {
  if (outbound === 'direct') return '直连'
  if (outbound === 'block') return '拦截（拒绝连接）'
  const label = props.outboundOptions.find((o) => o.value === outbound)?.label ?? outbound
  return label.includes('（策略组）') ? `${outbound} 策略组` : `${outbound} 出站`
}

/** 一句话中文预览：把当前表单翻译成「命中什么 → 走哪里」 */
const preview = computed(() => {
  const target = outboundPreview(form.outbound)
  if (form.matchType === 'rule_set') {
    const name = selectedRuleSet.value?.name
    return name
      ? `命中 规则集 ${name} 的流量 → 走 ${target}`
      : `选择规则集后：命中该规则集内的流量 → 走 ${target}`
  }
  const input = form.value.trim().replace(/\s*,\s*/g, '、')
  const kind = form.matchType === 'ip' ? 'IP 网段' : `域名${matchMethodLabel(form.matchMethod)}`
  if (!input) return `填写${form.matchType === 'ip' ? '网段' : '域名'}后：命中的流量 → 走 ${target}`
  return `命中 ${kind} ${input} 的流量 → 走 ${target}`
})

/** 匹配方式按匹配对象取默认：IP=网段、规则集=无、域名=后缀 */
function methodForType(type: RuleMatchType): RuleMatchMethod | '' {
  if (type === 'ip') return 'cidr'
  if (type === 'rule_set') return ''
  return 'suffix'
}

/** 切换匹配对象：校正匹配方式；匹配值在域名与网段之间互不通用，跨类切换时清空 */
function handleMatchTypeChange(type?: RuleMatchType): void {
  if (!type || type === form.matchType) return
  form.matchType = type
  form.value = ''
  form.matchMethod = methodForType(type)
}

// 打开即回填：新建给默认值（默认出站取当前候选里的第一个策略组），编辑按行数据
watch(visible, (open) => {
  if (!open) return
  const row = props.editing
  Object.assign(form, {
    matchType: row?.matchType ?? 'domain',
    matchMethod: row ? row.matchMethod || methodForType(row.matchType) : 'suffix',
    value: row?.value ?? '',
    ruleSetId: row?.ruleSetId || undefined,
    outbound: row?.outbound ?? defaultOutbound.value,
  })
})

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  const payload: RulePayload =
    form.matchType === 'rule_set'
      ? { matchType: form.matchType, ruleSetId: form.ruleSetId, outbound: form.outbound }
      : {
          matchType: form.matchType,
          matchMethod: form.matchMethod,
          value: form.value.trim(),
          outbound: form.outbound,
        }
  try {
    if (props.editing) await updateRule(props.editing.id, payload)
    else await createRule(payload)
    message.success(props.editing ? '规则已更新' : '规则已创建')
    visible.value = false
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

/** 去维护规则集：叠一层弹窗，保留当前规则草稿；维护完规则集选项自动刷新 */
function handleMaintain(): void {
  const ruleSet = selectedRuleSet.value
  if (ruleSet) emit('maintainRuleSet', ruleSet)
}
</script>

<template>
  <u-dialog v-model="visible" :title="editing ? '编辑规则' : '新建规则'" :style="dialogStyle(600)">
    <u-form ref="formRef" :model="form" :cols="1" label-width="92px">
      <u-form-item
        label="匹配对象"
        tips="拿什么去比对流量：域名=按域名匹配；IP=按目标 IP 网段匹配；规则集=引用规则集里维护的一整批域名 / IP"
      >
        <!-- 匹配对象是规则的骨架，清空只会让界面回落到原始英文值，故禁止清除 -->
        <u-select
          :model-value="form.matchType"
          :options="matchTypeOptions"
          :clearable="false"
          @update:model-value="handleMatchTypeChange"
        />
      </u-form-item>

      <!-- IP 固定按网段匹配，展示为不可改的说明行而非可选下拉 -->
      <u-form-item v-if="form.matchType === 'ip'" label="匹配方式">
        <span class="fixed-method">网段（CIDR）</span>
      </u-form-item>
      <u-form-item
        v-else-if="form.matchType === 'domain'"
        label="匹配方式"
        field="matchMethod"
        :rules="{ required: '请选择匹配方式' }"
        tips="精确=只命中该域名本身；后缀=该域名及所有子域；关键字=域名含该词即命中；正则=按正则表达式匹配"
      >
        <u-select v-model="form.matchMethod" :options="domainMethodOptions" :clearable="false" filterable />
      </u-form-item>

      <u-form-item
        v-if="form.matchType === 'rule_set'"
        label="规则集"
        field="ruleSetId"
        :rules="{ required: '请选择规则集' }"
        tips="规则集在「规则集」页签维护：一个规则集可有多条来源，同一时刻只有一条启用，规则只需引用规则集本身"
      >
        <div class="ruleset-field">
          <u-select
            v-model="form.ruleSetId"
            :options="ruleSetOptions"
            :text="ruleSetText"
            placeholder="选择规则集"
            filterable
            clearable
            class="ruleset-select"
          />
          <u-button text type="primary" :disabled="!selectedRuleSet" @click="handleMaintain">去维护规则集</u-button>
        </div>
      </u-form-item>

      <u-form-item v-else label="匹配值" field="value" :rules="{ required: '请输入匹配值' }" :tips="valueTips">
        <u-input v-model="form.value" :placeholder="valuePlaceholder" clearable />
      </u-form-item>

      <u-form-item label="出站" field="outbound" :rules="{ required: '请选择出站' }" :tips="outboundTips">
        <!-- text 兜底：规则引用的出站已被删除时仍显示原 tag，不至于看起来空白 -->
        <u-select
          v-model="form.outbound"
          :options="outboundOptions"
          :text="form.outbound || undefined"
          :clearable="false"
          placeholder="选择出站"
          filterable
        />
      </u-form-item>
    </u-form>

    <!-- 实时一句话预览：把当前表单翻译成规则的实际效果 -->
    <div class="preview">{{ preview }}</div>

    <template #footer="{ close }">
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>

<style scoped>
.fixed-method {
  color: var(--u-text-color-second);
}

/* 规则集下拉与「去维护」同一行，下拉占满剩余宽度 */
.ruleset-field {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
}

.ruleset-select {
  flex: 1;
  min-width: 0;
}

/* 预览条：弱底色 + 主色文字，与表单错误区区分 */
.preview {
  margin-top: 4px;
  padding: 8px 12px;
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-hover);
  color: var(--u-text-color-main);
  font-size: var(--u-font-size-main-small);
}
</style>
