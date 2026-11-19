// TLS 证书管理接口：面板上传/粘贴存储（managed）与服务器本地路径引用（path）两种方式。
import { request } from './http'

/** 证书记录 */
export interface Certificate {
  id: number
  name: string
  mode: 'managed' | 'path'
  certPath: string
  keyPath: string
  createdAt: string
  updatedAt: string
}

/** 创建/更新证书请求；managed 传 PEM 内容，path 传服务器本地路径 */
export interface CertPayload {
  name: string
  mode: 'managed' | 'path'
  certPem?: string
  keyPem?: string
  certPath?: string
  keyPath?: string
}

/** 查询证书列表 */
export function listCerts(): Promise<Certificate[]> {
  return request<Certificate[]>('/api/certs')
}

/** 新增证书 */
export function createCert(payload: CertPayload): Promise<Certificate> {
  return request<Certificate>('/api/certs', { method: 'POST', body: payload })
}

/** 更新证书（managed 可整体替换 PEM，path 可更换路径） */
export function updateCert(id: number, payload: CertPayload): Promise<Certificate> {
  return request<Certificate>(`/api/certs/${id}`, { method: 'PUT', body: payload })
}

/** 删除证书（被入站引用时后端拒绝） */
export function deleteCert(id: number): Promise<void> {
  return request<void>(`/api/certs/${id}`, { method: 'DELETE' })
}
