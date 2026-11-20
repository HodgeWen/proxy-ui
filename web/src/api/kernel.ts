// 内核接口：内置内核的只读状态与日志。
// 内核随面板自动运行，配置改动保存即热应用，因此没有启停与「应用配置」接口。
import { request } from './http'

/** 内核运行状态 */
export interface KernelStatus {
  installed: boolean
  running: boolean
  /** 内置内核版本（编译期固定） */
  version?: string
  uptimeSeconds?: number
  /** 恒为 true：内核已内置 */
  embedded: boolean
}

/** 内核日志行 */
export interface KernelLogLine {
  time: string
  stream: string
  line: string
}

/** 查询内核运行状态 */
export function fetchKernelStatus(): Promise<KernelStatus> {
  return request<KernelStatus>('/api/kernel/status')
}

/** 查询内核日志（limit 为返回的最近行数） */
export function fetchKernelLogs(limit = 500): Promise<KernelLogLine[]> {
  return request<KernelLogLine[]>(`/api/kernel/logs?limit=${limit}`)
}
