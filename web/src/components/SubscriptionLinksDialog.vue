<!-- 订阅链接弹窗：按格式分 tab（自动适配 / sing-box / clash·mihomo），每个 tab 展示该格式的
     链接、复制按钮与对应二维码；停用/到期/超额时在顶部提示链接当前不可用 -->
<script setup lang="ts">
import { qrcode } from 'etiket/qr'
import { UAlert, UButton, UDialog, UTabs } from '@veltra/desktop'
import type { TabItem } from '@veltra/desktop'
import { computed, shallowRef, watch } from 'vue'

import { subscriptionLink } from '../api/subscription'
import type { SubscriptionToken } from '../api/subscription'
import { dialogStyle } from '../dialog-style'
import { copyText } from '../utils/clipboard'

const visible = defineModel<boolean>('visible', { default: false })
const { token } = defineProps<{ token: SubscriptionToken | null }>()

/** 三种客户端格式的链接与说明 */
const formats = computed(() => {
  const raw = token?.token
  if (!raw) return []
  return [
    {
      key: 'auto',
      label: '自动适配',
      link: subscriptionLink(raw),
      tip: '用客户端扫描导入，格式按客户端 UA 自动适配；链接是面板同源子路径，换域名或反代后自动跟随。',
    },
    {
      key: 'singbox',
      label: 'sing-box',
      link: subscriptionLink(raw, 'singbox'),
      tip: '适合 sing-box 客户端（含基于 sing-box 内核的三方客户端）。',
    },
    {
      key: 'clash',
      label: 'clash / mihomo',
      link: subscriptionLink(raw, 'clash'),
      tip: '适合 Clash / mihomo（Clash.Meta）系客户端。',
    },
  ]
})

const activeFormat = shallowRef('auto')
const tabs = computed<TabItem[]>(() => formats.value.map(({ key, label }) => ({ key, name: label })))

// 每次打开回到第一个 tab
watch(visible, (open) => {
  if (open) activeFormat.value = 'auto'
})

/** 当前 tab 的二维码 SVG（etiket 生成，内容为面板自身链接，可信） */
const qrSVG = computed(() => {
  const link = formats.value.find((f) => f.key === activeFormat.value)?.link
  return link ? qrcode(link, { size: 200, margin: 2 }) : ''
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
</script>

<template>
  <u-dialog
    v-model="visible"
    :title="`订阅链接 · ${token?.name || '未命名'}`"
    :style="dialogStyle(560)"
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

      <u-tabs v-model="activeFormat" :items="tabs">
        <template v-for="fmt in formats" :key="fmt.key" #[fmt.key]>
          <div class="link-head">
            <code class="link-value">{{ fmt.link }}</code>
            <u-button size="small" text type="primary" @click="copyText(fmt.link)">复制</u-button>
          </div>
          <div class="qr">
            <!-- etiket 生成的订阅二维码 SVG（内容为面板自身链接，可信），当前 tab 才渲染 -->
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div class="qr-svg" v-html="fmt.key === activeFormat ? qrSVG : ''"></div>
            <p class="qr-tip">{{ fmt.tip }}</p>
          </div>
        </template>
      </u-tabs>
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

.link-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.link-value {
  flex: 1;
  min-width: 0;
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
  margin-top: 14px;
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

.qr-tip {
  margin: 0;
  min-width: 0;
  flex: 1;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}
</style>
