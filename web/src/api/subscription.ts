// 订阅接口：订阅用户管理与订阅链接拼装。
import { request } from './http'

/** 订阅用户：每台设备/每位使用者一个，持有独立订阅链接（底层为匿名令牌） */
export interface SubscriptionToken {
  id: number
  token: string
  name: string
  revoked: boolean
  createdAt: string
  lastUsedAt: string | null
}

/** 查询订阅用户列表 */
export function getSubscription(): Promise<SubscriptionToken[]> {
  return request('/api/subscription')
}

/** 新建订阅用户 */
export function createSubscriptionToken(name: string): Promise<SubscriptionToken> {
  return request('/api/subscription/tokens', { method: 'POST', body: { name } })
}

/** 停用/启用订阅用户；停用后客户端立即拉不到订阅 */
export function revokeSubscriptionToken(id: number, revoked: boolean): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/revoked`, { method: 'PATCH', body: { enabled: revoked } })
}

/** 重置订阅链接（换新随机令牌并恢复可用，旧链接立即失效） */
export function resetSubscriptionToken(id: number): Promise<SubscriptionToken> {
  return request(`/api/subscription/tokens/${id}/reset`, { method: 'POST', body: {} })
}

/** 删除订阅用户 */
export function deleteSubscriptionToken(id: number): Promise<{ deleted: number }> {
  return request(`/api/subscription/tokens/${id}`, { method: 'DELETE' })
}

/** 订阅链接：面板同源子路径，客户端从哪个地址访问面板就从哪个地址拉取；format 省略时由客户端 UA 自适应 */
export function subscriptionLink(token: string, format?: 'clash' | 'singbox'): string {
  const base = `${window.location.origin}/sub/${token}`
  return format ? `${base}?format=${format}` : base
}
