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
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, reactive, ref, shallowRef } from 'vue'

import { useEnabledOverride } from '../composables/useEnabledOverride'
import { confirmAction } from '../confirm-action'
import { dialogStyle } from '../dialog-style'
import EmptyState from './EmptyState.vue'
import DialogFooter from './DialogFooter.vue'
import FormGroupCard from './FormGroupCard.vue'

const props = defineProps<{
  groups: PolicyGroup[]
  outboundOptions: { label: string; value: string }[]
  /** 首屏加载中：不渲染空状态，避免闪一下「暂无策略组」 */
  loading?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

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

/** 启停走本地覆写表驱动开关（USwitch 失败不下刷的 workaround，详见 useEnabledOverride） */
const { rowEnabled, togglingId, toggle: handleToggle } = useEnabledOverride(
  () => props.groups,
  async (row, enabled) => {
    await setPolicyGroupEnabled(row.id, enabled)
    emit('changed')
  },
)

async function handleDelete(row: PolicyGroup): Promise<void> {
  const ok = await confirmAction(
    '删除策略组',
    `确认删除策略组「${row.name}」吗？被规则引用时内核将无法重建，你会看到具体原因。`,
    true,
    '删除',
  )
  if (!ok) return
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
      <!-- 页签已表明对象，卡片头只留主操作工具栏；列表为空时主入口由空状态 CTA 承担，保持每屏 primary ≤1 -->
      <div class="card-ops">
        <u-button v-if="groups.length" type="primary" @click="openCreate">新建策略组</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <u-table :columns="columns" :data="groups" row-key="id" stripe>
        <template #empty>
          <EmptyState
            v-if="!loading"
            title="暂无策略组"
            description="策略组会随订阅下发，在手机/电脑客户端上呈现为「节点选择 / 自动选择」这类分组；可先在「兜底与模板」页签应用预设模板生成，或新建一个"
            cta-text="新建策略组"
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

  <u-dialog v-model="formVisible" :title="editing ? `编辑策略组 · ${editing.name}` : '新建策略组'" :style="dialogStyle(760)">
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
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>
<!-- card-ops 为 App.vue 全局类（卡片头工具栏，靠右与表格操作列对齐），本组件无其它局部样式 -->
