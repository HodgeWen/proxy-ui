<!-- 全站空状态唯一组件：图标（可传）+ 标题 + 一句说明 + 可选 CTA；
     有可操作动作的空状态必须给 CTA，动作入口已相邻可见时可不传 -->
<script setup lang="ts">
import { UButton } from '@veltra/desktop'
import { Empty } from '@veltra/icons/normal'
import type { Component } from 'vue'

defineProps<{
  /** 标题（空状态主文案） */
  title: string
  /** 一句说明 */
  description?: string
  /** 语义图标组件，默认收件箱 */
  icon?: Component
  /** CTA 按钮文字，传入即渲染 */
  ctaText?: string
}>()

const emit = defineEmits<{ cta: [] }>()
</script>

<template>
  <div class="empty-state">
    <component :is="icon ?? Empty" class="empty-state-icon" />
    <p class="empty-state-title">{{ title }}</p>
    <p v-if="description" class="empty-state-desc">{{ description }}</p>
    <div v-if="ctaText" class="empty-state-cta">
      <u-button type="primary" @click="emit('cta')">{{ ctaText }}</u-button>
    </div>
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px 16px;
  text-align: center;
}

.empty-state-icon {
  width: 48px;
  height: 48px;
  margin-bottom: 8px;
  color: var(--u-text-color-assist);
}

.empty-state-title {
  margin: 0;
  font-size: var(--u-font-size-main-default);
  color: var(--u-text-color-main);
}

.empty-state-desc {
  margin: 4px 0 0;
  max-width: 420px;
  font-size: var(--u-font-size-assist-default);
  color: var(--u-text-color-second);
}

.empty-state-cta {
  margin-top: 12px;
}
</style>
