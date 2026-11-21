<!-- 设置页「主题」页签：三态外观模式 + 12 套预设 chip + UTheme 变量编辑器（自定义覆盖持久化） -->
<script setup lang="ts">
import { UButton, UCard, UCardContent, UCardHeader, USegment, UTag, UTheme, message } from '@veltra/desktop'
import { Check } from '@veltra/icons/normal'
import { computed, shallowRef } from 'vue'

import FormGroupCard from '../../components/FormGroupCard.vue'
import {
  activeSeries,
  captureThemeOverrides,
  darkPresets,
  lightPresets,
  presetLabels,
  setDarkPreset,
  setLightPreset,
  setThemeMode,
  setThemeOverrides,
  useTheme,
} from '../../theme'
import type { DarkPresetId, LightPresetId, ThemeMode, ThemeOverrides } from '../../theme'

const { mode, lightId, darkId, overrides } = useTheme()

const modeItems: { label: string; value: ThemeMode }[] = [
  { label: '亮色', value: 'light' },
  { label: '暗色', value: 'dark' },
  { label: '跟随系统', value: 'system' },
]

/** 预设选择项：展示名 + 取自预设本体的色板预览（运行时值，非 CSS 字面量） */
const lightPresetItems = (Object.keys(lightPresets) as LightPresetId[]).map((id) => ({
  id,
  label: presetLabels[id],
  theme: lightPresets[id].theme,
}))
const darkPresetItems = (Object.keys(darkPresets) as DarkPresetId[]).map((id) => ({
  id,
  label: presetLabels[id],
  theme: darkPresets[id].theme,
}))

/** UTheme 变量编辑器默认折叠（spec：编辑器 640px 高，展开才占屏） */
const editorCollapsed = shallowRef(true)

/** 已持久化自定义覆盖的叶子变量数（0 表示当前为纯预设） */
const overrideCount = computed(() => countLeaves(overrides.value))

function countLeaves(value: unknown): number {
  if (typeof value === 'object' && value !== null) {
    return Object.values(value).reduce<number>((sum, child) => sum + countLeaves(child), 0)
  }
  return 1
}

/** 把 UTheme 实时编辑结果（写入 currentTheme）相对当前预设的差异持久化为覆盖 */
function handleSaveOverrides(): void {
  const captured: ThemeOverrides = captureThemeOverrides()
  setThemeOverrides(captured)
  message.success(
    countLeaves(captured) ? '自定义主题已保存，将在亮 / 暗两态持续生效' : '当前没有相对预设的改动，已保存为空覆盖',
  )
}

function handleClearOverrides(): void {
  setThemeOverrides({})
  message.success('已清除自定义主题覆盖，恢复所选预设')
}
</script>

<template>
  <u-card>
    <u-card-header>主题设置</u-card-header>
    <u-card-content>
      <section class="theme-section">
        <div class="theme-section-title">外观模式</div>
        <u-segment :model-value="mode" :items="modeItems" @update:model-value="setThemeMode" />
        <p class="hint">「跟随系统」随操作系统外观实时切换；选择即时生效并持久化。</p>
      </section>

      <section class="theme-section">
        <div class="theme-section-title">
          浅色预设
          <u-tag v-if="activeSeries === 'light'" size="small">当前生效</u-tag>
        </div>
        <div class="preset-row">
          <button
            v-for="item in lightPresetItems"
            :key="item.id"
            type="button"
            class="preset-chip"
            :class="{ 'is-active': lightId === item.id }"
            @click="setLightPreset(item.id)"
          >
            <span class="preset-swatches">
              <span class="preset-swatch" :style="{ background: item.theme.color.primary }"></span>
              <span class="preset-swatch" :style="{ background: item.theme.bg.color.bottom }"></span>
              <span class="preset-swatch" :style="{ background: item.theme['text-color'].title }"></span>
            </span>
            <span class="preset-name">{{ item.label }}</span>
            <Check v-if="lightId === item.id" class="preset-check" />
          </button>
        </div>
      </section>

      <section class="theme-section">
        <div class="theme-section-title">
          深色预设
          <u-tag v-if="activeSeries === 'dark'" size="small">当前生效</u-tag>
        </div>
        <div class="preset-row">
          <button
            v-for="item in darkPresetItems"
            :key="item.id"
            type="button"
            class="preset-chip"
            :class="{ 'is-active': darkId === item.id }"
            @click="setDarkPreset(item.id)"
          >
            <span class="preset-swatches">
              <span class="preset-swatch" :style="{ background: item.theme.color.primary }"></span>
              <span class="preset-swatch" :style="{ background: item.theme.bg.color.bottom }"></span>
              <span class="preset-swatch" :style="{ background: item.theme['text-color'].title }"></span>
            </span>
            <span class="preset-name">{{ item.label }}</span>
            <Check v-if="darkId === item.id" class="preset-check" />
          </button>
        </div>
        <p class="hint">点击立即预览并持久化；另一侧的预设保留选择，切到对应外观时自动应用。</p>
      </section>

      <FormGroupCard v-model:collapsed="editorCollapsed" title="高级定制（主题变量编辑器）">
        <div class="theme-editor">
          <!-- UTheme 缺省用法：克隆当前主题热替换，编辑实时预览全站 -->
          <UTheme style="height: 640px" />
          <p class="hint">
            编辑器实时改写当前主题用于预览；「保存自定义」把改动相对所选预设的差异写入本浏览器，
            与外观模式 / 预设选择正交组合。未保存的改动在切换模式或预设后丢失。
          </p>
          <div class="card-actions">
            <u-button :disabled="!overrideCount" @click="handleClearOverrides">清除自定义</u-button>
            <u-button type="primary" @click="handleSaveOverrides">
              {{ overrideCount ? `保存自定义（已有 ${overrideCount} 项）` : '保存自定义' }}
            </u-button>
          </div>
        </div>
      </FormGroupCard>
    </u-card-content>
  </u-card>
</template>

<style scoped>
/* hint / card-actions 为 App.vue 全局类 */

.theme-section {
  margin-bottom: 12px;
}

.theme-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  font-size: var(--u-font-size-main-default);
  font-weight: 600;
}

/* 预设 chip 组：单行流式排布，色板 + 名字，替代大块预设卡片 */
.preset-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.preset-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px 3px 5px;
  font: inherit;
  font-size: var(--u-font-size-main-small);
  text-align: left;
  color: var(--u-text-color-main);
  background: var(--u-bg-color-top);
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  cursor: pointer;
}

.preset-chip:hover {
  border-color: var(--u-color-primary);
}

.preset-chip.is-active {
  border-color: var(--u-color-primary);
  box-shadow: 0 0 0 1px var(--u-color-primary);
}

.preset-swatches {
  display: flex;
  flex-shrink: 0;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
  overflow: hidden;
}

.preset-swatch {
  width: 12px;
  height: 12px;
}

.preset-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preset-check {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
  color: var(--u-color-primary);
}

/* FormGroupCard 主体为断点栅格，编辑器整组占满整行 */
.theme-editor {
  grid-column: 1 / -1;
  display: grid;
  gap: 12px;
}
</style>
