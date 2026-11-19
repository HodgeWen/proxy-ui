// 全局规则接口：预设模板、路由规则、策略组、anti-AD 规则源与兜底出站。
import { request } from './http'

/** 路由规则记录 */
export interface RuleRecord {
  id: number
  type: string
  value: string
  outbound: string
  order: number
  enabled: boolean
  createdAt: string
  updatedAt: string
}

/** 创建/更新规则请求 */
export interface RulePayload {
  type: string
  value: string
  outbound: string
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

/** anti-AD 规则源设置 */
export interface AntiADSetting {
  url: string
  default: string
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

/** 查询 anti-AD 规则源设置 */
export function getAntiAD(): Promise<AntiADSetting> {
  return request<AntiADSetting>('/api/anti-ad')
}

/** 保存 anti-AD 规则源地址；空串恢复官方默认源 */
export function setAntiAD(url: string): Promise<AntiADSetting> {
  return request<AntiADSetting>('/api/anti-ad', { method: 'PUT', body: { url } })
}

/** 查询兜底出站设置 */
export function getRouteFinal(): Promise<{ final: string }> {
  return request<{ final: string }>('/api/rules/final')
}

/** 保存兜底出站设置；空串表示内核默认 */
export function setRouteFinal(final: string): Promise<{ final: string }> {
  return request<{ final: string }>('/api/rules/final', { method: 'PUT', body: { final } })
}
