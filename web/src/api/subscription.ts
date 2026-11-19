// 订阅接口：令牌管理、节点服务器地址与订阅链接拼装。
import { request } from './http'

/** 订阅令牌记录 */
export interface SubscriptionToken {
  id: number
  token: string
  name: string
  revoked: boolean
  createdAt: string
  lastUsedAt: string | null
}

/** 订阅设置与令牌列表 */
export interface SubscriptionInfo {
  server: string
  tokens: SubscriptionToken[]
}

/** 查询订阅设置与令牌列表 */
export function getSubscription(): Promise<SubscriptionInfo> {
  return request<SubscriptionInfo>('/api/subscription')
}

/** 保存订阅节点服务器地址；空串表示按访问 Host 推断 */
export function setSubscriptionServer(server: string): Promise<{ server: string }> {
  return request('/api/subscription/server', { method: 'PUT', body: { server } })
}

/** 生成订阅令牌 */
export function createSubscriptionToken(name: string): Promise<SubscriptionToken> {
  return request('/api/subscription/tokens', { method: 'POST', body: { name } })
}

/** 吊销/恢复订阅令牌 */
export function revokeSubscriptionToken(id: number, revoked: boolean): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/revoked`, { method: 'PATCH', body: { enabled: revoked } })
}

/** 重置订阅令牌（换新随机令牌并恢复可用） */
export function resetSubscriptionToken(id: number): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/reset`, { method: 'POST', body: {} })
}

/** 订阅链接；format 省略时由客户端 UA 自适应 */
export function subscriptionLink(token: string, format?: 'clash' | 'singbox'): string {
  const base = `${window.location.origin}/sub/${token}`
  return format ? `${base}?format=${format}` : base
}
