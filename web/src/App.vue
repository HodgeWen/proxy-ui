<script setup lang="ts">
// 根组件：仅承载路由出口，页面结构由各视图自带；弹窗 chrome 覆写与页面通用小样式见下方全局样式。
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

/* ---------- 页面 / 卡片通用小样式：从各视图 scoped 收敛而来，勿再在组件里重复定义；
     与某处确有差异的属性（高度、内距、行距等）在该处 scoped 里按同名类补充覆盖 ---------- */

/* 页面 / 卡片标题行：标题在左、操作在右 */
.page-header,
.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
}

/* 页面标题行右侧操作组 */
.page-ops {
  display: flex;
  gap: 8px;
}

/* 卡片头工具栏：仅剩主操作，靠右与表格操作列对齐 */
.card-ops {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex: 1;
}

/* 卡片内容里的内嵌操作行（不右对齐、间距更松） */
.ops-row {
  display: flex;
  gap: 12px;
}

/* 页顶说明卡（标题 + 要点列表 + 备注） */
.intro-card {
  margin-bottom: 16px;
}

.intro-list {
  margin: 8px 0;
  padding-left: 18px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.intro-note {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 表格 / 列表里的弱化文案 */
.cell-dim,
.dim {
  color: var(--u-text-color-assist);
}

/* 行内等宽文本 */
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
}

/* 等宽代码 / 日志块：统一 chrome（边框、底色、等宽字体与基础内距） */
.code-block {
  margin: 0;
  padding: 10px 12px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-middle);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
  overflow: auto;
}

/* 卡片内容底部的次级提示行 */
.hint {
  margin: 6px 0 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

/* 设置页卡片的纵向节奏与卡片底部操作区 */
.settings-card {
  margin-bottom: 12px;
}

.card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

/* 只读信息行 */
.info-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 8px;
  font-size: 14px;
}
</style>
