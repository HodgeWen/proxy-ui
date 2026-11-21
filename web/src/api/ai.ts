// AI 助手相关接口：多供应商/多模型配置增删改查、上游 /models 拉取、transport 元数据与面板工具执行。
import { request } from './http'

/** 推理等级：value 写入请求 reasoning_effort，label 为 UI 文案 */
export interface AiReasoningLevel {
  value: string
  label: string
}

/** AI 供应商（api-key 不回显，仅 hasKey；含其下模型） */
export interface AiProvider {
  id: number
  name: string
  baseUrl: string
  enabled: boolean
  sort: number
  hasKey: boolean
  models: AiModel[]
}

/** AI 模型（modelId 即发往上游的模型名，跨供应商全局唯一） */
export interface AiModel {
  id: number
  providerId: number
  modelId: string
  label: string
  description: string
  reasoningLevels: AiReasoningLevel[]
  defaultReasoningLevel: string
  contextWindow: number
  enabled: boolean
}

/** 供应商创建/更新入参（apiKey 留空表示保留已存 Key） */
export interface AiProviderInput {
  name: string
  baseUrl: string
  apiKey: string
  enabled: boolean
}

/** 模型创建/更新入参 */
export interface AiModelInput {
  modelId: string
  label: string
  description: string
  reasoningLevels: AiReasoningLevel[]
  defaultReasoningLevel: string
  contextWindow: number
  enabled: boolean
}

/** 上游 /models 拉取条目 */
export interface AiUpstreamModel {
  id: string
  ownedBy: string
}

/** transport 供应商（createOpenAITransport({ providers }) 入参形态） */
export interface AiTransportProvider {
  id: string
  label: string
  endpoint: string
  mode: 'chat'
  models: AiTransportModel[]
}

/** transport 模型（仅启用中的模型） */
export interface AiTransportModel {
  id: string
  label?: string
  description?: string
  reasoningLevels?: AiReasoningLevel[]
  defaultReasoningLevel?: string
  contextWindow?: number
}

/** GET /api/ai/transport 返回（可用供应商为空时前端引导去设置页） */
export interface AiTransport {
  providers: AiTransportProvider[]
}

/** 面板工具描述（后端注册，执行回落到业务模块） */
export interface AiToolSchema {
  name: string
  description: string
  parameters: Record<string, unknown>
  needsConfirm: boolean
}

/** 查询供应商列表（含各自模型） */
export function fetchAIProviders(): Promise<AiProvider[]> {
  return request<AiProvider[]>('/api/ai/providers')
}

/** 新增供应商 */
export function createAIProvider(input: AiProviderInput): Promise<AiProvider> {
  return request<AiProvider>('/api/ai/providers', { method: 'POST', body: input })
}

/** 更新供应商（apiKey 留空保留已存 Key） */
export function updateAIProvider(id: number, input: AiProviderInput): Promise<AiProvider> {
  return request<AiProvider>(`/api/ai/providers/${id}`, { method: 'PUT', body: input })
}

/** 删除供应商及其下全部模型 */
export function deleteAIProvider(id: number): Promise<void> {
  return request<void>(`/api/ai/providers/${id}`, { method: 'DELETE' })
}

/** 手工新增模型 */
export function createAIModel(providerId: number, input: AiModelInput): Promise<AiModel> {
  return request<AiModel>(`/api/ai/providers/${providerId}/models`, { method: 'POST', body: input })
}

/** 更新模型 */
export function updateAIModel(id: number, input: AiModelInput): Promise<AiModel> {
  return request<AiModel>(`/api/ai/models/${id}`, { method: 'PUT', body: input })
}

/** 删除模型 */
export function deleteAIModel(id: number): Promise<void> {
  return request<void>(`/api/ai/models/${id}`, { method: 'DELETE' })
}

/** 拉取供应商上游 /models 模型列表 */
export function fetchAIUpstreamModels(providerId: number): Promise<AiUpstreamModel[]> {
  return request<AiUpstreamModel[]>(`/api/ai/providers/${providerId}/models/fetch`, { method: 'POST' })
}

/** 批量导入模型 id（已被占用的跳过），返回实际导入条数 */
export function importAIModels(providerId: number, modelIds: string[]): Promise<{ imported: number }> {
  return request<{ imported: number }>(`/api/ai/providers/${providerId}/models/import`, {
    method: 'POST',
    body: { modelIds },
  })
}

/** 查询 createOpenAITransport 元数据（启用且配置完整的供应商与其启用模型）；静默请求，失败由对话弹框内呈现 */
export function fetchAITransport(): Promise<AiTransport> {
  return request<AiTransport>('/api/ai/transport', { silent: true })
}

/** 查询面板工具描述列表；静默请求，失败由对话弹框内呈现 */
export function fetchAITools(): Promise<AiToolSchema[]> {
  return request<AiToolSchema[]>('/api/ai/tools', { silent: true })
}

/** 执行面板工具（静默请求，错误抛给对话组件以在工具卡片内呈现） */
export async function callAITool(name: string, args: Record<string, unknown>): Promise<unknown> {
  const { result } = await request<{ result: unknown }>('/api/ai/tools/call', {
    method: 'POST',
    body: { name, arguments: args },
    silent: true,
  })
  return result
}
