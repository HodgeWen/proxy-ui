// 入站声明式表单定义：七协议元信息、字段 schema、连接层默认值与弱配置警示。
//
// 口径说明：入站是一个真实监听（协议 + 监听地址/端口 + TLS/Reality/混淆/带宽/拥塞算法），
// 不含任何用户凭据；用户凭据由后端为入站内每个用户独立生成，多用户共享同一监听端口。
// 默认值与警示口径和后端 internal/singbox（NewTemplateConfig / InboundWarnings）保持一致。
import type { Certificate } from './api/cert'

/** 表单字段类型 */
export type FieldType = 'input' | 'password' | 'number' | 'select' | 'multiselect' | 'switch'

/** 下拉选项 */
export interface FieldOption {
  label: string
  value: string | number
}

/** 声明式字段定义；field 为表单模型内路径（支持点号嵌套，如 reality.enabled） */
export interface FieldDef {
  field: string
  label: string
  type: FieldType
  /** 用途说明（这是什么、影响什么、典型值），渲染为 UFormItem tips */
  description?: string
  placeholder?: string
  /** UFormItem 栅格占位；缺省 1 列，'full' 占整行（长文本/路径类字段） */
  span?: number | 'full'
  options?: FieldOption[]
  when?: (cfg: Record<string, any>) => boolean
  min?: number
  max?: number
}

/** 协议定义：元信息 + 字段 schema */
export interface ProtocolDef {
  protocol: string
  name: string
  desc: string
  recommended: boolean
  tls: boolean
  basic: FieldDef[]
  advanced: FieldDef[]
}

/** 弱配置警示；jump 指向可快捷跳转的表单区块 */
export interface InboundWarning {
  text: string
  jump?: 'tls'
}

/** 监听地址选项：语义化三档，避免用户填错地址格式 */
export const LISTEN_OPTIONS: FieldOption[] = [
  { label: '双栈（IPv4 + IPv6）', value: '::' },
  { label: '仅 IPv4', value: '0.0.0.0' },
  { label: '仅本机（走反向代理）', value: '127.0.0.1' },
]

/** 监听地址 + 端口的展示串；IPv6 地址按 URL 习惯加方括号 */
export function listenLabel(listen: string, port: number): string {
  return `${listen.includes(':') ? `[${listen}]` : listen}:${port}`
}

/** TLS/证书区块字段（启用 TLS 的协议共用） */
export const TLS_FIELDS: FieldDef[] = [
  {
    field: 'tls.enabled',
    label: '启用 TLS',
    type: 'switch',
    description: '启用后流量经 TLS 加密传输；Reality 已启用时内核会强制 TLS，无需再配证书',
  },
  {
    field: 'tls.serverName',
    label: '服务器名称',
    type: 'input',
    placeholder: 'SNI，如 example.com',
    description: 'SNI 服务器名称，客户端握手时使用的域名，需与证书域名匹配',
  },
  {
    field: 'tls.certMode',
    label: '证书引用',
    type: 'select',
    description: '证书来源：面板存储（上传/粘贴 PEM，随配置部署到内核）或服务器本地路径',
    options: [
      { label: '面板存储证书', value: 'managed' },
      { label: '服务器本地路径', value: 'path' },
    ],
    when: (cfg) => !!cfg.tls?.enabled,
  },
  // tls.certId 的选项由表单组件按证书列表动态生成
  {
    field: 'tls.certId',
    label: '选择证书',
    type: 'select',
    description: '选择面板「证书管理」中已存储的证书，应用配置时自动写出对应 PEM',
    when: (cfg) => !!cfg.tls?.enabled && cfg.tls?.certMode === 'managed',
  },
  {
    field: 'tls.certPath',
    label: '证书路径',
    type: 'input',
    placeholder: '服务器上证书文件绝对路径',
    description: '服务器上证书文件的绝对路径，内核进程需有读取权限',
    span: 'full',
    when: (cfg) => !!cfg.tls?.enabled && cfg.tls?.certMode === 'path',
  },
  {
    field: 'tls.keyPath',
    label: '私钥路径',
    type: 'input',
    placeholder: '服务器上私钥文件绝对路径',
    description: '服务器上私钥文件的绝对路径，与证书路径配套使用',
    span: 'full',
    when: (cfg) => !!cfg.tls?.enabled && cfg.tls?.certMode === 'path',
  },
]

/** ALPN 选项 */
const ALPN_OPTIONS: FieldOption[] = [
  { label: 'h3', value: 'h3' },
  { label: 'h2', value: 'h2' },
  { label: 'http/1.1', value: 'http/1.1' },
]

/** TLS 高级字段（归入高级选项折叠区） */
export const TLS_ADVANCED: FieldDef[] = [
  {
    field: 'tls.alpn',
    label: 'ALPN',
    type: 'multiselect',
    description:
      'TLS ALPN 协商协议列表。QUIC 类协议（Hysteria2 / TUIC）必须含 h3（留空时内核会补 h3，但建议显式填上以便与客户端对齐）；普通 TLS 常用 h2 / http/1.1',
    options: ALPN_OPTIONS,
  },
]

const FLOW_OPTIONS: FieldOption[] = [
  { label: 'xtls-rprx-vision', value: 'xtls-rprx-vision' },
  { label: '不使用', value: '' },
]

const CC_OPTIONS: FieldOption[] = [
  { label: 'bbr（推荐）', value: 'bbr' },
  { label: 'cubic', value: 'cubic' },
  { label: 'new_reno', value: 'new_reno' },
]

const BBR_PROFILE_OPTIONS: FieldOption[] = [
  { label: 'standard（默认，通用）', value: 'standard' },
  { label: 'conservative（保守，低带宽 / 弱机器）', value: 'conservative' },
  { label: 'aggressive（激进，高带宽 / 低丢包）', value: 'aggressive' },
]

const SS_METHODS: FieldOption[] = [
  { label: '2022-blake3-aes-128-gcm', value: '2022-blake3-aes-128-gcm' },
  { label: '2022-blake3-aes-256-gcm', value: '2022-blake3-aes-256-gcm' },
]

/** 七协议定义，推荐组排前 */
export const PROTOCOL_DEFS: ProtocolDef[] = [
  {
    protocol: 'vless',
    name: 'VLESS + Reality',
    desc: 'Reality 伪装握手，无需自有证书',
    recommended: true,
    tls: true,
    basic: [
      {
        field: 'flow',
        label: '流控',
        type: 'select',
        description: '流控模式；xtls-rprx-vision 可提升 TLS 场景吞吐，Reality 场景建议保持',
        options: FLOW_OPTIONS,
      },
      {
        field: 'reality.enabled',
        label: '启用 Reality',
        type: 'switch',
        description: '启用 Reality 伪装握手，借用他人证书外观、无需自有证书；关闭后需配置真实证书',
      },
    ],
    advanced: [
      {
        field: 'reality.privateKey',
        label: 'Reality 私钥',
        type: 'input',
        description:
          'x25519 私钥（base64 raw url），内核用于完成 Reality 握手；创建入站时已自动生成，需与公钥配套。本入站所有用户共用同一对密钥',
      },
      {
        field: 'reality.publicKey',
        label: 'Reality 公钥',
        type: 'input',
        description: '与私钥配套的 x25519 公钥，客户端/订阅链接凭它连接本入站；内核本身不消费',
      },
      {
        field: 'reality.shortId',
        label: 'Short ID',
        type: 'input',
        placeholder: 'hex，0-8 位',
        description: 'Reality 短 ID（hex，0-8 位），客户端需填写一致才能通过握手；留空表示不校验',
      },
      {
        field: 'reality.handshakeServer',
        label: '握手目标站点',
        type: 'input',
        placeholder: 'www.microsoft.com',
        description: 'Reality 借用握手的目标站点域名，建议选稳定的大站（如 www.microsoft.com）',
      },
      {
        field: 'reality.handshakePort',
        label: '握手目标端口',
        type: 'number',
        min: 1,
        max: 65535,
        description: '握手目标站点端口，通常为 443',
      },
    ],
  },
  {
    protocol: 'hysteria2',
    name: 'Hysteria2',
    desc: '基于 QUIC 的高速协议',
    recommended: true,
    tls: true,
    basic: [
      {
        field: 'obfsEnabled',
        label: '启用混淆',
        type: 'switch',
        description: 'salamander 混淆把 QUIC 流量伪装成普通 UDP，可降低被识别概率；密码为本入站所有用户共用，默认关闭',
      },
      {
        field: 'obfsPassword',
        label: '混淆密码',
        type: 'password',
        description: 'salamander 混淆密码，本入站所有用户共用；启用混淆时必填，需与客户端保持一致，建议用长随机串',
        when: (cfg) => !!cfg.obfsEnabled,
      },
    ],
    advanced: [
      {
        field: 'upMbps',
        label: '上行带宽上限 (Mbps)',
        type: 'number',
        min: 0,
        description: '留空（0）为不限制。不限带宽时客户端会使用 BBR（吞吐最好）；一旦设置带宽上限，客户端将被强制改用 Brutal',
      },
      {
        field: 'downMbps',
        label: '下行带宽上限 (Mbps)',
        type: 'number',
        min: 0,
        description: '留空（0）为不限制，含义同上行带宽',
      },
      {
        field: 'ignoreClientBandwidth',
        label: '忽略客户端带宽设置',
        type: 'switch',
        description: '开启后要求客户端放弃 Brutal、统一使用 BBR 拥塞控制（需带宽不限）',
      },
      {
        field: 'bbrProfile',
        label: 'BBR 档位',
        type: 'select',
        description: '内核 BBR 拥塞控制档位；不确定就用 standard，高带宽低丢包可试 aggressive',
        options: BBR_PROFILE_OPTIONS,
      },
    ],
  },
  {
    protocol: 'tuic',
    name: 'TUIC v5',
    desc: 'QUIC 低延迟协议',
    recommended: true,
    tls: true,
    basic: [],
    advanced: [
      {
        field: 'congestionControl',
        label: '拥塞控制',
        type: 'select',
        description: 'QUIC 拥塞控制算法；bbr 在高丢包链路下表现最好，为面板默认值',
        options: CC_OPTIONS,
      },
    ],
  },
  {
    protocol: 'anytls',
    name: 'AnyTLS',
    desc: 'TLS 流量复用协议',
    recommended: true,
    tls: true,
    basic: [],
    advanced: [],
  },
  {
    protocol: 'trojan',
    name: 'Trojan',
    desc: 'TLS 加密，需配置证书',
    recommended: false,
    tls: true,
    basic: [],
    advanced: [],
  },
  {
    protocol: 'vmess',
    name: 'VMess + WS + TLS',
    desc: 'WebSocket 传输，CDN 友好',
    recommended: false,
    tls: true,
    basic: [
      {
        field: 'wsPath',
        label: 'WS 路径',
        type: 'input',
        placeholder: '/',
        description: 'WebSocket 路径，需以 / 开头（如 /ws）；套 CDN 时需与转发规则保持一致',
      },
    ],
    advanced: [],
  },
  {
    protocol: 'shadowsocks',
    name: 'Shadowsocks-2022',
    desc: 'AEAD 2022 系列加密',
    recommended: false,
    tls: false,
    basic: [
      {
        field: 'method',
        label: '加密方法',
        type: 'select',
        description: 'Shadowsocks-2022 加密方法，决定各节点密钥长度；默认 128 位已足够安全且开销更低',
        options: SS_METHODS,
      },
    ],
    advanced: [],
  },
]

/** 按协议取定义 */
export function protocolDef(protocol: string): ProtocolDef | undefined {
  return PROTOCOL_DEFS.find((p) => p.protocol === protocol)
}

/** 证书下拉选项 */
export function certOptions(certs: Certificate[]): FieldOption[] {
  return certs.map((c) => ({ label: c.name, value: c.id }))
}

// ---------- 连接层默认值（凭据由后端为入站内每个用户独立生成） ----------

/** n 字节安全随机数的 hex 串 */
function randHex(n: number): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(n)), (b) => b.toString(16).padStart(2, '0')).join('')
}

// ---------- X25519（RFC 7748 ladder 实现，纯 BigInt；不依赖 crypto.subtle 的安全上下文） ----------

const P = (1n << 255n) - 19n
const A24 = 121665n

/** 小端字节 → BigInt */
function decodeLE(bytes: Uint8Array): bigint {
  let v = 0n
  for (let i = bytes.length - 1; i >= 0; i--) v = (v << 8n) | BigInt(bytes[i])
  return v
}

/** BigInt → 32 字节小端 */
function encodeLE(n: bigint): Uint8Array {
  const out = new Uint8Array(32)
  for (let i = 0; i < 32; i++) {
    out[i] = Number(n & 0xffn)
    n >>= 8n
  }
  return out
}

function mod(n: bigint): bigint {
  return ((n % P) + P) % P
}

/** 模幂 base^exp mod P（平方-乘） */
function modPow(base: bigint, exp: bigint): bigint {
  let result = 1n
  let b = mod(base)
  let e = exp
  while (e > 0n) {
    if (e & 1n) result = mod(result * b)
    b = mod(b * b)
    e >>= 1n
  }
  return result
}

function cswap(swap: bigint, a2: bigint, a3: bigint): [bigint, bigint] {
  return swap === 1n ? [a3, a2] : [a2, a3]
}

/** X25519 标量乘（已通过 RFC 7748 §5.2 测试向量与 WebCrypto 交叉验证） */
function x25519(k: Uint8Array, u: Uint8Array): Uint8Array {
  const kk = new Uint8Array(k)
  kk[0] &= 248
  kk[31] &= 127
  kk[31] |= 64
  const kInt = decodeLE(kk)
  const uu = new Uint8Array(u)
  uu[31] &= 127
  const x1 = decodeLE(uu) % P
  let x2 = 1n
  let z2 = 0n
  let x3 = x1
  let z3 = 1n
  let swap = 0n
  for (let t = 254; t >= 0; t--) {
    const kt = (kInt >> BigInt(t)) & 1n
    swap ^= kt
    ;[x2, x3] = cswap(swap, x2, x3)
    ;[z2, z3] = cswap(swap, z2, z3)
    swap = kt
    const a = mod(x2 + z2)
    const aa = mod(a * a)
    const b = mod(x2 - z2)
    const bb = mod(b * b)
    const e = mod(aa - bb)
    const c = mod(x3 + z3)
    const d = mod(x3 - z3)
    const da = mod(d * a)
    const cb = mod(c * b)
    x3 = mod(mod(da + cb) ** 2n)
    z3 = mod(x1 * mod(mod(da - cb) ** 2n))
    x2 = mod(aa * bb)
    z2 = mod(e * mod(aa + A24 * e))
  }
  ;[x2, x3] = cswap(swap, x2, x3)
  ;[z2, z3] = cswap(swap, z2, z3)
  return encodeLE(mod(x2 * modPow(z2, P - 2n)))
}

/** 生成 Reality x25519 密钥对（base64 raw url，与 sing-box 一致） */
function realityKeypair(): { privateKey: string; publicKey: string } {
  const privateKey = new Uint8Array(32)
  crypto.getRandomValues(privateKey)
  const base9 = new Uint8Array(32)
  base9[0] = 9
  const publicKey = x25519(privateKey, base9)
  const b64url = (bytes: Uint8Array) =>
    btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  return { privateKey: b64url(privateKey), publicKey: b64url(publicKey) }
}

/**
 * 生成协议连接层默认配置，不含任何用户凭据（凭据由后端为入站内每个用户生成）。
 * QUIC 类协议默认 ALPN 含 h3、拥塞算法默认 bbr；Shadowsocks 的服务端 PSK 由后端补齐，
 * Hysteria2 混淆默认关闭（混淆密码是入站级共享密钥，由使用者自行填写，不在前端随机生成）。
 */
export function defaultInboundConfig(protocol: string): Record<string, any> {
  const managedTLS = { enabled: true, certMode: 'managed' }
  switch (protocol) {
    case 'vless': {
      const keypair = realityKeypair()
      return {
        flow: 'xtls-rprx-vision',
        reality: {
          enabled: true,
          privateKey: keypair.privateKey,
          publicKey: keypair.publicKey,
          shortId: randHex(4),
          handshakeServer: 'www.microsoft.com',
          handshakePort: 443,
        },
        tls: { enabled: true, serverName: 'www.microsoft.com' },
      }
    }
    case 'hysteria2':
      return {
        obfsEnabled: false,
        obfsPassword: '',
        upMbps: 0,
        downMbps: 0,
        bbrProfile: 'standard',
        // QUIC 类协议 ALPN 必须含 h3：创建入站即给默认值，避免保存后握手失败
        tls: { enabled: true, certMode: 'managed', alpn: ['h3'] },
      }
    case 'tuic':
      return {
        congestionControl: 'bbr',
        tls: { enabled: true, certMode: 'managed', alpn: ['h3'] },
      }
    case 'anytls':
    case 'trojan':
      return { tls: { ...managedTLS } }
    case 'vmess':
      return { wsPath: '/', tls: { ...managedTLS } }
    case 'shadowsocks':
      return { method: '2022-blake3-aes-128-gcm' }
  }
  throw new Error(`不支持的入站协议: ${protocol}`)
}

// ---------- 弱配置警示（保存前在表单内联展示，口径与后端 InboundWarnings 一致） ----------

/** QUIC 类协议（ALPN 必须含 h3） */
function isQuicProtocol(protocol: string): boolean {
  return protocol === 'hysteria2' || protocol === 'tuic'
}

/** 计算弱配置警示；jump 指向表单内可修复该问题的区块 */
export function computeInboundWarnings(protocol: string, cfg: Record<string, any> = {}): InboundWarning[] {
  const warnings: InboundWarning[] = []
  const tls = cfg.tls ?? {}
  const certWarning = () => {
    const missing =
      !tls.certMode ||
      (tls.certMode === 'managed' && !tls.certId) ||
      (tls.certMode === 'path' && (!tls.certPath || !tls.keyPath))
    if (tls.enabled && missing) {
      warnings.push({ text: 'TLS 已启用但未配置证书，应用配置将失败', jump: 'tls' })
    }
  }
  /** QUIC 协议若显式配置了 ALPN 但不含 h3，握手会失败 */
  const alpnWarning = () => {
    const alpn: string[] = tls.alpn ?? []
    if (alpn.length > 0 && !alpn.includes('h3')) {
      warnings.push({ text: 'QUIC 类协议 ALPN 需含 h3，当前配置不含 h3' })
    }
  }
  switch (protocol) {
    case 'vless': {
      const reality = cfg.reality?.enabled
      if (!reality && !tls.enabled) {
        warnings.push({ text: '未启用 Reality 或 TLS，流量将明文传输', jump: 'tls' })
      } else if (!reality) {
        warnings.push({ text: '未启用 Reality（推荐）' })
        certWarning()
      } else if (!tls.enabled) {
        warnings.push({ text: 'Reality 已启用，将忽略 TLS 开关强制启用 TLS（无需证书）' })
      }
      break
    }
    case 'hysteria2':
      if (!tls.enabled) warnings.push({ text: 'Hysteria2 必须启用 TLS，当前已关闭', jump: 'tls' })
      else {
        certWarning()
        alpnWarning()
      }
      if (cfg.obfsEnabled && !cfg.obfsPassword) warnings.push({ text: '已启用混淆但混淆密码为空' })
      if ((cfg.upMbps ?? 0) > 0 && (cfg.downMbps ?? 0) > 0) {
        warnings.push({ text: '已设置带宽上限，客户端将无法使用 BBR（如需 BBR 请保持带宽不限）' })
      }
      break
    case 'tuic':
      if (!tls.enabled) warnings.push({ text: 'TUIC 必须启用 TLS，当前已关闭', jump: 'tls' })
      else {
        certWarning()
        alpnWarning()
      }
      break
    case 'anytls':
      if (!tls.enabled) warnings.push({ text: 'AnyTLS 必须启用 TLS，当前已关闭', jump: 'tls' })
      else certWarning()
      break
    case 'trojan':
      if (!tls.enabled) warnings.push({ text: 'Trojan 未启用 TLS，密码将明文传输', jump: 'tls' })
      else certWarning()
      break
    case 'vmess':
      if (!tls.enabled) warnings.push({ text: 'VMess 未启用 TLS，流量将明文传输', jump: 'tls' })
      else certWarning()
      break
    case 'shadowsocks': {
      if (!String(cfg.method ?? '').startsWith('2022-')) warnings.push({ text: '非 Shadowsocks-2022 加密方法，强度较弱' })
      break
    }
  }
  return warnings
}

/** 端口/节点相关提示：QUIC 协议需要 UDP 可达 */
export function protocolNotes(protocol: string): string[] {
  if (isQuicProtocol(protocol)) {
    return [
      'QUIC 类协议走 UDP：若前面有防火墙或安全组，需放行 UDP 端口',
      '若入站监听仅本机（走反向代理），UDP 通常无法被 Nginx 转发，建议直连公网端口',
    ]
  }
  return []
}
