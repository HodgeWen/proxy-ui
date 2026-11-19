<!-- 设置页：账号分区改密；内核分区提供镜像前缀、版本下载安装、启停与日志；面板更新 / AI 供应商分区占位 -->
<script setup lang="ts">
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UEmpty,
  UForm,
  UInput,
  UPasswordInput,
  UTable,
  UTabs,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import type { FormExposed, TabItem } from '@veltra/desktop'
import { nextTick, onBeforeUnmount, reactive, shallowRef, watch } from 'vue'

import {
  fetchKernelLogs,
  fetchKernelStatus,
  fetchKernelVersions,
  fetchMirrorPrefix,
  installKernel,
  kernelAction,
  updateMirrorPrefix,
} from '../api/kernel'
import type { KernelLogLine, KernelStatus, KernelVersion } from '../api/kernel'
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
const versions = shallowRef<KernelVersion[]>([])
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

watch(
  active,
  (tab) => {
    if (tab !== 'kernel') {
      stopPolling()
      return
    }
    void refreshStatus(false)
    void refreshLogs()
    void refreshVersions()
    void loadMirror()
    startPolling()
  },
)

onBeforeUnmount(stopPolling)
</script>

<template>
  <u-tabs v-model="active" :items="tabs">
    <template #account>
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
          <u-button type="primary" :loading="saving" @click="handleSave">保存密码</u-button>
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
            <u-button type="primary" :loading="savingMirror" @click="saveMirror">保存</u-button>
          </div>
          <p class="hint">配置后所有 GitHub 请求（版本查询与内核下载）均经该前缀代理。</p>
        </u-card-content>
      </u-card>

      <u-card class="kernel-card">
        <u-card-header>sing-box 内核</u-card-header>
        <u-card-content>
          <u-empty
            v-if="status && !status.installed"
            text="尚未安装 sing-box 内核，请在下方版本列表选择版本一键安装"
            block
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
              <u-button size="small" :loading="loadingVersions" @click="refreshVersions">刷新版本</u-button>
              <u-button
                size="small"
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
            <u-button size="small" @click="refreshLogs(false)">刷新日志</u-button>
          </div>
        </u-card-header>
        <u-card-content>
          <u-empty v-if="!logs.length" text="暂无内核日志" block />
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
      <u-card>
        <u-card-header>面板更新</u-card-header>
        <u-card-content>
          <u-empty text="面板在线检查更新与一键升级将在后续版本提供" block />
        </u-card-content>
      </u-card>
    </template>

    <template #ai>
      <u-card>
        <u-card-header>AI 供应商</u-card-header>
        <u-card-content>
          <u-empty text="OpenAI 兼容供应商配置将在后续版本提供" block />
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
  font-size: 12px;
  color: var(--u-text-color-second, #71717a);
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
  border-radius: 6px;
  background: var(--u-bg-color-middle, #fafafa);
  border: 1px solid var(--u-border-color, #e4e4e7);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
}

.kernel-log-line {
  display: flex;
  gap: 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

.kernel-log-time {
  color: var(--u-text-color-second, #71717a);
  flex-shrink: 0;
}

.kernel-log-stream {
  flex-shrink: 0;
  color: var(--u-color-primary, #2563eb);
}

.kernel-log-stream.is-stderr {
  color: var(--u-color-error, #dc2626);
}

.kernel-log-stream.is-panel {
  color: var(--u-color-warning, #d97706);
}
</style>
