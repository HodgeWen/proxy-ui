// 统计接口：实时速率/内存、当前连接列表、按入站累计流量。
import { request } from './http'

/** 实时统计概览（后端缓存最近一秒采集样本） */
export interface RealtimeStats {
  up: number
  down: number
  memory: number
  uploadTotal: number
  downloadTotal: number
  activeConnections: number
  at: string
}

/** 当前连接条目 */
export interface StatsConnection {
  id: string
  network: string
  inbound: string
  source: string
  destination: string
  host: string
  upload: number
  download: number
  start: string
  chain: string[] | null
  rule: string
}

/** 连接快照 */
export interface ConnectionsSnapshot {
  uploadTotal: number
  downloadTotal: number
  memory: number
  connections: StatsConnection[]
}

/** 按入站累计流量行 */
export interface InboundTrafficRow {
  inboundId: number
  name: string
  protocol: string
  enabled: boolean
  uploadBytes: number
  downloadBytes: number
}

/** 查询实时速率、内存与连接概览 */
export function fetchRealtimeStats(): Promise<RealtimeStats> {
  return request<RealtimeStats>('/api/stats/realtime', { silent: true })
}

/** 查询当前连接列表 */
export function fetchConnections(): Promise<ConnectionsSnapshot> {
  return request<ConnectionsSnapshot>('/api/stats/connections', { silent: true })
}

/** 查询按入站累计流量 */
export function fetchInboundTraffic(): Promise<InboundTrafficRow[]> {
  return request<InboundTrafficRow[]>('/api/stats/inbound-traffic')
}
