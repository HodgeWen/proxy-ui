<!-- 全局规则：按对象分四个 tab——路由规则 / 规则集 / 策略组 / 兜底与模板；数据一次性并发拉取，切 tab 不重复请求 -->
<script setup lang="ts">
import PolicyGroupCard from '../components/PolicyGroupCard.vue'
import RuleListCard from '../components/RuleListCard.vue'
import RuleSetCard from '../components/RuleSetCard.vue'
import RuleSetDialog from '../components/RuleSetDialog.vue'
import { listOutbounds } from '../api/outbound'
import {
  applyRuleTemplate,
  getRouteFinal,
  listPolicyGroups,
  listRuleSets,
  listRuleTemplates,
  listRules,
  setRouteFinal,
} from '../api/rule'
import type { PolicyGroup, RuleRecord, RuleSet, RuleTemplate } from '../api/rule'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  USelect,
  UTabs,
  UTag,
  message,
  messageConfirm,
} from '@veltra/desktop'
import type { TabItem } from '@veltra/desktop'
import { computed, onMounted, shallowRef } from 'vue'

const activeTab = shallowRef('routes')
const tabs: TabItem[] = [
  { key: 'routes', name: '路由规则' },
  { key: 'sets', name: '规则集' },
  { key: 'groups', name: '策略组' },
  { key: 'final', name: '兜底与模板' },
]

const rules = shallowRef<RuleRecord[]>([])
const ruleSets = shallowRef<RuleSet[]>([])
const groups = shallowRef<PolicyGroup[]>([])
const templates = shallowRef<RuleTemplate[]>([])
const outboundTags = shallowRef<string[]>([])
/** 首屏未加载完时各卡片不渲染空状态，避免闪一下「暂无数据」 */
const loading = shallowRef(true)
/** 规则模型详细说明：默认收起，首屏只留一行说明 */
const guideExpanded = shallowRef(false)
const applyingTemplate = shallowRef('')
/** 兜底出站：模板应用与「当前已应用」标记都要读它，故先于模板区块声明 */
const finalOutbound = shallowRef<string | undefined>('')
const finalSaving = shallowRef(false)

/** 出站 tag 选项：direct/拦截 + 启用中的自定义出站 + 启用中的策略组 */
const outboundOptions = computed(() => [
  { label: 'direct（直连）', value: 'direct' },
  { label: 'block（拦截）', value: 'block' },
  // direct 出站由固定选项承担，避免下拉里出现两个同名项
  ...outboundTags.value.filter((t) => t !== 'direct').map((t) => ({ label: t, value: t })),
  // 停用的策略组不进配置，不能作为出站供选择
  ...groups.value
    .filter((g) => g.enabled)
    .map((g) => ({ label: `${g.name}（策略组）`, value: g.name })),
])

/** 拉取页面全部数据；供挂载与各子卡片的 changed 事件复用 */
async function refresh(): Promise<void> {
  try {
    const [ruleList, setList, groupList, templateList, outboundList] = await Promise.all([
      listRules(),
      listRuleSets(),
      listPolicyGroups(),
      listRuleTemplates(),
      listOutbounds(),
    ])
    rules.value = ruleList
    ruleSets.value = setList
    groups.value = groupList
    templates.value = templateList
    outboundTags.value = outboundList.filter((o) => o.enabled).map((o) => o.name)
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    loading.value = false
  }
}

// ---------- 规则集维护弹窗 ----------

const setDialogVisible = shallowRef(false)
/** 当前维护的规则集 ID；刷新后按 ID 重新取，保证弹窗内容跟着更新 */
const maintainingId = shallowRef(0)
const maintaining = computed(() => ruleSets.value.find((s) => s.id === maintainingId.value) ?? null)

function openRuleSet(ruleSet: RuleSet): void {
  maintainingId.value = ruleSet.id
  setDialogVisible.value = true
}

// ---------- 预设模板 ----------

/** 模板固定引用的内置规则集 tag（服务启动时自动补齐） */
const ANTI_AD = 'anti-ad'
const GEO_CN = 'geosite-cn'
const GEO_IP_CN = 'geoip-cn'

/** 规则签名：匹配对象|匹配方式|规则集 ID|匹配值|出站；任一字段变化即视为偏离模板 */
function ruleSignature(rule: RuleRecord): string {
  return `${rule.matchType}|${rule.matchMethod}|${rule.ruleSetId}|${rule.value}|${rule.outbound}`
}

/** 按「规则集 tag + 出站」拼出模板规则的签名；规则集未加载时返回空串让标记自然失效 */
function ruleSetSignature(entries: [tag: string, outbound: string][]): string {
  return entries
    .map(([tag, outbound]) => {
      const id = ruleSets.value.find((s) => s.name === tag)?.id
      // 规则集规则的 matchMethod / value 在后端恒为空串，与 ruleSignature 保持同一口径
      return id === undefined ? '' : `rule_set||${id}||${outbound}`
    })
    .join(';')
}

/** 当前规则/策略组/兜底出站与某模板生成结果完全一致时视为「当前已应用」；任一改动即失效 */
const appliedTemplate = computed<string | null>(() => {
  const adblockOnly = ruleSetSignature([[ANTI_AD, 'block']])
  if (!adblockOnly) return null
  // 停用的规则/策略组不参与生成，等价于偏离模板，需一并纳入判定
  if (!rules.value.every((r) => r.enabled)) return null
  if (!groups.value.every((g) => g.enabled)) return null
  const seq = rules.value.map(ruleSignature).join(';')
  const proxyOnly =
    groups.value.length === 1 && groups.value[0].name === 'proxy' && groups.value[0].mode === 'auto'
  if (seq === adblockOnly && proxyOnly && finalOutbound.value === 'proxy') return 'global-proxy-adblock'
  if (seq === adblockOnly && !groups.value.length && finalOutbound.value === 'direct') return 'direct-adblock'
  const cnDirect = ruleSetSignature([
    [ANTI_AD, 'block'],
    [GEO_CN, 'direct'],
    [GEO_IP_CN, 'direct'],
  ])
  if (seq === cnDirect && proxyOnly && finalOutbound.value === 'proxy') return 'cn-direct-adblock'
  return null
})

async function handleApplyTemplate(tpl: RuleTemplate): Promise<void> {
  const action = await messageConfirm({
    title: '应用预设模板',
    message: `将应用「${tpl.name}」并覆盖现有全部规则与策略组，确认继续吗？`,
    confirmButtonText: '应用',
    cancelButtonText: '取消',
  }).onClosed
  if (action !== 'confirm') return
  applyingTemplate.value = tpl.id
  try {
    const res = await applyRuleTemplate(tpl.id)
    message.success(`模板已应用：${res.rules} 条规则、${res.groups} 个策略组，可切到「路由规则」「策略组」继续调整`)
    // 模板会重写兜底出站，同步本地值供「当前已应用」标记即时判定
    finalOutbound.value = res.final
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    applyingTemplate.value = ''
  }
}

// ---------- 兜底出站 ----------

/**
 * 兜底出站候选：在出站候选基础上再去掉 block。
 * 与规则不同，兜底是「未命中时的默认去向」，没有可拦截的对象。
 */
const finalOptions = computed(() => outboundOptions.value.filter((o) => o.value !== 'block'))

/** 已保存的兜底出站不在候选里（出站/策略组被删除）时提示，避免保存了却不知道指向何处 */
const finalDangling = computed(
  () => !!finalOutbound.value && !finalOptions.value.some((o) => o.value === finalOutbound.value),
)

async function handleSaveFinal(): Promise<void> {
  finalSaving.value = true
  try {
    // 下拉可清除，清除即「内核默认」，显式传空串而不是 undefined
    const res = await setRouteFinal(finalOutbound.value ?? '')
    finalOutbound.value = res.final
    message.success('兜底出站已保存')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    finalSaving.value = false
  }
}

onMounted(async () => {
  await refresh()
  try {
    const final = await getRouteFinal()
    finalOutbound.value = final.final
  } catch {
    // 错误已由请求封装统一提示
  }
})
</script>

<template>
  <u-tabs v-model="activeTab" :items="tabs">
    <!-- 路由规则：tab 顶部一行说明 + 主操作「新建规则」（在卡片头），规则模型详细说明默认收起 -->
    <template #routes>
      <div class="tab-head">
        <div class="tab-desc">
          每条规则 = 匹配对象 × 匹配方式 + 匹配值 → 出站，流量自上而下逐条匹配、命中即走对应出站，顺序影响结果（可用「上移 /
          下移」调整）；整批域名 / IP 先在「规则集」建好再引用，全部未命中走「兜底与模板」里的兜底出站。
        </div>
        <u-button text type="primary" @click="guideExpanded = !guideExpanded">
          {{ guideExpanded ? '收起说明' : '详细说明' }}
        </u-button>
      </div>

      <!-- 规则模型引导：三种匹配对象 + 匹配流向 + 双端生效说明，默认收起不占首屏 -->
      <div v-if="guideExpanded" class="guide-detail">
        <div class="guide-models">
          <div class="guide-model">
            <span class="guide-model-name">域名</span>
            <span class="guide-model-desc">精确 / 后缀 / 关键字 / 正则，填 google.com 这类域名</span>
          </div>
          <div class="guide-model">
            <span class="guide-model-name">IP</span>
            <span class="guide-model-desc">按网段 CIDR 匹配，填 10.0.0.0/8 这类网段</span>
          </div>
          <div class="guide-model">
            <span class="guide-model-name">规则集</span>
            <span class="guide-model-desc">引用「规则集」页签维护的集合，命中其中任意一条域名或 IP</span>
          </div>
        </div>
        <div class="guide-flow">
          <span class="flow-step">流量</span>
          <span class="flow-arrow">→</span>
          <span class="flow-step">自上而下逐条匹配</span>
          <span class="flow-arrow">→</span>
          <span class="flow-result">命中：走对应出站</span>
          <span class="flow-result flow-result-dim">全部未命中：走兜底出站</span>
        </div>
        <div class="guide-note">
          这份规则双端生效：既写入服务端 sing-box 配置（服务器自身转发的流量），也随订阅下发到客户端（手机/电脑上的代理客户端本地分流）。规则集同理——一条规则集同一时刻只有一条内容项启用，其中
          sing-box 源地址进 sing-box 配置，mihomo 源地址 + 行为进 mihomo 订阅，两端各取所需。
        </div>
      </div>

      <div class="tab-body">
        <rule-list-card
          :rules="rules"
          :rule-sets="ruleSets"
          :outbound-options="outboundOptions"
          :loading="loading"
          @changed="refresh"
          @maintain-rule-set="openRuleSet"
        />
      </div>
    </template>

    <!-- 规则集：来源包维护，供路由规则引用 -->
    <template #sets>
      <div class="tab-head">
        <div class="tab-desc">
          规则集是整批域名 / IP 的「一个 tag + 多条来源」规则来源包，供「路由规则」引用；同一时刻只有一条来源启用，切换来源不影响引用它的规则。
        </div>
      </div>
      <div class="tab-body">
        <rule-set-card :rule-sets="ruleSets" :loading="loading" @changed="refresh" @maintain="openRuleSet" />
      </div>
    </template>

    <!-- 策略组：成员来自出站与其它策略组，供规则与兜底引用 -->
    <template #groups>
      <div class="tab-head">
        <div class="tab-desc">
          策略组聚合多个出站成员供「路由规则」与兜底出站引用，并随订阅下发为客户端里的分组选择器；成员来自「出站」页与其它策略组。
        </div>
      </div>
      <div class="tab-body">
        <policy-group-card :groups="groups" :outbound-options="outboundOptions" :loading="loading" @changed="refresh" />
      </div>
    </template>

    <!-- 兜底与模板：预设模板一键应用 + 兜底出站设置 -->
    <template #final>
      <div class="tab-head">
        <div class="tab-desc">
          兜底出站是未命中任何规则的流量的最终去向；预设模板一键生成整套「路由规则 + 策略组 +
          兜底出站」的起点，应用会覆盖现有全部规则与策略组。
        </div>
      </div>

      <div class="tab-body">
        <u-card class="section-card">
          <u-card-header>预设模板</u-card-header>
          <u-card-content>
            <div class="templates">
              <div v-for="tpl in templates" :key="tpl.id" class="template-item">
                <div class="template-info">
                  <div class="template-name">
                    {{ tpl.name }}
                    <u-tag v-if="appliedTemplate === tpl.id" type="success">当前已应用</u-tag>
                  </div>
                  <div class="template-desc">{{ tpl.description }}</div>
                </div>
                <!-- plain 形态：多个模板同屏，守住「每屏实心主按钮 ≤1」；应用期间全部禁用，避免叠第二个覆盖确认 -->
                <u-button
                  type="primary"
                  plain
                  :loading="applyingTemplate === tpl.id"
                  :disabled="!!applyingTemplate && applyingTemplate !== tpl.id"
                  @click="handleApplyTemplate(tpl)"
                >
                  一键应用
                </u-button>
              </div>
            </div>
          </u-card-content>
        </u-card>

        <u-card>
          <u-card-header>兜底出站</u-card-header>
          <u-card-content>
            <div class="final-row">
              <div class="final-body">
                <u-select
                  v-model="finalOutbound"
                  :options="finalOptions"
                  :text="finalOutbound || undefined"
                  placeholder="内核默认（首个出站）"
                  clearable
                  filterable
                  class="final-input"
                />
                <div class="section-desc">
                  可选直连、自定义出站或「策略组」页签里的策略组；不确定时保持 direct
                  即可。广告拦截由内置规则集 anti-ad 承担，可在「规则集」页签的「维护内容项」里切换其下载来源。
                </div>
                <div v-if="finalDangling" class="final-warn">
                  当前兜底出站「{{ finalOutbound }}」已不在出站或策略组列表中，保存后内核将无法重建，请重新选择后保存。
                </div>
              </div>
              <!-- 同 tab 内主操作额度已给「一键应用」，本卡保存降次级 -->
              <u-button :loading="finalSaving" @click="handleSaveFinal">保存</u-button>
            </div>
          </u-card-content>
        </u-card>
      </div>
    </template>
  </u-tabs>

  <!-- 规则集内容项维护弹窗：页面级持有，路由规则弹窗里的「去维护规则集」跨 tab 也能打开 -->
  <rule-set-dialog v-model:visible="setDialogVisible" :rule-set="maintaining" @changed="refresh" />
</template>

<style scoped>
/* tab 顶部说明行：主文案次级色不抢视线，右侧挂展开入口 */
.tab-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.tab-desc {
  flex: 1;
  min-width: 0;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 规则模型详细说明：收起时不占首屏，展开后弱化呈现 */
.guide-detail {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
  padding: 12px 16px;
  border: var(--u-border);
  border-radius: var(--u-radius-default);
}

/* 三种匹配对象一览：三列自适应，窄屏折成单列 */
.guide-models {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 8px;
}

.guide-model {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 12px;
  border: var(--u-border);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-top);
}

.guide-model-name {
  font-weight: 600;
}

.guide-model-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 极简流量路径示意：步骤块 + 箭头 + 两种结果 */
.guide-flow {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.flow-step {
  padding: 2px 10px;
  border: var(--u-border);
  border-radius: var(--u-radius-default);
  background: var(--u-bg-color-hover);
}

.flow-arrow {
  color: var(--u-text-color-assist);
}

.flow-result {
  color: var(--u-text-color-main);
}

.flow-result-dim {
  color: var(--u-text-color-second);
}

.guide-note {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 区块说明行：统一次级文案形态 */
.section-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 「兜底与模板」tab 内两卡的纵向节奏：首卡与末卡之间 16px */
.section-card {
  margin-bottom: 16px;
}

.templates {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.template-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  border: var(--u-border);
  border-radius: var(--u-radius-default);
}

.template-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.template-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.template-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 兜底出站：下拉占主宽度，说明在下，保存按钮与表单同高对齐 */
.final-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.final-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.final-input {
  width: 100%;
  max-width: 420px;
}

.final-warn {
  color: var(--u-color-warning);
  font-size: var(--u-font-size-main-small);
}
</style>
