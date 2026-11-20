// 系统优化接口：拥塞控制（BBR）状态查询与一键开关。
import { request } from './http'

/** 系统优化状态 */
export interface SystemStatus {
  platform: string // 运行平台：linux / darwin / windows
  supported: boolean // 是否可在面板内一键开启（仅 Linux + root + 内核支持 bbr）
  privileged: boolean // 当前进程是否有 root 权限
  bbrEnabled: boolean // 是否已启用 BBR
  congestionCtl: string // 当前 TCP 拥塞控制算法，读取失败为空串
  availableCc: string[] // 内核支持的拥塞控制算法
  availableQdisc: string[] // 内核支持的默认队列规则
  reason: string // 不可一键开启的原因，可开启时为空串
  manualCommand: string // 手动执行命令
}

/** 查询系统优化状态 */
export function getSystemStatus(): Promise<SystemStatus> {
  return request<SystemStatus>('/api/system/status')
}

/** 一键开启 / 关闭 BBR，返回最新状态；无权限或非 Linux 时后端返回 400 */
export function setBBR(enable: boolean): Promise<SystemStatus> {
  return request<SystemStatus>('/api/system/bbr', { method: 'POST', body: { enable } })
}
