// 订阅接口：订阅用户（配额/到期）与其在各入站下的节点。
import { request } from './http'

/** 订阅用户的一个节点（= 该用户在某入站下的身份，共享入站端口） */
export interface TokenNode {
  /** 入站用户 ID */
  userId: number
  inboundId: number
  inboundName: string
  protocol: string
  /** 入站端口（同入站下所有用户共享） */
  listenPort: number
  /** 用户名，即客户端节点名 */
  name: string
  enabled: boolean
  /** 管理员手动停用 */
  disabled: boolean
  suspended: boolean
  uploadBytes: number
  downloadBytes: number
}

/** 订阅用户：独立订阅链接 + 流量配额 + 到期时间 */
export interface SubscriptionToken {
  id: number
  token: string
  name: string
  revoked: boolean
  /** 流量配额字节，0 表示不限 */
  trafficLimitBytes: number
  /** 已用流量（上+下行） */
  usedBytes: number
  expiresAt: string | null
  expired: boolean
  overQuota: boolean
  createdAt: string
  lastUsedAt: string | null
  nodes: TokenNode[]
}

/** 创建/更新订阅用户请求（字段缺省表示不修改；nodeInbounds 为整体替换的入站 ID 列表） */
export interface SubscriptionPayload {
  name?: string
  trafficLimitBytes?: number
  expiresAt?: string | null
  clearExpiry?: boolean
  nodeInbounds?: number[]
}

/** 查询订阅用户列表 */
export function getSubscription(): Promise<SubscriptionToken[]> {
  return request('/api/subscription')
}

/** 新建订阅用户 */
export function createSubscriptionToken(payload: SubscriptionPayload): Promise<SubscriptionToken> {
  return request('/api/subscription/tokens', { method: 'POST', body: payload })
}

/** 更新订阅用户（配额/到期/勾选的入站） */
export function updateSubscriptionToken(id: number, payload: SubscriptionPayload): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}`, { method: 'PUT', body: payload })
}

/** 停用/启用订阅用户；停用后客户端立即拉不到订阅，其用户也会被暂停 */
export function revokeSubscriptionToken(id: number, revoked: boolean): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/revoked`, { method: 'PATCH', body: { enabled: revoked } })
}

/** 重置订阅链接（换新随机令牌并恢复可用，旧链接立即失效） */
export function resetSubscriptionToken(id: number): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/reset`, { method: 'POST', body: {} })
}

/** 清零该用户全部节点的累计流量 */
export function resetSubscriptionTraffic(id: number): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/reset-traffic`, { method: 'POST', body: {} })
}

/** 删除订阅用户（连同其全部节点用户与累计流量） */
export function deleteSubscriptionToken(id: number): Promise<{ deleted: number }> {
  return request(`/api/subscription/tokens/${id}`, { method: 'DELETE' })
}

/** 订阅链接：面板同源子路径，客户端从哪个地址访问面板就从哪个地址拉取；format 省略时由客户端 UA 自适应 */
export function subscriptionLink(token: string, format?: 'clash' | 'singbox'): string {
  const base = `${window.location.origin}/sub/${token}`
  return format ? `${base}?format=${format}` : base
}

/** 预览格式：sing-box 或 mihomo */
export type PreviewFormat = 'sing-box' | 'mihomo'

/** 订阅配置预览结果：该用户真实导出的渲染文本 */
export interface SubscriptionPreview {
  format: PreviewFormat
  content: string
}

/** 预览订阅用户的客户端配置：忽略配额与到期，无可用节点时后端返回 400 与原因；
 *  错误不在全局 toast，由预览弹框内展示原因 */
export function previewSubscription(
  id: number,
  format: PreviewFormat,
): Promise<SubscriptionPreview> {
  return request(`/api/subscription/tokens/${id}/preview?format=${format}`, { silent: true })
}
