<!-- 设置页：主题（三态/12 预设/UTheme 编辑器）；账号改密；内置内核（只读状态与日志）；面板在线更新；AI 供应商与模型（多供应商多模型多推理等级）；DNS 与系统优化（BBR） -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import {
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UDialog,
  UForm,
  UFormItem,
  UInput,
  UMultiSelect,
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
import { Check, TriangleAlert } from '@veltra/icons/normal'
import { computed, nextTick, onBeforeUnmount, reactive, shallowRef, useTemplateRef, watch } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import FormGroupCard from '../components/FormGroupCard.vue'
import { dialogStyle } from '../dialog-style'

import {
  createAIModel,
  createAIProvider,
  deleteAIModel,
  deleteAIProvider,
  fetchAIProviders,
  fetchAIUpstreamModels,
  importAIModels,
  updateAIModel,
  updateAIProvider,
} from '../api/ai'
import type { AiModel, AiModelInput, AiProvider, AiReasoningLevel, AiUpstreamModel } from '../api/ai'
import { fetchHealth } from '../api/auth'
import { getDNS, saveDNS } from '../api/dns'
import type { DNSPreset, DNSRule, DNSServer, DNSSettings } from '../api/dns'
import { fetchKernelLogs, fetchKernelStatus } from '../api/kernel'
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

/** 内置内核的只读运行状态与日志（内核编进面板随面板自动运行，不可启停或换版本） */
const status = shallowRef<KernelStatus | null>(null)
const logs = shallowRef<KernelLogLine[]>([])
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

// ---------- AI 供应商与模型 ----------

/** 供应商列表（视图内含各自模型） */
const aiProviders = shallowRef<AiProvider[]>([])
const aiLoading = shallowRef(false)

async function loadAIProviders(silent = true): Promise<void> {
  aiLoading.value = true
  try {
    aiProviders.value = await fetchAIProviders()
  } catch {
    if (!silent) message.error('查询 AI 供应商失败')
  } finally {
    aiLoading.value = false
  }
}

/** 供应商编辑弹框（新增/编辑共用；apiKey 留空表示保留已存 Key） */
const aiProviderDialog = shallowRef(false)
const aiProviderEditingId = shallowRef(0)
const aiProviderSaving = shallowRef(false)
const aiProviderHasKey = shallowRef(false)
const aiProviderForm = reactive({ name: '', baseUrl: '', apiKey: '', enabled: true })

function openProviderCreate(): void {
  aiProviderEditingId.value = 0
  aiProviderForm.name = ''
  aiProviderForm.baseUrl = ''
  aiProviderForm.apiKey = ''
  aiProviderForm.enabled = true
  aiProviderHasKey.value = false
  aiProviderDialog.value = true
}

function openProviderEdit(provider: AiProvider): void {
  aiProviderEditingId.value = provider.id
  aiProviderForm.name = provider.name
  aiProviderForm.baseUrl = provider.baseUrl
  aiProviderForm.apiKey = ''
  aiProviderForm.enabled = provider.enabled
  aiProviderHasKey.value = provider.hasKey
  aiProviderDialog.value = true
}

async function saveAIProvider(): Promise<void> {
  if (!aiProviderForm.name.trim() || !aiProviderForm.baseUrl.trim()) {
    message.error('供应商名称与接口地址不能为空')
    return
  }
  aiProviderSaving.value = true
  try {
    const input = {
      name: aiProviderForm.name.trim(),
      baseUrl: aiProviderForm.baseUrl.trim(),
      apiKey: aiProviderForm.apiKey.trim(),
      enabled: aiProviderForm.enabled,
    }
    if (aiProviderEditingId.value) {
      await updateAIProvider(aiProviderEditingId.value, input)
    } else {
      await createAIProvider(input)
    }
    message.success('AI 供应商已保存')
    aiProviderDialog.value = false
    await loadAIProviders()
  } catch {
    // 校验失败等错误已由请求封装统一提示
  } finally {
    aiProviderSaving.value = false
  }
}

/** 启停供应商（apiKey 留空语义为保留已存 Key） */
async function toggleAIProvider(provider: AiProvider): Promise<void> {
  try {
    await updateAIProvider(provider.id, {
      name: provider.name,
      baseUrl: provider.baseUrl,
      apiKey: '',
      enabled: !provider.enabled,
    })
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  }
}

async function removeAIProvider(provider: AiProvider): Promise<void> {
  const action = await messageConfirm({
    title: '删除 AI 供应商',
    message: `将删除「${provider.name}」及其下 ${provider.models.length} 个模型，不可恢复。确认删除吗？`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteAIProvider(provider.id)
    message.success('AI 供应商已删除')
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  }
}

/** 模型编辑弹框（新增/编辑共用；推理等级为行编辑器） */
const aiModelDialog = shallowRef(false)
const aiModelProviderId = shallowRef(0)
const aiModelEditingId = shallowRef(0)
const aiModelSaving = shallowRef(false)
const aiModelForm = reactive({
  modelId: '',
  label: '',
  description: '',
  contextWindow: null as number | null,
  defaultReasoningLevel: '',
  enabled: true,
})
/** 推理等级行（值 + 文案；默认等级单独下拉选择） */
const aiLevelRows = reactive<{ value: string; label: string }[]>([])

/** 默认等级选项：取行编辑器里非空且不重复的值 */
const aiLevelOptions = computed(() => {
  const seen = new Set<string>()
  const options: { label: string; value: string }[] = []
  for (const row of aiLevelRows) {
    const value = row.value.trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    options.push({ label: row.label.trim() || value, value })
  }
  return options
})

function openModelCreate(providerId: number): void {
  aiModelProviderId.value = providerId
  aiModelEditingId.value = 0
  aiModelForm.modelId = ''
  aiModelForm.label = ''
  aiModelForm.description = ''
  aiModelForm.contextWindow = null
  aiModelForm.defaultReasoningLevel = ''
  aiModelForm.enabled = true
  aiLevelRows.splice(0, aiLevelRows.length)
  aiModelDialog.value = true
}

function openModelEdit(providerId: number, model: AiModel): void {
  aiModelProviderId.value = providerId
  aiModelEditingId.value = model.id
  aiModelForm.modelId = model.modelId
  aiModelForm.label = model.label
  aiModelForm.description = model.description
  aiModelForm.contextWindow = model.contextWindow || null
  aiModelForm.defaultReasoningLevel = model.defaultReasoningLevel
  aiModelForm.enabled = model.enabled
  aiLevelRows.splice(0, aiLevelRows.length, ...model.reasoningLevels.map((level) => ({ ...level })))
  aiModelDialog.value = true
}

function addLevelRow(): void {
  aiLevelRows.push({ value: '', label: '' })
}

function removeLevelRow(index: number): void {
  const [removed] = aiLevelRows.splice(index, 1)
  if (removed && aiModelForm.defaultReasoningLevel === removed.value) {
    aiModelForm.defaultReasoningLevel = ''
  }
}

/** 与后端校验对齐的提交前检查，通过则拼出模型入参 */
function buildModelInput(): AiModelInput | null {
  const modelId = aiModelForm.modelId.trim()
  if (!modelId) {
    message.error('模型 id 不能为空')
    return null
  }
  const levels: AiReasoningLevel[] = []
  const seen = new Set<string>()
  for (const row of aiLevelRows) {
    const value = row.value.trim()
    if (!value) {
      message.error('推理等级的值不能为空')
      return null
    }
    if (seen.has(value)) {
      message.error(`推理等级值重复：${value}`)
      return null
    }
    seen.add(value)
    levels.push({ value, label: row.label.trim() || value })
  }
  if (!levels.length && aiModelForm.defaultReasoningLevel) {
    message.error('未配置推理等级时不能指定默认等级')
    return null
  }
  if (aiModelForm.defaultReasoningLevel && !seen.has(aiModelForm.defaultReasoningLevel)) {
    message.error('默认推理等级不在等级列表内')
    return null
  }
  return {
    modelId,
    label: aiModelForm.label.trim(),
    description: aiModelForm.description.trim(),
    reasoningLevels: levels,
    defaultReasoningLevel: aiModelForm.defaultReasoningLevel,
    contextWindow: aiModelForm.contextWindow ?? 0,
    enabled: aiModelForm.enabled,
  }
}

async function saveAIModel(): Promise<void> {
  const input = buildModelInput()
  if (!input) return
  aiModelSaving.value = true
  try {
    if (aiModelEditingId.value) {
      await updateAIModel(aiModelEditingId.value, input)
    } else {
      await createAIModel(aiModelProviderId.value, input)
    }
    message.success('模型已保存')
    aiModelDialog.value = false
    await loadAIProviders()
  } catch {
    // 模型 id 重复等错误已由请求封装统一提示
  } finally {
    aiModelSaving.value = false
  }
}

async function removeAIModel(model: AiModel): Promise<void> {
  const action = await messageConfirm({
    title: '删除模型',
    message: `确认删除模型「${model.modelId}」吗？`,
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  try {
    await deleteAIModel(model.id)
    message.success('模型已删除')
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  }
}

/** 上游 /models 拉取 + 多选导入弹框 */
const aiImportDialog = shallowRef(false)
const aiImportProviderId = shallowRef(0)
const aiImportFetching = shallowRef(false)
const aiImportSaving = shallowRef(false)
const aiUpstreamModels = shallowRef<AiUpstreamModel[]>([])
const aiImportSelected = shallowRef<string[]>([])

/** 全库已存在的模型 id（选项上标记「已存在」，导入时后端也会跳过） */
const aiExistingIds = computed(() => new Set(aiProviders.value.flatMap((p) => p.models.map((m) => m.modelId))))

const aiImportOptions = computed(() =>
  aiUpstreamModels.value.map((m) => ({
    label: aiExistingIds.value.has(m.id) ? `${m.id}（已存在）` : m.ownedBy ? `${m.id}（${m.ownedBy}）` : m.id,
    value: m.id,
  })),
)

async function openImport(provider: AiProvider): Promise<void> {
  aiImportProviderId.value = provider.id
  aiImportSelected.value = []
  aiUpstreamModels.value = []
  aiImportDialog.value = true
  aiImportFetching.value = true
  try {
    aiUpstreamModels.value = await fetchAIUpstreamModels(provider.id)
  } catch {
    // 拉取失败错误已由请求封装统一提示；弹框内给出重试入口
  } finally {
    aiImportFetching.value = false
  }
}

async function retryImport(): Promise<void> {
  const provider = aiProviders.value.find((p) => p.id === aiImportProviderId.value)
  if (provider) await openImport(provider)
}

async function confirmImport(): Promise<void> {
  if (!aiImportSelected.value.length) {
    message.error('请先选择要导入的模型')
    return
  }
  aiImportSaving.value = true
  try {
    const { imported } = await importAIModels(aiImportProviderId.value, aiImportSelected.value)
    message.success(`已导入 ${imported} 个模型（已存在的自动跳过）`)
    aiImportDialog.value = false
    await loadAIProviders()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    aiImportSaving.value = false
  }
}

/** 模型行推理等级摘要：值（文案）列表 + 默认标记 */
function levelSummary(model: AiModel): string {
  const levels = model.reasoningLevels.map((level) =>
    level.label && level.label !== level.value ? `${level.value}（${level.label}）` : level.value,
  )
  const text = levels.length ? levels.join('、') : '未配置'
  return model.defaultReasoningLevel ? `${text}，默认 ${model.defaultReasoningLevel}` : text
}

/** 上下文窗口展示（如 128K / 1M tokens） */
function formatContextWindow(tokens: number): string {
  if (!tokens) return ''
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(tokens % 1_000_000 === 0 ? 0 : 1)}M tokens`
  }
  return `${(tokens / 1000).toFixed(tokens % 1000 === 0 ? 0 : 1)}K tokens`
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
    message.success('DNS 配置已保存并已按新配置重建内核')
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
      void loadAIProviders()
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
          <div class="info-meta">
            <u-tag :type="status?.running ? 'success' : 'info'">
              {{ status?.running ? '运行中' : '已停止' }}
            </u-tag>
            <span>版本：<strong>{{ status?.version || '内置' }}</strong></span>
            <span>运行时长：{{ formatUptime(status?.uptimeSeconds) }}</span>
          </div>
          <p class="hint">
            内核随面板启动自动运行，无需手动启停；入站、出站、规则、DNS、证书等任一配置保存后都会立即按新配置重建，
            新配置起不来时自动回到上一份可用配置并在保存处报错。
          </p>
          <p class="hint">
            内核已编译进面板二进制，版本随面板版本一同升级，无需也不能单独安装或切换版本。
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
          <div class="info-meta">
            <span>当前版本：<strong>{{ panelVersion || '未知' }}</strong></span>
            <span>最新稳定版：<strong>{{ latestPanel?.tag ?? '未检查' }}</strong></span>
            <u-tag v-if="panelState === 'latest'" type="success">已是最新版</u-tag>
          </div>
          <div class="card-ops">
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
            从未保存过 DNS 时，面板默认就按「国内外分流（推荐）」这套预设运行，下方编辑器里显示的就是它。
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
                    <u-input v-model="server.tag" placeholder="tag，如 ali" title="服务器名字：规则与默认服务器靠它引用，必填且不可重复" />
                    <div v-if="serverTagHint(server)" class="editor-warn">{{ serverTagHint(server) }}</div>
                  </div>
                  <u-select
                    v-model="server.type"
                    :options="typeOptions"
                    :clearable="false"
                    placeholder="类型"
                    title="查询方式：udp / tcp 为明文（可能被劫持污染），tls=DoT、https=DoH、h3=DoH3（加密防污染），local=系统本地解析"
                  />
                  <u-input
                    v-if="server.type !== 'local'"
                    v-model="server.server"
                    placeholder="地址，如 223.5.5.5"
                    title="DNS 服务器地址（域名或 IP）；国内推荐 223.5.5.5，加密 DNS 推荐 1.1.1.1 / 8.8.8.8"
                  />
                  <u-number-input
                    v-if="server.type !== 'local'"
                    v-model="server.serverPort"
                    placeholder="端口（可空）"
                    :min="1"
                    :max="65535"
                    title="留空按类型取默认端口：udp/tcp 53、tls 853、https/h3 443"
                  />
                  <u-input
                    v-if="showPath(server.type)"
                    v-model="server.path"
                    placeholder="DoH 路径，默认 /dns-query"
                    title="DoH / DoH3 的请求路径，绝大多数服务商用默认的 /dns-query，无需修改"
                  />
                  <u-input
                    v-model="server.detour"
                    placeholder="经由出站（可空，如 proxy）"
                    title="留空表示直连发起查询；填出站 tag 则由该出站代发，可避开本地运营商的 DNS 劫持"
                  />
                </div>
                <u-button text size="small" type="danger" @click="removeServer(index)">删除</u-button>
              </div>
              <p v-if="!dnsForm.servers.length" class="section-desc">还没有服务器，请至少添加一个。</p>
              <p class="hint">
                tag 是规则与默认服务器引用它的名字，必须非空且唯一；类型 udp / tcp 为明文查询（可能被劫持污染），
                tls / https / h3 为加密查询（DoT / DoH / DoH3，防污染推荐），local 使用系统本地解析、不需要地址与端口；
                地址是国内推荐 223.5.5.5、加密推荐 1.1.1.1；端口留空按类型取默认（53 / 853 / 443）；
                https / h3 可填 DoH 路径（默认 /dns-query）；「经由出站」留空表示直连查询，填出站 tag
                则由该出站发起查询（走代理防污染时用它）。
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
                  <u-input
                    v-model="rule.domain"
                    placeholder="精确域名，逗号分隔"
                    title="只匹配填写的域名本身，不含子域；如 www.qq.com"
                  />
                  <u-input
                    v-model="rule.domainSuffix"
                    placeholder="域名后缀，逗号分隔"
                    title="匹配该后缀及其所有子域；如 cn（匹配所有 .cn 域名）、baidu.com"
                  />
                  <u-input
                    v-model="rule.domainKeyword"
                    placeholder="域名关键字，逗号分隔"
                    title="域名中只要包含该词就命中；如 taobao"
                  />
                  <div class="editor-cell">
                    <u-select
                      v-model="rule.server"
                      :options="tagOptions"
                      :clearable="false"
                      placeholder="目标服务器"
                      title="命中条件的查询交给哪个 DNS 服务器（上方服务器列表里的 tag）"
                    />
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
          <p class="hint">保存后立即按新配置重建内核；新配置起不来时自动回到上一份可用配置，并在保存处报错。</p>
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
            BBR 是 Google 提出的 TCP 拥塞控制算法，解决的是「传统算法把丢包当拥塞，丢包一多速度就被压死」的问题：
            在有丢包或高延迟的链路上能显著提升吞吐、减少排队延迟，对跨运营商、跨境线路的代理转发尤其明显。
          </p>
          <p class="hint">
            什么时候该开：服务器走跨境 / 跨运营商等丢包较高的线路、以代理转发为主时建议开启；
            纯内网或线路很优质时收益不明显，可不开。副作用：开启会把拥塞控制与队列调度（fq）一起改掉，
            且对本机所有 TCP 连接生效、不只代理流量；面板无权限改 sysctl 或内核不支持（如 OpenVZ 容器）时无法一键开启，
            按页面下方指引手动处理。
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
      <u-card class="settings-card">
        <u-card-header>
          <div class="card-header-row">
            <span>AI 供应商（支持任意 OpenAI 兼容供应商）</span>
            <div class="card-ops">
              <u-button size="small" :loading="aiLoading" @click="loadAIProviders(false)">重新加载</u-button>
              <u-button size="small" type="primary" @click="openProviderCreate">新增供应商</u-button>
            </div>
          </div>
        </u-card-header>
        <u-card-content>
          <p class="section-desc">
            新增供应商（接口地址 + API Key）后，从其 /models 接口拉取并多选导入模型，也可手工添加；
            每个模型可维护推理等级与默认等级。Key 只保存在面板服务端，对话经面板代理流式转发；
            配置完成后从顶栏「AI 助手」按钮打开对话弹框。
          </p>
          <p v-if="!aiProviders.length" class="section-desc">还没有供应商，点右上角「新增供应商」开始配置。</p>

          <!-- 供应商区块：头部信息与操作 + 其下模型列表 -->
          <section v-for="provider in aiProviders" :key="provider.id" class="ai-provider">
            <header class="ai-provider-head">
              <div class="ai-provider-info">
                <span class="ai-provider-name">{{ provider.name }}</span>
                <u-tag v-if="!provider.enabled" type="info" size="small">已停用</u-tag>
                <u-tag v-else-if="!provider.hasKey" type="warning" size="small">缺 API Key</u-tag>
                <span class="ai-provider-url">{{ provider.baseUrl }}</span>
              </div>
              <div class="ai-provider-ops">
                <u-switch :model-value="provider.enabled" @update:model-value="() => toggleAIProvider(provider)" />
                <u-button size="small" @click="openProviderEdit(provider)">编辑</u-button>
                <u-button size="small" type="danger" plain @click="removeAIProvider(provider)">删除</u-button>
              </div>
            </header>

            <div class="ai-model-toolbar">
              <span class="ai-model-count">模型 {{ provider.models.length }} 个</span>
              <u-button size="small" @click="openModelCreate(provider.id)">手工添加模型</u-button>
              <u-button size="small" @click="openImport(provider)">从 /models 导入</u-button>
            </div>

            <div v-for="model in provider.models" :key="model.id" class="ai-model-row">
              <div class="ai-model-body">
                <div class="ai-model-main">
                  <span class="ai-model-id">{{ model.modelId }}</span>
                  <span v-if="model.label && model.label !== model.modelId" class="ai-model-label">
                    {{ model.label }}
                  </span>
                  <u-tag v-if="!model.enabled" type="info" size="small">已停用</u-tag>
                  <span v-if="model.contextWindow" class="ai-model-meta">
                    {{ formatContextWindow(model.contextWindow) }}
                  </span>
                </div>
                <p v-if="model.description" class="ai-model-desc">{{ model.description }}</p>
                <p class="ai-model-levels">推理等级：{{ levelSummary(model) }}</p>
              </div>
              <div class="ai-model-actions">
                <u-button text size="small" type="primary" @click="openModelEdit(provider.id, model)">编辑</u-button>
                <u-button text size="small" type="danger" @click="removeAIModel(model)">删除</u-button>
              </div>
            </div>
            <p v-if="!provider.models.length" class="hint">还没有模型：从 /models 导入或手工添加。</p>
          </section>
        </u-card-content>
      </u-card>

      <!-- 供应商新增/编辑 -->
      <u-dialog
        v-model="aiProviderDialog"
        :title="aiProviderEditingId ? '编辑 AI 供应商' : '新增 AI 供应商'"
        :style="dialogStyle(520)"
      >
        <u-form :model="aiProviderForm" :cols="1" label-width="96px">
          <u-input
            label="名称"
            field="name"
            clearable
            placeholder="如 OpenAI / DeepSeek / 本地 vLLM"
            tips="仅用于界面展示的供应商名称"
          />
          <u-input
            label="接口地址"
            field="baseUrl"
            clearable
            placeholder="如 https://api.openai.com/v1"
            tips="OpenAI 兼容服务的 base-url，通常以 /v1 结尾；支持任意 OpenAI 兼容供应商"
          />
          <u-password-input
            label="API Key"
            field="apiKey"
            :placeholder="aiProviderHasKey ? '已配置，留空保持不变' : '供应商 API Key（仅保存在服务端）'"
            tips="仅保存在面板服务端且不回显；已配置后留空表示保持不变"
          />
          <u-form-item label="启用">
            <u-switch v-model="aiProviderForm.enabled" active-text="参与对话与模型选择" />
          </u-form-item>
        </u-form>
        <template #footer="{ close }">
          <u-button text @click="close()">取消</u-button>
          <u-button type="primary" :loading="aiProviderSaving" @click="saveAIProvider">保存</u-button>
        </template>
      </u-dialog>

      <!-- 模型新增/编辑（含推理等级行编辑器） -->
      <u-dialog
        v-model="aiModelDialog"
        :title="aiModelEditingId ? '编辑模型' : '手工添加模型'"
        :style="dialogStyle(560)"
      >
        <u-form :model="aiModelForm" :cols="1" label-width="96px">
          <u-input
            label="模型 id"
            field="modelId"
            clearable
            placeholder="如 gpt-6.2 / deepseek-v4.1-flash"
            tips="发往上游的模型名，须为该供应商支持的模型且跨供应商全局唯一"
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
                v-model="aiModelForm.contextWindow"
                :min="0"
                :step="1000"
                placeholder="不限"
                class="ai-field-control"
              />
              <div class="ai-field-hint">模型上下文窗口（token），0 或留空表示未知</div>
            </div>
          </u-form-item>
          <u-form-item label="启用">
            <u-switch v-model="aiModelForm.enabled" active-text="参与对话与模型选择" />
          </u-form-item>
          <u-form-item label="推理等级">
            <div class="ai-field-column">
              <div v-for="(row, index) in aiLevelRows" :key="index" class="ai-level-row">
                <u-input v-model="row.value" placeholder="值，如 low / medium / high" />
                <u-input v-model="row.label" placeholder="文案，如 低（可空，缺省用值）" />
                <u-button text size="small" type="danger" @click="removeLevelRow(index)">删除</u-button>
              </div>
              <u-button size="small" @click="addLevelRow">添加等级</u-button>
              <u-select
                v-if="aiLevelOptions.length"
                v-model="aiModelForm.defaultReasoningLevel"
                :options="aiLevelOptions"
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
          <u-button text @click="close()">取消</u-button>
          <u-button type="primary" :loading="aiModelSaving" @click="saveAIModel">保存</u-button>
        </template>
      </u-dialog>

      <!-- 从供应商 /models 拉取并多选导入 -->
      <u-dialog v-model="aiImportDialog" title="从 /models 导入模型" :style="dialogStyle(560)">
        <EmptyState v-if="aiImportFetching" title="正在从供应商 /models 拉取…" />
        <template v-else-if="aiUpstreamModels.length">
          <u-multi-select
            v-model="aiImportSelected"
            :options="aiImportOptions"
            filterable
            placeholder="搜索并多选要导入的模型"
            class="ai-field-control"
          />
          <p class="hint">已存在的模型 id 会自动跳过；导入后可在模型列表里补推理等级与上下文窗口。</p>
        </template>
        <EmptyState
          v-else
          :icon="TriangleAlert"
          title="拉取失败或没有可用模型"
          description="请确认供应商接口地址与 API Key 可用后重试"
          cta-text="重试"
          @cta="retryImport"
        />
        <template #footer="{ close }">
          <u-button text @click="close()">取消</u-button>
          <u-button
            type="primary"
            :disabled="!aiImportSelected.length"
            :loading="aiImportSaving"
            @click="confirmImport"
          >
            导入所选（{{ aiImportSelected.length }}）
          </u-button>
        </template>
      </u-dialog>
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

/* ---------- AI 供应商与模型 ---------- */

/* 供应商区块：边框卡片，多个纵向堆叠 */
.ai-provider {
  padding: 12px 16px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
}

.ai-provider + .ai-provider {
  margin-top: 12px;
}

.ai-provider-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.ai-provider-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}

.ai-provider-name {
  font-weight: 600;
}

.ai-provider-url {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
}

.ai-provider-ops {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-model-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 10px 0;
}

.ai-model-count {
  margin-right: auto;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.ai-model-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-top: 1px dashed var(--u-border-color);
}

.ai-model-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.ai-model-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ai-model-id {
  font-weight: 600;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.ai-model-label,
.ai-model-meta {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.ai-model-desc,
.ai-model-levels {
  margin: 0;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.ai-model-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

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

/* 只读信息行：内核状态与面板版本共用 */
.info-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  margin-bottom: 16px;
  font-size: 14px;
}

/* 卡片操作行 */
.card-ops {
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
