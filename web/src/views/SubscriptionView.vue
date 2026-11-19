<!-- 订阅导出：令牌管理（生成/吊销/重置）、订阅链接与二维码（etiket 生成 SVG/PNG） -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import { qrcode } from 'etiket/qr'
import { qrcodePNGDataURI } from 'etiket/png'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UEmpty,
  UInput,
  message,
  messageConfirm,
} from '@veltra/desktop'
import { computed, onMounted, shallowRef } from 'vue'
import {
  createSubscriptionToken,
  getSubscription,
  resetSubscriptionToken,
  revokeSubscriptionToken,
  setSubscriptionServer,
  subscriptionLink,
} from '../api/subscription'
import type { SubscriptionToken } from '../api/subscription'

const tokens = shallowRef<SubscriptionToken[]>([])
const serverAddr = shallowRef('')
const serverSaving = shallowRef(false)
const newName = shallowRef('')
const creating = shallowRef(false)
const selectedID = shallowRef(0)
const actingID = shallowRef(0)

/** 当前选中的令牌（默认选中第一个可用令牌） */
const selected = computed(
  () => tokens.value.find((t) => t.id === selectedID.value) ?? tokens.value.find((t) => !t.revoked) ?? null,
)

/** 当前展示二维码的订阅链接（自动适配格式） */
const link = computed(() => (selected.value ? subscriptionLink(selected.value.token) : ''))
const qrSVG = computed(() => (link.value ? qrcode(link.value, { size: 220, margin: 2 }) : ''))

async function refresh(): Promise<SubscriptionToken[]> {
  const info = await getSubscription()
  tokens.value = info.tokens
  serverAddr.value = info.server
  return info.tokens
}

async function handleSaveServer(): Promise<void> {
  serverSaving.value = true
  try {
    await setSubscriptionServer(serverAddr.value.trim())
    message.success(serverAddr.value.trim() ? '节点服务器地址已保存' : '已恢复按访问 Host 推断节点地址')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    serverSaving.value = false
  }
}

async function handleCreate(): Promise<void> {
  creating.value = true
  try {
    const created = await createSubscriptionToken(newName.value.trim())
    newName.value = ''
    await refresh()
    selectedID.value = created.id
    message.success('订阅令牌已生成')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    creating.value = false
  }
}

/** 危险操作（吊销/重置）二次确认 */
async function confirmAction(title: string, tip: string): Promise<boolean> {
  const action = await messageConfirm({
    title,
    message: tip,
    confirmButtonText: '确认',
    cancelButtonText: '取消',
  }).onClosed
  return action === 'confirm'
}

async function handleRevoke(t: SubscriptionToken): Promise<void> {
  if (!t.revoked) {
    const ok = await confirmAction('吊销订阅令牌', `吊销后携带该令牌的订阅链接立即失效，确认吊销「${t.name || t.token}」吗？`)
    if (!ok) return
  }
  actingID.value = t.id
  try {
    await revokeSubscriptionToken(t.id, !t.revoked)
    await refresh()
    message.success(t.revoked ? '订阅令牌已恢复' : '订阅令牌已吊销')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleReset(t: SubscriptionToken): Promise<void> {
  const ok = await confirmAction(
    '重置订阅令牌',
    `重置将生成新令牌，「${t.name || t.token}」的旧订阅链接立即失效，需在客户端重新订阅，确认继续吗？`,
  )
  if (!ok) return
  actingID.value = t.id
  try {
    await resetSubscriptionToken(t.id)
    await refresh()
    selectedID.value = t.id
    message.success('订阅令牌已重置，请在客户端更新订阅链接')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    actingID.value = 0
  }
}

async function handleCopy(text: string): Promise<void> {
  try {
    await clipboard.copy(text)
    message.success('已复制')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

/** 下载当前订阅链接的 PNG 二维码 */
function handleDownloadPNG(): void {
  if (!link.value) return
  const a = document.createElement('a')
  a.href = qrcodePNGDataURI(link.value, { moduleSize: 12, margin: 2 })
  a.download = 'subscription-qr.png'
  a.click()
}

function formatTime(value: string | null): string {
  return value ? new Date(value).toLocaleString() : '从未使用'
}

onMounted(async () => {
  try {
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  }
})
</script>

<template>
  <u-card class="server-card">
    <u-card-header>节点服务器地址</u-card-header>
    <u-card-content>
      <div class="server-row">
        <u-input
          v-model="serverAddr"
          placeholder="留空按客户端访问面板的 Host 自动推断"
          class="server-input"
        />
        <u-button size="small" :loading="serverSaving" @click="handleSaveServer">保存</u-button>
      </div>
      <div class="server-tip">
        订阅内节点（入站）的服务器地址取该设置；使用域名或公网 IP，客户端才能跨网络连接。
      </div>
    </u-card-content>
  </u-card>

  <u-card class="tokens-card">
    <u-card-header>订阅令牌</u-card-header>
    <u-card-content>
      <div class="create-row">
        <u-input v-model="newName" placeholder="备注（如：我的手机）" class="create-input" @keyup.enter="handleCreate" />
        <u-button type="primary" size="small" :loading="creating" @click="handleCreate">生成令牌</u-button>
      </div>
      <u-empty v-if="tokens.length === 0" text="尚无订阅令牌，先生成一个" block />
      <div v-for="t in tokens" v-else :key="t.id" class="token-item" :class="{ revoked: t.revoked }">
        <div class="token-info">
          <div class="token-name">
            {{ t.name || '未备注' }}
            <span v-if="t.revoked" class="token-badge">已吊销</span>
          </div>
          <div class="token-value">{{ t.token }}</div>
          <div class="token-meta">最近使用：{{ formatTime(t.lastUsedAt) }}</div>
        </div>
        <div class="token-actions">
          <u-button size="small" :disabled="t.revoked" @click="selectedID = t.id">二维码</u-button>
          <u-button size="small" :disabled="t.revoked" @click="handleCopy(subscriptionLink(t.token))">复制链接</u-button>
          <u-button size="small" :loading="actingID === t.id" @click="handleRevoke(t)">
            {{ t.revoked ? '恢复' : '吊销' }}
          </u-button>
          <u-button size="small" :loading="actingID === t.id" @click="handleReset(t)">重置</u-button>
        </div>
      </div>
    </u-card-content>
  </u-card>

  <u-card v-if="selected">
    <u-card-header>订阅链接与二维码{{ selected.name ? ` · ${selected.name}` : '' }}</u-card-header>
    <u-card-content>
      <div class="links">
        <div class="link-row">
          <span class="link-label">自动适配</span>
          <code class="link-value">{{ subscriptionLink(selected.token) }}</code>
          <u-button size="small" @click="handleCopy(subscriptionLink(selected.token))">复制</u-button>
        </div>
        <div class="link-row">
          <span class="link-label">sing-box</span>
          <code class="link-value">{{ subscriptionLink(selected.token, 'singbox') }}</code>
          <u-button size="small" @click="handleCopy(subscriptionLink(selected.token, 'singbox'))">复制</u-button>
        </div>
        <div class="link-row">
          <span class="link-label">clash / mihomo</span>
          <code class="link-value">{{ subscriptionLink(selected.token, 'clash') }}</code>
          <u-button size="small" @click="handleCopy(subscriptionLink(selected.token, 'clash'))">复制</u-button>
        </div>
      </div>
      <div class="qr">
        <!-- etiket 生成的订阅二维码 SVG（内容为面板自身链接，可信） -->
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div class="qr-svg" v-html="qrSVG"></div>
        <div class="qr-actions">
          <div class="qr-tip">客户端扫描导入订阅，格式按客户端 UA 自动适配</div>
          <u-button size="small" @click="handleDownloadPNG">下载 PNG</u-button>
        </div>
      </div>
    </u-card-content>
  </u-card>
</template>

<style scoped>
.server-card,
.tokens-card {
  margin-bottom: 16px;
}

.server-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.server-input {
  flex: 1;
}

.server-tip {
  margin-top: 8px;
  color: var(--u-text-color-second, #71717a);
  font-size: 13px;
}

.create-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.create-input {
  flex: 1;
}

.token-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid var(--u-border-color, #e4e4e7);
  border-radius: 8px;
  margin-bottom: 8px;
}

.token-item.revoked {
  opacity: 0.6;
}

.token-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.token-name {
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.token-badge {
  font-size: 12px;
  font-weight: 400;
  color: #fff;
  background: var(--u-color-danger, #ef4444);
  border-radius: 4px;
  padding: 1px 6px;
}

.token-value {
  font-family: var(--u-font-family-mono, monospace);
  font-size: 13px;
  word-break: break-all;
}

.token-meta {
  color: var(--u-text-color-second, #71717a);
  font-size: 12px;
}

.token-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.links {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.link-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.link-label {
  width: 110px;
  flex-shrink: 0;
  color: var(--u-text-color-second, #71717a);
}

.link-value {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  word-break: break-all;
  padding: 6px 10px;
  border: 1px solid var(--u-border-color, #e4e4e7);
  border-radius: 6px;
}

.qr {
  display: flex;
  align-items: center;
  gap: 20px;
}

.qr-svg {
  width: 220px;
  height: 220px;
  flex-shrink: 0;
}

.qr-svg :deep(svg) {
  width: 100%;
  height: 100%;
}

.qr-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.qr-tip {
  color: var(--u-text-color-second, #71717a);
  font-size: 13px;
}
</style>
