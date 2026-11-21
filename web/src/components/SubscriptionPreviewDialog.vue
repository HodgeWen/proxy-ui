<!-- 订阅配置预览弹框：sing-box / mihomo 双格式切换，等宽只读代码区展示该用户的真实导出；
     预览忽略配额与到期，只要有启用中的节点就渲染，无节点时给出明确原因 -->
<script setup lang="ts">
import { UAlert, UButton, UDialog, USegment, vLoading } from '@veltra/desktop'
import { shallowRef, watch } from 'vue'

import { previewSubscription } from '../api/subscription'
import type { PreviewFormat, SubscriptionToken } from '../api/subscription'
import { dialogStyle } from '../dialog-style'
import { copyText } from '../utils/clipboard'

const visible = defineModel<boolean>('visible', { default: false })
const { token } = defineProps<{ token: SubscriptionToken | null }>()

const FORMAT_ITEMS: { label: string; value: PreviewFormat }[] = [
  { label: 'sing-box', value: 'sing-box' },
  { label: 'mihomo', value: 'mihomo' },
]

const format = shallowRef<PreviewFormat>('sing-box')
const content = shallowRef('')
const loading = shallowRef(false)
const error = shallowRef('')

async function load(): Promise<void> {
  if (!token) return
  loading.value = true
  error.value = ''
  try {
    const res = await previewSubscription(token.id, format.value)
    content.value = res.content
  } catch (e) {
    content.value = ''
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

// 打开即加载默认格式（sing-box）；关闭时重置格式，避免重开时触发 format watcher 并发发两次请求
watch(visible, (open) => {
  if (open) {
    void load()
  } else {
    format.value = 'sing-box'
  }
})
watch(format, () => {
  if (visible.value) void load()
})
</script>

<template>
  <u-dialog
    v-model="visible"
    :title="`订阅配置预览 · ${token?.name || '未命名'}`"
    :style="dialogStyle(1080, 94)"
  >
    <template #default="{ maximized }">
      <div class="preview" :style="{ height: maximized ? '100%' : 'calc(90vh - 150px)' }">
        <div class="toolbar">
          <u-segment v-model="format" :items="FORMAT_ITEMS" />
          <u-button size="small" :disabled="!content" @click="copyText(content)">复制</u-button>
        </div>
        <p class="note">预览忽略配额与到期状态，仅展示该用户当前可用节点的真实导出。</p>
        <u-alert
          v-if="error"
          class="error"
          type="warning"
          title="无法生成预览"
          :description="error"
          show-icon
        />
        <pre v-else v-loading="loading" class="code-block code">{{ content }}</pre>
      </div>
    </template>
  </u-dialog>
</template>

<style scoped>
.preview {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.note {
  margin: 0;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.error {
  margin: 0;
}

/* 等宽只读代码区：chrome 取全局 code-block，这里补占满剩余高度与更宽的内距行距 */
.code {
  flex: 1;
  min-height: 0;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}
</style>
