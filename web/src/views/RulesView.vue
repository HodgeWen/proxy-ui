<!-- 全局规则：预设模板一键应用（覆盖前确认）、路由规则与策略组编辑、anti-AD 源与兜底出站配置 -->
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
  <u-card class="templates-card">
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
          <u-input
            v-model="antiADURL"
            :placeholder="`默认官方源 ${antiADDefault}`"
            class="settings-input"
          />
        </div>
        <div class="settings-field">
          <div class="settings-label">未匹配规则的流量走</div>
          <u-select
            v-model="finalOutbound"
            :options="finalOptions"
            placeholder="内核默认（首个出站）"
            clearable
            filterable
            class="settings-input"
          />
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
.templates-card {
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

.settings-row {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-field {
  display: flex;
  align-items: center;
  gap: 12px;
}

.settings-label {
  width: 150px;
  flex-shrink: 0;
}

.settings-input {
  flex: 1;
}

/* 页面级表单操作区：卡片底部右侧 */
.card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
