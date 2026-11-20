// 统计接口：实时速率、按用户累计流量。
import { request } from './http'

/** 实时统计概览（后端缓存最近一次采集样本） */
export interface RealtimeStats {
  up: number
  down: number
  at: string
}

/** 按用户累计流量行 */
export interface UserTrafficRow {
  /** 用户名 */
  tag: string
  /** 归属订阅用户备注（未绑定为空） */
  subscriber: string
  inboundName: string
  protocol: string
  uploadBytes: number
  downloadBytes: number
}

/** 查询实时速率 */
export function fetchRealtimeStats(): Promise<RealtimeStats> {
  return request<RealtimeStats>('/api/stats/realtime', { silent: true })
}

/** 查询按用户累计流量 */
export function fetchUserTraffic(): Promise<UserTrafficRow[]> {
  return request<UserTrafficRow[]>('/api/stats/user-traffic')
}
