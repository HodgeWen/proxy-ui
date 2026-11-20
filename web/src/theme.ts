// 主题基建：亮/暗/跟随系统三态切换 + localStorage 持久化 + matchMedia 跟随系统。
// 深浅切换一律换主题对象（loadTheme 热替换），业务代码不写 [data-theme] 分支配色。
import {
  auroraTheme,
  celadonTheme,
  darkTheme,
  gildedTheme,
  guochaoTheme,
  irisTheme,
  lightTheme,
  loadTheme,
  metalTheme,
  midnightTheme,
  neonTheme,
  newspaperTheme,
  wildTheme,
} from '@veltra/styles/theme'
import type { Theme, ThemeSeries, UITheme } from '@veltra/styles/theme'
import { ref } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'

/** 12 预设 id → 主题对象映射（id 即 @veltra/styles/theme 导出的主题对象名），供设置页预设选择复用 */
export const lightPresets = {
  lightTheme,
  irisTheme,
  celadonTheme,
  guochaoTheme,
  wildTheme,
  newspaperTheme,
} as const satisfies Record<string, UITheme>

export const darkPresets = {
  darkTheme,
  auroraTheme,
  midnightTheme,
  neonTheme,
  gildedTheme,
  metalTheme,
} as const satisfies Record<string, UITheme>

export type LightPresetId = keyof typeof lightPresets
export type DarkPresetId = keyof typeof darkPresets

/** UITheme#new 入参的深 Partial（库未导出 RecursivePartial，等价重声明），UTheme 编辑器产物写入此处 */
type DeepPartial<T> = { [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P] }
export type ThemeOverrides = DeepPartial<Theme>

/** localStorage 持久化 key（spec「主题持久化格式」） */
const STORAGE_KEY = 'proxy-ui-theme'

interface PersistedTheme {
  mode: ThemeMode
  light: LightPresetId
  dark: DarkPresetId
  overrides: ThemeOverrides
}

// 模块级共享状态：useTheme 在多处调用返回同一份
const mode = ref<ThemeMode>('system')
const lightId = ref<LightPresetId>('lightTheme')
const darkId = ref<DarkPresetId>('darkTheme')
const overrides = ref<ThemeOverrides>({})

let media: MediaQueryList | undefined
/** 最近一次已 loadTheme 的配置指纹；目标主题未变化时跳过重复 loadTheme */
let appliedKey = ''

function readPersisted(): Partial<PersistedTheme> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    const saved = JSON.parse(raw ?? '{}')
    return typeof saved === 'object' && saved !== null ? saved : {}
  } catch {
    return {} // 值损坏时回落默认
  }
}

function persist(): void {
  const value: PersistedTheme = {
    mode: mode.value,
    light: lightId.value,
    dark: darkId.value,
    overrides: overrides.value,
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
}

/** system 模式按 OS 外观解析实际系列；light/dark 即自身 */
function resolveSeries(): ThemeSeries {
  if (mode.value === 'system') {
    return media?.matches ? 'dark' : 'light'
  }
  return mode.value
}

/** 按当前配置派生主题并热切换（不刷新页面）；系列/预设/覆盖均未变化时不重复 loadTheme */
function applyTheme(): void {
  const series = resolveSeries()
  const key = `${series}:${series === 'dark' ? darkId.value : lightId.value}:${JSON.stringify(overrides.value)}`
  if (key === appliedKey) return

  const base = series === 'dark' ? darkPresets[darkId.value] : lightPresets[lightId.value]
  // 深色态显式带 series: 'dark'；预设本身 reactive:false，改派生字段后需重新 loadTheme
  loadTheme(base.new(overrides.value, { series }))
  appliedKey = key
}

/** 入口（main.ts）mount 前调用：恢复持久化、注册跟随系统监听并加载主题，首帧即正确系列（不闪白） */
export function initTheme(): void {
  const saved = readPersisted()
  if (saved.mode === 'light' || saved.mode === 'dark' || saved.mode === 'system') {
    mode.value = saved.mode
  }
  if (typeof saved.light === 'string' && saved.light in lightPresets) {
    lightId.value = saved.light
  }
  if (typeof saved.dark === 'string' && saved.dark in darkPresets) {
    darkId.value = saved.dark
  }
  if (typeof saved.overrides === 'object' && saved.overrides !== null) {
    overrides.value = saved.overrides
  }

  media = window.matchMedia('(prefers-color-scheme: dark)')
  media.addEventListener('change', applyTheme)
  applyTheme()
}

/** 切换三态：更新状态、持久化并热切换面板主题 */
export function setThemeMode(next: ThemeMode): void {
  mode.value = next
  persist()
  applyTheme()
}

/** 主题状态 composable：mode 三态、浅/深色预设 id、UTheme 编辑器变量覆盖 */
export function useTheme() {
  return { mode, lightId, darkId, overrides }
}
