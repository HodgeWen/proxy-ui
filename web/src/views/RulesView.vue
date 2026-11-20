<!-- 全局规则：规则模型引导、预设模板一键应用（覆盖前确认）、路由规则、规则集内容项维护、策略组与兜底出站 -->
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
  UTag,
  message,
  messageConfirm,
} from '@veltra/desktop'
import { computed, onMounted, shallowRef } from 'vue'

const rules = shallowRef<RuleRecord[]>([])
const ruleSets = shallowRef<RuleSet[]>([])
const groups = shallowRef<PolicyGroup[]>([])
const templates = shallowRef<RuleTemplate[]>([])
const outboundTags = shallowRef<string[]>([])
/** 首屏未加载完时各卡片不渲染空状态，避免闪一下「暂无数据」 */
const loading = shallowRef(true)
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
    message.success(`模板已应用：${res.rules} 条规则、${res.groups} 个策略组，可在下方继续修改后应用配置`)
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
  <!-- 引导卡 size=small 收紧内边距与字号，不与下方业务卡抢视觉 -->
  <u-card size="small" class="guide-card">
    <u-card-header>规则是如何工作的</u-card-header>
    <u-card-content>
      <div>
        每条规则由「匹配对象 × 匹配方式 + 匹配值」决定命中的流量，再指定出站：单个域名用域名规则，整批域名 / IP 用规则集引用。
      </div>
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
          <span class="guide-model-desc">引用下方维护的规则集，命中其中任意一条域名或 IP</span>
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
        这份规则双端生效：既写入服务端 sing-box 配置（服务器自身转发的流量），也随订阅下发到客户端（手机/电脑上的代理客户端本地分流）。规则集同理——一条规则集同一时刻只有一条内容项启用，其中 sing-box 源地址进 sing-box 配置，mihomo 源地址 + 行为进 mihomo 订阅，两端各取所需。
      </div>
    </u-card-content>
  </u-card>

  <u-card class="section-card">
    <u-card-header>预设模板</u-card-header>
    <u-card-content>
      <div class="section-desc">
        常用规则的起点：一键生成一组规则、策略组与兜底出站，应用会覆盖现有全部规则与策略组，之后可继续修改。
      </div>
      <div class="templates">
        <div v-for="tpl in templates" :key="tpl.id" class="template-item">
          <div class="template-info">
            <div class="template-name">
              {{ tpl.name }}
              <u-tag v-if="appliedTemplate === tpl.id" type="success">当前已应用</u-tag>
            </div>
            <div class="template-desc">{{ tpl.description }}</div>
          </div>
          <!-- 次级 plain 形态：页面主操作让位给下方列表卡的新建入口；应用期间全部禁用，避免叠第二个覆盖确认 -->
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

  <!-- 子卡片是多根节点（卡片 + 弹窗），class 无法透传，故由外层 div 承担纵向间距 -->
  <div class="section-card">
    <rule-list-card
      :rules="rules"
      :rule-sets="ruleSets"
      :outbound-options="outboundOptions"
      :loading="loading"
      @changed="refresh"
      @maintain-rule-set="openRuleSet"
    />
  </div>
  <div class="section-card">
    <rule-set-card :rule-sets="ruleSets" :loading="loading" @changed="refresh" @maintain="openRuleSet" />
  </div>
  <div class="section-card">
    <policy-group-card :groups="groups" :outbound-options="outboundOptions" :loading="loading" @changed="refresh" />
  </div>

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
            未命中任何规则的流量的最终去向，可选直连、自定义出站或策略组；不确定时保持 direct 即可。广告拦截由内置规则集
            anti-ad 承担，可在上方「规则集」卡片的「维护内容项」里切换其下载来源。
          </div>
          <div v-if="finalDangling" class="final-warn">
            当前兜底出站「{{ finalOutbound }}」已不在出站或策略组列表中，应用配置会失败，请重新选择后保存。
          </div>
        </div>
        <!-- 实心主按钮额度留给「新建规则」，本卡保存降次级 -->
        <u-button :loading="finalSaving" @click="handleSaveFinal">保存</u-button>
      </div>
    </u-card-content>
  </u-card>

  <rule-set-dialog v-model:visible="setDialogVisible" :rule-set="maintaining" @changed="refresh" />
</template>

<style scoped>
/* 页面纵向节奏：业务卡之间统一 16px，末卡不加 */
.section-card {
  margin-bottom: 16px;
}

.guide-card {
  margin-bottom: 16px;
}

/* 三种匹配对象一览：三列自适应，窄屏折成单列 */
.guide-models {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 8px;
  margin: 10px 0;
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
  margin: 8px 0;
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

.templates {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 12px;
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
