<!-- 设置页「DNS」页签：预设下拉 + UTableEditor 服务器 / 规则双表格（进入页签加载一次，切走保留未保存编辑） -->
<script setup lang="ts">
import {
  UAlert,
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UInput,
  UNumberInput,
  USelect,
  USwitch,
  UTableEditor,
  UIcon,
  UTip,
  message,
  vLoading,
} from '@veltra/desktop'
import { copy } from '@cat-kit/core'
import type { TableEditorColumn } from '@veltra/desktop'
import { QuestionFilled } from '@veltra/icons/normal'
import { computed, onMounted, reactive, shallowRef, watch } from 'vue'

import { getDNS, saveDNS } from '../../api/dns'
import type { DNSPreset, DNSRule, DNSServer, DNSSettings } from '../../api/dns'
import { confirmAction } from '../../confirm-action'

const dnsLoading = shallowRef(true)
const dnsSaving = shallowRef(false)
const dnsTypes = shallowRef<string[]>([])
const dnsStrategies = shallowRef<string[]>([])
const dnsPresets = shallowRef<DNSPreset[]>([])

/** 本地编辑副本：进入页签时填充（组件保活，切走再回来不重新拉取），保存成功用后端返回回填 */
const dnsForm = reactive({
  enabled: true,
  strategy: '',
  servers: [] as DNSServer[],
  rules: [] as DNSRule[],
  final: '',
})

/** 预设下拉的选中态：只代表选择器，点「应用」才填入编辑器 */
const selectedPresetId = shallowRef('')

const typeOptions = computed(() => dnsTypes.value.map((type) => ({ label: type, value: type })))

const tipStyle = { maxWidth: '360px', lineHeight: '1.6' }

const STRATEGY_META: Record<string, { label: string; desc: string }> = {
  '': {
    label: '不指定（内核默认）',
    desc: '不显式写入 strategy，由 sing-box 按系统网络偏好决定；Clash 订阅保持双栈开启。',
  },
  prefer_ipv4: {
    label: 'prefer_ipv4（优先 IPv4）',
    desc: '同时查询 A 与 AAAA，优先返回 IPv4，无 IPv4 时回落 IPv6；兼顾兼容与双栈，日常最稳妥。',
  },
  prefer_ipv6: {
    label: 'prefer_ipv6（优先 IPv6）',
    desc: '同时查询 A 与 AAAA，优先返回 IPv6，无 IPv6 时回落 IPv4；适合双栈网络、缓解 IPv4 晚高峰拥堵。',
  },
  ipv4_only: {
    label: 'ipv4_only（仅 IPv4）',
    desc: '只查 A 记录、禁用 IPv6 解析。若想客户端经裸 IPv6 入口连服务器、同时服务端出站严格走 IPv4 防风控，选此项即可（仅填了域名型 IPv6 入口时才会放开 AAAA 解析）。',
  },
  ipv6_only: {
    label: 'ipv6_only（仅 IPv6）',
    desc: '只查 AAAA 记录；仅适用于纯 IPv6 环境，访问纯 IPv4 站点会解析失败。',
  },
}

const strategyOptions = computed(() => [
  { label: STRATEGY_META[''].label, value: '' },
  ...dnsStrategies.value.map((strategy) => ({
    label: STRATEGY_META[strategy]?.label ?? strategy,
    value: strategy,
  })),
])

const presetOptions = computed(() => dnsPresets.value.map((preset) => ({ label: preset.name, value: preset.id })))

const selectedPreset = computed(() => dnsPresets.value.find((preset) => preset.id === selectedPresetId.value))

/** 当前非空 tag，供默认服务器与规则引用 */
const dnsTags = computed(() => dnsForm.servers.map((item) => String(item.tag ?? '').trim()).filter(Boolean))

const tagOptions = computed(() => dnsTags.value.map((tag) => ({ label: tag, value: tag })))

/** 重复的非空 tag（tag 列校验引用） */
const duplicateTags = computed(() => {
  const seen = new Set<string>()
  const duplicated = new Set<string>()
  for (const tag of dnsTags.value) {
    if (seen.has(tag)) duplicated.add(tag)
    seen.add(tag)
  }
  return duplicated
})

/** 长说明收敛进 tooltip 的文案（原页面分节大段文字） */
const dnsScopeTip =
  'DNS 决定「域名解析成哪个 IP、由哪台服务器来解析」。这份配置会同时写进服务端 sing-box 自身（面板所在服务器出网解析）与下发给客户端的订阅，改一次两端行为一致。国内域名走国内 DNS、其余走 DoH 之类分流可兼顾速度与防污染；把查询 detour 到代理出站还能避开本地运营商的 DNS 劫持。'
const strategyTip =
  '控制服务端 sing-box 与下发订阅的默认域名解析偏好（影响访问目标域名，以及 sing-box 解析域名型节点地址）。prefer_ipv4 / prefer_ipv6 会同时查 A 与 AAAA 并按偏好排序、失败自动回落；ipv4_only / ipv6_only 则只查单栈记录。注意：sing-box 直接按该策略执行；Clash（mihomo）仅有 ipv6 开关，除 ipv4_only（且无 IPv6 节点入口）外均开启双栈。'
const serverTip =
  'tag 是规则与默认服务器引用它的名字，必须非空且唯一；类型 udp / tcp 为明文查询（可能被劫持污染），tls / https / h3 为加密查询（DoT / DoH / DoH3，防污染推荐），local 使用系统本地解析、不需要地址与端口；地址国内推荐 223.5.5.5、加密推荐 1.1.1.1；端口留空按类型取默认（53 / 853 / 443）；「经由出站」留空直连查询，填出站 tag 则由该出站代发（走代理防污染时用）。'
const ruleTip =
  '规则自上而下匹配：同一行填多个条件时任一命中即生效，多个值用英文逗号分隔（如 qq.com,baidu.com）；未命中任何规则的查询走下方「默认服务器」。'

/** 表格覆盖不到的跨字段问题提示（首屏加载完成前不报空表误警；单元格级问题由 UTableEditor 列校验标出） */
const dnsProblems = computed<string[]>(() => {
  if (dnsLoading.value || !dnsForm.enabled) return []
  const problems: string[] = []
  if (!dnsForm.servers.length) problems.push('启用 DNS 时至少需要一个 DNS 服务器。')
  if (dnsForm.final && !dnsTags.value.includes(dnsForm.final)) problems.push('默认服务器不在现有 tag 中。')
  return problems
})

/** http3 / h3 需要 DoH 路径；local 不需要地址与端口 */
function showPath(type: string): boolean {
  return type === 'https' || type === 'h3'
}

/** 服务器表格列：tag / 类型 / 地址 / 端口 / DoH 路径 / 经由出站 */
const serverColumns: TableEditorColumn[] = [
  {
    key: 'tag',
    name: 'tag',
    minWidth: 110,
    rules: {
      required: 'tag 必填，规则与默认服务器靠它引用',
      validator: (value: string) => {
        const tag = String(value ?? '').trim()
        return tag && duplicateTags.value.has(tag) ? `tag「${tag}」与其它服务器重复` : ''
      },
    },
  },
  { key: 'type', name: '类型', width: 106, rules: { required: '请选择查询类型' } },
  {
    key: 'server',
    name: '地址',
    minWidth: 150,
    rules: {
      validator: (value: string, data: Record<string, unknown>) =>
        data.type && data.type !== 'local' && !String(value ?? '').trim() ? '除 local 类型外地址必填' : '',
    },
  },
  { key: 'serverPort', name: '端口', width: 96 },
  { key: 'path', name: 'DoH 路径', width: 110 },
  { key: 'detour', name: '经由出站', minWidth: 110 },
]

/** 分流规则表格列：三类域名条件 + 目标服务器 */
const ruleColumns: TableEditorColumn[] = [
  { key: 'domain', name: '精确域名', minWidth: 130 },
  { key: 'domainSuffix', name: '域名后缀', minWidth: 130 },
  { key: 'domainKeyword', name: '域名关键字', minWidth: 110 },
  {
    key: 'server',
    name: '目标服务器',
    width: 140,
    rules: {
      required: '请选择目标服务器',
      validator: (value: string) => {
        const tag = String(value ?? '').trim()
        return tag && !dnsTags.value.includes(tag) ? '目标服务器不在上方服务器列表中' : ''
      },
    },
  },
]

// UTableEditor 的「新增 / 复制 / 空态添加」插入空对象：补默认类型，对齐原「新增服务器」的起点
watch(
  () => dnsForm.servers,
  (servers) => {
    for (const item of servers) {
      if (!item.type) item.type = dnsTypes.value[0] ?? 'udp'
    }
  },
)

/** 深拷贝一份配置回填本地表单，避免预设对象被表单改动污染 */
function fillDNS(config: DNSSettings): void {
  const next = copy(config)
  Object.assign(dnsForm, {
    enabled: next.enabled,
    strategy: next.strategy ?? '',
    servers: next.servers ?? [],
    rules: next.rules ?? [],
    final: next.final ?? '',
  })
}

async function loadDNS(): Promise<void> {
  dnsLoading.value = true
  try {
    const data = await getDNS()
    dnsTypes.value = data.types
    dnsStrategies.value = data.strategies
    dnsPresets.value = data.presets
    fillDNS(data.config)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    dnsLoading.value = false
  }
}

/** 应用预设只覆盖本地表单（会丢弃未保存改动，先确认），需再点「保存 DNS」才落库 */
async function handleApplyPreset(): Promise<void> {
  const preset = selectedPreset.value
  if (!preset) return
  const ok = await confirmAction(
    '应用 DNS 预设',
    `将用「${preset.name}」覆盖当前编辑中的服务器与规则，未保存的改动会丢失。确认继续吗？`,
    false,
    '应用',
  )
  if (!ok) return
  // 预设本身不带启用状态，保留用户当前的开关选择
  fillDNS({ ...preset.config, enabled: dnsForm.enabled })
  message.success(
    dnsForm.enabled ? '预设已填入，确认无误后点击「保存 DNS」' : '预设已填入，但 DNS 当前处于关闭状态，需打开开关并保存',
  )
}

/** 提交前清洗：去掉空白可选字段与空端口，避免被后端当作有效值 */
function buildDNSPayload(): DNSSettings {
  return {
    enabled: dnsForm.enabled,
    strategy: dnsForm.strategy || '',
    servers: dnsForm.servers.map((item) => {
      const type = item.type || dnsTypes.value[0] || 'udp'
      const server: DNSServer = { tag: String(item.tag ?? '').trim(), type }
      const address = String(item.server ?? '').trim()
      if (type !== 'local' && address) server.server = address
      if (type !== 'local' && item.serverPort) server.serverPort = item.serverPort
      if (showPath(type) && item.path?.trim()) server.path = item.path.trim()
      if (item.detour?.trim()) server.detour = item.detour.trim()
      return server
    }),
    rules: dnsForm.rules.map((rule) => {
      const next: DNSRule = { server: rule.server || '' }
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

// 首次进入页签才拉取；页签组件被 UTabs 保活，切走再回来不覆盖未保存的编辑，需要刷新请点「重新加载」
onMounted(loadDNS)
</script>

<template>
  <u-card class="settings-card">
    <u-card-header>
      <div class="card-header-row">
        <span class="card-title">
          DNS
          <u-tip :content="dnsScopeTip" direction="bottom" alignment="start" :style="tipStyle">
            <u-icon :size="14" class="tip-icon"><QuestionFilled /></u-icon>
          </u-tip>
        </span>
        <u-button size="small" :loading="dnsLoading" @click="loadDNS">重新加载</u-button>
      </div>
    </u-card-header>
    <u-card-content>
      <div v-loading="dnsLoading && !dnsTypes.length">
      <div class="settings-row">
        <div class="settings-field">
          <div class="settings-label">启用自定义 DNS</div>
          <div class="settings-body">
            <u-switch v-model="dnsForm.enabled" />
            <div class="settings-desc">关闭后服务端配置与订阅都不注入 dns 段，解析交还内核 / 客户端默认行为。</div>
          </div>
        </div>
        <div v-if="dnsForm.enabled" class="settings-field">
          <div class="settings-label">
            <span>解析策略</span>
            <u-tip :content="strategyTip" direction="bottom" alignment="start" :style="tipStyle">
              <u-icon :size="14" class="tip-icon"><QuestionFilled /></u-icon>
            </u-tip>
          </div>
          <div class="settings-body">
            <u-select
              v-model="dnsForm.strategy"
              :options="strategyOptions"
              :clearable="false"
              class="settings-input"
            />
            <div class="settings-desc">
              {{
                STRATEGY_META[dnsForm.strategy]?.desc ??
                '优先返回 IPv4 还是 IPv6；拿不准选「不指定」，由内核按系统偏好决定。'
              }}
            </div>
          </div>
        </div>
        <div v-if="dnsForm.enabled" class="settings-field">
          <div class="settings-label">预设方案</div>
          <div class="settings-body">
            <div class="preset-line">
              <u-select
                v-model="selectedPresetId"
                :options="presetOptions"
                placeholder="选择预设"
                class="preset-select"
              />
              <u-button size="small" :disabled="!selectedPreset" @click="handleApplyPreset">应用</u-button>
            </div>
            <div class="settings-desc">
              {{ selectedPreset ? selectedPreset.description : '应用只把模板填入下方编辑器，需再点「保存 DNS」落库；从未保存过时面板默认按「国内外分流（推荐）」运行。' }}
            </div>
          </div>
        </div>
      </div>

      <u-alert
        v-if="!dnsForm.enabled"
        type="info"
        class="block-alert"
        title="DNS 已关闭"
        description="服务端配置与订阅都不会注入 DNS 段，需要分流或防污染时再打开。"
      />

      <template v-else>
        <!-- 服务器表格：tag / 类型 / 地址 / 端口 / DoH 路径 / 经由出站，操作列由 UTableEditor 内置 -->
        <section class="editor-block">
          <div class="editor-head">
            <div class="editor-title">
              DNS 服务器
              <u-tip :content="serverTip" direction="bottom" alignment="start" :style="tipStyle">
                <u-icon :size="14" class="tip-icon"><QuestionFilled /></u-icon>
              </u-tip>
            </div>
          </div>
          <u-table-editor v-model="dnsForm.servers" :columns="serverColumns" size="small" border>
            <template #column:tag="{ model }">
              <u-input
                v-bind="model"
                placeholder="如 ali"
                title="服务器名字：规则与默认服务器靠它引用，必填且不可重复"
              />
            </template>
            <template #column:type="{ model }">
              <u-select
                v-bind="model"
                :options="typeOptions"
                :clearable="false"
                title="查询方式：udp / tcp 明文（可能被劫持污染），tls=DoT、https=DoH、h3=DoH3（加密防污染），local=系统本地解析"
              />
            </template>
            <template #column:server="{ model, rowData }">
              <u-input
                v-if="rowData.type !== 'local'"
                v-bind="model"
                placeholder="如 223.5.5.5"
                title="DNS 服务器地址（域名或 IP）；国内推荐 223.5.5.5，加密 DNS 推荐 1.1.1.1 / 8.8.8.8"
              />
              <span v-else class="cell-dim">—</span>
            </template>
            <template #column:serverPort="{ model, rowData }">
              <u-number-input
                v-if="rowData.type !== 'local'"
                v-bind="model"
                :min="1"
                :max="65535"
                placeholder="默认"
                title="留空按类型取默认端口：udp/tcp 53、tls 853、https/h3 443"
              />
              <span v-else class="cell-dim">—</span>
            </template>
            <template #column:path="{ model, rowData }">
              <u-input
                v-if="showPath(rowData.type)"
                v-bind="model"
                placeholder="/dns-query"
                title="DoH / DoH3 的请求路径，绝大多数服务商用默认值即可，无需修改"
              />
              <span v-else class="cell-dim">—</span>
            </template>
            <template #column:detour="{ model }">
              <u-input
                v-bind="model"
                placeholder="可空，如 proxy"
                title="留空直连查询；填出站 tag 则由该出站代发，可避开本地运营商的 DNS 劫持"
              />
            </template>
          </u-table-editor>
        </section>

        <!-- 分流规则表格：域名条件 → 指定服务器 -->
        <section class="editor-block">
          <div class="editor-head">
            <div class="editor-title">
              分流规则
              <u-tip :content="ruleTip" direction="bottom" alignment="start" :style="tipStyle">
                <u-icon :size="14" class="tip-icon"><QuestionFilled /></u-icon>
              </u-tip>
            </div>
          </div>
          <u-table-editor v-model="dnsForm.rules" :columns="ruleColumns" size="small" border>
            <template #column:domain="{ model }">
              <u-input v-bind="model" placeholder="如 www.qq.com" title="只匹配填写的域名本身，不含子域；多个值英文逗号分隔" />
            </template>
            <template #column:domainSuffix="{ model }">
              <u-input v-bind="model" placeholder="如 cn、baidu.com" title="匹配该后缀及其所有子域；多个值英文逗号分隔" />
            </template>
            <template #column:domainKeyword="{ model }">
              <u-input v-bind="model" placeholder="如 taobao" title="域名中只要包含该词就命中；多个值英文逗号分隔" />
            </template>
            <template #column:server="{ model }">
              <u-select
                v-bind="model"
                :options="tagOptions"
                :clearable="false"
                title="命中条件的查询交给哪个 DNS 服务器（上方服务器列表里的 tag）"
              />
            </template>
          </u-table-editor>
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
      <p class="hint">保存后立即按新配置重建内核；起不来时自动回到上一份可用配置并提示失败原因。</p>
      </div>
    </u-card-content>
  </u-card>
</template>

<style scoped>
/* settings-card / hint / card-header-row / card-actions 为 App.vue 全局类 */

.block-alert {
  margin: 10px 0;
}

.settings-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.settings-field {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.settings-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  width: 118px;
  flex-shrink: 0;
  line-height: 28px;
}

.settings-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.settings-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.settings-input {
  width: 100%;
}

.card-title {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.tip-icon {
  color: var(--u-text-color-second);
  cursor: help;
  flex-shrink: 0;
}

/* 预设行：下拉选预设 + 应用按钮 */
.preset-line {
  display: flex;
  gap: 8px;
}

.preset-select {
  width: 260px;
  max-width: 100%;
}

/* 表格区块 */
.editor-block {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--u-border-color);
}

.editor-head {
  margin-bottom: 6px;
}

.editor-title {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-weight: 600;
}

/* 条件列不适用时的占位（如 local 类型无需地址）：与全局 cell-dim 同名但色值刻意更深（second 而非 assist），scoped 覆盖生效 */
.cell-dim {
  color: var(--u-text-color-second);
}
</style>
