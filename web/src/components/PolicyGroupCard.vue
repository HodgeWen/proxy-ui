<!-- 策略组卡片：三种模式（故障转移/固定代理/自动选择）增删改启停与出站成员选择 -->
<script setup lang="ts">
import { createPolicyGroup, deletePolicyGroup, setPolicyGroupEnabled, updatePolicyGroup } from '../api/rule'
import type { PolicyGroup, PolicyGroupPayload } from '../api/rule'
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
import { computed, reactive, ref, shallowRef, watch } from 'vue'

import EmptyState from './EmptyState.vue'
import FormGroupCard from './FormGroupCard.vue'

const props = defineProps<{
  groups: PolicyGroup[]
  outboundOptions: { label: string; value: string }[]
  /** 首屏加载中：不渲染空状态，避免闪一下「暂无策略组」 */
  loading?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const togglingId = shallowRef(0)

/** 策略组模式选项 */
const modeOptions = [
  { label: '故障转移（自动切换可用节点）', value: 'failover' },
  { label: '固定代理（默认首个成员）', value: 'fixed' },
  { label: '自动选择（测速择优）', value: 'auto' },
]

const columns = defineTableColumns([
  { key: 'name', name: '名称', minWidth: 140 },
  { key: 'mode', name: '模式', width: 130, align: 'center' },
  { key: 'members', name: '成员出站', minWidth: 220 },
  { key: 'enabled', name: '启用', width: 80, align: 'center' },
  { key: 'actions', name: '操作', width: 150, align: 'center', fixed: 'right' },
])

/** 模式短标签：列表只展示「故障转移 / 固定代理 / 自动选择」，完整说明留给下拉 */
const modeLabels: Record<string, string> = { failover: '故障转移', fixed: '固定代理', auto: '自动选择' }

function modeLabel(mode: string): string {
  return modeLabels[mode] ?? mode
}

/**
 * 启停：USwitch 是原生 checkbox 实现的受控组件，请求失败后父级数据不变、
 * `:checked` 不会重新下刷，会和 DOM 错位（下次点击发出的还是旧值）。
 * 用本地覆写表驱动，失败写回相反值触发真实下刷，回读后清空覆写。
 */
const enabledOverride = shallowRef<Record<number, boolean>>({})

function rowEnabled(row: PolicyGroup): boolean {
  return enabledOverride.value[row.id] ?? row.enabled
}

watch(
  () => props.groups,
  () => {
    enabledOverride.value = {}
  },
)

async function handleToggle(row: PolicyGroup, enabled: boolean): Promise<void> {
  togglingId.value = row.id
  enabledOverride.value = { ...enabledOverride.value, [row.id]: enabled }
  try {
    await setPolicyGroupEnabled(row.id, enabled)
    emit('changed')
  } catch {
    // 失败回滚：写回相反值让开关立即恢复
    enabledOverride.value = { ...enabledOverride.value, [row.id]: !enabled }
  } finally {
    togglingId.value = 0
  }
}

async function handleDelete(row: PolicyGroup): Promise<void> {
  const action = await messageConfirm({
    title: '删除策略组',
    message: `确认删除策略组「${row.name}」吗？被规则引用时内核将无法重建，你会看到具体原因。`,
    confirmButtonText: '删除',
    confirmButtonType: 'danger',
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
      <div class="card-desc">
        策略组是随订阅下发到客户端的分组选择器（客户端上的「节点选择 / 自动选择」），聚合多个成员供规则与兜底出站引用。三种模式：固定=固定用一个节点、自动=自动测延迟选最快、故障转移=自动测速切换到可用节点（带容差防抖，恢复后切回）。
      </div>
      <u-table :columns="columns" :data="groups" row-key="id" border stripe>
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无策略组"
            description="策略组会随订阅下发，在手机/电脑客户端上呈现为「节点选择 / 自动选择」这类分组；可应用上方预设模板生成，或新建一个"
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
          <!-- USwitch 没有 loading，用 disabled 在请求期间挡住重复点击 -->
          <u-switch
            size="small"
            :model-value="rowEnabled(rowData)"
            :disabled="togglingId === rowData.id"
            @update:model-value="(v: boolean) => handleToggle(rowData, v)"
          />
        </template>
        <template #column:actions="{ rowData }">
          <u-action-group :max="3">
            <u-action @run="openEdit(rowData)">编辑</u-action>
            <!-- 确认交给 messageConfirm（需说明引用影响），不再叠 need-confirm 气泡 -->
            <u-action type="danger" @run="handleDelete(rowData)">删除</u-action>
          </u-action-group>
        </template>
      </u-table>
    </u-card-content>
  </u-card>

  <u-dialog v-model="formVisible" :title="editing ? `编辑策略组 · ${editing.name}` : '新建策略组'" style="width: 760px; max-width: 96vw">
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
          tips="三者的差异：固定=手动锁定一个节点（默认首个成员，客户端里也可手动切换）；自动=定时测延迟、永远走最快；故障转移=尽量保持在靠前的可用节点，挂了才切下一个、恢复后切回。日常推荐「自动」，要把流量锁定在指定节点（如流媒体解锁）用「固定」，节点经常不稳用「故障转移」"
        >
          <u-select v-model="form.mode" :options="modeOptions" />
        </u-form-item>
      </FormGroupCard>

      <FormGroupCard v-model:collapsed="memberCollapsed" title="成员出站">
        <u-form-item
          label="成员出站"
          span="full"
          tips="成员顺序就是优先级：固定模式默认用首个成员，故障转移优先用靠前的可用成员，自动模式与顺序无关；可选 direct、自定义出站或其它策略组（不可选拦截）。客户端订阅实际使用入站节点、direct 与其它组，自定义出站仅在服务端配置生效"
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
</style>
