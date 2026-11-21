<!-- 设置页「AI 供应商」页签：供应商 / 模型双 CRUD（三个弹框）+ 从上游 /models 多选导入 -->
<script setup lang="ts">
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
  UNumberInput,
  UPasswordInput,
  USelect,
  USwitch,
  UTag,
  message,
} from '@veltra/desktop'
import { TriangleAlert } from '@veltra/icons/normal'
import { computed, onActivated, reactive, shallowRef } from 'vue'

import EmptyState from '../../components/EmptyState.vue'
import DialogFooter from '../../components/DialogFooter.vue'
import { confirmAction } from '../../confirm-action'
import { dialogStyle } from '../../dialog-style'
import {
  createAIModel,
  createAIProvider,
  deleteAIModel,
  deleteAIProvider,
  fetchAIProviders,
  fetchAIUpstreamModels,
  importAIModels,
  updateAIModel,
  updateAIProvider,
} from '../../api/ai'
import type { AiModel, AiModelInput, AiProvider, AiReasoningLevel, AiUpstreamModel } from '../../api/ai'

/** 供应商列表（视图内含各自模型） */
const aiProviders = shallowRef<AiProvider[]>([])
const aiLoading = shallowRef(false)

async function loadAIProviders(silent = true): Promise<void> {
  aiLoading.value = true
  try {
    aiProviders.value = await fetchAIProviders()
  } catch {
    if (!silent) message.error('查询 AI 供应商失败')
  } finally {
    aiLoading.value = false
  }
}

/** 供应商编辑弹框（新增/编辑共用；apiKey 留空表示保留已存 Key） */
const aiProviderDialog = shallowRef(false)
const aiProviderEditingId = shallowRef(0)
const aiProviderSaving = shallowRef(false)
const aiProviderHasKey = shallowRef(false)
const aiProviderForm = reactive({ name: '', baseUrl: '', apiKey: '', enabled: true })

/** 含 // 的示例 URL 放 script 侧绑定：模板属性字符串里的 // 会被 vue-tsc 的单行 props 改写误当注释，损坏生成代码 */
const baseUrlPlaceholder = '如 https://api.openai.com/v1'

function openProviderCreate(): void {
  aiProviderEditingId.value = 0
  aiProviderForm.name = ''
  aiProviderForm.baseUrl = ''
  aiProviderForm.apiKey = ''
  aiProviderForm.enabled = true
  aiProviderHasKey.value = false
  aiProviderDialog.value = true
}

function openProviderEdit(provider: AiProvider): void {
  aiProviderEditingId.value = provider.id
  aiProviderForm.name = provider.name
  aiProviderForm.baseUrl = provider.baseUrl
  aiProviderForm.apiKey = ''
  aiProviderForm.enabled = provider.enabled
  aiProviderHasKey.value = provider.hasKey
  aiProviderDialog.value = true
}

async function saveAIProvider(): Promise<void> {
  if (!aiProviderForm.name.trim() || !aiProviderForm.baseUrl.trim()) {
    message.error('供应商名称与接口地址不能为空')
    return
  }
  aiProviderSaving.value = true
  try {
    const input = {
      name: aiProviderForm.name.trim(),
      baseUrl: aiProviderForm.baseUrl.trim(),
      apiKey: aiProviderForm.apiKey.trim(),
      enabled: aiProviderForm.enabled,
    }
    if (aiProviderEditingId.value) {
      await updateAIProvider(aiProviderEditingId.value, input)
    } else {
      await createAIProvider(input)
    }
    message.success('AI 供应商已保存')
    aiProviderDialog.value = false
    await loadAIProviders()
  } catch {
    // 校验失败等错误已由请求封装统一提示
  } finally {
    aiProviderSaving.value = false
  }
}

/** 启停供应商（apiKey 留空语义为保留已存 Key） */
async function toggleAIProvider(provider: AiProvider): Promise<void> {
  try {
    await updateAIProvider(provider.id, {
      name: provider.name,
      baseUrl: provider.baseUrl,
      apiKey: '',
      enabled: !provider.enabled,
    })
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  }
}

async function removeAIProvider(provider: AiProvider): Promise<void> {
  const ok = await confirmAction(
    '删除 AI 供应商',
    `将删除「${provider.name}」及其下 ${provider.models.length} 个模型，不可恢复。确认删除吗？`,
    true,
    '删除',
  )
  if (!ok) return
  try {
    await deleteAIProvider(provider.id)
    message.success('AI 供应商已删除')
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  }
}

/** 模型编辑弹框（新增/编辑共用；推理等级为行编辑器） */
const aiModelDialog = shallowRef(false)
const aiModelProviderId = shallowRef(0)
const aiModelEditingId = shallowRef(0)
const aiModelSaving = shallowRef(false)
const aiModelForm = reactive({
  modelId: '',
  label: '',
  description: '',
  contextWindow: null as number | null,
  defaultReasoningLevel: '',
  enabled: true,
})
/** 推理等级行（值 + 文案；默认等级单独下拉选择） */
const aiLevelRows = reactive<{ value: string; label: string }[]>([])

/** 默认等级选项：取行编辑器里非空且不重复的值 */
const aiLevelOptions = computed(() => {
  const seen = new Set<string>()
  const options: { label: string; value: string }[] = []
  for (const row of aiLevelRows) {
    const value = row.value.trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    options.push({ label: row.label.trim() || value, value })
  }
  return options
})

function openModelCreate(providerId: number): void {
  aiModelProviderId.value = providerId
  aiModelEditingId.value = 0
  aiModelForm.modelId = ''
  aiModelForm.label = ''
  aiModelForm.description = ''
  aiModelForm.contextWindow = null
  aiModelForm.defaultReasoningLevel = ''
  aiModelForm.enabled = true
  aiLevelRows.splice(0, aiLevelRows.length)
  aiModelDialog.value = true
}

function openModelEdit(providerId: number, model: AiModel): void {
  aiModelProviderId.value = providerId
  aiModelEditingId.value = model.id
  aiModelForm.modelId = model.modelId
  aiModelForm.label = model.label
  aiModelForm.description = model.description
  aiModelForm.contextWindow = model.contextWindow || null
  aiModelForm.defaultReasoningLevel = model.defaultReasoningLevel
  aiModelForm.enabled = model.enabled
  aiLevelRows.splice(0, aiLevelRows.length, ...model.reasoningLevels.map((level) => ({ ...level })))
  aiModelDialog.value = true
}

function addLevelRow(): void {
  aiLevelRows.push({ value: '', label: '' })
}

function removeLevelRow(index: number): void {
  const [removed] = aiLevelRows.splice(index, 1)
  if (removed && aiModelForm.defaultReasoningLevel === removed.value) {
    aiModelForm.defaultReasoningLevel = ''
  }
}

/** 与后端校验对齐的提交前检查，通过则拼出模型入参 */
function buildModelInput(): AiModelInput | null {
  const modelId = aiModelForm.modelId.trim()
  if (!modelId) {
    message.error('模型 id 不能为空')
    return null
  }
  const levels: AiReasoningLevel[] = []
  const seen = new Set<string>()
  for (const row of aiLevelRows) {
    const value = row.value.trim()
    if (!value) {
      message.error('推理等级的值不能为空')
      return null
    }
    if (seen.has(value)) {
      message.error(`推理等级值重复：${value}`)
      return null
    }
    seen.add(value)
    levels.push({ value, label: row.label.trim() || value })
  }
  if (!levels.length && aiModelForm.defaultReasoningLevel) {
    message.error('未配置推理等级时不能指定默认等级')
    return null
  }
  if (aiModelForm.defaultReasoningLevel && !seen.has(aiModelForm.defaultReasoningLevel)) {
    message.error('默认推理等级不在等级列表内')
    return null
  }
  return {
    modelId,
    label: aiModelForm.label.trim(),
    description: aiModelForm.description.trim(),
    reasoningLevels: levels,
    defaultReasoningLevel: aiModelForm.defaultReasoningLevel,
    contextWindow: aiModelForm.contextWindow ?? 0,
    enabled: aiModelForm.enabled,
  }
}

async function saveAIModel(): Promise<void> {
  const input = buildModelInput()
  if (!input) return
  aiModelSaving.value = true
  try {
    if (aiModelEditingId.value) {
      await updateAIModel(aiModelEditingId.value, input)
    } else {
      await createAIModel(aiModelProviderId.value, input)
    }
    message.success('模型已保存')
    aiModelDialog.value = false
    await loadAIProviders()
  } catch {
    // 模型 id 重复等错误已由请求封装统一提示
  } finally {
    aiModelSaving.value = false
  }
}

async function removeAIModel(model: AiModel): Promise<void> {
  const ok = await confirmAction('删除模型', `确认删除模型「${model.modelId}」吗？`, true, '删除')
  if (!ok) return
  try {
    await deleteAIModel(model.id)
    message.success('模型已删除')
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  }
}

/** 上游 /models 拉取 + 多选导入弹框 */
const aiImportDialog = shallowRef(false)
const aiImportProviderId = shallowRef(0)
const aiImportFetching = shallowRef(false)
const aiImportSaving = shallowRef(false)
const aiUpstreamModels = shallowRef<AiUpstreamModel[]>([])
const aiImportSelected = shallowRef<string[]>([])

/** 全库已存在的模型 id（选项上标记「已存在」，导入时后端也会跳过） */
const aiExistingIds = computed(() => new Set(aiProviders.value.flatMap((p) => p.models.map((m) => m.modelId))))

const aiImportOptions = computed(() =>
  aiUpstreamModels.value.map((m) => ({
    label: aiExistingIds.value.has(m.id) ? `${m.id}（已存在）` : m.ownedBy ? `${m.id}（${m.ownedBy}）` : m.id,
    value: m.id,
  })),
)

async function openImport(provider: AiProvider): Promise<void> {
  aiImportProviderId.value = provider.id
  aiImportSelected.value = []
  aiUpstreamModels.value = []
  aiImportDialog.value = true
  aiImportFetching.value = true
  try {
    aiUpstreamModels.value = await fetchAIUpstreamModels(provider.id)
  } catch {
    // 拉取失败错误已由请求封装统一提示；弹框内给出重试入口
  } finally {
    aiImportFetching.value = false
  }
}

async function retryImport(): Promise<void> {
  const provider = aiProviders.value.find((p) => p.id === aiImportProviderId.value)
  if (provider) await openImport(provider)
}

async function confirmImport(): Promise<void> {
  if (!aiImportSelected.value.length) {
    message.error('请先选择要导入的模型')
    return
  }
  aiImportSaving.value = true
  try {
    const { imported } = await importAIModels(aiImportProviderId.value, aiImportSelected.value)
    message.success(`已导入 ${imported} 个模型（已存在的自动跳过）`)
    aiImportDialog.value = false
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    aiImportSaving.value = false
  }
}

/** 模型行推理等级摘要：值（文案）列表 + 默认标记 */
function levelSummary(model: AiModel): string {
  const levels = model.reasoningLevels.map((level) =>
    level.label && level.label !== level.value ? `${level.value}（${level.label}）` : level.value,
  )
  const text = levels.length ? levels.join('、') : '未配置'
  return model.defaultReasoningLevel ? `${text}，默认 ${model.defaultReasoningLevel}` : text
}

/** 上下文窗口展示（如 128K / 1M tokens） */
function formatContextWindow(tokens: number): string {
  if (!tokens) return ''
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(tokens % 1_000_000 === 0 ? 0 : 1)}M tokens`
  }
  return `${(tokens / 1000).toFixed(tokens % 1000 === 0 ? 0 : 1)}K tokens`
}

// 进入页签（含首次挂载）拉取供应商列表
onActivated(() => {
  void loadAIProviders()
})
</script>

<template>
  <u-card class="settings-card">
    <u-card-header>
      <div class="card-header-row">
        <span>AI 供应商（支持任意 OpenAI 兼容供应商）</span>
        <div class="ops-row">
          <u-button size="small" :loading="aiLoading" @click="loadAIProviders(false)">重新加载</u-button>
          <u-button size="small" type="primary" @click="openProviderCreate">新增供应商</u-button>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <p class="section-desc">
        新增供应商后从其 /models 导入或手工添加模型，每个模型可维护推理等级；Key 仅保存在服务端，
        对话经面板代理转发，从顶栏「AI 助手」打开。
      </p>
      <p v-if="!aiProviders.length" class="section-desc">还没有供应商，点右上角「新增供应商」开始配置。</p>

      <!-- 供应商区块：头部信息与操作 + 其下模型列表 -->
      <section v-for="provider in aiProviders" :key="provider.id" class="ai-provider">
        <header class="ai-provider-head">
          <div class="ai-provider-info">
            <span class="ai-provider-name">{{ provider.name }}</span>
            <u-tag v-if="!provider.enabled" type="info" size="small">已停用</u-tag>
            <u-tag v-else-if="!provider.hasKey" type="warning" size="small">缺 API Key</u-tag>
            <span class="ai-provider-url">{{ provider.baseUrl }}</span>
          </div>
          <div class="ai-provider-ops">
            <u-switch :model-value="provider.enabled" @update:model-value="() => toggleAIProvider(provider)" />
            <u-button size="small" @click="openProviderEdit(provider)">编辑</u-button>
            <u-button size="small" type="danger" plain @click="removeAIProvider(provider)">删除</u-button>
          </div>
        </header>

        <div class="ai-model-toolbar">
          <span class="ai-model-count">模型 {{ provider.models.length }} 个</span>
          <u-button size="small" @click="openModelCreate(provider.id)">手工添加模型</u-button>
          <u-button size="small" @click="openImport(provider)">从 /models 导入</u-button>
        </div>

        <div v-for="model in provider.models" :key="model.id" class="ai-model-row">
          <div class="ai-model-body">
            <div class="ai-model-main">
              <span class="ai-model-id">{{ model.modelId }}</span>
              <span v-if="model.label && model.label !== model.modelId" class="ai-model-label">
                {{ model.label }}
              </span>
              <u-tag v-if="!model.enabled" type="info" size="small">已停用</u-tag>
              <span v-if="model.contextWindow" class="ai-model-meta">
                {{ formatContextWindow(model.contextWindow) }}
              </span>
            </div>
            <p v-if="model.description" class="ai-model-desc">{{ model.description }}</p>
            <p class="ai-model-levels">推理等级：{{ levelSummary(model) }}</p>
          </div>
          <div class="ai-model-actions">
            <u-button text size="small" type="primary" @click="openModelEdit(provider.id, model)">编辑</u-button>
            <u-button text size="small" type="danger" @click="removeAIModel(model)">删除</u-button>
          </div>
        </div>
        <p v-if="!provider.models.length" class="hint">还没有模型：从 /models 导入或手工添加。</p>
      </section>
    </u-card-content>
  </u-card>

  <!-- 供应商新增/编辑 -->
  <u-dialog
    v-model="aiProviderDialog"
    :title="aiProviderEditingId ? '编辑 AI 供应商' : '新增 AI 供应商'"
    :style="dialogStyle(520)"
  >
    <u-form :model="aiProviderForm" :cols="1" label-width="96px">
      <u-input
        label="名称"
        field="name"
        clearable
        placeholder="如 OpenAI / DeepSeek / 本地 vLLM"
        tips="仅用于界面展示的供应商名称"
      />
      <u-input
        label="接口地址"
        field="baseUrl"
        clearable
        :placeholder="baseUrlPlaceholder"
        tips="OpenAI 兼容服务的 base-url，通常以 /v1 结尾；支持任意 OpenAI 兼容供应商"
      />
      <u-password-input
        label="API Key"
        field="apiKey"
        :placeholder="aiProviderHasKey ? '已配置，留空保持不变' : '供应商 API Key（仅保存在服务端）'"
        tips="仅保存在面板服务端且不回显；已配置后留空表示保持不变"
      />
      <u-form-item label="启用">
        <u-switch v-model="aiProviderForm.enabled" active-text="参与对话与模型选择" />
      </u-form-item>
    </u-form>
    <template #footer="{ close }">
      <DialogFooter :loading="aiProviderSaving" @cancel="close" @confirm="saveAIProvider" />
    </template>
  </u-dialog>

  <!-- 模型新增/编辑（含推理等级行编辑器） -->
  <u-dialog
    v-model="aiModelDialog"
    :title="aiModelEditingId ? '编辑模型' : '手工添加模型'"
    :style="dialogStyle(560)"
  >
    <u-form :model="aiModelForm" :cols="1" label-width="96px">
      <u-input
        label="模型 id"
        field="modelId"
        clearable
        placeholder="如 gpt-6.2 / deepseek-v4.1-flash"
        tips="发往上游的模型名，须为该供应商支持的模型且跨供应商全局唯一"
      />
      <u-input
        label="显示名"
        field="label"
        clearable
        placeholder="如 GPT 6.2（可空，缺省显示模型 id）"
        tips="对话弹框模型选择器里展示的名字"
      />
      <u-input
        label="描述"
        field="description"
        clearable
        placeholder="如 旗舰多模态，适合复杂任务（可空）"
        tips="展示在模型选择器，帮助区分模型用途"
      />
      <u-form-item label="上下文窗口">
        <div class="ai-field-column">
          <u-number-input
            v-model="aiModelForm.contextWindow"
            :min="0"
            :step="1000"
            placeholder="不限"
            class="ai-field-control"
          />
          <div class="ai-field-hint">模型上下文窗口（token），0 或留空表示未知</div>
        </div>
      </u-form-item>
      <u-form-item label="启用">
        <u-switch v-model="aiModelForm.enabled" active-text="参与对话与模型选择" />
      </u-form-item>
      <u-form-item label="推理等级">
        <div class="ai-field-column">
          <div v-for="(row, index) in aiLevelRows" :key="index" class="ai-level-row">
            <u-input v-model="row.value" placeholder="值，如 low / medium / high" />
            <u-input v-model="row.label" placeholder="文案，如 低（可空，缺省用值）" />
            <u-button text size="small" type="danger" @click="removeLevelRow(index)">删除</u-button>
          </div>
          <u-button size="small" @click="addLevelRow">添加等级</u-button>
          <u-select
            v-if="aiLevelOptions.length"
            v-model="aiModelForm.defaultReasoningLevel"
            :options="aiLevelOptions"
            clearable
            placeholder="默认推理等级（可空）"
            class="ai-field-control"
          />
          <div class="ai-field-hint">
            值写入请求的 reasoning_effort，文案用于界面展示；同一模型配出 2 档及以上时，
            对话弹框内会出现推理等级选择器，未显式选择时使用默认等级。
          </div>
        </div>
      </u-form-item>
    </u-form>
    <template #footer="{ close }">
      <DialogFooter :loading="aiModelSaving" @cancel="close" @confirm="saveAIModel" />
    </template>
  </u-dialog>

  <!-- 从供应商 /models 拉取并多选导入 -->
  <u-dialog v-model="aiImportDialog" title="从 /models 导入模型" :style="dialogStyle(560)">
    <EmptyState v-if="aiImportFetching" title="正在从供应商 /models 拉取…" />
    <template v-else-if="aiUpstreamModels.length">
      <u-multi-select
        v-model="aiImportSelected"
        :options="aiImportOptions"
        filterable
        placeholder="搜索并多选要导入的模型"
        class="ai-field-control"
      />
      <p class="hint">已存在的模型 id 会自动跳过；导入后可在模型列表里补推理等级与上下文窗口。</p>
    </template>
    <EmptyState
      v-else
      :icon="TriangleAlert"
      title="拉取失败或没有可用模型"
      description="请确认供应商接口地址与 API Key 可用后重试"
      cta-text="重试"
      @cta="retryImport"
    />
    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button
        type="primary"
        :disabled="!aiImportSelected.length"
        :loading="aiImportSaving"
        @click="confirmImport"
      >
        导入所选（{{ aiImportSelected.length }}）
      </u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
/* settings-card / hint / card-header-row / ops-row 为 App.vue 全局类 */

/* 区块说明行：统一次级文案形态 */
.section-desc {
  margin: 0 0 10px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.section-desc:last-child {
  margin-bottom: 0;
}

/* 供应商区块：边框卡片，多个纵向堆叠 */
.ai-provider {
  padding: 10px 12px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
}

.ai-provider + .ai-provider {
  margin-top: 10px;
}

.ai-provider-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.ai-provider-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}

.ai-provider-name {
  font-weight: 600;
}

.ai-provider-url {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
}

.ai-provider-ops {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-model-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 0;
}

.ai-model-count {
  margin-right: auto;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.ai-model-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 0;
  border-top: 1px dashed var(--u-border-color);
}

.ai-model-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.ai-model-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ai-model-id {
  font-weight: 600;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.ai-model-label,
.ai-model-meta {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.ai-model-desc,
.ai-model-levels {
  margin: 0;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.ai-model-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

/* 弹框内自定义复合控件的纵排容器（控件 + 行内说明） */
.ai-field-column {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.ai-field-control {
  width: 100%;
}

.ai-field-hint {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 推理等级行编辑器：值 + 文案 + 删除 */
.ai-level-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}
</style>
