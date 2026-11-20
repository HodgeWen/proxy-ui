<!-- 规则集内容项维护：内容项列表 + 行内编辑；同一时刻仅一条启用，启用项即规则下载来源 -->
<script setup lang="ts">
import {
  UButton,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  URadio,
  USelect,
  UTag,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, reactive, shallowRef, watch } from 'vue'

import { createRuleSetItem, deleteRuleSetItem, updateRuleSetItem } from '../api/rule'
import type { RuleSet, RuleSetItem, RuleSetItemPayload } from '../api/rule'
import { dialogStyle } from '../dialog-style'

const props = defineProps<{
  ruleSet: RuleSet | null
}>()
const visible = defineModel<boolean>('visible', { default: false })
const emit = defineEmits<{ changed: [] }>()

/** 新增态的占位 ID：作为一行渲染在列表里，让新增表单与其它内容项同处一列 */
const CREATE_ID = -1

/** mihomo 行为：决定 mihomo 侧把该源当成域名 / IP 段 / 混合规则解析 */
const behaviorOptions = [
  { label: 'domain（域名列表）', value: 'domain' },
  { label: 'ipcidr（IP 段列表）', value: 'ipcidr' },
  { label: 'classical（混合规则）', value: 'classical' },
]

/** 中文短标签：列表里只展示「域名 / IP 段 / 混合」，英文值留给编辑表单 */
const behaviorLabels: Record<string, string> = { domain: '域名', ipcidr: 'IP 段', classical: '混合' }

const items = computed(() => props.ruleSet?.items ?? [])
/** 启用中的内容项 ID，供单选回显 */
const activeItemId = computed(() => items.value.find((item) => item.enabled)?.id)
/** 删除至少保留一条：只剩一条时禁用删除 */
const canDelete = computed(() => items.value.length > 1)

// ---------- 内容项编辑 ----------

/** 0=未编辑，CREATE_ID=新增，其它为正在编辑的内容项 ID */
const editingId = shallowRef(0)
const saving = shallowRef(false)
const switchingId = shallowRef(0)

/**
 * 编辑表单在 v-for 内，用按行 ID 索引的函数 ref 取实例。
 * 单一 ref 会在「切到更靠前的行」时被旧表单的卸载回调置空（新表单先挂载、旧表单后卸载），
 * 导致校验永远拿不到实例、保存静默失败；按 ID 各自记账即可避开这个时序。
 */
const editorRefs = new Map<number, FormExposed>()

function bindEditor(id: number, el: unknown): void {
  if (el) editorRefs.set(id, el as FormExposed)
  else editorRefs.delete(id)
}

/** 编辑草稿；启用状态不在这里，保存时按「当前启用项」推导，避免草稿压过用户刚点的来源切换 */
const editor = reactive<Omit<RuleSetItemPayload, 'enabled'>>({
  remark: '',
  singBoxUrl: '',
  mihomoUrl: '',
  mihomoBehavior: 'domain',
})

/**
 * 保存后的启用状态：新增时若还没有来源则自动启用，编辑时保留该行原本是否为来源，
 * 不读草稿，避免编辑期间切换来源后被草稿覆盖。
 */
const draftEnabled = computed(() =>
  editingId.value === CREATE_ID ? !activeItemId.value : editingId.value === activeItemId.value,
)

/** 渲染列表：新增时追加一条占位项，编辑表单就落在列表末尾 */
const rows = computed<RuleSetItem[]>(() => {
  if (editingId.value !== CREATE_ID) return items.value
  const draft: RuleSetItem = {
    id: CREATE_ID,
    remark: '',
    singBoxUrl: '',
    mihomoUrl: '',
    mihomoBehavior: 'domain',
    enabled: false,
  }
  return [...items.value, draft]
})

/** 编辑区标题：新增与编辑共用同一份表单，标题按当前编辑目标区分 */
const editorTitle = computed(() => {
  if (editingId.value === CREATE_ID) return '新增内容'
  const target = items.value.find((item) => item.id === editingId.value)
  return `编辑内容 · ${target?.remark || '未命名来源'}`
})

/** 地址格式校验：留空跳过，填了必须带协议头（后端同样校验） */
const urlRule = {
  validator: (val: string) => (!val?.trim() || /^https?:\/\//i.test(val.trim()) ? '' : '地址需以 http:// 或 https:// 开头'),
}

/**
 * 示例地址放脚本里而非直接写进模板属性：属性值中的 `//` 会让 vue-tsc
 * 在 v-for 作用域内解析失败（报 Cannot find name 'item'）。
 * 取真实可用的公开规则集仓库作示例，用户可直接照抄换成自己需要的那份。
 */
const SINGBOX_PLACEHOLDER = 'https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs'
const MIHOMO_PLACEHOLDER = 'https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs'

function openCreate(): void {
  editingId.value = CREATE_ID
  Object.assign(editor, {
    remark: '',
    singBoxUrl: '',
    mihomoUrl: '',
    mihomoBehavior: 'domain',
  })
}

function openEdit(item: RuleSetItem): void {
  editingId.value = item.id
  Object.assign(editor, {
    remark: item.remark,
    singBoxUrl: item.singBoxUrl,
    mihomoUrl: item.mihomoUrl,
    mihomoBehavior: item.mihomoBehavior,
  })
}

function closeEditor(): void {
  editingId.value = 0
}

/** 保存内容项：新增走创建、其余走更新（enabled 为 true 时后端自动停用同集其它项） */
async function handleSave(): Promise<void> {
  if (!editor.singBoxUrl.trim() && !editor.mihomoUrl.trim()) {
    message.warn('sing-box 与 mihomo 地址至少填一个')
    return
  }
  const form = editorRefs.get(editingId.value)
  if (!form || !(await form.validate())) {
    message.warn('请先修正表单中的错误再保存')
    return
  }
  const ruleSet = props.ruleSet
  if (!ruleSet) return
  saving.value = true
  try {
    const payload: RuleSetItemPayload = {
      remark: editor.remark.trim(),
      singBoxUrl: editor.singBoxUrl.trim(),
      mihomoUrl: editor.mihomoUrl.trim(),
      mihomoBehavior: editor.mihomoBehavior,
      enabled: draftEnabled.value,
    }
    if (editingId.value === CREATE_ID) await createRuleSetItem(ruleSet.id, payload)
    else await updateRuleSetItem(editingId.value, payload)
    message.success('规则集内容已保存')
    editingId.value = 0
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

/** 切换启用来源：启用本条，后端停用同集其它项 */
async function handleEnable(item: RuleSetItem): Promise<void> {
  if (item.enabled || switchingId.value) return
  switchingId.value = item.id
  try {
    await updateRuleSetItem(item.id, {
      remark: item.remark,
      singBoxUrl: item.singBoxUrl,
      mihomoUrl: item.mihomoUrl,
      mihomoBehavior: item.mihomoBehavior,
      enabled: true,
    })
    message.success(`已切换下载来源：${item.remark || '未命名来源'}`)
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    switchingId.value = 0
  }
}

/** 删除内容项：仅剩一条时按钮已禁用，这里只做一次弹窗确认 */
async function handleDelete(item: RuleSetItem): Promise<void> {
  const action = await messageConfirm({
    title: '删除内容项',
    message: item.enabled
      ? `「${item.remark || '未命名来源'}」是当前启用中的下载来源，删除后会自动启用最早的一条，确认删除吗？`
      : `确认删除内容项「${item.remark || '未命名来源'}」吗？`,
    confirmButtonText: '删除',
    confirmButtonType: 'danger',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteRuleSetItem(item.id)
    message.success('内容项已删除')
    if (editingId.value === item.id) editingId.value = 0
    emit('changed')
  } catch {
    // 错误已由请求封装统一提示
  }
}

// 关闭后丢弃未保存的编辑草稿
watch(visible, (open) => {
  if (!open) editingId.value = 0
})
</script>

<template>
  <u-dialog v-model="visible" :title="`维护规则集内容 · ${ruleSet?.name ?? ''}`" :style="dialogStyle(900)">
    <template v-if="ruleSet">
      <div class="set-head">
        <div class="set-title">
          {{ ruleSet.name }}
          <u-tag v-if="ruleSet.builtin" type="info">内置</u-tag>
          <u-tag v-if="ruleSet.usedByRules" type="warning">被 {{ ruleSet.usedByRules }} 条规则引用</u-tag>
        </div>
        <div class="set-desc">{{ ruleSet.description || '未填写说明' }}</div>
        <div class="set-note">
          一个规则集 = 一个 tag + 多条内容项。每条内容项同时携带 sing-box（.srs）与 mihomo（.mrs / .yaml）源地址，同一时刻只有一条启用，启用项就是规则下载时使用的来源；切换来源 = 启用另一条内容项。
        </div>
        <div v-if="!activeItemId" class="set-warn">
          当前没有启用中的内容项，规则引用该规则集时内核将无法重建，请启用一条。
        </div>
      </div>

      <div class="list-head">
        <span class="list-title">内容项（{{ items.length }}）</span>
        <div class="list-head-ops">
          <span class="list-hint">启用 = 点左侧圆点：同一时刻只有一条启用，启用项就是下载来源</span>
          <u-button size="small" type="primary" :disabled="editingId === CREATE_ID" @click="openCreate">
            新增内容
          </u-button>
        </div>
      </div>

      <div class="item-list">
        <div
          v-for="item in rows"
          :key="item.id"
          class="item-row"
          :class="{ 'is-active': item.id !== CREATE_ID && item.enabled, 'is-editing': editingId === item.id }"
        >
          <!-- 单选即「启用」：点未启用项直接切换下载来源；新增占位行不参与 -->
          <u-radio
            class="item-radio"
            :model-value="activeItemId"
            :value="item.id"
            :disabled="item.id === CREATE_ID || switchingId === item.id"
            @update:model-value="handleEnable(item)"
          />

          <div class="item-main">
            <!-- 新增占位行没有内容可摘要；编辑时摘要保留，方便对照原有地址 -->
            <div v-if="item.id !== CREATE_ID" class="item-summary">
              <div class="item-title">
                <span class="item-remark">{{ item.remark || '未命名来源' }}</span>
                <u-tag v-if="item.enabled" type="success" size="small">启用中</u-tag>
                <u-tag type="info" size="small">
                  mihomo：{{ behaviorLabels[item.mihomoBehavior] ?? item.mihomoBehavior }}
                </u-tag>
              </div>
              <div class="item-url">
                <span class="item-url-label">sing-box</span>
                <span :class="{ 'is-empty': !item.singBoxUrl }">{{ item.singBoxUrl || '未配置' }}</span>
              </div>
              <div class="item-url">
                <span class="item-url-label">mihomo</span>
                <span :class="{ 'is-empty': !item.mihomoUrl }">{{ item.mihomoUrl || '未配置' }}</span>
              </div>
            </div>

            <!-- 编辑态：表单就地展开在该行内，不另开编辑区 -->
            <div v-if="editingId === item.id" class="item-editor">
              <div class="item-editor-title">{{ editorTitle }}</div>
                <!-- 单列：行内编辑区宽度随弹框收窄，固定 2 列会挤压 132px label 与控件 -->
                <u-form
                  :ref="(el: unknown) => bindEditor(item.id, el)"
                  :model="editor"
                  :cols="1"
                  label-width="132px"
                >
                <u-form-item
                  label="备注"
                  field="remark"
                  span="full"
                  tips="仅用于区分同一规则集下的多条来源，如「官方源 / 镜像源」；留空显示为「未命名来源」"
                >
                  <u-input v-model="editor.remark" placeholder="如 官方源" clearable />
                </u-form-item>
                <u-form-item
                  label="sing-box 源地址"
                  field="singBoxUrl"
                  span="full"
                  :rules="urlRule"
                  :tips="`.srs 二进制规则集地址，供服务端 sing-box 与 sing-box 客户端下载；与 mihomo 地址至少填一个。示例：${SINGBOX_PLACEHOLDER}`"
                >
                  <u-input v-model="editor.singBoxUrl" :placeholder="SINGBOX_PLACEHOLDER" clearable />
                </u-form-item>
                <u-form-item
                  label="mihomo 源地址"
                  field="mihomoUrl"
                  span="full"
                  :rules="urlRule"
                  :tips="`.mrs 或 .yaml 规则集地址，供 clash / mihomo 客户端下载；与 sing-box 地址至少填一个。示例：${MIHOMO_PLACEHOLDER}`"
                >
                  <u-input v-model="editor.mihomoUrl" :placeholder="MIHOMO_PLACEHOLDER" clearable />
                </u-form-item>
                <u-form-item
                  label="mihomo 行为"
                  field="mihomoBehavior"
                  tips="mihomo 侧按何种规则类型解析该源：domain=域名列表、ipcidr=IP 段、classical=混合规则；须与源文件实际内容一致，否则客户端解析失败"
                >
                  <u-select v-model="editor.mihomoBehavior" :options="behaviorOptions" />
                </u-form-item>
              </u-form>
              <div class="item-editor-note">
                <!-- 启用状态由列表里的圆点切换，表单只说明保存后的结果，避免两处都能改 -->
                <u-tag :type="draftEnabled ? 'success' : 'info'" size="small">
                  {{ draftEnabled ? '保存后成为当前下载来源' : '保存后作为备用来源（不改变当前来源）' }}
                </u-tag>
              </div>
              <div class="editor-actions">
                <u-button text @click="closeEditor">取消</u-button>
                <u-button type="primary" :loading="saving" @click="handleSave">保存内容</u-button>
              </div>
            </div>
          </div>

          <div v-if="item.id !== CREATE_ID" class="item-actions">
            <u-button text size="small" type="primary" @click="openEdit(item)">编辑</u-button>
            <!-- 至少保留一条内容项，禁用态用原生 title 说明原因 -->
            <u-button
              text
              size="small"
              type="danger"
              :disabled="!canDelete"
              :title="canDelete ? undefined : '每个规则集至少保留一条内容项'"
              @click="handleDelete(item)"
            >
              删除
            </u-button>
          </div>
        </div>
      </div>
    </template>

    <template #footer="{ close }">
      <u-button type="primary" @click="close()">完成</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.set-head {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--u-border-color);
}

.set-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.set-desc {
  color: var(--u-text-color-main);
  font-size: var(--u-font-size-main-small);
}

.set-note {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.set-warn {
  margin-top: 4px;
  color: var(--u-color-warning);
  font-size: var(--u-font-size-main-small);
}

.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 12px 0 8px;
}

.list-title {
  font-weight: 600;
}

.list-head-ops {
  display: flex;
  align-items: center;
  gap: 12px;
}

.list-hint {
  color: var(--u-text-color-assist);
  font-size: var(--u-font-size-main-small);
}

.item-list {
  display: flex;
  flex-direction: column;
}

.item-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--u-border-color);
}

.item-row.is-active {
  background: var(--u-color-success-a-5);
}

.item-row.is-editing {
  box-shadow: inset 0 0 0 1px var(--u-color-primary);
  border-radius: var(--u-radius-default);
}

.item-radio {
  margin-top: 2px;
  flex-shrink: 0;
}

.item-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.item-summary {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.item-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.item-remark {
  font-weight: 600;
  color: var(--u-text-color-main);
}

/* 地址行：小标签 + 可截断的地址文本 */
.item-url {
  display: flex;
  gap: 8px;
  min-width: 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

.item-url-label {
  width: 62px;
  flex-shrink: 0;
  color: var(--u-text-color-assist);
}

.item-url span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-url .is-empty {
  color: var(--u-text-color-assist);
}

.item-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.item-editor {
  margin-top: 8px;
  padding: 12px 14px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-top);
}

.item-editor-title {
  margin-bottom: 8px;
  font-weight: 600;
}

.editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.item-editor-note {
  margin-top: 4px;
}
</style>
