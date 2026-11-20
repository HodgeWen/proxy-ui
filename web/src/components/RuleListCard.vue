<!-- 路由规则卡片：匹配对象 × 匹配方式 + 匹配值 + 出站，排序、增删改启停与编辑弹窗 -->
<script setup lang="ts">
import { createRule, deleteRule, reorderRules, setRuleEnabled, updateRule } from '../api/rule'
import type { RuleMatchMethod, RuleMatchType, RulePayload, RuleRecord, RuleSet } from '../api/rule'
import {
  UAction,
  UActionGroup,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  USelect,
  USwitch,
  UTable,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, reactive, shallowRef, watch } from 'vue'

import EmptyState from './EmptyState.vue'
import { dialogStyle } from '../dialog-style'

const props = defineProps<{
  rules: RuleRecord[]
  ruleSets: RuleSet[]
  outboundOptions: { label: string; value: string }[]
  /** 首屏加载中：不渲染空状态，避免闪一下「暂无规则」 */
  loading?: boolean
}>()
const emit = defineEmits<{
  changed: []
  /** 从规则弹窗跳到对应规则集的内容项维护 */
  maintainRuleSet: [ruleSet: RuleSet]
}>()

const moving = shallowRef(false)
const togglingId = shallowRef(0)

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

const columns = defineTableColumns([
  { key: 'sort', name: '排序', width: 124, align: 'center' },
  { key: 'match', name: '匹配条件', minWidth: 260 },
  { key: 'outbound', name: '出站', width: 160 },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 140, align: 'center', fixed: 'right' },
])

function matchMethodLabel(method: string): string {
  switch (method) {
    case 'exact':
      return '精确'
    case 'suffix':
      return '后缀'
    case 'keyword':
      return '关键字'
    case 'regex':
      return '正则'
    case 'cidr':
      return '网段'
    default:
      return method
  }
}

/** 规则的一句话匹配条件：「域名后缀 · google.com」「规则集 · anti-ad」 */
function matchText(rule: RuleRecord): string {
  if (rule.matchType === 'rule_set') {
    return `规则集 · ${rule.ruleSetName || '已删除的规则集'}`
  }
  if (rule.matchType === 'ip') return `IP 网段 · ${rule.value}`
  return `域名${matchMethodLabel(rule.matchMethod)} · ${rule.value}`
}

function outboundLabel(outbound: string): string {
  return props.outboundOptions.find((o) => o.value === outbound)?.label ?? outbound
}

// ---------- 排序与启停 ----------

/** 上移/下移：按当前位置交换后整体提交顺序 */
async function move(index: number, delta: number): Promise<void> {
  const target = index + delta
  if (index < 0 || target < 0 || target >= props.rules.length) return
  const ids = props.rules.map((r) => r.id)
  ;[ids[index], ids[target]] = [ids[target], ids[index]]
  moving.value = true
  try {
    await reorderRules(ids)
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    moving.value = false
  }
}

/**
 * 启停：USwitch 内部是原生 checkbox，请求失败时父级数据不变、`:checked` 不会重新下刷，
 * 会和 DOM 状态错位（下一次点击发出的仍是旧值）。用本地覆写表驱动开关，
 * 失败时写回相反值即触发一次真实的下刷；回读后由 watch 清空覆写。
 */
const enabledOverride = shallowRef<Record<number, boolean>>({})

function rowEnabled(row: RuleRecord): boolean {
  return enabledOverride.value[row.id] ?? row.enabled
}

// 父级回读出新数据后，覆写表已完成使命，清空以免展示过期状态
watch(
  () => props.rules,
  () => {
    enabledOverride.value = {}
  },
)

async function handleToggle(row: RuleRecord, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  enabledOverride.value = { ...enabledOverride.value, [row.id]: enabled }
  try {
    await setRuleEnabled(row.id, enabled)
    emit('changed')
  } catch {
    // 失败回滚：写回相反值让开关立即恢复
    enabledOverride.value = { ...enabledOverride.value, [row.id]: !enabled }
  } finally {
    togglingId.value = 0
  }
}

async function handleDelete(row: RuleRecord): Promise<void> {
  try {
    await deleteRule(row.id)
    message.success('规则已删除')
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}

// ---------- 编辑弹窗 ----------

const formVisible = shallowRef(false)
const saving = shallowRef(false)
const editing = shallowRef<RuleRecord | null>(null)
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

/** 切换匹配对象：校正匹配方式；匹配值在域名与网段之间互不通用，跨类切换时清空 */
function handleMatchTypeChange(type?: RuleMatchType): void {
  if (!type || type === form.matchType) return
  form.matchType = type
  form.value = ''
  form.matchMethod = type === 'ip' ? 'cidr' : type === 'rule_set' ? '' : 'suffix'
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, {
    matchType: 'domain',
    matchMethod: 'suffix',
    value: '',
    ruleSetId: undefined,
    outbound: defaultOutbound.value,
  })
  formVisible.value = true
}

function openEdit(row: RuleRecord): void {
  editing.value = row
  Object.assign(form, {
    matchType: row.matchType,
    matchMethod: row.matchMethod || (row.matchType === 'ip' ? 'cidr' : row.matchType === 'rule_set' ? '' : 'suffix'),
    value: row.value,
    ruleSetId: row.ruleSetId || undefined,
    outbound: row.outbound,
  })
  formVisible.value = true
}

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
    if (editing.value) await updateRule(editing.value.id, payload)
    else await createRule(payload)
    message.success(editing.value ? '规则已更新' : '规则已创建')
    formVisible.value = false
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
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>路由规则</span>
        <div class="page-ops">
          <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
          <u-button v-if="rules.length" type="primary" @click="openCreate">新建规则</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <div class="card-desc">
        每条规则 = 匹配对象 × 匹配方式 + 匹配值 → 出站，流量自上而下逐条匹配、命中即走对应出站；顺序影响结果，可用「上移 / 下移」调整。
      </div>
      <!-- 不用 text-ellipsis：它会给含按钮的操作列也挂悬浮提示；只在匹配条件单元格内自行截断 -->
      <u-table :columns="columns" :data="rules" row-key="id" stripe>
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无规则"
            description="规则决定什么流量走哪个出站。可先应用上方预设模板一键生成常用规则（会覆盖现有），再逐条增改"
            cta-text="新建规则"
            @cta="openCreate"
          />
        </template>
        <template #column:sort="{ row }">
          <u-button text size="small" :disabled="moving || row.index === 0" @click="move(row.index, -1)">上移</u-button>
          <u-button
            text
            size="small"
            :disabled="moving || row.index === rules.length - 1"
            @click="move(row.index, 1)"
          >
            下移
          </u-button>
        </template>
        <template #column:match="{ rowData }">
          <span
            class="match-text"
            :class="{ 'is-off': !rowData.enabled, 'is-ruleset': rowData.matchType === 'rule_set' }"
            :title="matchText(rowData)"
          >
            {{ matchText(rowData) }}
          </span>
        </template>
        <template #column:outbound="{ rowData }">
          <u-tag :type="rowData.outbound === 'block' ? 'danger' : 'success'">{{ outboundLabel(rowData.outbound) }}</u-tag>
        </template>
        <template #column:enabled="{ rowData }">
          <!-- USwitch 没有 loading，用 disabled 在请求期间挡住重复点击 -->
          <u-switch
            size="small"
            :model-value="rowEnabled(rowData)"
            :disabled="togglingId === rowData.id"
            @update:model-value="(v: boolean) => handleToggle(rowData, v)"
          />
        </template>
        <template #column:actions="{ rowData }">
          <!-- 删除用气泡确认，不在 run 里再叠 messageConfirm，保持一次确认 -->
          <u-action-group :max="2">
            <u-action @run="openEdit(rowData)">编辑</u-action>
            <u-action type="danger" need-confirm @run="handleDelete(rowData)">删除</u-action>
          </u-action-group>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? '编辑规则' : '新建规则'" :style="dialogStyle(600)">
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
        tips="规则集在下方「规则集」卡片维护：一个规则集可有多条来源，同一时刻只有一条启用，规则只需引用规则集本身"
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
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.card-desc {
  margin-bottom: 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
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

/* 匹配条件列：单行截断，完整值靠 title 悬浮查看（表格级 text-ellipsis 会污染操作列） */
.match-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

/* 停用的规则不参与匹配，弱化以示区分 */
.match-text.is-off {
  color: var(--u-text-color-second);
  font-weight: 400;
}

/* 规则集引用非手写值，弱化以区分「规则集 · xxx」与域名条件 */
.match-text.is-ruleset {
  font-weight: 400;
  color: var(--u-text-color-main);
}

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
