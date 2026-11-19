// AI 助手相关接口：供应商配置读写、面板工具描述与执行。
import { request } from './http'

/** AI 供应商配置（api-key 不回显，仅 hasKey） */
export interface AIProviderConfig {
  baseUrl: string
  model: string
  reasoningEffort: string
  hasKey: boolean
  configured: boolean
}

/** 保存 AI 供应商配置请求（apiKey 留空保留已存 Key） */
export interface AIProviderConfigInput {
  baseUrl: string
  model: string
  apiKey: string
  reasoningEffort: string
}

/** 面板工具描述（后端注册，执行回落到业务模块） */
export interface AiToolSchema {
  name: string
  description: string
  parameters: Record<string, unknown>
  needsConfirm: boolean
}

/** 查询 AI 供应商配置 */
export function fetchAIConfig(): Promise<AIProviderConfig> {
  return request<AIProviderConfig>('/api/ai/config')
}

/** 保存 AI 供应商配置 */
export function saveAIConfig(config: AIProviderConfigInput): Promise<AIProviderConfig> {
  return request<AIProviderConfig>('/api/ai/config', { method: 'PUT', body: config })
}

/** 查询面板工具描述列表 */
export function fetchAITools(): Promise<AiToolSchema[]> {
  return request<AiToolSchema[]>('/api/ai/tools')
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
