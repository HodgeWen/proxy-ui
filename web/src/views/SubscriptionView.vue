<!-- 订阅导出：令牌管理（生成/吊销/重置）、订阅链接与二维码（etiket 生成 SVG/PNG）。
     布局：上方节点服务器地址设置（半宽输入），下方令牌管理与链接/二维码双栏铺满 -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import { qrcode } from 'etiket/qr'
import { qrcodePNGDataURI } from 'etiket/png'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UForm,
  UInput,
  message,
  messageConfirm,
} from '@veltra/desktop'
import { QrCode, Token } from '@veltra/icons/normal'
import { computed, onMounted, reactive, shallowRef } from 'vue'

import EmptyState from '../components/EmptyState.vue'
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
const serverForm = reactive({ addr: '' })
const createForm = reactive({ name: '' })
const serverSaving = shallowRef(false)
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

/** 三种客户端格式的订阅链接 */
const linkFormats = computed(() => {
  if (!selected.value) return []
  const token = selected.value.token
  return [
    { label: '自动适配', link: subscriptionLink(token) },
    { label: 'sing-box', link: subscriptionLink(token, 'singbox') },
    { label: 'clash / mihomo', link: subscriptionLink(token, 'clash') },
  ]
})

async function refresh(): Promise<SubscriptionToken[]> {
  const info = await getSubscription()
  tokens.value = info.tokens
  serverForm.addr = info.server
  return info.tokens
}

async function handleSaveServer(): Promise<void> {
  serverSaving.value = true
  try {
    const addr = serverForm.addr.trim()
    await setSubscriptionServer(addr)
    message.success(addr ? '节点服务器地址已保存' : '已恢复按访问 Host 推断节点地址')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    serverSaving.value = false
  }
}

async function handleCreate(): Promise<void> {
  creating.value = true
  try {
    const created = await createSubscriptionToken(createForm.name.trim())
    createForm.name = ''
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
      <u-form :model="serverForm" :cols="2" label-width="88px">
        <u-input
          label="服务器地址"
          field="addr"
          :span="1"
          placeholder="留空按客户端访问面板的 Host 自动推断"
          tips="订阅内节点（入站）的服务器地址取该设置；使用域名或公网 IP，客户端才能跨网络连接"
        />
      </u-form>
      <!-- 页面级表单操作区：卡片底部右侧；辅助设置，主操作档位让位给「生成令牌」 -->
      <div class="card-actions">
        <u-button :loading="serverSaving" @click="handleSaveServer">保存</u-button>
      </div>
    </u-card-content>
  </u-card>

  <div class="sub-grid">
    <u-card>
      <u-card-header>订阅令牌</u-card-header>
      <u-card-content class="tokens-content">
        <u-form :model="createForm" :cols="2" label-width="88px" class="create-form">
          <u-input
            label="备注"
            field="name"
            :span="1"
            placeholder="如：我的手机"
            tips="令牌备注仅用于辨识设备或客户端，不影响订阅内容，可留空"
            @keyup.enter="handleCreate"
          />
        </u-form>
        <EmptyState
          v-if="tokens.length === 0"
          :icon="Token"
          title="尚无订阅令牌"
          description="填写备注后点击「生成令牌」，即可获得供客户端订阅的链接与二维码"
        />
        <template v-else>
          <div v-for="t in tokens" :key="t.id" class="token-item" :class="{ revoked: t.revoked }">
            <div class="token-info">
              <div class="token-name">
                {{ t.name || '未备注' }}
                <span v-if="t.revoked" class="token-badge">已吊销</span>
              </div>
              <div class="token-value">{{ t.token }}</div>
              <div class="token-meta">最近使用：{{ formatTime(t.lastUsedAt) }}</div>
            </div>
            <div class="token-actions">
              <u-button size="small" :disabled="t.revoked" @click="selectedID = t.id">选中</u-button>
              <u-button size="small" :disabled="t.revoked" @click="handleCopy(subscriptionLink(t.token))">复制链接</u-button>
              <u-button size="small" :loading="actingID === t.id" @click="handleRevoke(t)">
                {{ t.revoked ? '恢复' : '吊销' }}
              </u-button>
              <u-button size="small" :loading="actingID === t.id" @click="handleReset(t)">重置</u-button>
            </div>
          </div>
        </template>
        <div class="card-actions">
          <u-button type="primary" :loading="creating" @click="handleCreate">生成令牌</u-button>
        </div>
      </u-card-content>
    </u-card>

    <u-card class="link-card">
      <u-card-header>订阅链接与二维码{{ selected?.name ? ` · ${selected.name}` : '' }}</u-card-header>
      <u-card-content>
        <EmptyState
          v-if="!selected"
          :icon="QrCode"
          title="暂无订阅链接"
          description="在左侧生成或选中令牌后，此处展示各客户端格式的订阅链接与二维码"
        />
        <template v-else>
          <div class="links">
            <div v-for="fmt in linkFormats" :key="fmt.label" class="link-item">
              <div class="link-head">
                <span class="link-label">{{ fmt.label }}</span>
                <u-button size="small" @click="handleCopy(fmt.link)">复制</u-button>
              </div>
              <code class="link-value">{{ fmt.link }}</code>
            </div>
          </div>
          <div class="qr">
            <!-- etiket 生成的订阅二维码 SVG（内容为面板自身链接，可信） -->
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div class="qr-svg" v-html="qrSVG"></div>
            <div class="qr-side">
              <div class="qr-tip">客户端扫描导入订阅，格式按客户端 UA 自动适配</div>
              <u-button size="small" @click="handleDownloadPNG">下载 PNG</u-button>
            </div>
          </div>
        </template>
      </u-card-content>
    </u-card>
  </div>
</template>

<style scoped>
.server-card {
  margin-bottom: 16px;
}

/* 令牌管理与链接/二维码双栏铺满页面宽度，消除下方大面积空白 */
.sub-grid {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
  gap: 16px;
  align-items: stretch;
}

.tokens-content .create-form {
  margin-bottom: 12px;
}

/* 页面级表单操作区：卡片底部右侧 */
.card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.token-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px 16px;
  padding: 12px 16px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-default);
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
  font-size: var(--u-font-size-main-small);
  font-weight: 400;
  color: var(--u-text-color-white);
  background: var(--u-color-danger);
  border-radius: var(--u-radius-small);
  padding: 1px 6px;
}

.token-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
}

.token-meta {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.token-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
  flex-shrink: 0;
}

.link-card {
  min-width: 0;
}

.links {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.link-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.link-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.link-label {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.link-value {
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
  padding: 6px 10px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
}

.qr {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--u-border-color);
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

.qr-side {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.qr-tip {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}
</style>
