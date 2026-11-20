// 面板自身更新相关接口：版本检查 / 一键更新。
import { request } from './http'

/** 面板 release 版本条目 */
export interface ReleaseVersion {
  tag: string
  version: string
  prerelease: boolean
  publishedAt: string
  installed: boolean
}

/** 面板版本检查结果 */
export interface PanelVersions {
  currentVersion: string
  versions: ReleaseVersion[]
}

/** 查询面板当前版本与可用版本列表 */
export function fetchPanelVersions(): Promise<PanelVersions> {
  return request<PanelVersions>('/api/panel/versions')
}

/** 一键更新面板（下载替换二进制并自动重启进程）；version 为空表示最新稳定版 */
export function updatePanel(version = ''): Promise<{ version: string }> {
  return request<{ version: string }>('/api/panel/update', { method: 'POST', body: { version } })
}
