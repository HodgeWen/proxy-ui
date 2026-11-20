<script setup lang="ts">
// 根组件：仅承载路由出口，页面结构由各视图自带；弹窗 chrome 全站覆写见下方全局样式。
</script>

<template>
  <router-view />
</template>

<style>
/* 弹窗 chrome：UDialog 默认右上角是 macOS 红绿灯圆点（彩色圆底、图标仅悬浮可见），
   全站统一覆写为标准图标按钮——无彩色圆底、图标常显、悬停浮面底（关闭悬停转危险色）。
   库未提供关闭按钮配置项，这里以子选择器提升优先级覆盖组件默认样式，不改库本体。 */
.u-dialog .u-dialog__btn-close,
.u-dialog .u-dialog__btn-maximize,
.u-dialog .u-dialog__btn-recover {
  width: 24px;
  height: 24px;
  border-radius: var(--u-radius-small);
  background-color: transparent;
  color: var(--u-text-color-second);
}

.u-dialog .u-dialog__btn-close:hover,
.u-dialog .u-dialog__btn-maximize:hover,
.u-dialog .u-dialog__btn-recover:hover {
  background-color: var(--u-bg-color-hover);
}

.u-dialog .u-dialog__btn-close:hover {
  color: var(--u-color-danger);
  background-color: var(--u-color-danger-a-8);
}

.u-dialog .u-dialog__btn-close > svg,
.u-dialog .u-dialog__btn-maximize > svg,
.u-dialog .u-dialog__btn-recover > svg {
  opacity: 1;
}
</style>
