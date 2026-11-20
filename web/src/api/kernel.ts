// 内核管理相关接口：状态 / 启停重启 / 日志 / 版本列表 / 安装。
import { request } from './http'

/** 内核运行状态 */
export interface KernelStatus {
  installed: boolean
  running: boolean
  pid?: number
  version?: string
  uptimeSeconds?: number
  corePath?: string
}

/** 内核日志行 */
export interface KernelLogLine {
  time: string
  stream: string
  line: string
}

/** GitHub Releases 版本条目（内核与面板更新共用） */
export interface ReleaseVersion {
  tag: string
  version: string
  prerelease: boolean
  publishedAt: string
  installed: boolean
}

/** 安装结果 */
export interface KernelInstallResult {
  version: string
  status: KernelStatus
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

/** 查询可用版本列表 */
export function fetchKernelVersions(): Promise<{ versions: ReleaseVersion[] }> {
  return request<{ versions: ReleaseVersion[] }>('/api/kernel/versions')
}

/** 下载并安装内核；version 为空表示最新稳定版 */
export function installKernel(version = ''): Promise<KernelInstallResult> {
  return request<KernelInstallResult>('/api/kernel/install', { method: 'POST', body: { version } })
}

/** 应用配置：聚合全部启用入站/出站重新生成并校验内核配置；内核运行中则按新配置重启 */
export function applyKernelConfig(): Promise<{ applied: boolean; status: KernelStatus }> {
  return request<{ applied: boolean; status: KernelStatus }>('/api/kernel/apply', { method: 'POST' })
}
