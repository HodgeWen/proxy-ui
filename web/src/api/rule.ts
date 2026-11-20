// 全局规则接口：预设模板、路由规则、规则集、策略组与兜底出站。
import { request } from './http'

/** 规则匹配对象 */
export type RuleMatchType = 'domain' | 'ip' | 'rule_set'

/** 规则匹配方式 */
export type RuleMatchMethod = 'exact' | 'suffix' | 'keyword' | 'regex' | 'cidr'

/** 路由规则记录 */
export interface RuleRecord {
  id: number
  matchType: RuleMatchType
  matchMethod: RuleMatchMethod | ''
  value: string
  ruleSetId: number
  ruleSetName: string
  outbound: string
  order: number
  enabled: boolean
  createdAt: string
  updatedAt: string
}

/** 创建/更新规则请求 */
export interface RulePayload {
  matchType: RuleMatchType
  matchMethod?: RuleMatchMethod | ''
  value?: string
  ruleSetId?: number
  outbound: string
}

/** 规则集内容项：一条内容 = 备注 + 各内核的规则源地址 */
export interface RuleSetItem {
  id: number
  remark: string
  /** sing-box 规则源（.srs） */
  singBoxUrl: string
  /** mihomo 规则源（.mrs / .yaml） */
  mihomoUrl: string
  /** mihomo 行为：domain / ipcidr / classical */
  mihomoBehavior: 'domain' | 'ipcidr' | 'classical'
  enabled: boolean
}

/** 规则集 */
export interface RuleSet {
  id: number
  name: string
  description: string
  /** 内置规则集不可删除，可切换来源 */
  builtin: boolean
  usedByRules: number
  items: RuleSetItem[]
  createdAt: string
  updatedAt: string
}

/** 创建/更新规则集请求 */
export interface RuleSetPayload {
  name: string
  description: string
}

/** 创建/更新规则集内容项请求 */
export interface RuleSetItemPayload {
  remark: string
  singBoxUrl: string
  mihomoUrl: string
  mihomoBehavior: 'domain' | 'ipcidr' | 'classical'
  enabled: boolean
}

/** 策略组记录 */
export interface PolicyGroup {
  id: number
  name: string
  mode: 'failover' | 'fixed' | 'auto'
  members: string[]
  enabled: boolean
  createdAt: string
  updatedAt: string
}

/** 创建/更新策略组请求 */
export interface PolicyGroupPayload {
  name: string
  mode: 'failover' | 'fixed' | 'auto'
  members: string[]
}

/** 预设规则模板 */
export interface RuleTemplate {
  id: string
  name: string
  description: string
}

/** 查询规则列表 */
export function listRules(): Promise<RuleRecord[]> {
  return request<RuleRecord[]>('/api/rules')
}

/** 创建规则 */
export function createRule(payload: RulePayload): Promise<RuleRecord> {
  return request<RuleRecord>('/api/rules', { method: 'POST', body: payload })
}

/** 更新规则 */
export function updateRule(id: number, payload: RulePayload): Promise<RuleRecord> {
  return request<RuleRecord>(`/api/rules/${id}`, { method: 'PUT', body: payload })
}

/** 按给定 ID 顺序重排规则 */
export function reorderRules(ids: number[]): Promise<void> {
  return request<void>('/api/rules/order', { method: 'PUT', body: { ids } })
}

/** 启停规则 */
export function setRuleEnabled(id: number, enabled: boolean): Promise<RuleRecord> {
  return request<RuleRecord>(`/api/rules/${id}/enabled`, { method: 'PATCH', body: { enabled } })
}

/** 删除规则 */
export function deleteRule(id: number): Promise<void> {
  return request<void>(`/api/rules/${id}`, { method: 'DELETE' })
}

/** 查询规则集列表（含内容项） */
export function listRuleSets(): Promise<RuleSet[]> {
  return request<RuleSet[]>('/api/rule-sets')
}

/** 新建规则集 */
export function createRuleSet(payload: RuleSetPayload): Promise<RuleSet> {
  return request<RuleSet>('/api/rule-sets', { method: 'POST', body: payload })
}

/** 更新规则集名称/说明 */
export function updateRuleSet(id: number, payload: RuleSetPayload): Promise<void> {
  return request<void>(`/api/rule-sets/${id}`, { method: 'PUT', body: payload })
}

/** 删除规则集（内置或有规则引用时后端拒绝） */
export function deleteRuleSet(id: number): Promise<void> {
  return request<void>(`/api/rule-sets/${id}`, { method: 'DELETE' })
}

/** 新增规则集内容项；enabled 为 true 时自动停用该规则集其它内容项 */
export function createRuleSetItem(ruleSetId: number, payload: RuleSetItemPayload): Promise<RuleSetItem> {
  return request<RuleSetItem>(`/api/rule-sets/${ruleSetId}/items`, { method: 'POST', body: payload })
}

/** 更新规则集内容项（含切换启用） */
export function updateRuleSetItem(itemId: number, payload: RuleSetItemPayload): Promise<RuleSetItem> {
  return request<RuleSetItem>(`/api/rule-sets/items/${itemId}`, { method: 'PUT', body: payload })
}

/** 删除规则集内容项（每个规则集至少保留一条） */
export function deleteRuleSetItem(itemId: number): Promise<void> {
  return request<void>(`/api/rule-sets/items/${itemId}`, { method: 'DELETE' })
}

/** 查询预设模板列表 */
export function listRuleTemplates(): Promise<RuleTemplate[]> {
  return request<RuleTemplate[]>('/api/rule-templates')
}

/** 一键应用预设模板（覆盖现有全部规则与策略组） */
export function applyRuleTemplate(id: string): Promise<{ template: string; rules: number; groups: number; final: string }> {
  return request(`/api/rule-templates/${id}/apply`, { method: 'POST', body: {} })
}

/** 查询策略组列表 */
export function listPolicyGroups(): Promise<PolicyGroup[]> {
  return request<PolicyGroup[]>('/api/policy-groups')
}

/** 创建策略组 */
export function createPolicyGroup(payload: PolicyGroupPayload): Promise<PolicyGroup> {
  return request<PolicyGroup>('/api/policy-groups', { method: 'POST', body: payload })
}

/** 更新策略组 */
export function updatePolicyGroup(id: number, payload: PolicyGroupPayload): Promise<PolicyGroup> {
  return request<PolicyGroup>(`/api/policy-groups/${id}`, { method: 'PUT', body: payload })
}

/** 启停策略组 */
export function setPolicyGroupEnabled(id: number, enabled: boolean): Promise<PolicyGroup> {
  return request<PolicyGroup>(`/api/policy-groups/${id}/enabled`, { method: 'PATCH', body: { enabled } })
}

/** 删除策略组 */
export function deletePolicyGroup(id: number): Promise<void> {
  return request<void>(`/api/policy-groups/${id}`, { method: 'DELETE' })
}

/** 查询兜底出站设置 */
export function getRouteFinal(): Promise<{ final: string }> {
  return request<{ final: string }>('/api/rules/final')
}

/** 保存兜底出站设置；空串表示内核默认 */
export function setRouteFinal(final: string): Promise<{ final: string }> {
  return request<{ final: string }>('/api/rules/final', { method: 'PUT', body: { final } })
}
