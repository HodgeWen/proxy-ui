// 入站管理接口：列表 / 增删改 / 启停；视图携带后端计算的弱配置警示。
import { request } from './http'

/** 入站记录（后端视图：config 为嵌套对象，warnings 为弱配置警示） */
export interface Inbound {
  id: number
  name: string
  protocol: string
  listenPort: number
  enabled: boolean
  config: Record<string, unknown>
  warnings: string[]
  createdAt: string
  updatedAt: string
}

/** 创建/更新入站请求 */
export interface InboundPayload {
  name: string
  protocol: string
  listenPort: number
  config: Record<string, unknown>
}

/** 查询入站列表 */
export function listInbounds(): Promise<Inbound[]> {
  return request<Inbound[]>('/api/inbounds')
}

/** 创建入站 */
export function createInbound(payload: InboundPayload): Promise<Inbound> {
  return request<Inbound>('/api/inbounds', { method: 'POST', body: payload })
}

/** 更新入站（全量替换） */
export function updateInbound(id: number, payload: InboundPayload): Promise<Inbound> {
  return request<Inbound>(`/api/inbounds/${id}`, { method: 'PUT', body: payload })
}

/** 启停入站 */
export function setInboundEnabled(id: number, enabled: boolean): Promise<Inbound> {
  return request<Inbound>(`/api/inbounds/${id}/enabled`, { method: 'PATCH', body: { enabled } })
}

/** 删除入站 */
export function deleteInbound(id: number): Promise<void> {
  return request<void>(`/api/inbounds/${id}`, { method: 'DELETE' })
}
