// TLS 证书管理接口：一键自签、上传/粘贴存储（managed）与服务器本地路径引用（path）三种来源。
import { request } from './http'

/** 证书记录（后端已补充从证书内容解析出的域名与到期信息） */
export interface Certificate {
  id: number
  name: string
  mode: 'managed' | 'path'
  certPath: string
  keyPath: string
  createdAt: string
  updatedAt: string
  /** 证书 CN（证书不可解析时为空） */
  commonName: string
  /** CN + SAN 域名/IP 汇总（证书不可解析时为空数组） */
  domains: string[]
  /** 到期时间（证书不可解析为 null） */
  notAfter: string | null
  /** 剩余整天数（已过期为负；证书不可解析为 null） */
  daysLeft: number | null
  /** 证书解析失败的可读原因 */
  certError?: string
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

/** 自签密钥类型 */
export type CertKeyType = 'p256' | 'rsa2048' | 'ed25519'

/** 一键自签请求：主域名作 CN 并入 SAN，days 为有效期天数 */
export interface SelfSignPayload {
  /** 名称（留空取主域名） */
  name?: string
  /** 主域名 */
  domain: string
  /** 附加 SAN（域名或 IP） */
  sans?: string[]
  /** 有效期天数（1-36500） */
  days: number
  /** 密钥类型（缺省 p256） */
  keyType?: CertKeyType
}

/** 查询证书列表（含到期时间与剩余天数） */
export function listCerts(): Promise<Certificate[]> {
  return request<Certificate[]>('/api/certs')
}

/** 新增证书（上传/粘贴存储或本地路径引用） */
export function createCert(payload: CertPayload): Promise<Certificate> {
  return request<Certificate>('/api/certs', { method: 'POST', body: payload })
}

/** 一键生成自签证书（面板存储口径，可直接被入站 TLS 引用） */
export function selfSignCert(payload: SelfSignPayload): Promise<Certificate> {
  return request<Certificate>('/api/certs/self-signed', { method: 'POST', body: payload })
}

/** 更新证书（managed 可整体替换 PEM，path 可更换路径） */
export function updateCert(id: number, payload: CertPayload): Promise<Certificate> {
  return request<Certificate>(`/api/certs/${id}`, { method: 'PUT', body: payload })
}

/** 删除证书（被入站 TLS 或 ECH 密钥引用时后端拒绝并给出引用方） */
export function deleteCert(id: number): Promise<void> {
  return request<void>(`/api/certs/${id}`, { method: 'DELETE' })
}
