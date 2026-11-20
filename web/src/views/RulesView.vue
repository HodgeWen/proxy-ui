<!-- 全局规则：页顶规则引导卡、预设模板一键应用（覆盖前确认）、路由规则与策略组编辑、anti-AD 源与兜底出站配置 -->
<script setup lang="ts">
import PolicyGroupCard from '../components/PolicyGroupCard.vue'
import RuleListCard from '../components/RuleListCard.vue'
import { listOutbounds } from '../api/outbound'
import {
  applyRuleTemplate,
  getAntiAD,
  getRouteFinal,
  listPolicyGroups,
  listRuleTemplates,
  listRules,
  setAntiAD,
  setRouteFinal,
} from '../api/rule'
import type { PolicyGroup, RuleRecord, RuleTemplate } from '../api/rule'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  UInput,
  USelect,
  UTag,
  message,
  messageConfirm,
} from '@veltra/desktop'
import { computed, onMounted, shallowRef } from 'vue'

const rules = shallowRef<RuleRecord[]>([])
const groups = shallowRef<PolicyGroup[]>([])
const templates = shallowRef<RuleTemplate[]>([])
const outboundTags = shallowRef<string[]>([])
const applyingTemplate = shallowRef('')

/** 出站 tag 选项：direct/拦截 + 自定义出站 + 策略组 */
const outboundOptions = computed(() => [
  { label: 'direct（直连）', value: 'direct' },
  { label: 'block（拦截）', value: 'block' },
  ...outboundTags.value.map((t) => ({ label: t, value: t })),
  ...groups.value.map((g) => ({ label: `${g.name}（策略组）`, value: g.name })),
])

async function refresh(): Promise<void> {
  const [ruleList, groupList, templateList, outboundList] = await Promise.all([
    listRules(),
    listPolicyGroups(),
    listRuleTemplates(),
    listOutbounds(),
  ])
  rules.value = ruleList
  groups.value = groupList
  templates.value = templateList
  outboundTags.value = outboundList.map((o) => o.name)
}

// ---------- 预设模板 ----------

/** 模板生成结果签名（与后端 HandleRuleTemplateApply 的规则/策略组/兜底一一对应） */
const ANTI_AD_RULE = 'rule_set|anti-ad|block'

/** 当前规则/策略组/兜底出站与某模板生成结果完全一致时视为「当前已应用」；任一改动即失效 */
const appliedTemplate = computed<string | null>(() => {
  const seq = rules.value.map((r) => `${r.type}|${r.value}|${r.outbound}`).join(';')
  const proxyOnly = groups.value.length === 1 && groups.value[0].name === 'proxy' && groups.value[0].mode === 'auto'
  if (seq === ANTI_AD_RULE && !groups.value.length && finalOutbound.value === 'direct') return 'direct-adblock'
  if (seq === ANTI_AD_RULE && proxyOnly && finalOutbound.value === 'proxy') return 'global-proxy-adblock'
  if (
    seq === `${ANTI_AD_RULE};geosite|cn|direct;geoip|cn|direct` &&
    proxyOnly &&
    finalOutbound.value === 'proxy'
  ) {
    return 'cn-direct-adblock'
  }
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
    message.success(`模板已应用：${res.rules} 条规则、${res.groups} 个策略组，可在上方继续修改后应用配置`)
    // 模板会重写兜底出站，同步本地值供「当前已应用」标记即时判定
    finalOutbound.value = res.final
    await refresh()
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    applyingTemplate.value = ''
  }
}

// ---------- anti-AD 源与兜底出站 ----------

const antiADURL = shallowRef('')
const antiADDefault = shallowRef('')
const finalOutbound = shallowRef('')
const globalSaving = shallowRef(false)

/** 兜底出站候选：direct + 自定义出站 + 策略组 */
const finalOptions = computed(() => [
  { label: 'direct（直连）', value: 'direct' },
  ...outboundTags.value.filter((t) => t !== 'direct').map((t) => ({ label: t, value: t })),
  ...groups.value.map((g) => ({ label: `${g.name}（策略组）`, value: g.name })),
])

/** 页面级表单统一保存：anti-AD 源与兜底出站一并提交 */
async function handleSaveGlobal(): Promise<void> {
  globalSaving.value = true
  try {
    const [antiAD] = await Promise.all([setAntiAD(antiADURL.value.trim()), setRouteFinal(finalOutbound.value)])
    antiADURL.value = antiAD.url
    message.success('广告拦截与兜底出站已保存')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    globalSaving.value = false
  }
}

onMounted(async () => {
  try {
    await refresh()
    const [antiAD, final] = await Promise.all([getAntiAD(), getRouteFinal()])
    antiADURL.value = antiAD.url
    antiADDefault.value = antiAD.default
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
      <div>规则决定「什么流量走哪个出站」：每条规则由匹配条件与出站去向组成。</div>
      <div class="guide-flow">
        <span class="flow-step">流量</span>
        <span class="flow-arrow">→</span>
        <span class="flow-step">自上而下逐条匹配</span>
        <span class="flow-arrow">→</span>
        <span class="flow-result">命中：走对应出站</span>
        <span class="flow-result flow-result-dim">全部未命中：走兜底出站</span>
      </div>
      <div class="guide-note">
        这份规则双端生效：既写入服务端 sing-box 配置（服务器自身转发的流量），也随订阅下发到客户端（手机/电脑上的代理客户端本地分流），两层用的是同一份规则。
      </div>
    </u-card-content>
  </u-card>

  <u-card class="templates-card">
    <u-card-header>预设模板</u-card-header>
    <u-card-content>
      <div class="section-desc">常用规则的起点：一键生成一组规则、策略组与兜底出站，应用会覆盖现有全部规则与策略组，之后可继续修改。</div>
      <div class="templates">
        <div v-for="tpl in templates" :key="tpl.id" class="template-item">
          <div class="template-info">
            <div class="template-name">
              {{ tpl.name }}
              <u-tag v-if="appliedTemplate === tpl.id" type="success">当前已应用</u-tag>
            </div>
            <div class="template-desc">{{ tpl.description }}</div>
          </div>
          <!-- 次级 plain 形态：页面主操作让位给下方列表卡的新建入口 -->
          <u-button
            type="primary"
            plain
            :loading="applyingTemplate === tpl.id"
            @click="handleApplyTemplate(tpl)"
          >
            一键应用
          </u-button>
        </div>
      </div>
    </u-card-content>
  </u-card>

  <rule-list-card :rules="rules" :outbound-options="outboundOptions" @changed="refresh" />
  <policy-group-card :groups="groups" :outbound-options="outboundOptions" @changed="refresh" />

  <u-card>
    <u-card-header>广告拦截与兜底出站</u-card-header>
    <u-card-content>
      <div class="settings-row">
        <div class="settings-field">
          <div class="settings-label">anti-AD 规则源</div>
          <div class="settings-body">
            <u-input
              v-model="antiADURL"
              :placeholder="`默认官方源 ${antiADDefault}`"
              class="settings-input"
            />
            <div class="settings-desc">
              广告拦截在客户端本地生效：sing-box 客户端按此地址自动下载并更新规则集（clash 客户端固定使用 anti-ad 官方 clash 源），在手机/电脑上拦广告；服务端配置同时也会拒绝这些广告域名（双端拦截）。
            </div>
          </div>
        </div>
        <div class="settings-field">
          <div class="settings-label">未匹配规则的流量走</div>
          <div class="settings-body">
            <u-select
              v-model="finalOutbound"
              :options="finalOptions"
              placeholder="内核默认（首个出站）"
              clearable
              filterable
              class="settings-input"
            />
            <div class="settings-desc">
              兜底出站：未命中任何规则的流量的最终去向，可选直连、自定义出站或策略组；不确定时保持 direct 即可。
            </div>
          </div>
        </div>
      </div>
      <div class="card-actions">
        <!-- 实心主按钮额度留给「新建规则」，本卡保存降次级 -->
        <u-button :loading="globalSaving" @click="handleSaveGlobal">保存</u-button>
      </div>
    </u-card-content>
  </u-card>
</template>

<style scoped>
.guide-card {
  margin-bottom: 16px;
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

.templates-card {
  margin-bottom: 16px;
}

/* 区块说明行：统一次级文案形态 */
.section-desc {
  margin-bottom: 12px;
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
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

.settings-row {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-field {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.settings-label {
  width: 150px;
  flex-shrink: 0;
  line-height: 32px;
}

.settings-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.settings-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

.settings-input {
  width: 100%;
}

/* 页面级表单操作区：卡片底部右侧 */
.card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
