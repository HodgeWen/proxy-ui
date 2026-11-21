<!-- 弹框表单分组卡片：加粗标题栏（底部 1px 分隔线）+ chevron 点击折叠，默认展开；
     主体固定 2 列栅格（弹框宽度恒定，无需断点响应），表单项经 span 占列、'full' 占整行 -->
<script setup lang="ts">
import { UGrid } from '@veltra/desktop'
import { Down } from '@veltra/icons/normal'

defineProps<{
  /** 分组标题（标题栏整行点击折叠） */
  title: string
  /** 高亮脉冲（快捷跳转目标提示，短暂描边后淡出） */
  highlight?: boolean
}>()

const collapsed = defineModel<boolean>('collapsed', { default: false })
</script>

<template>
  <section class="form-group-card" :class="{ 'is-highlight': highlight }">
    <button type="button" class="form-group-card__header" @click="collapsed = !collapsed">
      <span class="form-group-card__title">{{ title }}</span>
      <Down class="form-group-card__chevron" :class="{ 'is-collapsed': collapsed }" />
    </button>
    <div v-show="!collapsed" class="form-group-card__body">
      <!-- cols 固定 2 列：弹框宽度恒定（dialogStyle），无需断点；纯数字 cols 也不挂
           ResizeObserver，避免对象断点在弹框打开动画期间触发 ResizeObserver loop 告警 -->
      <u-grid :cols="2" gap="0 12px">
        <slot />
      </u-grid>
    </div>
  </section>
</template>

<style scoped>
.form-group-card {
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-top);
  box-shadow: 0 0 0 0 transparent;
  transition: box-shadow var(--u-transition-slow) var(--u-transition-ease-out);
}

.form-group-card.is-highlight {
  box-shadow: 0 0 0 2px var(--u-color-primary);
}

/* 表单栅格行间距为 0，相邻分组卡片由自身补垂直间距 */
.form-group-card:not(:last-child) {
  margin-bottom: 12px;
}

.form-group-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 10px 14px;
  font: inherit;
  text-align: left;
  color: var(--u-text-color-main);
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default) var(--u-radius-default) 0 0;
  cursor: pointer;
}

.form-group-card__title {
  font-size: var(--u-font-size-main-default);
  font-weight: 600;
}

.form-group-card__chevron {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  color: var(--u-text-color-second);
  transition: transform var(--u-transition-fast) var(--u-transition-ease-out);
}

.form-group-card__chevron.is-collapsed {
  transform: rotate(-90deg);
}

.form-group-card__body {
  padding: 12px 14px;
}
</style>

<style>
/* 占整行只对表单栅格内的卡片有意义（scoped 样式选不到祖先，故单独一条非 scoped 规则）：
   不在 u-form 里（如设置页主题卡内直接摆放）时保持常规块级流，不给无关父容器写栅格坐标 */
.u-form .form-group-card {
  grid-column: 1 / -1;
}
</style>
