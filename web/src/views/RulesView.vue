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
const antiADSaving = shallowRef(false)
const finalOutbound = shallowRef('')
const finalSaving = shallowRef(false)

/** 兜底出站候选：direct + 自定义出站 + 策略组 */
const finalOptions = computed(() => [
  { label: 'direct（直连）', value: 'direct' },
  ...outboundTags.value.filter((t) => t !== 'direct').map((t) => ({ label: t, value: t })),
  ...groups.value.map((g) => ({ label: `${g.name}（策略组）`, value: g.name })),
])

async function handleSaveAntiAD(): Promise<void> {
  antiADSaving.value = true
  try {
    const res = await setAntiAD(antiADURL.value.trim())
    antiADURL.value = res.url
    message.success(res.url ? 'anti-AD 规则源已保存' : '已恢复 anti-AD 官方默认源')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    antiADSaving.value = false
  }
}

async function handleSaveFinal(): Promise<void> {
  finalSaving.value = true
  try {
    await setRouteFinal(finalOutbound.value)
    message.success('兜底出站已保存')
  } catch {
    // 错误已由请求封装统一提示
  } finally {
    finalSaving.value = false
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
            <div class="template-name">{{ tpl.name }}</div>
            <div class="template-desc">{{ tpl.description }}</div>
          </div>
          <u-button
            size="small"
            type="primary"
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
          <u-button size="small" :loading="antiADSaving" @click="handleSaveAntiAD">保存</u-button>
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
          <u-button size="small" :loading="finalSaving" @click="handleSaveFinal">保存</u-button>
        </div>
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
  border: 1px solid var(--u-border, #e5e7eb);
  border-radius: 8px;
}

.template-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.template-name {
  font-weight: 600;
}

.template-desc {
  color: var(--u-text-secondary, #6b7280);
  font-size: 13px;
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
</style>
