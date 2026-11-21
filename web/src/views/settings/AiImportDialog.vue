<!-- 从 AI 供应商 /models 拉取并多选导入模型的弹框 -->
<script setup lang="ts">
import { UButton, UDialog, UMultiSelect, message } from '@veltra/desktop'
import { TriangleAlert } from '@veltra/icons/normal'
import { computed, shallowRef, watch } from 'vue'

import { fetchAIUpstreamModels, importAIModels } from '../../api/ai'
import type { AiUpstreamModel } from '../../api/ai'
import EmptyState from '../../components/EmptyState.vue'
import { dialogStyle } from '../../dialog-style'

const visible = defineModel<boolean>('visible', { default: false })
const { providerId, existingIds } = defineProps<{
  providerId: number
  existingIds: Set<string>
}>()
const emit = defineEmits<{ saved: [] }>()

const fetching = shallowRef(false)
const saving = shallowRef(false)
const upstreamModels = shallowRef<AiUpstreamModel[]>([])
const selected = shallowRef<string[]>([])

const options = computed(() =>
  upstreamModels.value.map((m) => ({
    label: existingIds.has(m.id) ? `${m.id}（已存在）` : m.ownedBy ? `${m.id}（${m.ownedBy}）` : m.id,
    value: m.id,
  })),
)

async function loadUpstream(): Promise<void> {
  if (!providerId) return
  selected.value = []
  upstreamModels.value = []
  fetching.value = true
  try {
    upstreamModels.value = await fetchAIUpstreamModels(providerId)
  } catch {
    // 拉取失败错误已由请求封装统一提示；弹框内给出重试入口
  } finally {
    fetching.value = false
  }
}

watch(visible, (open) => {
  if (open) void loadUpstream()
})

async function handleConfirm(): Promise<void> {
  if (!selected.value.length) return
  saving.value = true
  try {
    const { imported } = await importAIModels(providerId, selected.value)
    message.success(`已导入 ${imported} 个模型（已存在的自动跳过）`)
    visible.value = false
    emit('saved')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <u-dialog v-model="visible" title="从 /models 导入模型" :style="dialogStyle(560)">
    <EmptyState v-if="fetching" title="正在从供应商 /models 拉取…" />
    <template v-else-if="upstreamModels.length">
      <u-multi-select
        v-model="selected"
        :options="options"
        filterable
        placeholder="搜索并多选要导入的模型"
        class="import-select"
      />
      <p class="hint">已存在的模型 id 会自动跳过；导入后可在模型列表里补推理等级与上下文窗口。</p>
    </template>
    <EmptyState
      v-else
      :icon="TriangleAlert"
      title="拉取失败或没有可用模型"
      description="请确认供应商接口地址与 API Key 可用后重试"
      cta-text="重试"
      @cta="loadUpstream"
    />
    <template #footer="{ close }">
      <u-button text @click="close()">取消</u-button>
      <u-button type="primary" :disabled="!selected.length" :loading="saving" @click="handleConfirm">
        导入所选（{{ selected.length }}）
      </u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
/* hint 为 App.vue 全局类 */
.import-select {
  width: 100%;
}
</style>
