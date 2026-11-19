<!-- 路由规则卡片：规则列表排序、增删改启停与编辑弹窗 -->
<script setup lang="ts">
import { applyKernelConfig } from '../api/kernel'
import { createRule, deleteRule, reorderRules, setRuleEnabled, updateRule } from '../api/rule'
import type { RuleRecord } from '../api/rule'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  UEmpty,
  UForm,
  UInput,
  USelect,
  USwitch,
  UTable,
  UTag,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { reactive, shallowRef } from 'vue'

const props = defineProps<{
  rules: RuleRecord[]
  outboundOptions: { label: string; value: string }[]
}>()
const emit = defineEmits<{ changed: [] }>()

const applying = shallowRef(false)
const moving = shallowRef(false)
const togglingId = shallowRef(0)

/** 规则类型选项（geosite/geoip/rule_set 生成时映射为远程规则集） */
const ruleTypeOptions = [
  { label: '域名', value: 'domain' },
  { label: '域名后缀', value: 'domain_suffix' },
  { label: '域名关键字', value: 'domain_keyword' },
  { label: 'IP-CIDR', value: 'ip_cidr' },
  { label: 'Geosite 规则集', value: 'geosite' },
  { label: 'GeoIP 规则集', value: 'geoip' },
  { label: '规则集', value: 'rule_set' },
]

const columns = defineTableColumns([
  { key: 'sort', name: '排序', width: 96, align: 'center' },
  { key: 'type', name: '类型', width: 140, align: 'center' },
  { key: 'value', name: '匹配值', minWidth: 200 },
  { key: 'outbound', name: '出站', width: 150 },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 130, align: 'center' },
])

function typeLabel(type: string): string {
  return ruleTypeOptions.find((o) => o.value === type)?.label ?? type
}

function outboundLabel(outbound: string): string {
  return props.outboundOptions.find((o) => o.value === outbound)?.label ?? outbound
}

// ---------- 排序与启停 ----------

/** 上移/下移后整体重排 */
async function move(row: RuleRecord, delta: number): Promise<void> {
  const idx = props.rules.findIndex((r) => r.id === row.id)
  const target = idx + delta
  if (idx < 0 || target < 0 || target >= props.rules.length) return
  const ids = props.rules.map((r) => r.id)
  ;[ids[idx], ids[target]] = [ids[target], ids[idx]]
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

async function handleToggle(row: RuleRecord, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  try {
    await setRuleEnabled(row.id, enabled)
    row.enabled = enabled
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    togglingId.value = 0
  }
}

async function handleDelete(row: RuleRecord): Promise<void> {
  const action = await messageConfirm({
    title: '删除规则',
    message: `确认删除规则「${typeLabel(row.type)} ${row.value}」吗？`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
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
const form = reactive({ type: '', value: '', outbound: '' })

function openCreate(): void {
  editing.value = null
  Object.assign(form, { type: 'domain_suffix', value: '', outbound: 'direct' })
  formVisible.value = true
}

function openEdit(row: RuleRecord): void {
  editing.value = row
  Object.assign(form, { type: row.type, value: row.value, outbound: row.outbound })
  formVisible.value = true
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    if (editing.value) await updateRule(editing.value.id, form)
    else await createRule(form)
    message.success(editing.value ? '规则已更新' : '规则已创建')
    formVisible.value = false
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

// ---------- 应用配置 ----------

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
</script>

<template>
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>路由规则</span>
        <div class="page-ops">
          <u-button size="small" :loading="applying" @click="handleApply">应用配置</u-button>
          <u-button size="small" type="primary" @click="openCreate">新建规则</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <u-empty v-if="!rules.length" text="暂无规则，可先应用预设模板或新建规则" block />
      <u-table v-else :columns="columns" :data="rules" row-key="id" border>
        <template #column:sort="{ rowData, rowIndex }">
          <u-button text size="small" :disabled="moving || rowIndex === 0" @click="move(rowData, -1)">上移</u-button>
          <u-button text size="small" :disabled="moving || rowIndex === rules.length - 1" @click="move(rowData, 1)">下移</u-button>
        </template>
        <template #column:type="{ rowData }">
          <u-tag type="info">{{ typeLabel(rowData.type) }}</u-tag>
        </template>
        <template #column:outbound="{ rowData }">
          <u-tag :type="rowData.outbound === 'block' ? 'error' : 'success'">{{ outboundLabel(rowData.outbound) }}</u-tag>
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
          <u-button text type="primary" @click="openEdit(rowData)">编辑</u-button>
          <u-button text type="error" @click="handleDelete(rowData)">删除</u-button>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? '编辑规则' : '新建规则'" style="width: 560px; max-width: 96vw">
    <u-form ref="formRef" :model="form" :cols="1" label-width="92px">
      <u-select label="类型" field="type" :options="ruleTypeOptions" :rules="{ required: '请选择类型' }" />
      <u-input
        label="匹配值"
        field="value"
        placeholder="如 google.com；多个用英文逗号分隔"
        :rules="{ required: '请输入匹配值' }"
      />
      <u-select
        label="出站"
        field="outbound"
        :options="outboundOptions"
        placeholder="选择或输入出站 tag"
        filterable
        creatable
        :rules="{ required: '请选择出站' }"
      />
    </u-form>
    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
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
