<!-- AI 供应商新增 / 编辑弹框；apiKey 留空表示保留已存 Key -->
<script setup lang="ts">
import { UDialog, UForm, UFormItem, UInput, UPasswordInput, USwitch, message } from '@veltra/desktop'
import type { FormExposed } from '@veltra/desktop'
import { nextTick, reactive, shallowRef, useTemplateRef, watch } from 'vue'

import { createAIProvider, updateAIProvider } from '../../api/ai'
import type { AiProvider } from '../../api/ai'
import DialogFooter from '../../components/DialogFooter.vue'
import { dialogStyle } from '../../dialog-style'

const visible = defineModel<boolean>('visible', { default: false })
const { provider = null } = defineProps<{ provider?: AiProvider | null }>()
const emit = defineEmits<{ saved: [] }>()

function defaultProviderForm() {
  return { name: '', baseUrl: '', apiKey: '', enabled: true }
}

/** 含 // 的示例 URL 放 script 侧绑定：模板属性字符串里的 // 会被 vue-tsc 的单行 props 改写误当注释，损坏生成代码 */
const baseUrlPlaceholder = '如 https://api.openai.com/v1'

const saving = shallowRef(false)
const formRef = useTemplateRef<FormExposed>('formRef')
const form = reactive(defaultProviderForm())

watch(visible, (open) => {
  if (!open) return
  Object.assign(
    form,
    defaultProviderForm(),
    provider ? { name: provider.name, baseUrl: provider.baseUrl, enabled: provider.enabled } : {},
  )
  void nextTick(() => formRef.value?.clearValidate())
})

async function handleSave(): Promise<void> {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    const input = {
      name: form.name.trim(),
      baseUrl: form.baseUrl.trim(),
      apiKey: form.apiKey.trim(),
      enabled: form.enabled,
    }
    if (provider) await updateAIProvider(provider.id, input)
    else await createAIProvider(input)
    message.success('AI 供应商已保存')
    visible.value = false
    emit('saved')
  } catch {
    // 校验失败等错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <u-dialog
    v-model="visible"
    :title="provider ? '编辑 AI 供应商' : '新增 AI 供应商'"
    :style="dialogStyle(520)"
  >
    <u-form ref="formRef" :model="form" :cols="1" label-width="96px">
      <u-input
        label="名称"
        field="name"
        clearable
        placeholder="如 OpenAI / DeepSeek / 本地 vLLM"
        tips="仅用于界面展示的供应商名称"
        :rules="{ required: '请输入供应商名称', validator: (v: string) => (v.trim() ? '' : '请输入供应商名称') }"
      />
      <u-input
        label="接口地址"
        field="baseUrl"
        clearable
        :placeholder="baseUrlPlaceholder"
        tips="OpenAI 兼容服务的 base-url，通常以 /v1 结尾；支持任意 OpenAI 兼容供应商"
        :rules="{ required: '请输入接口地址', validator: (v: string) => (v.trim() ? '' : '请输入接口地址') }"
      />
      <u-password-input
        label="API Key"
        field="apiKey"
        :placeholder="provider?.hasKey ? '已配置，留空保持不变' : '供应商 API Key（仅保存在服务端）'"
        tips="仅保存在面板服务端且不回显；已配置后留空表示保持不变"
      />
      <u-form-item label="启用">
        <u-switch v-model="form.enabled" active-text="参与对话与模型选择" />
      </u-form-item>
    </u-form>
    <template #footer="{ close }">
      <DialogFooter :loading="saving" @cancel="close" @confirm="handleSave" />
    </template>
  </u-dialog>
</template>
