// 出站管理接口：direct 与自定义出站的增删改查与启停。
import { request } from './http'

/** 出站记录 */
export interface Outbound {
  id: number
  name: string
  type: string
  enabled: boolean
  config: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

/** 创建/更新出站请求；config 为 type/tag 之外的出站字段 */
export interface OutboundPayload {
  name: string
  type: string
  config: Record<string, unknown>
}

/** 查询出站列表 */
export function listOutbounds(): Promise<Outbound[]> {
  return request<Outbound[]>('/api/outbounds')
}

/** 创建出站 */
export function createOutbound(payload: OutboundPayload): Promise<Outbound> {
  return request<Outbound>('/api/outbounds', { method: 'POST', body: payload })
}

/** 更新出站 */
export function updateOutbound(id: number, payload: OutboundPayload): Promise<Outbound> {
  return request<Outbound>(`/api/outbounds/${id}`, { method: 'PUT', body: payload })
}

/** 启停出站 */
export function setOutboundEnabled(id: number, enabled: boolean): Promise<Outbound> {
  return request<Outbound>(`/api/outbounds/${id}/enabled`, { method: 'PATCH', body: { enabled } })
}

/** 删除出站 */
export function deleteOutbound(id: number): Promise<void> {
  return request<void>(`/api/outbounds/${id}`, { method: 'DELETE' })
}
