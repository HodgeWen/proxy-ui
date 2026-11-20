<!-- 设置页：主题（三态/12 预设/UTheme 编辑器）；账号改密；内核（镜像前缀/版本/启停/日志）；面板与内核在线更新；AI 供应商配置 -->
<script setup lang="ts">
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UForm,
  UInput,
  UPasswordInput,
  USegment,
  USelect,
  UTable,
  UTabs,
  UTag,
  UTheme,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import type { FormExposed, TabItem } from '@veltra/desktop'
import { Check } from '@veltra/icons/normal'
import { computed, nextTick, onBeforeUnmount, reactive, shallowRef, useTemplateRef, watch } from 'vue'

import EmptyState from '../components/EmptyState.vue'
import FormGroupCard from '../components/FormGroupCard.vue'

import { fetchAIConfig, saveAIConfig } from '../api/ai'
import { fetchHealth } from '../api/auth'
import {
  fetchKernelLogs,
  fetchKernelStatus,
  fetchKernelVersions,
  fetchMirrorPrefix,
  installKernel,
  kernelAction,
  updateMirrorPrefix,
} from '../api/kernel'
import type { KernelLogLine, KernelStatus, ReleaseVersion } from '../api/kernel'
import { fetchPanelVersions, updatePanel } from '../api/panel'
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

/** 镜像前缀 */
const mirrorForm = reactive({ prefix: '' })
const savingMirror = shallowRef(false)

/** 内核状态与日志 */
const status = shallowRef<KernelStatus | null>(null)
const logs = shallowRef<KernelLogLine[]>([])
const acting = shallowRef<'start' | 'stop' | 'restart' | ''>('')
const logBox = useTemplateRef<HTMLElement>('logBox')

/** 版本列表 */
const versions = shallowRef<ReleaseVersion[]>([])
const loadingVersions = shallowRef(false)
/** 正在安装的版本 tag；LATEST 表示「安装最新稳定版」 */
const installing = shallowRef('')

let pollTimer: ReturnType<typeof setInterval> | undefined

const versionColumns = defineTableColumns([
  { key: 'version', name: '版本', minWidth: 120 },
  { key: 'channel', name: '类型', width: 90, align: 'center' },
  { key: 'publishedAt', name: '发布时间', minWidth: 170 },
  { key: 'actions', name: '操作', width: 110, align: 'center' },
])

function formatDateTime(iso?: string): string {
  return iso ? new Date(iso).toLocaleString() : '-'
}

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

async function refreshVersions() {
  loadingVersions.value = true
  try {
    const { versions: list } = await fetchKernelVersions()
    versions.value = list
  } catch {
    // 网络错误已由请求封装统一提示
  } finally {
    loadingVersions.value = false
  }
}

async function loadMirror() {
  try {
    const { prefix } = await fetchMirrorPrefix()
    mirrorForm.prefix = prefix
  } catch {
    // 静默：下次保存会重新拉起错误
  }
}

async function saveMirror() {
  savingMirror.value = true
  try {
    const { prefix } = await updateMirrorPrefix(mirrorForm.prefix)
    mirrorForm.prefix = prefix
    message.success('镜像前缀已保存')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    savingMirror.value = false
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

async function handleInstall(tag = '') {
  installing.value = tag || 'LATEST'
  try {
    const result = await installKernel(tag)
    message.success(`内核 ${result.version} 安装成功`)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    installing.value = ''
    await Promise.all([refreshStatus(false), refreshVersions()])
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

// ---------- 面板与内核在线更新 ----------

/** 面板当前版本与可用版本 */
const panelVersion = shallowRef('')
const panelVersions = shallowRef<ReleaseVersion[]>([])
const panelChecking = shallowRef(false)
const panelUpdating = shallowRef(false)
/** 面板更新阶段提示（下载替换 / 等待重启恢复） */
const panelPhase = shallowRef('')

/** 最新稳定版（面板与内核通用取法） */
const latestPanel = computed(() => panelVersions.value.find((item) => !item.prerelease))
const latestKernel = computed(() => versions.value.find((item) => !item.prerelease))

type UpdateState = 'unchecked' | 'latest' | 'available'

/** 更新按钮三态：未检查（禁用并给出原因）/ 已是最新（次级重装）/ 有新版（主操作） */
const panelState = computed<UpdateState>(() => {
  if (!latestPanel.value) return 'unchecked'
  return latestPanel.value.installed ? 'latest' : 'available'
})
const kernelState = computed<UpdateState>(() => {
  if (!latestKernel.value) return 'unchecked'
  return latestKernel.value.installed ? 'latest' : 'available'
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
      void refreshVersions()
      void loadMirror()
      startPolling()
      return
    }
    if (tab === 'update') {
      void checkPanelUpdate()
      void refreshStatus(false)
      void refreshVersions()
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
      <u-card class="kernel-card">
        <u-card-header>下载镜像</u-card-header>
        <u-card-content>
          <u-form :model="mirrorForm" :cols="2" label-width="96px" class="settings-form">
            <u-input
              label="镜像前缀"
              field="prefix"
              :span="1"
              placeholder="镜像前缀，如 https://ghproxy.example；留空直连 GitHub"
              clearable
              tips="配置后所有 GitHub 请求（版本查询与内核下载）均经该前缀代理；面板与内核更新共用"
            />
          </u-form>
          <div class="card-actions">
            <u-button :loading="savingMirror" @click="saveMirror">保存</u-button>
          </div>
        </u-card-content>
      </u-card>

      <u-card class="kernel-card">
        <u-card-header>sing-box 内核</u-card-header>
        <u-card-content>
          <EmptyState
            v-if="status && !status.installed"
            title="尚未安装 sing-box 内核"
            description="在下方版本列表选择版本一键安装，安装后即可在此启动内核"
          />
          <template v-else>
            <div class="kernel-meta">
              <u-tag :type="status?.running ? 'success' : 'info'">
                {{ status?.running ? '运行中' : '已停止' }}
              </u-tag>
              <span>版本：<strong>{{ status?.version || '未知' }}</strong></span>
              <span v-if="status?.running">PID：{{ status.pid }}</span>
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
                type="error"
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
          </template>
        </u-card-content>
      </u-card>

      <u-card class="kernel-card">
        <u-card-header>
          <div class="card-header-row">
            <span>可用版本</span>
            <div class="card-header-ops">
              <u-button :loading="loadingVersions" @click="refreshVersions">刷新版本</u-button>
              <u-button
                type="primary"
                plain
                :loading="installing === 'LATEST'"
                @click="handleInstall()"
              >
                安装最新稳定版
              </u-button>
            </div>
          </div>
        </u-card-header>
        <u-card-content>
          <u-table
            :columns="versionColumns"
            :data="versions"
            row-key="tag"
            border
            :loading="loadingVersions"
          >
            <template #column:channel="{ rowData }">
              <u-tag :type="rowData.prerelease ? 'warning' : 'success'">
                {{ rowData.prerelease ? '预发布' : '稳定版' }}
              </u-tag>
            </template>
            <template #column:publishedAt="{ rowData }">
              {{ formatDateTime(rowData.publishedAt) }}
            </template>
            <template #column:actions="{ rowData }">
              <u-tag v-if="rowData.installed" type="info">已安装</u-tag>
              <u-button
                v-else
                text
                size="small"
                type="primary"
                :loading="installing === rowData.tag"
                @click="handleInstall(rowData.tag)"
              >
                安装
              </u-button>
            </template>
          </u-table>
        </u-card-content>
      </u-card>

      <u-card class="kernel-card">
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
      <u-card class="kernel-card">
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
          <p class="hint">
            更新会替换面板二进制并自动重启进程，数据与配置保留；GitHub
            下载镜像前缀在「内核」分区配置，面板与内核更新共用。
          </p>
        </u-card-content>
      </u-card>

      <u-card class="kernel-card">
        <u-card-header>sing-box 内核更新</u-card-header>
        <u-card-content>
          <div class="kernel-meta">
            <span>
              当前版本：<strong>{{ status?.version || (status?.installed ? '未知' : '未安装') }}</strong>
            </span>
            <span>最新稳定版：<strong>{{ latestKernel?.tag ?? '未检查' }}</strong></span>
            <u-tag v-if="kernelState === 'latest'" type="success">已是最新版</u-tag>
          </div>
          <div class="kernel-actions">
            <u-button :loading="loadingVersions" @click="refreshVersions">检查更新</u-button>
            <u-button
              v-if="kernelState === 'available'"
              type="primary"
              plain
              :loading="installing === 'LATEST'"
              @click="handleInstall()"
            >
              一键更新到最新
            </u-button>
            <u-button v-else-if="kernelState === 'latest'" :loading="installing === 'LATEST'" @click="handleInstall()">
              重装最新版
            </u-button>
            <u-button v-else disabled>一键更新到最新</u-button>
          </div>
          <p v-if="kernelState === 'unchecked'" class="hint">尚未获取可用版本，请先「检查更新」。</p>
          <p class="hint">内核运行中时请先在「内核」分区停止；完整版本列表与内核日志见「内核」分区。</p>
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

.kernel-card {
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

.card-header-ops {
  display: flex;
  gap: 8px;
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
