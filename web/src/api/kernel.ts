// 内核管理接口：内置内核的状态 / 启停重启 / 日志 / 应用配置。
// 内核已编进面板（无需下载安装），版本随面板版本固定。
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

/** 启动 / 停止 / 重启内核 */
export function kernelAction(action: 'start' | 'stop' | 'restart'): Promise<KernelStatus> {
  return request<KernelStatus>(`/api/kernel/${action}`, { method: 'POST' })
}

/** 应用配置：聚合全部启用入站/用户/出站重新生成内核配置；内核运行中则按新配置重建 */
export function applyKernelConfig(): Promise<{ applied: boolean; status: KernelStatus }> {
  return request<{ applied: boolean; status: KernelStatus }>('/api/kernel/apply', { method: 'POST' })
}
