// 入站接口：真实监听的入站（协议 + 端口 + TLS 等）。
// 入站用户统一在「订阅用户」页管理（勾选入站即生成、取消勾选即删除），入站只带用户数。
import { request } from './http'

/** 监听地址语义 */
export type ListenAddress = '::' | '0.0.0.0' | '127.0.0.1'

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
  /** 该入站下的用户数（来自订阅用户勾选） */
  userCount: number
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

/** 按协议取新建默认配置模板（后端 NewTemplateConfig 单一来源，含预生成的 Reality 密钥与 SS PSK）；
    silent：拉取失败由调用方兜底空配置，不弹统一错误提示 */
export async function fetchInboundTemplate(protocol: string): Promise<Record<string, unknown>> {
  const { config } = await request<{ config: Record<string, unknown> }>(
    `/api/inbounds/template/${protocol}`,
    { silent: true },
  )
  return config
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
