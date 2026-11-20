<!-- 设置页：主题（三态/12 预设/UTheme 编辑器）；账号改密；内置内核（只读版本/启停/日志）；面板在线更新；AI 供应商配置；DNS 与系统优化（BBR） -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import {
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UForm,
  UInput,
  UNumberInput,
  UPasswordInput,
  USegment,
  USelect,
  USwitch,
  UTabs,
  UTag,
  UTheme,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { FormExposed, TabItem } from '@veltra/desktop'
import { Check } from '@veltra/icons/normal'
import { computed, nextTick, onBeforeUnmount, reactive, shallowRef, useTemplateRef, watch } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import FormGroupCard from '../components/FormGroupCard.vue'

import { fetchAIConfig, saveAIConfig } from '../api/ai'
import { fetchHealth } from '../api/auth'
import { getDNS, saveDNS } from '../api/dns'
import type { DNSPreset, DNSRule, DNSServer, DNSSettings } from '../api/dns'
import { fetchKernelLogs, fetchKernelStatus, kernelAction } from '../api/kernel'
import type { KernelLogLine, KernelStatus } from '../api/kernel'
import { fetchPanelVersions, updatePanel } from '../api/panel'
import type { ReleaseVersion } from '../api/panel'
import { getSystemStatus, setBBR } from '../api/system'
import type { SystemStatus } from '../api/system'
import { useAuthStore } from '../stores/auth'
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
} from '../theme'
import type { DarkPresetId, LightPresetId, ThemeMode, ThemeOverrides } from '../theme'

const auth = useAuthStore()

const active = shallowRef('theme')
const tabs: TabItem[] = [
  { key: 'theme', name: '主题' },
  { key: 'account', name: '账号' },
  { key: 'kernel', name: '内核' },
  { key: 'update', name: '面板更新' },
  { key: 'ai', name: 'AI 供应商' },
  { key: 'dns', name: 'DNS' },
  { key: 'system', name: '系统优化' },
]

// ---------- 主题设置 ----------

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

// ---------- 账号 ----------

const formRef = useTemplateRef<FormExposed>('formRef')
const saving = shallowRef(false)
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })

async function handleSave() {
  if (!(await formRef.value?.validate())) return
  saving.value = true
  try {
    await auth.doChangePassword(form.oldPassword, form.newPassword)
    message.success('密码已修改')
    formRef.value?.reset()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

// ---------- 内核管理 ----------

/** 内置内核的运行状态与日志（内核编进面板，版本随面板固定，不可换版本） */
const status = shallowRef<KernelStatus | null>(null)
const logs = shallowRef<KernelLogLine[]>([])
const acting = shallowRef<'start' | 'stop' | 'restart' | ''>('')
const logBox = useTemplateRef<HTMLElement>('logBox')

let pollTimer: ReturnType<typeof setInterval> | undefined

function formatUptime(seconds?: number): string {
  if (seconds === undefined) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  return [h, m, s].map((n) => String(n).padStart(2, '0')).join(':')
}

async function refreshStatus(silent = true) {
  try {
    status.value = await fetchKernelStatus()
  } catch {
    if (!silent) message.error('查询内核状态失败')
  }
}

async function refreshLogs(silent = true) {
  try {
    logs.value = await fetchKernelLogs(500)
    await nextTick()
    if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
  } catch {
    if (!silent) message.error('查询内核日志失败')
  }
}

async function handleKernelAction(action: 'start' | 'stop' | 'restart') {
  acting.value = action
  try {
    await kernelAction(action)
    message.success(action === 'start' ? '内核已启动' : action === 'stop' ? '内核已停止' : '内核已重启')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    acting.value = ''
    await refreshStatus()
    await refreshLogs()
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    void refreshStatus()
    void refreshLogs()
  }, 5000)
}

function stopPolling() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = undefined
}

// ---------- 面板在线更新 ----------

/** 面板当前版本与可用版本 */
const panelVersion = shallowRef('')
const panelVersions = shallowRef<ReleaseVersion[]>([])
const panelChecking = shallowRef(false)
const panelUpdating = shallowRef(false)
/** 面板更新阶段提示（下载替换 / 等待重启恢复） */
const panelPhase = shallowRef('')

/** 最新稳定版 */
const latestPanel = computed(() => panelVersions.value.find((item) => !item.prerelease))

type UpdateState = 'unchecked' | 'latest' | 'available'

/** 更新按钮三态：未检查（禁用并给出原因）/ 已是最新（次级重装）/ 有新版（主操作） */
const panelState = computed<UpdateState>(() => {
  if (!latestPanel.value) return 'unchecked'
  return latestPanel.value.installed ? 'latest' : 'available'
})

async function checkPanelUpdate() {
  panelChecking.value = true
  try {
    const { currentVersion, versions: list } = await fetchPanelVersions()
    panelVersion.value = currentVersion
    panelVersions.value = list
  } catch {
    // 网络错误已由请求封装统一提示
  } finally {
    panelChecking.value = false
  }
}

async function handlePanelUpdate() {
  if (!latestPanel.value) return
  panelUpdating.value = true
  panelPhase.value = '正在下载并替换面板二进制…'
  try {
    const { version } = await updatePanel('')
    panelPhase.value = `已更新到 ${version}，面板正在重启，等待服务恢复…`
    if (await waitPanelOnline(version)) {
      message.success(`面板已更新到 ${version}`)
      window.location.reload()
    } else {
      message.warn('面板服务恢复超时，请稍后手动刷新查看')
      panelPhase.value = ''
    }
  } catch {
    panelPhase.value = ''
    // 下载失败错误已由请求封装统一提示
  } finally {
    panelUpdating.value = false
  }
}

/** 等待面板重启恢复且版本号变为目标版本；60 秒内未恢复返回 false */
async function waitPanelOnline(targetVersion: string): Promise<boolean> {
  for (let i = 0; i < 60; i++) {
    await new Promise((resolve) => setTimeout(resolve, 1000))
    try {
      const health = await fetchHealth()
      if (health.version === targetVersion) return true
    } catch {
      // 服务重启中，继续等待
    }
  }
  return false
}

// ---------- AI 供应商 ----------

/** AI 供应商表单（apiKey 留空表示保留已存 Key） */
const aiForm = reactive({ baseUrl: '', model: '', apiKey: '', reasoningEffort: '' })
/** 是否已配置 api-key（决定输入框占位提示） */
const aiHasKey = shallowRef(false)
const aiSaving = shallowRef(false)

/** 推理强度选项：留空不指定 */
const reasoningOptions = [
  { label: '不指定', value: '' },
  { label: 'low（低）', value: 'low' },
  { label: 'medium（中）', value: 'medium' },
  { label: 'high（高）', value: 'high' },
]

async function loadAIConfig() {
  try {
    const config = await fetchAIConfig()
    aiForm.baseUrl = config.baseUrl
    aiForm.model = config.model
    aiForm.reasoningEffort = config.reasoningEffort
    aiForm.apiKey = ''
    aiHasKey.value = config.hasKey
  } catch {
    // 静默：下次保存会重新拉起错误
  }
}

async function saveAI() {
  if (!aiForm.baseUrl.trim() || !aiForm.model.trim()) {
    message.error('接口地址与模型名不能为空')
    return
  }
  aiSaving.value = true
  try {
    const config = await saveAIConfig({
      baseUrl: aiForm.baseUrl.trim(),
      model: aiForm.model.trim(),
      apiKey: aiForm.apiKey.trim(),
      reasoningEffort: aiForm.reasoningEffort,
    })
    aiForm.apiKey = ''
    aiHasKey.value = config.hasKey
    message.success(config.configured ? 'AI 供应商配置已保存' : '已保存，还需填写 API Key 才能对话')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    aiSaving.value = false
  }
}

// ---------- DNS ----------

const dnsLoading = shallowRef(false)
const dnsSaving = shallowRef(false)
/** 是否已加载过：再次切回本页签不覆盖未保存的编辑，需要刷新请点「重新加载」 */
const dnsLoaded = shallowRef(false)
const dnsTypes = shallowRef<string[]>([])
const dnsStrategies = shallowRef<string[]>([])
const dnsPresets = shallowRef<DNSPreset[]>([])

/** 本地编辑副本：进入页签时填充，保存成功用后端返回回填 */
const dnsForm = reactive({
  enabled: true,
  strategy: '',
  servers: [] as DNSServer[],
  rules: [] as DNSRule[],
  final: '',
})

const typeOptions = computed(() => dnsTypes.value.map((type) => ({ label: type, value: type })))

const strategyOptions = computed(() => [
  { label: '不指定（内核默认）', value: '' },
  ...dnsStrategies.value.map((strategy) => ({ label: strategy, value: strategy })),
])

/** 当前非空 tag，供默认服务器与规则引用 */
const dnsTags = computed(() => dnsForm.servers.map((item) => item.tag.trim()).filter(Boolean))

const tagOptions = computed(() => dnsTags.value.map((tag) => ({ label: tag, value: tag })))

/** 重复的非空 tag（行内提示 + 提交前拦截） */
const duplicateTags = computed(() => {
  const seen = new Set<string>()
  const duplicated = new Set<string>()
  for (const tag of dnsTags.value) {
    if (seen.has(tag)) duplicated.add(tag)
    seen.add(tag)
  }
  return duplicated
})

/** 单行提示：tag 为空或重复时返回提示文案 */
function serverTagHint(server: DNSServer): string {
  const tag = server.tag.trim()
  if (!tag) return 'tag 必填，规则与默认服务器靠它引用'
  if (duplicateTags.value.has(tag)) return `tag「${tag}」与其它服务器重复`
  return ''
}

/** 与后端 Validate 对齐的前端静态检查，给出行内提示（最终仍以后端 400 为准） */
const dnsProblems = computed<string[]>(() => {
  if (!dnsForm.enabled) return []
  const problems: string[] = []
  if (!dnsForm.servers.length) problems.push('启用 DNS 时至少需要一个 DNS 服务器。')
  if (dnsForm.servers.some((item) => !item.tag.trim())) problems.push('服务器 tag 不能为空。')
  if (duplicateTags.value.size) problems.push(`服务器 tag 重复：${[...duplicateTags.value].join('、')}。`)
  if (dnsForm.servers.some((item) => item.type !== 'local' && !(item.server ?? '').trim())) {
    problems.push('除 local 类型外，服务器地址必填。')
  }
  if (dnsForm.final && !dnsTags.value.includes(dnsForm.final)) problems.push('默认服务器不在现有 tag 中。')
  if (dnsForm.rules.some((rule) => !dnsTags.value.includes(rule.server))) problems.push('存在规则没有选择有效的目标服务器。')
  return problems
})

/** 深拷贝一份配置，避免预设对象被表单改动污染 */
function cloneSettings(config: DNSSettings): DNSSettings {
  return {
    enabled: config.enabled,
    strategy: config.strategy ?? '',
    servers: (config.servers ?? []).map((item) => ({ ...item })),
    rules: (config.rules ?? []).map((item) => ({ ...item })),
    final: config.final ?? '',
  }
}

function fillDNS(config: DNSSettings): void {
  const next = cloneSettings(config)
  dnsForm.enabled = next.enabled
  dnsForm.strategy = next.strategy
  dnsForm.servers = next.servers
  dnsForm.rules = next.rules
  dnsForm.final = next.final
}

async function loadDNS(): Promise<void> {
  dnsLoading.value = true
  try {
    const data = await getDNS()
    dnsTypes.value = data.types
    dnsStrategies.value = data.strategies
    dnsPresets.value = data.presets
    fillDNS(data.config)
    dnsLoaded.value = true
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    dnsLoading.value = false
  }
}

/** 应用预设只覆盖本地表单（会丢弃未保存改动，先确认），需再点「保存 DNS」才落库 */
async function handleApplyPreset(preset: DNSPreset): Promise<void> {
  const action = await messageConfirm({
    title: '应用 DNS 预设',
    message: `将用「${preset.name}」覆盖当前编辑中的服务器与规则，未保存的改动会丢失。确认继续吗？`,
    confirmButtonText: '应用',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  // 预设本身不带启用状态，保留用户当前的开关选择
  fillDNS({ ...preset.config, enabled: dnsForm.enabled })
  message.success(
    dnsForm.enabled ? '预设已填入，确认无误后点击「保存 DNS」' : '预设已填入，但 DNS 当前处于关闭状态，需打开开关并保存',
  )
}

function addServer(): void {
  dnsForm.servers.push({ tag: '', type: dnsTypes.value[0] ?? 'udp', server: '' })
}

function removeServer(index: number): void {
  dnsForm.servers.splice(index, 1)
}

function addRule(): void {
  dnsForm.rules.push({ domainSuffix: '', server: dnsTags.value[0] ?? '' })
}

function removeRule(index: number): void {
  dnsForm.rules.splice(index, 1)
}

/** https / h3 需要 DoH 路径；local 不需要地址与端口 */
function showPath(type: string): boolean {
  return type === 'https' || type === 'h3'
}

/** 提交前清洗：去掉空白可选字段与空端口，避免被后端当作有效值 */
function buildDNSPayload(): DNSSettings {
  return {
    enabled: dnsForm.enabled,
    strategy: dnsForm.strategy || '',
    servers: dnsForm.servers.map((item) => {
      const server: DNSServer = { tag: item.tag.trim(), type: item.type }
      const address = (item.server ?? '').trim()
      if (item.type !== 'local' && address) server.server = address
      if (item.type !== 'local' && item.serverPort) server.serverPort = item.serverPort
      if (showPath(item.type) && item.path?.trim()) server.path = item.path.trim()
      if (item.detour?.trim()) server.detour = item.detour.trim()
      return server
    }),
    rules: dnsForm.rules.map((rule) => {
      const next: DNSRule = { server: rule.server }
      if (rule.domain?.trim()) next.domain = rule.domain.trim()
      if (rule.domainSuffix?.trim()) next.domainSuffix = rule.domainSuffix.trim()
      if (rule.domainKeyword?.trim()) next.domainKeyword = rule.domainKeyword.trim()
      return next
    }),
    final: dnsForm.final || '',
  }
}

async function handleSaveDNS(): Promise<void> {
  dnsSaving.value = true
  try {
    const saved = await saveDNS(buildDNSPayload())
    fillDNS(saved)
    message.success('DNS 配置已保存；到「入站」页点「应用配置」后服务端生效')
  } catch {
    // 校验失败等错误已由请求封装统一提示
  } finally {
    dnsSaving.value = false
  }
}

// ---------- 系统优化 ----------

const systemStatus = shallowRef<SystemStatus | null>(null)
const systemLoading = shallowRef(false)
const bbrActing = shallowRef(false)

const platformLabels: Record<string, string> = { linux: 'Linux', darwin: 'macOS', windows: 'Windows' }

const platformLabel = computed(() => {
  const platform = systemStatus.value?.platform ?? ''
  return platform ? (platformLabels[platform] ?? platform) : '-'
})

async function loadSystemStatus(silent = true): Promise<void> {
  systemLoading.value = true
  try {
    systemStatus.value = await getSystemStatus()
  } catch {
    if (!silent) message.error('查询系统状态失败')
  } finally {
    systemLoading.value = false
  }
}

async function handleBBR(enable: boolean): Promise<void> {
  const action = await messageConfirm({
    title: enable ? '一键开启 BBR' : '关闭 BBR',
    message: enable
      ? '将写入 /etc/sysctl.d/99-proxy-ui-bbr.conf（fq + bbr）并执行 sysctl --system 立即生效，重启后仍然保留。确认开启吗？'
      : '将把拥塞控制恢复为 cubic、队列规则恢复为 fq_codel，并执行 sysctl --system 立即生效。确认关闭 BBR 吗？',
    confirmButtonText: enable ? '开启' : '关闭',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  bbrActing.value = true
  try {
    systemStatus.value = await setBBR(enable)
    message.success(enable ? 'BBR 已开启' : '已关闭 BBR，恢复 cubic')
  } catch {
    // 无权限等错误已由请求封装统一提示
  } finally {
    bbrActing.value = false
  }
}

async function copyManualCommand(): Promise<void> {
  const command = systemStatus.value?.manualCommand
  if (!command) return
  try {
    await clipboard.copy(command)
    message.success('命令已复制，请在服务器上以 root 执行')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

watch(
  active,
  (tab) => {
    if (tab !== 'kernel') {
      stopPolling()
    }
    if (tab === 'ai') {
      void loadAIConfig()
      return
    }
    if (tab === 'kernel') {
      void refreshStatus(false)
      void refreshLogs()
      startPolling()
      return
    }
    if (tab === 'update') {
      void checkPanelUpdate()
      return
    }
    if (tab === 'dns') {
      // 首次进入才拉取，避免切走再回来时覆盖正在编辑的内容
      if (!dnsLoaded.value) void loadDNS()
      return
    }
    if (tab === 'system') {
      void loadSystemStatus(false)
    }
  },
)

onBeforeUnmount(stopPolling)
</script>

<template>
  <u-tabs v-model="active" :items="tabs">
    <template #theme>
      <u-card>
        <u-card-header>主题设置</u-card-header>
        <u-card-content>
          <section class="theme-section">
            <div class="theme-section-title">外观模式</div>
            <u-segment :model-value="mode" :items="modeItems" @update:model-value="setThemeMode" />
            <p class="hint">「跟随系统」随操作系统外观实时切换亮色 / 暗色；选择即时生效并持久化在本浏览器。</p>
          </section>

          <section class="theme-section">
            <div class="theme-section-title">
              浅色预设
              <u-tag v-if="activeSeries === 'light'" size="small">当前生效</u-tag>
            </div>
            <div class="preset-grid">
              <button
                v-for="item in lightPresetItems"
                :key="item.id"
                type="button"
                class="preset-item"
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
            <div class="preset-grid">
              <button
                v-for="item in darkPresetItems"
                :key="item.id"
                type="button"
                class="preset-item"
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
            <p class="hint">点击预设立即预览并持久化；未生效一侧的预设保留选择，切到对应亮 / 暗外观时自动应用。</p>
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

    <template v-slot:account>
      <u-card>
        <u-card-header>
          <div class="card-header-row">
            <span>管理员账号</span>
            <span class="header-meta">当前账号：{{ auth.user?.username || '-' }}</span>
          </div>
        </u-card-header>
        <u-card-content>
          <u-form ref="formRef" :model="form" :cols="2" label-width="96px" class="settings-form">
            <u-password-input
              label="原密码"
              field="oldPassword"
              :span="1"
              placeholder="请输入原密码"
              tips="当前登录账号的密码，修改密码前需先验证身份"
              :rules="{ required: '请输入原密码' }"
            />
            <u-password-input
              label="新密码"
              field="newPassword"
              :span="1"
              placeholder="至少 8 位"
              tips="新密码至少 8 位、最多 72 位；建议混合字母、数字与符号"
              :rules="{ required: '请输入新密码', minLen: [8, '密码至少 8 位'], maxLen: [72, '密码最多 72 位'] }"
            />
            <u-password-input
              label="确认新密码"
              field="confirm"
              :span="1"
              placeholder="再次输入新密码"
              tips="再次输入一遍新密码，两次不一致将无法保存"
              :rules="{
                required: '请再次输入新密码',
                validator: (value: string) =>
                  value === form.newPassword ? '' : '两次输入的密码不一致',
              }"
            />
          </u-form>
          <div class="card-actions">
            <u-button type="primary" :loading="saving" @click="handleSave">保存密码</u-button>
          </div>
        </u-card-content>
      </u-card>
    </template>

    <template #kernel>
      <u-card class="settings-card">
        <u-card-header>
          <div class="card-header-row">
            <span>内置 sing-box 内核</span>
            <u-tag type="info">随面板编译，不可更换版本</u-tag>
          </div>
        </u-card-header>
        <u-card-content>
          <div class="kernel-meta">
            <u-tag :type="status?.running ? 'success' : 'info'">
              {{ status?.running ? '运行中' : '已停止' }}
            </u-tag>
            <span>版本：<strong>{{ status?.version || '内置' }}</strong></span>
            <span v-if="status?.running">运行时长：{{ formatUptime(status.uptimeSeconds) }}</span>
          </div>
          <div class="kernel-actions">
            <u-button
              type="primary"
              :disabled="status?.running"
              :loading="acting === 'start'"
              @click="handleKernelAction('start')"
            >
              启动
            </u-button>
            <u-button
              type="danger"
              :disabled="!status?.running"
              :loading="acting === 'stop'"
              @click="handleKernelAction('stop')"
            >
              停止
            </u-button>
            <u-button :disabled="!status?.running" :loading="acting === 'restart'" @click="handleKernelAction('restart')">
              重启
            </u-button>
          </div>
          <p class="hint">
            内核已编译进面板二进制，版本随面板版本一同升级，无需也不能单独安装或切换版本。
            配置改动请到「入站」页点「应用配置」重新生成。
          </p>
        </u-card-content>
      </u-card>

      <u-card class="settings-card">
        <u-card-header>
          <div class="card-header-row">
            <span>内核日志</span>
            <u-button @click="refreshLogs(false)">刷新日志</u-button>
          </div>
        </u-card-header>
        <u-card-content>
          <EmptyState v-if="!logs.length" title="暂无内核日志" />
          <div v-else ref="logBox" class="kernel-log">
            <div v-for="(item, index) in logs" :key="index" class="kernel-log-line">
              <span class="kernel-log-time">{{ new Date(item.time).toLocaleTimeString() }}</span>
              <span :class="['kernel-log-stream', `is-${item.stream}`]">{{ item.stream }}</span>
              <span class="kernel-log-text">{{ item.line }}</span>
            </div>
          </div>
        </u-card-content>
      </u-card>
    </template>

    <template #update>
      <u-card class="settings-card">
        <u-card-header>面板更新</u-card-header>
        <u-card-content>
          <div class="kernel-meta">
            <span>当前版本：<strong>{{ panelVersion || '未知' }}</strong></span>
            <span>最新稳定版：<strong>{{ latestPanel?.tag ?? '未检查' }}</strong></span>
            <u-tag v-if="panelState === 'latest'" type="success">已是最新版</u-tag>
          </div>
          <div class="kernel-actions">
            <u-button :loading="panelChecking" @click="checkPanelUpdate">检查更新</u-button>
            <u-button
              v-if="panelState === 'available'"
              type="primary"
              :loading="panelUpdating"
              @click="handlePanelUpdate"
            >
              一键更新
            </u-button>
            <u-button v-else-if="panelState === 'latest'" :loading="panelUpdating" @click="handlePanelUpdate">
              重装最新版
            </u-button>
            <u-button v-else disabled>一键更新</u-button>
          </div>
          <p v-if="panelState === 'unchecked'" class="hint">尚未获取可用版本，「一键更新」待「检查更新」完成后可用。</p>
          <p v-if="panelPhase" class="hint">{{ panelPhase }}</p>
          <p class="hint">更新会替换面板二进制并自动重启进程，数据与配置保留；内置内核随面板一同升级。</p>
        </u-card-content>
      </u-card>
    </template>

    <!-- 插槽书写顺序与页签显示顺序无关（显示顺序由 tabs 数组决定）；放在 #ai 之前是为了规避 vue-tsc 对末尾插槽的类型推断问题 -->
    <template #dns>
      <u-card size="small" class="settings-card">
        <u-card-header>DNS 在这里作用在哪</u-card-header>
        <u-card-content>
          <p class="section-desc">
            DNS 决定「域名解析成哪个 IP、由哪台服务器来解析」。面板里的这份配置会同时写进两处：服务端 sing-box
            自身的配置（面板所在服务器自己出网时的解析），以及下发给客户端的订阅（sing-box / mihomo
            客户端本地的解析）。因此这里改一次，服务端与客户端行为保持一致。
          </p>
          <p class="section-desc">
            国内域名走国内 DNS、其余走 DoH 之类分流可以兼顾速度与防污染；把查询 detour 到代理出站还能避开本地运营商的
            DNS 劫持。
          </p>
        </u-card-content>
      </u-card>

      <u-card class="settings-card">
        <u-card-header>预设方案</u-card-header>
        <u-card-content>
          <div class="section-desc">
            预设只是一份模板：点击应用会把服务器、规则与默认服务器填进下方编辑器并丢弃未保存的改动，不会自动保存。
          </div>
          <div class="preset-list">
            <div v-for="preset in dnsPresets" :key="preset.id" class="dns-preset">
              <div class="dns-preset-info">
                <div class="dns-preset-name">{{ preset.name }}</div>
                <div class="dns-preset-desc">{{ preset.description }}</div>
              </div>
              <u-button type="primary" plain @click="handleApplyPreset(preset)">应用此预设</u-button>
            </div>
            <div v-if="!dnsPresets.length" class="section-desc">暂无可用预设。</div>
          </div>
        </u-card-content>
      </u-card>

      <u-card class="settings-card">
        <u-card-header>
          <div class="card-header-row">
            <span>DNS 配置</span>
            <u-button :loading="dnsLoading" @click="loadDNS">重新加载</u-button>
          </div>
        </u-card-header>
        <u-card-content>
          <div class="settings-row">
            <div class="settings-field">
              <div class="settings-label">启用自定义 DNS</div>
              <div class="settings-body">
                <u-switch v-model="dnsForm.enabled" />
                <div class="settings-desc">
                  关闭后服务端配置与订阅都不会注入 dns 段，域名解析交还给内核 / 客户端默认行为。
                </div>
              </div>
            </div>
            <div v-if="dnsForm.enabled" class="settings-field">
              <div class="settings-label">默认解析策略</div>
              <div class="settings-body">
                <u-select
                  v-model="dnsForm.strategy"
                  :options="strategyOptions"
                  :clearable="false"
                  class="settings-input"
                />
                <div class="settings-desc">
                  优先返回 IPv4 还是 IPv6。拿不准就选「不指定（内核默认）」，由内核按系统偏好决定。
                </div>
              </div>
            </div>
          </div>

          <u-alert
            v-if="!dnsForm.enabled"
            type="info"
            class="block-alert"
            title="DNS 已关闭"
            description="当前不会向服务端配置与订阅注入 DNS 段：面板服务器与客户端都使用各自的内核默认解析。需要分流或防污染时再打开。"
          />

          <template v-else>
            <!-- 服务器列表：tag / 类型 / 地址 / 端口 / DoH 路径 / 经由出站 -->
            <section class="editor-block">
              <div class="editor-head">
                <div class="editor-title">
                  DNS 服务器
                  <u-tag size="small" type="info">{{ dnsForm.servers.length }} 个</u-tag>
                </div>
                <u-button size="small" @click="addServer">新增服务器</u-button>
              </div>
              <div v-for="(server, index) in dnsForm.servers" :key="index" class="editor-row">
                <div class="editor-fields">
                  <div class="editor-cell">
                    <u-input v-model="server.tag" placeholder="tag，如 ali" />
                    <div v-if="serverTagHint(server)" class="editor-warn">{{ serverTagHint(server) }}</div>
                  </div>
                  <u-select
                    v-model="server.type"
                    :options="typeOptions"
                    :clearable="false"
                    placeholder="类型"
                  />
                  <u-input
                    v-if="server.type !== 'local'"
                    v-model="server.server"
                    placeholder="地址，如 223.5.5.5"
                  />
                  <u-number-input
                    v-if="server.type !== 'local'"
                    v-model="server.serverPort"
                    placeholder="端口（可空）"
                    :min="1"
                    :max="65535"
                  />
                  <u-input
                    v-if="showPath(server.type)"
                    v-model="server.path"
                    placeholder="DoH 路径，默认 /dns-query"
                  />
                  <u-input v-model="server.detour" placeholder="经由出站（可空，如 proxy）" />
                </div>
                <u-button text size="small" type="danger" @click="removeServer(index)">删除</u-button>
              </div>
              <p v-if="!dnsForm.servers.length" class="section-desc">还没有服务器，请至少添加一个。</p>
              <p class="hint">
                tag 是规则与默认服务器引用它的名字，必须非空且唯一；local 类型使用系统本地解析，不需要地址与端口；
                https / h3 可填 DoH 路径；「经由出站」留空表示直连查询，填出站 tag 则由该出站发起查询。
              </p>
            </section>

            <!-- 分流规则：域名条件 → 指定服务器 -->
            <section class="editor-block">
              <div class="editor-head">
                <div class="editor-title">
                  分流规则
                  <u-tag size="small" type="info">{{ dnsForm.rules.length }} 条</u-tag>
                </div>
                <u-button size="small" @click="addRule">新增规则</u-button>
              </div>
              <div v-for="(rule, index) in dnsForm.rules" :key="index" class="editor-row">
                <div class="editor-fields">
                  <u-input v-model="rule.domain" placeholder="精确域名，逗号分隔" />
                  <u-input v-model="rule.domainSuffix" placeholder="域名后缀，逗号分隔" />
                  <u-input v-model="rule.domainKeyword" placeholder="域名关键字，逗号分隔" />
                  <div class="editor-cell">
                    <u-select v-model="rule.server" :options="tagOptions" :clearable="false" placeholder="目标服务器" />
                    <div v-if="!rule.server" class="editor-warn">请选择目标服务器</div>
                  </div>
                </div>
                <u-button text size="small" type="danger" @click="removeRule(index)">删除</u-button>
              </div>
              <p v-if="!dnsForm.rules.length" class="section-desc">
                没有规则时全部查询都走默认服务器。
              </p>
              <p class="hint">
                规则自上而下匹配：同一行填多个条件时任一命中即生效；多个值用英文逗号分隔（如
                qq.com,baidu.com）。未命中任何规则的查询走下方默认服务器。
              </p>
            </section>

            <div class="settings-row">
              <div class="settings-field">
                <div class="settings-label">默认服务器</div>
                <div class="settings-body">
                  <u-select
                    v-model="dnsForm.final"
                    :options="tagOptions"
                    placeholder="留空则取第一个服务器"
                    class="settings-input"
                  />
                  <div class="settings-desc">未命中任何规则的查询交给它；留空时使用服务器列表中的第一个。</div>
                </div>
              </div>
            </div>

            <u-alert
              v-if="dnsProblems.length"
              type="warning"
              class="block-alert"
              title="配置还有问题"
              :description="dnsProblems.join(' ')"
            />
          </template>

          <div class="card-actions">
            <u-button type="primary" :loading="dnsSaving" @click="handleSaveDNS">保存 DNS</u-button>
          </div>
          <p class="hint">保存只写入面板配置；需到「入站」页点击「应用配置」重新生成并重启内核后才真正生效。</p>
        </u-card-content>
      </u-card>
    </template>

    <template #system>
      <u-card class="settings-card">
        <u-card-header>
          <div class="card-header-row">
            <span>系统优化（TCP 拥塞控制）</span>
            <u-button :loading="systemLoading" @click="loadSystemStatus(false)">刷新状态</u-button>
          </div>
        </u-card-header>
        <u-card-content>
          <div class="status-grid">
            <div class="status-item">
              <div class="status-label">运行平台</div>
              <div class="status-value">{{ platformLabel }}</div>
            </div>
            <div class="status-item">
              <div class="status-label">当前拥塞控制算法</div>
              <div class="status-value">
                <u-tag :type="systemStatus?.congestionCtl ? 'primary' : 'info'">
                  {{ systemStatus?.congestionCtl || '读取不到' }}
                </u-tag>
              </div>
            </div>
            <div class="status-item">
              <div class="status-label">BBR 状态</div>
              <div class="status-value">
                <u-tag :type="systemStatus?.bbrEnabled ? 'success' : 'info'">
                  {{ systemStatus?.bbrEnabled ? '已开启' : '未开启' }}
                </u-tag>
              </div>
            </div>
            <div class="status-item">
              <div class="status-label">内核支持的算法</div>
              <div class="status-value status-tags">
                <u-tag v-for="cc in systemStatus?.availableCc ?? []" :key="cc" size="small">{{ cc }}</u-tag>
                <span v-if="!systemStatus?.availableCc?.length" class="status-dim">读取不到</span>
              </div>
            </div>
            <div class="status-item">
              <div class="status-label">可用队列规则</div>
              <div class="status-value status-tags">
                <u-tag v-for="qdisc in systemStatus?.availableQdisc ?? []" :key="qdisc" size="small">
                  {{ qdisc }}
                </u-tag>
                <span v-if="!systemStatus?.availableQdisc?.length" class="status-dim">读取不到</span>
              </div>
            </div>
          </div>

          <!-- 可一键开启：主操作；已开启：展示状态 + 次级关闭（仅在面板有权限改 sysctl 时提供） -->
          <div v-if="systemStatus?.bbrEnabled || systemStatus?.supported" class="bbr-actions">
            <template v-if="systemStatus.bbrEnabled">
              <u-tag type="success">已开启</u-tag>
              <u-button :disabled="!systemStatus.supported" :loading="bbrActing" @click="handleBBR(false)">
                关闭 BBR
              </u-button>
            </template>
            <u-button
              v-else-if="systemStatus.supported"
              type="primary"
              :loading="bbrActing"
              @click="handleBBR(true)"
            >
              一键开启 BBR
            </u-button>
          </div>
          <p v-else-if="!systemStatus" class="section-desc">正在读取系统状态…</p>

          <u-alert
            v-if="systemStatus && !systemStatus.supported"
            type="warning"
            class="block-alert"
            title="无法在面板内一键开启"
            :description="systemStatus.reason || '当前环境不支持在面板内修改拥塞控制算法，请用下方命令手动开启。'"
          />

          <p class="hint">
            BBR 是 Google 提出的拥塞控制算法，在有丢包或高延迟的链路上能显著提升吞吐、减少排队延迟，
            对跨运营商、跨境线路的代理转发尤其明显。开启后对本机所有 TCP 连接生效。
          </p>
          <p class="hint">
            这里只管系统级的 TCP 拥塞控制；入站若使用 QUIC / Hysteria 之类的协议，其单独的拥塞控制设置在各入站的
            「高级选项」里配置。
          </p>
        </u-card-content>
      </u-card>

      <u-card v-if="systemStatus && !systemStatus.supported" class="settings-card">
        <u-card-header>
          <div class="card-header-row">
            <span>手动开启（在服务器上以 root 执行）</span>
            <u-button size="small" @click="copyManualCommand">复制</u-button>
          </div>
        </u-card-header>
        <u-card-content>
          <pre class="code-block">{{ systemStatus.manualCommand }}</pre>
          <p class="hint">
            命令会写入 /etc/sysctl.d/99-proxy-ui-bbr.conf 并执行 sysctl --system 立即生效，重启后仍然保留。
            执行完回到本页点「刷新状态」确认。
          </p>
        </u-card-content>
      </u-card>
    </template>

    <template #ai>
      <u-card>
        <u-card-header>AI 供应商（OpenAI 兼容）</u-card-header>
        <u-card-content>
          <u-form :model="aiForm" :cols="2" label-width="96px" class="settings-form">
            <u-input
              label="接口地址"
              field="baseUrl"
              span="full"
              clearable
              placeholder="如 https://api.openai.com/v1（OpenAI 兼容 base-url）"
              tips="OpenAI 兼容服务的 base-url，通常以 /v1 结尾；对话由面板代理流式转发到该地址"
            />
            <u-input
              label="模型"
              field="model"
              :span="1"
              clearable
              placeholder="如 gpt-4o-mini / deepseek-chat"
              tips="对话使用的模型名，需为上方供应商支持的模型"
            />
            <u-password-input
              label="API Key"
              field="apiKey"
              :span="1"
              :placeholder="aiHasKey ? '已配置，留空保持不变' : '供应商 API Key（仅保存在服务端）'"
              tips="供应商 API Key，仅保存在面板服务端且不回显；已配置后留空表示保持不变"
            />
            <u-select
              label="推理强度"
              field="reasoningEffort"
              :span="1"
              :options="reasoningOptions"
              tips="推理模型的思考深度；非推理模型选「不指定」"
            />
          </u-form>
          <div class="card-actions">
            <u-button type="primary" :loading="aiSaving" @click="saveAI">保存配置</u-button>
          </div>
          <p class="hint">
            Key 只保存在面板服务端，对话经面板代理流式转发；配置后可在「AI 助手」页对话并让 AI
            协助查询与修改面板配置。
          </p>
        </u-card-content>
      </u-card>
    </template>

  </u-tabs>
</template>

<style scoped>
.settings-form {
  margin-bottom: 16px;
}

.hint {
  margin: 8px 0 0;
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

/* 页面级表单操作区：卡片底部右侧 */
.card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.header-meta {
  font-size: var(--u-font-size-main-small);
  color: var(--u-text-color-second);
}

/* ---------- DNS 与系统优化 ---------- */

.settings-card {
  margin-bottom: 16px;
}

/* 区块说明行：统一次级文案形态 */
.section-desc {
  margin: 0 0 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.section-desc:last-child {
  margin-bottom: 0;
}

.dns-preset {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
}

.preset-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.dns-preset-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.dns-preset-name {
  font-weight: 600;
}

.dns-preset-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.block-alert {
  margin: 12px 0;
}

.settings-row {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-field {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.settings-label {
  width: 150px;
  flex-shrink: 0;
  line-height: 32px;
}

.settings-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.settings-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.settings-input {
  width: 100%;
}

/* 服务器 / 规则行编辑器 */
.editor-block {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--u-border-color);
}

.editor-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.editor-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.editor-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 0;
}

.editor-row + .editor-row {
  border-top: 1px dashed var(--u-border-color);
}

.editor-fields {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 8px;
  min-width: 0;
}

/* 单格：控件 + 行内校验提示 */
.editor-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.editor-warn {
  color: var(--u-color-warning);
  font-size: var(--u-font-size-main-small);
}

/* 系统优化状态栅格 */
.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.status-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.status-label {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.status-value {
  font-size: 14px;
}

.status-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.status-dim {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.bbr-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

/* 手动命令代码块：等宽 + 可横向滚动 */
.code-block {
  margin: 0;
  padding: 12px 14px;
  overflow-x: auto;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-middle);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
  white-space: pre;
}

/* ---------- 主题设置 ---------- */

.theme-section {
  margin-bottom: 20px;
}

.theme-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  font-size: var(--u-font-size-main-default);
  font-weight: 600;
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 10px;
}

.preset-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  font: inherit;
  text-align: left;
  color: var(--u-text-color-main);
  background: var(--u-bg-color-top);
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
  cursor: pointer;
}

.preset-item:hover {
  border-color: var(--u-color-primary);
}

.preset-item.is-active {
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
  width: 14px;
  height: 14px;
}

.preset-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preset-check {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  color: var(--u-color-primary);
}

/* FormGroupCard 主体为断点栅格，编辑器整组占满整行 */
.theme-editor {
  grid-column: 1 / -1;
  display: grid;
  gap: 12px;
}

/* ---------- 内核管理 ---------- */

.kernel-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  margin-bottom: 16px;
  font-size: 14px;
}

.kernel-actions {
  display: flex;
  gap: 12px;
}

.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 1;
}

.kernel-log {
  height: 320px;
  overflow: auto;
  padding: 10px 12px;
  border-radius: var(--u-radius-small);
  background: var(--u-bg-color-middle);
  border: 1px solid var(--u-border-color);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.kernel-log-line {
  display: flex;
  gap: 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

.kernel-log-time {
  color: var(--u-text-color-second);
  flex-shrink: 0;
}

.kernel-log-stream {
  flex-shrink: 0;
  color: var(--u-color-primary);
}

.kernel-log-stream.is-stderr {
  color: var(--u-color-danger);
}

.kernel-log-stream.is-panel {
  color: var(--u-color-warning);
}
</style>
