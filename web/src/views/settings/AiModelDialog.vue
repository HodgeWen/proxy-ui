<!-- AI 模型新增 / 编辑弹框（含推理等级行编辑器，行使用递增 id 作为稳定 :key） -->
<script setup lang="ts">
import { UButton, UDialog, UForm, UFormItem, UInput, UNumberInput, USelect, USwitch, message } from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { computed, nextTick, reactive, shallowRef, useTemplateRef, watch } from 'vue'

import { createAIModel, updateAIModel } from '../../api/ai'
import type { AiModel, AiModelInput, AiReasoningLevel } from '../../api/ai'
import DialogFooter from '../../components/DialogFooter.vue'
import { dialogStyle } from '../../dialog-style'

interface LevelRow extends AiReasoningLevel {
  id: number
}

const visible = defineModel<boolean>('visible', { default: false })
const { providerId, model = null } = defineProps<{
  providerId: number
  model?: AiModel | null
}>()
const emit = defineEmits<{ saved: [] }>()

let nextLevelId = 1

function defaultModelForm() {
  return {
    modelId: '',
    label: '',
    description: '',
    contextWindow: null as number | null,
    defaultReasoningLevel: '',
    enabled: true,
    reasoningLevels: [] as LevelRow[],
  }
}

const saving = shallowRef(false)
const formRef = useTemplateRef<FormExposed>('formRef')
const form = reactive(defaultModelForm())

/** 默认等级选项：取行编辑器里非空且不重复的值 */
const levelOptions = computed(() => {
  const seen = new Set<string>()
  const options: { label: string; value: string }[] = []
  for (const row of form.reasoningLevels) {
    const value = row.value.trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    options.push({ label: row.label.trim() || value, value })
  }
  return options
})

watch(visible, (open) => {
  if (!open) return
  Object.assign(
    form,
    defaultModelForm(),
    model
      ? {
          modelId: model.modelId,
          label: model.label,
          description: model.description,
          contextWindow: model.contextWindow || null,
          defaultReasoningLevel: model.defaultReasoningLevel,
          enabled: model.enabled,
          reasoningLevels: model.reasoningLevels.map((level) => ({ id: nextLevelId++, ...level })),
        }
      : {},
  )
  void nextTick(() => formRef.value?.clearValidate())
})

function addLevelRow(): void {
  form.reasoningLevels.push({ id: nextLevelId++, value: '', label: '' })
}

function removeLevelRow(index: number): void {
  const removed = form.reasoningLevels[index]
  form.reasoningLevels = form.reasoningLevels.filter((_, i) => i !== index)
  if (removed && form.defaultReasoningLevel === removed.value.trim()) {
    form.defaultReasoningLevel = ''
  }
}

function validateReasoningLevels(): string {
  const seen = new Set<string>()
  for (const row of form.reasoningLevels) {
    const value = row.value.trim()
    if (!value) return '推理等级的值不能为空'
    if (seen.has(value)) return `推理等级值重复：${value}`
    seen.add(value)
  }
  if (!seen.size && form.defaultReasoningLevel) return '未配置推理等级时不能指定默认等级'
  if (form.defaultReasoningLevel && !seen.has(form.defaultReasoningLevel)) return '默认推理等级不在等级列表内'
  return ''
}

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  const input: AiModelInput = {
    modelId: form.modelId.trim(),
    label: form.label.trim(),
    description: form.description.trim(),
    reasoningLevels: form.reasoningLevels.map((row) => {
      const value = row.value.trim()
      return { value, label: row.label.trim() || value }
    }),
    defaultReasoningLevel: form.defaultReasoningLevel,
    contextWindow: form.contextWindow ?? 0,
    enabled: form.enabled,
  }
  saving.value = true
  try {
    if (model) await updateAIModel(model.id, input)
    else await createAIModel(providerId, input)
    message.success('模型已保存')
    visible.value = false
    emit('saved')
  } catch {
    // 模型 id 重复等错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <u-dialog v-model="visible" :title="model ? '编辑模型' : '手工添加模型'" :style="dialogStyle(560)">
    <u-form ref="formRef" :model="form" :cols="1" label-width="96px">
      <u-input
        label="模型 id"
        field="modelId"
        clearable
        placeholder="如 gpt-6.2 / deepseek-v4.1-flash"
        tips="发往上游的模型名，须为该供应商支持的模型且跨供应商全局唯一"
        :rules="{ required: '模型 id 不能为空', validator: (v: string) => (v.trim() ? '' : '模型 id 不能为空') }"
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
            v-model="form.contextWindow"
            :min="0"
            :step="1000"
            placeholder="不限"
            class="ai-field-control"
          />
          <div class="ai-field-hint">模型上下文窗口（token），0 或留空表示未知</div>
        </div>
      </u-form-item>
      <u-form-item label="启用">
        <u-switch v-model="form.enabled" active-text="参与对话与模型选择" />
      </u-form-item>
      <u-form-item label="推理等级" field="reasoningLevels" :rules="{ validator: validateReasoningLevels }">
        <div class="ai-field-column">
          <div v-for="(row, index) in form.reasoningLevels" :key="row.id" class="ai-level-row">
            <u-input v-model="row.value" placeholder="值，如 low / medium / high" />
            <u-input v-model="row.label" placeholder="文案，如 低（可空，缺省用值）" />
            <u-button text size="small" type="danger" @click="removeLevelRow(index)">删除</u-button>
          </div>
          <u-button size="small" @click="addLevelRow">添加等级</u-button>
          <u-select
            v-if="levelOptions.length"
            v-model="form.defaultReasoningLevel"
            :options="levelOptions"
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
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>

<style scoped>
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
