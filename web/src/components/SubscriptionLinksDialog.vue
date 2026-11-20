<!-- 订阅链接弹窗：三种客户端格式的链接 + 自动适配链接的二维码 + 节点清单；
     停用/到期/超额时在顶部提示链接当前不可用 -->
<script setup lang="ts">
import { clipboard } from '@cat-kit/fe'
import { qrcode } from 'etiket/qr'
import { qrcodePNGDataURI } from 'etiket/png'
import {
  UAlert,
  UButton,
  UDialog,
  UTable,
  UTag,
  defineTableColumns,
  message,
} from '@veltra/desktop'
import { computed } from 'vue'

import { formatBytes, subscriptionLink } from '../api/subscription'
import type { SubscriptionToken } from '../api/subscription'
import { dialogStyle } from '../dialog-style'
import { protocolDef } from '../inbound-schema'

const visible = defineModel<boolean>('visible', { default: false })
const { token } = defineProps<{ token: SubscriptionToken | null }>()

const nodeColumns = defineTableColumns([
  { key: 'name', name: '用户名', minWidth: 180 },
  { key: 'protocol', name: '协议', width: 150 },
  { key: 'listenPort', name: '端口', width: 90, align: 'center' },
  { key: 'usage', name: '该节点用量', width: 120, align: 'right' },
  { key: 'state', name: '状态', width: 100, align: 'center' },
])

/** 自动适配格式的链接（二维码内容） */
const autoLink = computed(() => (token ? subscriptionLink(token.token) : ''))
const qrSVG = computed(() => (autoLink.value ? qrcode(autoLink.value, { size: 200, margin: 2 }) : ''))

const formats = computed(() => {
  const raw = token?.token
  if (!raw) return []
  return [
    { label: '自动适配', link: subscriptionLink(raw) },
    { label: 'sing-box', link: subscriptionLink(raw, 'singbox') },
    { label: 'clash / mihomo', link: subscriptionLink(raw, 'clash') },
  ]
})

/** 链接当前不可用的原因：停用令牌直接失效，到期/超额返回 403；节点状态由后端暂停 */
const blockedReasons = computed(() => {
  const reasons: string[] = []
  if (!token) return reasons
  if (token.revoked) reasons.push('该订阅用户已停用，链接已失效')
  if (token.expired) reasons.push('订阅已到期，客户端拉取订阅返回 403')
  if (token.overQuota) reasons.push('流量配额已用尽，客户端拉取订阅返回 403')
  return reasons
})

/** 节点所在入站（= 端口）数量：同一入站的节点共用一个端口，这里点明，避免管理员以为端口丢了 */
const inboundCount = computed(() => new Set((token?.nodes ?? []).map((n) => n.inboundId)).size)

function protocolName(protocol: string): string {
  return protocolDef(protocol)?.name ?? protocol
}

async function handleCopy(text: string): Promise<void> {
  try {
    await clipboard.copy(text)
    message.success('已复制')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

/** 下载自动适配链接的二维码 PNG */
function handleDownloadPNG(): void {
  if (!autoLink.value) return
  const a = document.createElement('a')
  a.href = qrcodePNGDataURI(autoLink.value, { moduleSize: 12, margin: 2 })
  a.download = `subscription-${token?.name || token?.id || 'qr'}.png`
  a.click()
}
</script>

<template>
  <u-dialog
    v-model="visible"
    :title="`订阅链接 · ${token?.name || '未命名'}`"
    :style="dialogStyle(780)"
  >
    <template v-if="token">
      <u-alert
        v-if="blockedReasons.length"
        class="blocked"
        type="warning"
        title="该链接当前不可用"
        :description="blockedReasons.join('；')"
        show-icon
      />

      <div class="links">
        <div v-for="fmt in formats" :key="fmt.label" class="link-item">
          <div class="link-head">
            <span class="link-label">{{ fmt.label }}</span>
            <u-button size="small" text type="primary" @click="handleCopy(fmt.link)">复制</u-button>
          </div>
          <code class="link-value">{{ fmt.link }}</code>
        </div>
      </div>

      <div class="qr">
        <!-- etiket 生成的订阅二维码 SVG（内容为面板自身链接，可信） -->
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div class="qr-svg" v-html="qrSVG"></div>
        <div class="qr-side">
          <p class="qr-tip">
            用客户端扫描导入订阅，格式按客户端 UA 自动适配；链接是面板同源子路径，换域名或反代后自动跟随。
          </p>
          <u-button size="small" @click="handleDownloadPNG">下载 PNG</u-button>
        </div>
      </div>

      <p class="section-title">客户端会看到的节点</p>
      <p v-if="token.nodes.length" class="section-tip">
        共 {{ token.nodes.length }} 个节点，来自 {{ inboundCount }} 个入站。
        <b>同一入站的节点共用该入站的端口</b>，客户端靠各自的凭据区分用户，因此端口重复是正常的。
      </p>
      <u-table :columns="nodeColumns" :data="token.nodes" row-key="userId" size="small">
        <template #empty>
          <span class="cell-dim">尚未勾选入站，客户端订阅里没有任何节点</span>
        </template>
        <template #column:protocol="{ rowData }">
          {{ protocolName(rowData.protocol) }}
          <span class="cell-dim">· {{ rowData.inboundName }}</span>
        </template>
        <template #column:usage="{ rowData }">
          {{ formatBytes(rowData.uploadBytes + rowData.downloadBytes) }}
        </template>
        <template #column:state="{ rowData }">
          <u-tag v-if="rowData.suspended" type="warning">已暂停</u-tag>
          <u-tag v-else-if="!rowData.enabled" type="info">已停用</u-tag>
          <u-tag v-else type="success">启用</u-tag>
        </template>
      </u-table>
    </template>

    <template #footer="{ close }">
      <u-button type="primary" @click="close()">关闭</u-button>
    </template>
  </u-dialog>
</template>

<style scoped>
.blocked {
  margin-bottom: 14px;
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
  padding: 6px 10px;
  border: 1px solid var(--u-border-color);
  border-radius: var(--u-radius-small);
  font-size: var(--u-font-size-main-small);
  word-break: break-all;
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
  width: 200px;
  height: 200px;
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
  flex: 1;
}

.qr-tip {
  margin: 0;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.section-title {
  margin: 20px 0 8px;
  font-size: var(--u-font-size-main-small);
  font-weight: 600;
  color: var(--u-text-color-second);
}

.section-tip {
  margin: 0 0 8px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.cell-dim {
  color: var(--u-text-color-assist);
}
</style>
