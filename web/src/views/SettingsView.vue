<!-- 设置页：账号分区改密；内核分区提供镜像前缀、版本下载安装、启停与日志；更新分区面板与内核在线更新；AI 分区配置 OpenAI 兼容供应商 -->
<script setup lang="ts">
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UForm,
  UInput,
  UPasswordInput,
  USelect,
  UTable,
  UTabs,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import type { FormExposed, TabItem } from '@veltra/desktop'
import { computed, nextTick, onBeforeUnmount, reactive, shallowRef, watch } from 'vue'

import EmptyState from '../components/EmptyState.vue'

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

const auth = useAuthStore()

const active = shallowRef('account')
const tabs: TabItem[] = [
  { key: 'account', name: '账号' },
  { key: 'kernel', name: '内核' },
  { key: 'update', name: '面板更新' },
  { key: 'ai', name: 'AI 供应商' },
]

const formRef = shallowRef<FormExposed>()
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
const mirror = shallowRef('')
const savingMirror = shallowRef(false)

/** 内核状态与日志 */
const status = shallowRef<KernelStatus | null>(null)
const logs = shallowRef<KernelLogLine[]>([])
const acting = shallowRef<'start' | 'stop' | 'restart' | ''>('')
const logBox = shallowRef<HTMLElement>()

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
    mirror.value = prefix
  } catch {
    // 静默：下次保存会重新拉起错误
  }
}

async function saveMirror() {
  savingMirror.value = true
  try {
    const { prefix } = await updateMirrorPrefix(mirror.value)
    mirror.value = prefix
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
    <template v-slot:account>
      <u-card>
        <u-card-header>管理员账号</u-card-header>
        <u-card-content>
          <u-form ref="formRef" :model="form" :cols="1" label-width="88px" class="settings-form">
            <u-password-input
              label="原密码"
              field="oldPassword"
              placeholder="请输入原密码"
              :rules="{ required: '请输入原密码' }"
            />
            <u-password-input
              label="新密码"
              field="newPassword"
              placeholder="至少 8 位"
              :rules="{ required: '请输入新密码', minLen: [8, '密码至少 8 位'], maxLen: [72, '密码最多 72 位'] }"
            />
            <u-password-input
              label="确认新密码"
              field="confirm"
              placeholder="再次输入新密码"
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
          <div class="mirror-row">
            <u-input
              v-model="mirror"
              placeholder="镜像前缀，如 https://ghproxy.example；留空直连 GitHub"
              clearable
            />
          </div>
          <p class="hint">配置后所有 GitHub 请求（版本查询与内核下载）均经该前缀代理。</p>
          <div class="card-actions">
            <u-button type="primary" :loading="savingMirror" @click="saveMirror">保存</u-button>
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
          </div>
          <div class="kernel-actions">
            <u-button :loading="panelChecking" @click="checkPanelUpdate">检查更新</u-button>
            <u-button
              type="primary"
              :loading="panelUpdating"
              :disabled="!latestPanel"
              @click="handlePanelUpdate"
            >
              {{ latestPanel?.installed ? '重装最新版' : '一键更新' }}
            </u-button>
          </div>
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
          </div>
          <div class="kernel-actions">
            <u-button :loading="loadingVersions" @click="refreshVersions">检查更新</u-button>
            <u-button
              type="primary"
              :loading="installing === 'LATEST'"
              :disabled="!latestKernel"
              @click="handleInstall()"
            >
              {{ latestKernel?.installed ? '重装最新版' : '一键更新到最新' }}
            </u-button>
          </div>
          <p class="hint">内核运行中时请先在「内核」分区停止；完整版本列表与内核日志见「内核」分区。</p>
        </u-card-content>
      </u-card>
    </template>

    <template #ai>
      <u-card>
        <u-card-header>AI 供应商（OpenAI 兼容）</u-card-header>
        <u-card-content>
          <u-form :model="aiForm" :cols="1" label-width="88px" class="settings-form">
            <u-input
              label="接口地址"
              field="baseUrl"
              placeholder="如 https://api.openai.com/v1（OpenAI 兼容 base-url）"
            />
            <u-input label="模型" field="model" placeholder="如 gpt-4o-mini / deepseek-chat" />
            <u-password-input
              label="API Key"
              field="apiKey"
              :placeholder="aiHasKey ? '已配置，留空保持不变' : '供应商 API Key（仅保存在服务端）'"
            />
            <u-select label="推理强度" field="reasoningEffort" :options="reasoningOptions" />
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
  max-width: 420px;
  margin-bottom: 16px;
}

.kernel-card {
  margin-bottom: 16px;
}

.mirror-row {
  display: flex;
  gap: 12px;
  max-width: 560px;
}

.mirror-row .u-input {
  flex: 1;
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
