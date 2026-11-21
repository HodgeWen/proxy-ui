<!-- 路由规则卡片：匹配对象 × 匹配方式 + 匹配值 + 出站，排序、增删与启停；新建 / 编辑弹窗在 RuleEditDialog -->
<script setup lang="ts">
import { deleteRule, reorderRules, setRuleEnabled } from '../api/rule'
import type { RuleRecord, RuleSet } from '../api/rule'
import {
  UAction,
  UActionGroup,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  USwitch,
  UTable,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import { shallowRef } from 'vue'

import { useEnabledOverride } from '../composables/useEnabledOverride'
import { matchMethodLabel } from '../rule-labels'
import EmptyState from './EmptyState.vue'
import RuleEditDialog from './RuleEditDialog.vue'

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

const columns = defineTableColumns([
  { key: 'sort', name: '排序', width: 124, align: 'center' },
  { key: 'match', name: '匹配条件', minWidth: 260 },
  { key: 'outbound', name: '出站', width: 160 },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 140, align: 'center', fixed: 'right' },
])

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

/** 启停走本地覆写表驱动开关（USwitch 失败不下刷的 workaround，详见 useEnabledOverride） */
const { rowEnabled, togglingId, toggle: handleToggle } = useEnabledOverride(
  () => props.rules,
  async (row, enabled) => {
    await setRuleEnabled(row.id, enabled)
    emit('changed')
  },
)

async function handleDelete(row: RuleRecord): Promise<void> {
  try {
    await deleteRule(row.id)
    message.success('规则已删除')
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}

// ---------- 新建 / 编辑弹窗 ----------

const formVisible = shallowRef(false)
const editing = shallowRef<RuleRecord | null>(null)

function openCreate(): void {
  editing.value = null
  formVisible.value = true
}

function openEdit(row: RuleRecord): void {
  editing.value = row
  formVisible.value = true
}
</script>

<template>
  <u-card>
    <u-card-header>
      <!-- 页签已表明对象，卡片头只留主操作工具栏 -->
      <div class="card-ops">
        <!-- 列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
        <u-button v-if="rules.length" type="primary" @click="openCreate">新建规则</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <!-- 不用 text-ellipsis：它会给含按钮的操作列也挂悬浮提示；只在匹配条件单元格内自行截断 -->
      <u-table :columns="columns" :data="rules" row-key="id" stripe>
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无规则"
            description="规则决定什么流量走哪个出站。可先在「兜底与模板」页签应用预设模板一键生成常用规则（会覆盖现有），再逐条增改"
            cta-text="新建规则"
            @cta="openCreate"
          />
        </template>
        <template #column:sort="{ row: { index } }">
          <u-button text size="small" :disabled="moving || index === 0" @click="move(index, -1)">上移</u-button>
          <u-button
            text
            size="small"
            :disabled="moving || index === rules.length - 1"
            @click="move(index, 1)"
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

  <RuleEditDialog
    v-model:visible="formVisible"
    :editing="editing"
    :rule-sets="ruleSets"
    :outbound-options="outboundOptions"
    @changed="emit('changed')"
    @maintain-rule-set="(ruleSet: RuleSet) => emit('maintainRuleSet', ruleSet)"
  />
</template>

<style scoped>
/* card-ops 为 App.vue 全局类（卡片头工具栏，靠右与表格操作列对齐） */

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
</style>
