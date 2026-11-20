<!-- 规则集卡片：名称 / 说明 / 启用来源 / 被引用数 / 内容项数，新建·编辑·删除与内容项维护入口 -->
<script setup lang="ts">
import { createRuleSet, deleteRuleSet, updateRuleSet } from '../api/rule'
import type { RuleSet } from '../api/rule'
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
  UTable,
  UTag,
  defineTableColumns,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { reactive, shallowRef } from 'vue'

import EmptyState from './EmptyState.vue'
import { dialogStyle } from '../dialog-style'

defineProps<{
  ruleSets: RuleSet[]
  /** 首屏加载中：不渲染空状态，避免闪一下「暂无规则集」 */
  loading?: boolean
}>()
const emit = defineEmits<{
  changed: []
  /** 打开内容项维护弹窗（弹窗实例由页面持有，规则卡片也要复用） */
  maintain: [ruleSet: RuleSet]
}>()

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 170 },
  { key: 'description', name: '说明', minWidth: 200 },
  { key: 'source', name: '启用来源', minWidth: 150 },
  { key: 'usedByRules', name: '被引用', width: 90, align: 'center' },
  { key: 'itemCount', name: '内容项', width: 90, align: 'center' },
  { key: 'actions', name: '操作', width: 240, align: 'center', fixed: 'right' },
])

/**
 * 启用中的内容项备注；null 表示没有启用项（真正的异常态，保存后内核无法重建）。
 * 与「有启用项但没写备注」区分开，避免把正常的空备注误报成未配置来源。
 */
function activeSource(ruleSet: RuleSet): string | null {
  const active = ruleSet.items.find((item) => item.enabled)
  return active ? active.remark || '未命名来源' : null
}

/** 删除按钮禁用原因：内置与仍被引用时后端会拒绝，提前说明省掉一次失败 */
function deleteBlockReason(ruleSet: RuleSet): string {
  if (ruleSet.builtin) return '内置规则集不可删除，可在「维护内容项」里切换来源'
  if (ruleSet.usedByRules > 0) return `被 ${ruleSet.usedByRules} 条规则引用，需先修改或删除这些规则`
  return ''
}

// ---------- 新建 / 编辑弹窗 ----------

const formVisible = shallowRef(false)
const saving = shallowRef(false)
const editing = shallowRef<RuleSet | null>(null)
const formRef = shallowRef<FormExposed>()
const form = reactive({ name: '', description: '' })

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', description: '' })
  formVisible.value = true
}

function openEdit(row: RuleSet): void {
  editing.value = row
  Object.assign(form, { name: row.name, description: row.description })
  formVisible.value = true
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    if (editing.value) await updateRuleSet(editing.value.id, form)
    else await createRuleSet(form)
    message.success(editing.value ? '规则集已更新' : '规则集已创建')
    formVisible.value = false
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

/** 删除：按钮已按内置/引用状态禁用，这里只做一次弹窗确认 */
async function handleDelete(row: RuleSet): Promise<void> {
  const action = await messageConfirm({
    title: '删除规则集',
    message: `确认删除规则集「${row.name}」及其全部内容项吗？`,
    confirmButtonText: '删除',
    confirmButtonType: 'danger',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteRuleSet(row.id)
    message.success('规则集已删除')
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}
</script>

<template>
  <u-card>
    <u-card-header>
      <!-- 页签已表明对象，卡片头只留主操作工具栏；列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
      <div class="card-ops">
        <u-button v-if="ruleSets.length" type="primary" @click="openCreate">新建规则集</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <u-table :columns="columns" :data="ruleSets" row-key="id" stripe>
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无规则集"
            description="内置的 anti-ad / geosite-cn / geoip-cn 会在服务启动时自动补齐；也可以自建规则集，先建空集再添加内容项来源"
            cta-text="新建规则集"
            @cta="openCreate"
          />
        </template>
        <template #column:name="{ rowData }">
          <div class="name-cell">
            <span class="name-text">{{ rowData.name }}</span>
            <u-tag v-if="rowData.builtin" type="info" size="small">内置</u-tag>
          </div>
        </template>
        <template #column:description="{ rowData }">
          <span class="dim ellipsis" :title="rowData.description || undefined">
            {{ rowData.description || '—' }}
          </span>
        </template>
        <template #column:source="{ rowData }">
          <span v-if="activeSource(rowData) !== null">{{ activeSource(rowData) }}</span>
          <span v-else class="warn">未配置来源</span>
        </template>
        <template #column:usedByRules="{ rowData }">
          <span :class="{ dim: !rowData.usedByRules }">{{ rowData.usedByRules }}</span>
        </template>
        <template #column:itemCount="{ rowData }">
          {{ rowData.items.length }}
        </template>
        <template #column:actions="{ rowData }">
          <!-- max=3 恰好容纳三项，避免操作被收进「更多」下拉 -->
          <u-action-group :max="3">
            <u-action @run="emit('maintain', rowData)">维护内容项</u-action>
            <u-action @run="openEdit(rowData)">编辑</u-action>
            <!-- UButton 的 disabled 只加类名、不设原生 disabled，因此 title 仍可悬停显示禁用原因 -->
            <u-action
              type="danger"
              :disabled="!!deleteBlockReason(rowData)"
              :title="deleteBlockReason(rowData) || undefined"
              @run="handleDelete(rowData)"
            >
              删除
            </u-action>
          </u-action-group>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? `编辑规则集 · ${editing.name}` : '新建规则集'" :style="dialogStyle(520)">
    <u-form ref="formRef" :model="form" :cols="1" label-width="80px">
      <u-form-item
        label="名称"
        field="name"
        :rules="{ required: '请输入名称' }"
        :tips="
          editing?.builtin
            ? '内置规则集的名称是模板与订阅沿用的固定 tag，不可修改；如需自定义用途请另建规则集'
            : '规则集 tag，规则引用它时下载此规则集；只能包含字母、数字、下划线、点与短横线，且不可与已有规则集重名'
        "
      >
        <!-- 内置集合改名后模板会指向新建的同名集合，来源配置随之丢失，故锁死名称 -->
        <u-input
          v-model="form.name"
          :disabled="!!editing?.builtin"
          placeholder="如 my-ads"
          clearable
        />
      </u-form-item>
      <u-form-item label="说明" field="description" tips="一句话说明这个规则集的用途，仅面板内展示">
        <u-input v-model="form.description" placeholder="如 自建广告拦截列表" clearable />
      </u-form-item>
    </u-form>
    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :loading="saving" @click="handleSave">保存</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
/* 卡片头工具栏：仅剩主操作，靠右与表格操作列对齐 */
.card-ops {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex: 1;
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.name-text {
  font-weight: 600;
}

.dim {
  color: var(--u-text-color-assist);
}

/* 说明列可能很长：单元格内单行截断，完整内容看 title */
.ellipsis {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 规则集没有启用来源时内核无法重建，用警示色而非灰色提示 */
.warn {
  color: var(--u-color-warning);
}
</style>
