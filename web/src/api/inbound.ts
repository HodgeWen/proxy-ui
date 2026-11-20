// 入站接口：真实监听的入站（协议 + 端口 + TLS 等），入站内可挂多个用户。
import { request } from './http'

/** 监听地址语义 */
export type ListenAddress = '::' | '0.0.0.0' | '127.0.0.1'

/** 入站内的用户（内核认证身份，用户名即客户端节点名与统计维度） */
export interface InboundUser {
  id: number
  inboundId: number
  name: string
  /** 生效状态（= 未手动停用且未被暂停） */
  enabled: boolean
  /** 管理员手动停用 */
  disabled: boolean
  /** 是否被系统暂停（配额用尽 / 到期 / 订阅停用） */
  suspended: boolean
  uploadBytes: number
  downloadBytes: number
  /** 归属订阅用户 ID（0 表示未绑定，为手动创建的用户） */
  tokenId: number
  /** 归属订阅用户备注 */
  tokenName: string
  createdAt: string
}

/** 入站（后端视图：config 为连接层嵌套对象，warnings 为弱配置警示） */
export interface Inbound {
  id: number
  name: string
  protocol: string
  listen: ListenAddress
  listenPort: number
  enabled: boolean
  config: Record<string, unknown>
  warnings: string[]
  users: InboundUser[]
  createdAt: string
  updatedAt: string
}

/** 创建/更新入站请求（listenPort 缺省时后端分配随机空闲端口） */
export interface InboundPayload {
  name: string
  protocol: string
  listen: ListenAddress
  listenPort?: number
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

/** 删除入站（连同其用户与订阅绑定） */
export function deleteInbound(id: number): Promise<void> {
  return request<void>(`/api/inbounds/${id}`, { method: 'DELETE' })
}

/** 查询全部入站用户（跨入站汇总） */
export function listUsers(): Promise<InboundUser[]> {
  return request<InboundUser[]>('/api/users')
}

/** 在某入站下新建手动用户（自动生成凭据） */
export function createUser(inboundId: number, name: string): Promise<InboundUser> {
  return request<InboundUser>(`/api/inbounds/${inboundId}/users`, { method: 'POST', body: { name } })
}

/** 启停入站用户 */
export function setUserEnabled(id: number, enabled: boolean): Promise<InboundUser> {
  return request<InboundUser>(`/api/users/${id}/enabled`, { method: 'PATCH', body: { enabled } })
}

/** 删除入站用户 */
export function deleteUser(id: number): Promise<void> {
  return request<void>(`/api/users/${id}`, { method: 'DELETE' })
}
