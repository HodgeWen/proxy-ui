<!-- 全局规则：按对象分四个 tab——路由规则 / 规则集 / 策略组 / 兜底与模板；数据一次性并发拉取，切 tab 不重复请求 -->
<script setup lang="ts">
import PolicyGroupCard from '../components/PolicyGroupCard.vue'
import RuleListCard from '../components/RuleListCard.vue'
import RuleSetCard from '../components/RuleSetCard.vue'
import RuleSetDialog from '../components/RuleSetDialog.vue'
import FinalTemplatesCard from '../components/FinalTemplatesCard.vue'
import { listOutbounds } from '../api/outbound'
import { getRouteFinal, listPolicyGroups, listRuleSets, listRuleTemplates, listRules } from '../api/rule'
import type { PolicyGroup, RuleRecord, RuleSet, RuleTemplate } from '../api/rule'
import { UButton, UTabs } from '@veltra/desktop'
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
/** 兜底出站：挂载时拉初值；保存 / 应用模板的更新经 v-model:final 从 FinalTemplatesCard 同步回来 */
const finalOutbound = shallowRef<string | undefined>('')

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
        <final-templates-card
          v-model:final="finalOutbound"
          :rules="rules"
          :rule-sets="ruleSets"
          :groups="groups"
          :templates="templates"
          :outbound-options="outboundOptions"
          @refresh="refresh"
        />
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
</style>
