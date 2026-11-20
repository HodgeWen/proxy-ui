// 入站声明式表单定义：七协议元信息、字段 schema、连接层默认值与弱配置警示。
//
// 口径说明：入站是一个真实监听（协议 + 监听地址/端口 + TLS/Reality/混淆/带宽/拥塞算法），
// 不含任何用户凭据；用户凭据由后端为入站内每个用户独立生成，多用户共享同一监听端口。
// 默认值与警示口径和后端 internal/singbox（NewTemplateConfig / InboundWarnings）保持一致。
import type { Certificate } from './api/cert'

/** 表单字段类型 */
export type FieldType = 'input' | 'password' | 'number' | 'select' | 'multiselect' | 'switch' | 'textarea'

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
    description: '选择面板「证书管理」中已存储的证书，保存后自动写出对应 PEM',
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

/** TLS 版本选项（与 sing-box min_version / max_version 枚举一致） */
const TLS_VERSION_OPTIONS: FieldOption[] = [
  { label: '不限制（内核默认 1.2 起）', value: '' },
  { label: '1.0', value: '1.0' },
  { label: '1.1', value: '1.1' },
  { label: '1.2', value: '1.2' },
  { label: '1.3', value: '1.3' },
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
  {
    field: 'tls.minVersion',
    label: 'TLS 最低版本',
    type: 'select',
    description:
      '握手可协商的最低 TLS 版本，挡住降级到老版本。留空沿用内核默认（1.2）；要求更严可设 1.3，老客户端将无法连接',
    options: TLS_VERSION_OPTIONS,
  },
  {
    field: 'tls.maxVersion',
    label: 'TLS 最高版本',
    type: 'select',
    description: '握手可协商的最高 TLS 版本。留空不限制（推荐），仅在需要兼容特殊客户端下调低',
    options: TLS_VERSION_OPTIONS,
  },
  {
    field: 'tls.ech.enabled',
    label: '启用 ECH',
    type: 'switch',
    description:
      'ECH（Encrypted Client Hello）把 SNI 一并加密，避免中间人从握手看到访问域名。需域名已发布 ECH 配置且客户端支持，默认关闭',
  },
  {
    field: 'tls.ech.key',
    label: 'ECH 密钥',
    type: 'textarea',
    span: 'full',
    placeholder: '-----BEGIN ECH KEYS-----',
    description:
      'ECH 密钥 PEM 文本（BEGIN ECH KEYS）。可用内核自带命令由域名生成；与「ECH 密钥路径」二选一，填了密钥内核才真正启用 ECH',
    when: (cfg) => !!cfg.tls?.ech?.enabled,
  },
  {
    field: 'tls.ech.keyPath',
    label: 'ECH 密钥路径',
    type: 'input',
    span: 'full',
    placeholder: '服务器上 ECH 密钥文件绝对路径',
    description: '服务器上 ECH 密钥文件的绝对路径，适合密钥由外部工具轮换的场景；与内联密钥二选一',
    when: (cfg) => !!cfg.tls?.ech?.enabled,
  },
]

/** 监听层选项：TCP Fast Open 全协议可用 */
const TCP_FAST_OPEN_FIELD: FieldDef = {
  field: 'tcpFastOpen',
  label: 'TCP Fast Open',
  type: 'switch',
  description:
    '首个数据随 SYN 包发出，省掉一次往返握手。需服务端与客户端内核都开启，部分中间设备会丢弃带数据的 SYN；高延迟链路收益明显，默认关闭',
}

/** 多路复用字段：sing-box 仅消费 VLESS / Trojan / VMess / Shadowsocks 入站的 multiplex，
    其它协议（QUIC / AnyTLS 原生多路）仍保留开关，开启时由弱配置警示提示无效 */
function multiplexFields(effective: boolean): FieldDef[] {
  return [
    {
      field: 'multiplex.enabled',
      label: '多路复用',
      type: 'switch',
      description: effective
        ? '把多条连接复用进同一条 TCP 连接，减少反复握手；客户端也需开启同一选项，默认关闭（单连接场景无收益，长肥管道可能因队头阻塞变慢）'
        : '该协议本身已在协议层多路复用（QUIC / AnyTLS 单连接承载多流），内核不接受额外 multiplex 配置；此开关留作统一表单，开启会被内核忽略',
    },
    {
      field: 'multiplex.padding',
      label: '多路复用填充',
      type: 'switch',
      description: '仅在启用多路复用后生效：随机化复用流的长度，降低按包长识别协议的概率，代价是少量带宽开销',
      when: (cfg) => !!cfg.multiplex?.enabled,
    },
  ]
}

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

/** Shadowsocks 生效网络（留空 = TCP + UDP） */
const SS_NETWORK_OPTIONS: FieldOption[] = [
  { label: 'TCP + UDP（默认）', value: '' },
  { label: '仅 TCP', value: 'tcp' },
  { label: '仅 UDP', value: 'udp' },
]

/** Hysteria2 masquerade 类型（留空 = 不启用） */
const MASQUERADE_TYPE_OPTIONS: FieldOption[] = [
  { label: '不启用', value: '' },
  { label: '静态目录（file）', value: 'file' },
  { label: '反代站点（proxy）', value: 'proxy' },
  { label: '固定响应（string）', value: 'string' },
]

/** TUIC 心跳间隔（留空用内核默认 10s） */
const HEARTBEAT_OPTIONS: FieldOption[] = [
  { label: '内核默认（10s）', value: '' },
  { label: '5s', value: '5s' },
  { label: '10s', value: '10s' },
  { label: '30s', value: '30s' },
  { label: '60s', value: '60s' },
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
      TCP_FAST_OPEN_FIELD,
      ...multiplexFields(true),
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
        description:
          'salamander 混淆把 QUIC 流量伪装成普通 UDP，降低被识别概率；密码为本入站所有用户共用。新建入站默认开启并自动生成随机密码，客户端需填同一密码',
      },
      {
        field: 'obfsPassword',
        label: '混淆密码',
        type: 'password',
        description:
          'salamander 混淆密码，本入站所有用户共用；新建入站时已由面板生成随机值，编辑已有入站不会改动它，客户端需填同一密码',
        when: (cfg) => !!cfg.obfsEnabled,
      },
    ],
    advanced: [
      TCP_FAST_OPEN_FIELD,
      ...multiplexFields(false),
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
      {
        field: 'masquerade.type',
        label: '伪装站点',
        type: 'select',
        description:
          'Hysteria2 的 HTTP/3 masquerade：被主动探测时返回一个正常站点，避免直接暴露协议。默认不启用；有静态站点可指向目录，也可反代到已有网站',
        options: MASQUERADE_TYPE_OPTIONS,
      },
      {
        field: 'masquerade.directory',
        label: '静态目录',
        type: 'input',
        span: 'full',
        placeholder: '服务器上静态站点目录绝对路径',
        description: '该目录下的文件会作为 HTTP 响应返回；目录不存在时内核将无法启动',
        when: (cfg) => cfg.masquerade?.type === 'file',
      },
      {
        field: 'masquerade.proxyUrl',
        label: '反代目标 URL',
        type: 'input',
        span: 'full',
        placeholder: 'https://example.com',
        description: '探测请求被反代到该站点，响应原样返回；需以 http:// 或 https:// 开头',
        when: (cfg) => cfg.masquerade?.type === 'proxy',
      },
      {
        field: 'masquerade.statusCode',
        label: '响应状态码',
        type: 'number',
        min: 100,
        max: 599,
        description: '固定响应的 HTTP 状态码，留空按 200 处理',
        when: (cfg) => cfg.masquerade?.type === 'string',
      },
      {
        field: 'masquerade.content',
        label: '响应正文',
        type: 'textarea',
        span: 'full',
        placeholder: '<html><body>Not Found</body></html>',
        description: '固定返回的 HTML 正文，留空会返回空响应，探测方易判定异常',
        when: (cfg) => cfg.masquerade?.type === 'string',
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
      TCP_FAST_OPEN_FIELD,
      ...multiplexFields(false),
      {
        field: 'congestionControl',
        label: '拥塞控制',
        type: 'select',
        description: 'QUIC 拥塞控制算法；bbr 在高丢包链路下表现最好，为面板默认值',
        options: CC_OPTIONS,
      },
      {
        field: 'zeroRttHandshake',
        label: '0-RTT 握手',
        type: 'switch',
        description: '允许恢复会话时在首个包里带数据，重连快一个 RTT；代价是失去前向安全性，默认关闭',
      },
      {
        field: 'heartbeat',
        label: '心跳间隔',
        type: 'select',
        description: '服务端心跳包间隔，用于及时发现死连接；留空用内核默认 10s，移动网络差时可放宽',
        options: HEARTBEAT_OPTIONS,
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
    advanced: [TCP_FAST_OPEN_FIELD, ...multiplexFields(false)],
  },
  {
    protocol: 'trojan',
    name: 'Trojan',
    desc: 'TLS 加密，需配置证书',
    recommended: false,
    tls: true,
    basic: [],
    advanced: [TCP_FAST_OPEN_FIELD, ...multiplexFields(true)],
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
    advanced: [TCP_FAST_OPEN_FIELD, ...multiplexFields(true)],
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
      {
        field: 'network',
        label: '生效网络',
        type: 'select',
        description: '该入站允许的流量类型：留空为 TCP + UDP（常规），也可只开其一以收窄暴露面',
        options: SS_NETWORK_OPTIONS,
      },
    ],
    advanced: [TCP_FAST_OPEN_FIELD, ...multiplexFields(true)],
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

/** n 字节安全随机数的 base64 串（与后端 randBase64 口径一致，Hysteria2 默认混淆密码用） */
function randBase64(n: number): string {
  return btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(n))))
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
 * 口径与后端 NewTemplateConfig 一致：QUIC 类协议默认 ALPN 含 h3、TUIC 拥塞算法默认 bbr、
 * Hysteria2 默认开启混淆并随机生成密码、BBR 档位默认 standard、多路复用默认关闭；
 * Shadowsocks 的服务端 PSK 由后端补齐。
 */
export function defaultInboundConfig(protocol: string): Record<string, any> {
  const managedTLS = { enabled: true, certMode: 'managed' }
  /** 监听层选项默认值：TFO 与多路复用默认关闭，字段先占位，编辑时由已有记录覆盖 */
  const connect = { tcpFastOpen: false, multiplex: { enabled: false, padding: false } }
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
        tls: { enabled: true, serverName: 'www.microsoft.com', ech: { enabled: false } },
        ...connect,
      }
    }
    case 'hysteria2':
      return {
        obfsEnabled: true,
        // 新建入站即给随机混淆密码：与后端默认一致，避免「开了混淆却没密码」
        obfsPassword: randBase64(16),
        upMbps: 0,
        downMbps: 0,
        bbrProfile: 'standard',
        masquerade: { type: '' },
        // QUIC 类协议 ALPN 必须含 h3：创建入站即给默认值，避免保存后握手失败
        tls: { enabled: true, certMode: 'managed', alpn: ['h3'], ech: { enabled: false } },
        ...connect,
      }
    case 'tuic':
      return {
        congestionControl: 'bbr',
        zeroRttHandshake: false,
        heartbeat: '',
        tls: { enabled: true, certMode: 'managed', alpn: ['h3'], ech: { enabled: false } },
        ...connect,
      }
    case 'anytls':
    case 'trojan':
      return { tls: { ...managedTLS, ech: { enabled: false } }, ...connect }
    case 'vmess':
      return { wsPath: '/', tls: { ...managedTLS, ech: { enabled: false } }, ...connect }
    case 'shadowsocks':
      return { method: '2022-blake3-aes-128-gcm', network: '', ...connect }
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
      warnings.push({ text: 'TLS 已启用但未配置证书，保存后内核将无法重建', jump: 'tls' })
    }
  }
  /** QUIC 协议若显式配置了 ALPN 但不含 h3，握手会失败 */
  const alpnWarning = () => {
    const alpn: string[] = tls.alpn ?? []
    if (alpn.length > 0 && !alpn.includes('h3')) {
      warnings.push({ text: 'QUIC 类协议 ALPN 需含 h3，当前配置不含 h3' })
    }
  }
  /** TLS 版本与 ECH 的弱配置（与后端 InboundWarnings.advancedWarnings 同口径） */
  const advancedTlsWarning = () => {
    const versions = ['1.0', '1.1', '1.2', '1.3']
    if (tls.minVersion && !versions.includes(tls.minVersion)) {
      warnings.push({ text: `TLS 最低版本 ${tls.minVersion} 不是合法值（1.0/1.1/1.2/1.3）` })
    }
    if (tls.maxVersion && !versions.includes(tls.maxVersion)) {
      warnings.push({ text: `TLS 最高版本 ${tls.maxVersion} 不是合法值（1.0/1.1/1.2/1.3）` })
    }
    if (tls.minVersion && tls.maxVersion && tls.minVersion > tls.maxVersion) {
      warnings.push({ text: 'TLS 最低版本高于最高版本，握手将失败' })
    }
    if (tls.ech?.enabled && !tls.ech.key && !tls.ech.keyPath) {
      warnings.push({ text: '已启用 ECH 但未提供 ECH 密钥或密钥路径，内核将忽略该配置' })
    }
  }
  /** 多路复用只在 sing-box 的 VLESS / Trojan / VMess / Shadowsocks 入站生效 */
  const multiplexWarning = () => {
    const supported = ['vless', 'trojan', 'vmess', 'shadowsocks']
    if (cfg.multiplex?.enabled && !supported.includes(protocol)) {
      warnings.push({ text: `${protocol} 入站不支持多路复用（内核会忽略该设置），QUIC/AnyTLS 自身已多路复用` })
    }
    if (cfg.tcpFastOpen && isQuicProtocol(protocol)) {
      warnings.push({ text: `${protocol} 走 QUIC（UDP），TCP Fast Open 对其无效，可关闭` })
    }
  }
  /** Hysteria2 masquerade 弱配置 */
  const masqueradeWarning = () => {
    const m = cfg.masquerade ?? {}
    if (m.type === 'file' && !m.directory) warnings.push({ text: 'masquerade 类型为静态目录但未填目录，内核将无法启动' })
    if (m.type === 'proxy' && !m.proxyUrl) warnings.push({ text: 'masquerade 类型为反代站点但未填反代 URL，内核将无法启动' })
    if (m.type === 'proxy' && m.proxyUrl && !/^https?:\/\//.test(m.proxyUrl)) {
      warnings.push({ text: 'masquerade 反代 URL 需以 http:// 或 https:// 开头' })
    }
    if (m.type === 'string' && !m.content) warnings.push({ text: 'masquerade 类型为固定响应但正文为空，将返回空响应' })
  }
  multiplexWarning()
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
      if (reality && tls.ech?.enabled) {
        warnings.push({ text: 'Reality 已启用，ECH 不会写入内核配置（内核不允许两者并存），如需 ECH 请关闭 Reality' })
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
      masqueradeWarning()
      break
    case 'tuic':
      if (!tls.enabled) warnings.push({ text: 'TUIC 必须启用 TLS，当前已关闭', jump: 'tls' })
      else {
        certWarning()
        alpnWarning()
      }
      if (cfg.zeroRttHandshake && tls.enabled) {
        warnings.push({ text: '已启用 0-RTT 握手：可省一个 RTT，但会牺牲前向安全性' })
      }
      if (cfg.heartbeat && !/^\d+(\.\d+)?(ns|us|µs|ms|s|m|h)$/.test(cfg.heartbeat)) {
        warnings.push({ text: `心跳间隔 ${cfg.heartbeat} 不是合法时长（如 10s / 30s）` })
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
      if (cfg.network && cfg.network !== 'tcp' && cfg.network !== 'udp') {
        warnings.push({ text: 'Shadowsocks 网络只能留空（TCP + UDP）、tcp 或 udp' })
      }
      break
    }
  }
  if (protocol !== 'shadowsocks') advancedTlsWarning()
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
