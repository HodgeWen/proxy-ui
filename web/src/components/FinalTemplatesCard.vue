<!-- 「兜底与模板」页签内容：预设模板一键应用（覆盖现有规则与策略组）+ 兜底出站选择与保存；
     「当前已应用」标记按规则 / 策略组 / 兜底出站与模板生成结果的签名比对得出 -->
<script setup lang="ts">
import { applyRuleTemplate, setRouteFinal } from '../api/rule'
import type { PolicyGroup, RuleRecord, RuleSet, RuleTemplate } from '../api/rule'
import {
  UButton,
  UCard,
  UCardContent,
  UCardHeader,
  USelect,
  UTag,
  message,
} from '@veltra/desktop'
import { computed, shallowRef } from 'vue'

import { confirmAction } from '../confirm-action'

const props = defineProps<{
  rules: RuleRecord[]
  ruleSets: RuleSet[]
  groups: PolicyGroup[]
  templates: RuleTemplate[]
  /** 出站候选（direct/拦截/自定义出站/启用中的策略组）；兜底在此基础上再排除拦截 */
  outboundOptions: { label: string; value: string }[]
}>()
const emit = defineEmits<{ refresh: [] }>()
/** 兜底出站：初值由页面拉取（getRouteFinal），保存与应用模板后在此更新并同步回页面 */
const finalOutbound = defineModel<string | undefined>('final', { required: true })

const applyingTemplate = shallowRef('')
const finalSaving = shallowRef(false)

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
      const id = props.ruleSets.find((s) => s.name === tag)?.id
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
  if (!props.rules.every((r) => r.enabled)) return null
  if (!props.groups.every((g) => g.enabled)) return null
  const seq = props.rules.map(ruleSignature).join(';')
  const proxyOnly =
    props.groups.length === 1 && props.groups[0].name === 'proxy' && props.groups[0].mode === 'auto'
  if (seq === adblockOnly && proxyOnly && finalOutbound.value === 'proxy') return 'global-proxy-adblock'
  if (seq === adblockOnly && !props.groups.length && finalOutbound.value === 'direct') return 'direct-adblock'
  const cnDirect = ruleSetSignature([
    [ANTI_AD, 'block'],
    [GEO_CN, 'direct'],
    [GEO_IP_CN, 'direct'],
  ])
  if (seq === cnDirect && proxyOnly && finalOutbound.value === 'proxy') return 'cn-direct-adblock'
  return null
})

async function handleApplyTemplate(tpl: RuleTemplate): Promise<void> {
  const ok = await confirmAction('应用预设模板', `将应用「${tpl.name}」并覆盖现有全部规则与策略组，确认继续吗？`, false, '应用')
  if (!ok) return
  applyingTemplate.value = tpl.id
  try {
    const res = await applyRuleTemplate(tpl.id)
    message.success(`模板已应用：${res.rules} 条规则、${res.groups} 个策略组，可切到「路由规则」「策略组」继续调整`)
    // 模板会重写兜底出站，同步本地值供「当前已应用」标记即时判定
    finalOutbound.value = res.final
    emit('refresh')
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
const finalOptions = computed(() => props.outboundOptions.filter((o) => o.value !== 'block'))

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
</script>

<template>
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
</template>

<style scoped>
/* 区块说明行：统一次级文案形态 */
.section-desc {
  color: var(--u-text-color-second);
  font-size: var(--u-font-size-main-small);
}

/* 两卡的纵向节奏：首卡与末卡之间 16px */
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
