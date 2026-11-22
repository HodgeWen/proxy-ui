<!-- 设置页「AI 供应商」页签：供应商与模型列表展示、启停与删除交互（新增/编辑/导入弹框已拆出） -->
<script setup lang="ts">
import { UButton, UCard, UCardContent, UCardHeader, USwitch, UTag, message, vLoading } from '@veltra/desktop'
import { computed, onActivated, shallowRef } from 'vue'

import { deleteAIModel, deleteAIProvider, fetchAIProviders, updateAIProvider } from '../../api/ai'
import type { AiModel, AiProvider } from '../../api/ai'
import { useEnabledOverride } from '../../composables/useEnabledOverride'
import { confirmAction } from '../../confirm-action'
import AiImportDialog from './AiImportDialog.vue'
import AiModelDialog from './AiModelDialog.vue'
import AiProviderDialog from './AiProviderDialog.vue'

/** 供应商列表（视图内含各自模型） */
const aiProviders = shallowRef<AiProvider[]>([])
const aiLoading = shallowRef(true)
const deletingProviderId = shallowRef(0)
const deletingModelId = shallowRef(0)

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

/** 启停供应商（apiKey 留空语义为保留已存 Key；走本地覆写表避免失败时开关错位） */
const { rowEnabled, togglingId, toggle: toggleAIProvider } = useEnabledOverride(
  aiProviders,
  async (provider, enabled) => {
    await updateAIProvider(provider.id, {
      name: provider.name,
      baseUrl: provider.baseUrl,
      apiKey: '',
      enabled,
    })
    aiProviders.value = aiProviders.value.map((p) => (p.id === provider.id ? { ...p, enabled } : p))
    await loadAIProviders()
  },
)

async function removeAIProvider(provider: AiProvider): Promise<void> {
  const ok = await confirmAction(
    '删除 AI 供应商',
    `将删除「${provider.name}」及其下 ${provider.models.length} 个模型，不可恢复。确认删除吗？`,
    true,
    '删除',
  )
  if (!ok) return
  deletingProviderId.value = provider.id
  try {
    await deleteAIProvider(provider.id)
    message.success('AI 供应商已删除')
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    deletingProviderId.value = 0
  }
}

async function removeAIModel(model: AiModel): Promise<void> {
  const ok = await confirmAction('删除模型', `确认删除模型「${model.modelId}」吗？`, true, '删除')
  if (!ok) return
  deletingModelId.value = model.id
  try {
    await deleteAIModel(model.id)
    message.success('模型已删除')
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    deletingModelId.value = 0
  }
}

// ---------- 弹框状态 ----------

const providerDialogVisible = shallowRef(false)
const editingProvider = shallowRef<AiProvider | null>(null)

function openProviderCreate(): void {
  editingProvider.value = null
  providerDialogVisible.value = true
}

function openProviderEdit(provider: AiProvider): void {
  editingProvider.value = provider
  providerDialogVisible.value = true
}

const modelDialogVisible = shallowRef(false)
const modelProviderId = shallowRef(0)
const editingModel = shallowRef<AiModel | null>(null)

function openModelCreate(providerId: number): void {
  modelProviderId.value = providerId
  editingModel.value = null
  modelDialogVisible.value = true
}

function openModelEdit(providerId: number, model: AiModel): void {
  modelProviderId.value = providerId
  editingModel.value = model
  modelDialogVisible.value = true
}

const importDialogVisible = shallowRef(false)
const importProviderId = shallowRef(0)
/** 全库已存在的模型 id（选项上标记「已存在」，导入时后端也会跳过） */
const existingModelIds = computed(() => new Set(aiProviders.value.flatMap((p) => p.models.map((m) => m.modelId))))

function openImport(provider: AiProvider): void {
  importProviderId.value = provider.id
  importDialogVisible.value = true
}

// ---------- 展示格式化 ----------

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
      <div v-loading="aiLoading && !aiProviders.length" class="ai-list-wrap">
        <p class="section-desc">
          新增供应商后从其 /models 导入或手工添加模型，每个模型可维护推理等级；Key 仅保存在服务端，
          对话经面板代理转发，从顶栏「AI 助手」打开。
        </p>
        <p v-if="!aiLoading && !aiProviders.length" class="section-desc">
          还没有供应商，点右上角「新增供应商」开始配置。
        </p>

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
              <u-switch
                :model-value="rowEnabled(provider)"
                :disabled="togglingId === provider.id"
                @update:model-value="(v: boolean) => toggleAIProvider(provider, v)"
              />
              <u-button size="small" @click="openProviderEdit(provider)">编辑</u-button>
              <u-button
                size="small"
                type="danger"
                plain
                :loading="deletingProviderId === provider.id"
                @click="removeAIProvider(provider)"
              >
                删除
              </u-button>
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
              <u-button
                text
                size="small"
                type="danger"
                :loading="deletingModelId === model.id"
                @click="removeAIModel(model)"
              >
                删除
              </u-button>
            </div>
          </div>
          <p v-if="!provider.models.length" class="hint">还没有模型：从 /models 导入或手工添加。</p>
        </section>
      </div>
    </u-card-content>
  </u-card>

  <AiProviderDialog v-model:visible="providerDialogVisible" :provider="editingProvider" @saved="loadAIProviders" />
  <AiModelDialog
    v-model:visible="modelDialogVisible"
    :provider-id="modelProviderId"
    :model="editingModel"
    @saved="loadAIProviders"
  />
  <AiImportDialog
    v-model:visible="importDialogVisible"
    :provider-id="importProviderId"
    :existing-ids="existingModelIds"
    @saved="loadAIProviders"
  />
</template>

<style scoped>
/* settings-card / hint / card-header-row / ops-row 为 App.vue 全局类 */

/* 列表容器最小高度：首屏加载遮罩不压扁 */
.ai-list-wrap {
  min-height: 120px;
}

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
</style>
