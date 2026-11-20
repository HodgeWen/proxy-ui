// DNS 接口：当前配置、可选类型 / 策略与内置预设，保存后立即按新配置重建内核。
import { request } from './http'

/** DNS 服务器类型：udp / tcp / tls / https / h3 / local */
export interface DNSServer {
  tag: string // 服务器 tag（规则与 final 引用）
  type: string // 类型
  server?: string // 服务器地址（local 类型留空）
  serverPort?: number // 端口，空则按类型默认（53/853/443）
  path?: string // DoH 路径，默认 /dns-query
  detour?: string // 经由此出站发起查询，空为直连
}

/** 按域名把查询分流到指定 DNS 服务器 */
export interface DNSRule {
  domain?: string // 精确域名（逗号分隔多值）
  domainSuffix?: string // 域名后缀（逗号分隔多值）
  domainKeyword?: string // 域名关键字（逗号分隔多值）
  server: string // 目标 DNS 服务器 tag
}

/** DNS 配置 */
export interface DNSSettings {
  enabled: boolean // 是否启用自定义 DNS
  strategy: string // 默认解析策略，空串表示内核默认
  servers: DNSServer[] // 服务器列表
  rules: DNSRule[] // 分流规则（自上而下匹配）
  final: string // 默认 DNS 服务器 tag，空则取首个
}

/** 内置 DNS 预设 */
export interface DNSPreset {
  id: string
  name: string
  description: string
  config: DNSSettings
}

/** DNS 设置视图：当前配置 + 可选类型 / 策略 + 预设 */
export interface DNSResponse {
  config: DNSSettings
  types: string[]
  strategies: string[]
  presets: DNSPreset[]
}

/** 查询 DNS 设置 */
export function getDNS(): Promise<DNSResponse> {
  return request<DNSResponse>('/api/dns')
}

/** 保存 DNS 设置；配置非法时后端返回 400 并由请求封装统一提示 */
export function saveDNS(settings: DNSSettings): Promise<DNSSettings> {
  return request<DNSSettings>('/api/dns', { method: 'PUT', body: settings })
}
