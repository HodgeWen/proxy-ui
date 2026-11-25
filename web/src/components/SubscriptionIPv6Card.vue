<!-- 订阅 IPv6 节点入口配置卡：填写公网 IPv6 或纯 AAAA 域名后，订阅自动为 :: 双栈入站追加 -ipv6 节点 -->
<script setup lang="ts">
import { UButton, UCard, UCardContent, UCardHeader, UInput, UTag, message } from '@veltra/desktop'
import { onMounted, shallowRef } from 'vue'

import { getSubscriptionSettings, saveSubscriptionSettings } from '../api/subscription'

const loading = shallowRef(true)
const saving = shallowRef(false)
const serverIPv6 = shallowRef('')
const savedIPv6 = shallowRef('')
const detectedIPv6s = shallowRef<string[]>([])

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await getSubscriptionSettings()
    serverIPv6.value = res.serverIpv6
    savedIPv6.value = res.serverIpv6
    detectedIPv6s.value = res.detectedIpv6s ?? []
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

async function handleSave(): Promise<void> {
  saving.value = true
  try {
    const res = await saveSubscriptionSettings(serverIPv6.value)
    serverIPv6.value = res.serverIpv6
    savedIPv6.value = res.serverIpv6
    detectedIPv6s.value = res.detectedIpv6s ?? []
    message.success(res.serverIpv6 ? 'IPv6 节点入口已保存，客户端更新订阅后生效' : '已关闭 IPv6 附加节点')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <u-card size="small" class="intro-card">
    <u-card-header>
      <div class="card-header-row">
        <div class="header-title">
          <span>IPv6 节点入口</span>
          <u-tag size="small" :type="savedIPv6 ? 'success' : 'info'">
            {{ savedIPv6 ? '已启用' : '未启用' }}
          </u-tag>
        </div>
      </div>
    </u-card-header>
    <u-card-content>
      <p class="desc">
        填写服务器公网 IPv6 地址或纯 AAAA 域名后，订阅会自动为监听 <code>::</code> 的双栈入站额外生成
        <code>-ipv6</code> 节点并纳入策略组；留空关闭，保存后客户端更新订阅生效（无需重启内核）。
      </p>
      <div class="input-row">
        <u-input
          v-model="serverIPv6"
          class="ipv6-input"
          clearable
          :disabled="loading"
          placeholder="例如 2001:db8::1 或 v6.example.com（留空关闭）"
        />
        <u-button type="primary" plain :loading="saving" :disabled="loading" @click="handleSave">保存</u-button>
      </div>
      <div v-if="detectedIPv6s.length" class="detected-row">
        <span class="detected-label">检测到本机公网 IPv6：</span>
        <u-button
          v-for="ip in detectedIPv6s"
          :key="ip"
          size="small"
          class="mono"
          @click="serverIPv6 = ip"
        >
          {{ ip }}
        </u-button>
      </div>
    </u-card-content>
  </u-card>
</template>

<style scoped>
/* intro-card / card-header-row / mono 为 App.vue 全局类 */

.header-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.desc {
  margin: 0 0 10px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
  line-height: 1.7;
}

.input-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ipv6-input {
  flex: 1;
  max-width: 460px;
}

.detected-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.detected-label {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}
</style>
