// 鉴权相关接口：健康检查 / 首次设置 / 登录登出 / 当前管理员 / 修改密码。
import { request } from './http'

/** 健康检查信息 */
export interface HealthInfo {
  status: string
  initialized: boolean
  version: string
}

/** 管理员信息 */
export interface AdminInfo {
  username: string
  createdAt?: string
}

/** 账号密码载荷 */
export interface CredentialsPayload {
  username: string
  password: string
}

/** 修改密码载荷 */
export interface ChangePasswordPayload {
  oldPassword: string
  newPassword: string
}

/** 查询面板健康状态与初始化标记 */
export function fetchHealth(silent = true): Promise<HealthInfo> {
  return request<HealthInfo>('/api/health', { silent })
}

/** 首次设置：创建管理员（仅未初始化时可用） */
export function setupAdmin(payload: CredentialsPayload): Promise<AdminInfo> {
  return request<AdminInfo>('/api/setup', { method: 'POST', body: payload })
}

/** 登录 */
export function login(payload: CredentialsPayload): Promise<AdminInfo> {
  return request<AdminInfo>('/api/login', { method: 'POST', body: payload })
}

/** 登出当前会话 */
export function logout(): Promise<null> {
  return request<null>('/api/logout', { method: 'POST' })
}

/** 查询当前登录管理员 */
export function fetchMe(silent = true): Promise<AdminInfo> {
  return request<AdminInfo>('/api/me', { silent })
}

/** 修改管理员密码 */
export function changePassword(payload: ChangePasswordPayload): Promise<null> {
  return request<null>('/api/password', { method: 'PUT', body: payload })
}
