<!-- 策略组卡片：三种模式（故障转移/固定代理/自动选择）增删改启停与出站成员选择 -->
<script setup lang="ts">
import { createPolicyGroup, deletePolicyGroup, setPolicyGroupEnabled, updatePolicyGroup } from '../api/rule'
import type { PolicyGroup, PolicyGroupPayload } from '../api/rule'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UMultiSelect,
  USelect,
  USwitch,
  UTable,
  UTag,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, reactive, ref, shallowRef } from 'vue'

import EmptyState from './EmptyState.vue'
import FormGroupCard from './FormGroupCard.vue'

const props = defineProps<{
  groups: PolicyGroup[]
  outboundOptions: { label: string; value: string }[]
}>()
const emit = defineEmits<{ changed: [] }>()

const togglingId = shallowRef(0)

/** 策略组模式选项 */
const modeOptions = [
  { label: '故障转移（按顺序优先，近似）', value: 'failover' },
  { label: '固定代理（默认首个成员）', value: 'fixed' },
  { label: '自动选择（测速择优）', value: 'auto' },
]

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'mode', name: '模式', width: 130, align: 'center' },
  { key: 'members', name: '成员出站', minWidth: 220 },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 130, align: 'center' },
])

function modeLabel(mode: string): string {
  return modeOptions.find((o) => o.value === mode)?.label.split('（')[0] ?? mode
}

async function handleToggle(row: PolicyGroup, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  try {
    await setPolicyGroupEnabled(row.id, enabled)
    row.enabled = enabled
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    togglingId.value = 0
  }
}

async function handleDelete(row: PolicyGroup): Promise<void> {
  const action = await messageConfirm({
    title: '删除策略组',
    message: `确认删除策略组「${row.name}」吗？被规则引用时应用配置将失败。`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deletePolicyGroup(row.id)
    message.success('策略组已删除')
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}

// ---------- 编辑弹窗 ----------

const formVisible = shallowRef(false)
const saving = shallowRef(false)
const editing = shallowRef<PolicyGroup | null>(null)
const formRef = shallowRef<FormExposed>()
/** 分组卡片默认展开，重开弹框时复位 */
const memberCollapsed = ref(false)
const form = reactive<{ name: string; mode: PolicyGroupPayload['mode']; members: string[] }>({
  name: '',
  mode: 'auto',
  members: [],
})

/** 成员候选：出站与其它策略组（排除拦截动作与自身，避免无效成员） */
const memberOptions = computed(() =>
  props.outboundOptions.filter((o) => o.value !== 'block' && o.value !== editing.value?.name),
)

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', mode: 'auto', members: [] })
  memberCollapsed.value = false
  formVisible.value = true
}

function openEdit(row: PolicyGroup): void {
  editing.value = row
  Object.assign(form, { name: row.name, mode: row.mode, members: [...row.members] })
  memberCollapsed.value = false
  formVisible.value = true
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    if (editing.value) await updatePolicyGroup(editing.value.id, form)
    else await createPolicyGroup(form)
    message.success(editing.value ? '策略组已更新' : '策略组已创建')
    formVisible.value = false
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <u-card>
    <u-card-header>
      <div class="page-header">
        <span>策略组</span>
        <!-- 规则页实心主按钮固定留给「新建规则」，此处降次级并常驻 -->
        <u-button @click="openCreate">新建策略组</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <u-table :columns="columns" :data="groups" row-key="id" border stripe>
        <template #empty>
          <EmptyState
            title="暂无策略组"
            description="策略组可聚合多个出站做故障转移或自动选择；可应用上方预设模板生成，或新建一个"
            cta-text="新建策略组"
            cta-plain
            @cta="openCreate"
          />
        </template>
        <template #column:mode="{ rowData }">
          <u-tag type="info">{{ modeLabel(rowData.mode) }}</u-tag>
        </template>
        <template #column:members="{ rowData }">
          {{ rowData.members.join('，') || '—' }}
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
          <u-button text size="small" type="primary" @click="openEdit(rowData)">编辑</u-button>
          <u-button text size="small" type="error" @click="handleDelete(rowData)">删除</u-button>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? `编辑策略组 · ${editing.name}` : '新建策略组'" style="width: 1120px; max-width: 96vw">
    <div class="dialog-scroll">
      <u-form ref="formRef" :model="form" :cols="1" label-width="92px">
        <FormGroupCard title="基础配置">
          <u-form-item
            label="名称"
            field="name"
            :rules="{ required: '请输入名称' }"
            tips="策略组 tag，供路由规则与兜底出站引用；需与出站名不重复"
          >
            <u-input v-model="form.name" placeholder="策略组 tag，供规则引用" clearable />
          </u-form-item>
          <u-form-item
            label="模式"
            field="mode"
            :rules="{ required: '请选择模式' }"
            tips="组内成员的选择策略：故障转移按顺序自动切换、固定代理始终用首个成员、自动选择定时测速选最快"
          >
            <u-select v-model="form.mode" :options="modeOptions" />
          </u-form-item>
        </FormGroupCard>

        <FormGroupCard v-model:collapsed="memberCollapsed" title="成员出站">
          <u-form-item
            label="成员出站"
            span="full"
            tips="成员按选择顺序生效，故障转移模式下靠前者优先；可选自定义出站或其它策略组，不可选拦截"
          >
            <u-multi-select
              v-model="form.members"
              :options="memberOptions"
              placeholder="选择成员（出站或其它策略组）"
              clearable
              filterable
            />
          </u-form-item>
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
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
}

/* 弹框表单超出可视高度时内部滚动，footer 常驻 */
.dialog-scroll {
  max-height: calc(90vh - 160px);
  overflow-y: auto;
}
</style>
