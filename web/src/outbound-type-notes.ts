// 出站类型速查数据：常见出站类型的适用场景与附加配置要点，供出站表单的类型提示、
// 必填字段提示、附加字段样例与「类型速查」表共用（消费方：OutboundsView）。
//
// 口径说明：字段名与 sing-box 1.14 出站选项一致；required 为「无」表示只填服务器地址与端口即可用。
export const TYPE_NOTES: Record<string, { scene: string; required: string; sample?: Record<string, unknown> }> = {
  shadowsocks: {
    scene: '最通用简单的选择，客户端支持最广；用 2022 系列加密时性能与抗探测都不错，拿不准就选它',
    required: 'method（加密方式）、password',
    sample: { method: '2022-blake3-aes-128-gcm', password: '<随机生成的密钥>' },
  },
  vless: {
    scene: '轻量现代协议，无额外加密开销；配合 Reality / Vision 抗封锁能力最强，新建节点首选',
    required: 'uuid',
    sample: { uuid: '<服务端分配的 UUID>', flow: 'xtls-rprx-vision' },
  },
  vmess: {
    scene: 'V2Ray 经典协议，老牌生态兼容好；新部署更建议 vless',
    required: 'uuid',
    sample: { uuid: '<服务端分配的 UUID>' },
  },
  trojan: {
    scene: '伪装成普通 HTTPS 的老牌协议，配置简单，依赖服务端的 TLS 证书',
    required: 'password',
    sample: { password: '<服务端密码>' },
  },
  hysteria2: {
    scene: '基于 QUIC 的高速协议，在丢包多、线路差的场景下速度优势明显',
    required: 'password',
    sample: { password: '<服务端密码>' },
  },
  tuic: {
    scene: '基于 QUIC 的低延迟协议，握手快，适合手机等弱网设备',
    required: 'uuid、password',
    sample: { uuid: '<服务端 UUID>', password: '<服务端密码>', congestion_control: 'bbr', udp_relay_mode: 'native' },
  },
  anytls: {
    scene: '专为代理设计的新协议（2025），流量特征规整、连接复用高效',
    required: 'password',
    sample: { password: '<服务端密码>' },
  },
  socks: {
    scene: '通用 SOCKS5 代理，常用于对接本机或内网的其它代理服务',
    required: '无（账号密码按服务端配置）',
    sample: { version: '5' },
  },
  http: {
    scene: 'HTTP 明文代理，仅建议在可信内网使用',
    required: '无',
  },
  wireguard: {
    scene: 'WireGuard 隧道；对端地址与端口写在 peers 里，上方「服务器 / 服务器端口」可留空',
    required: 'private_key、address、peers（含 public_key）',
    sample: {
      private_key: '<本机私钥>',
      address: ['172.16.0.2/30'],
      peers: [{ address: '<对端地址>', port: 51820, public_key: '<对端公钥>' }],
    },
  },
  hysteria: {
    scene: 'hysteria2 的上一代协议，官方已停止演进，新建建议改用 hysteria2',
    required: 'auth_str、up_mbps、down_mbps',
  },
  ssh: {
    scene: 'SSH 隧道出站，直接复用已有服务器的 SSH 服务转发流量，无需另外部署代理',
    required: 'user',
    sample: { user: 'root' },
  },
}

/** 常见出站类型（下拉选项）；可输入创建其它类型 */
export const typeOptions = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'socks', 'http', 'wireguard', 'hysteria', 'ssh'].map(
  (t) => ({ label: t, value: t }),
)

/** 新建出站的默认类型：最常见通用的推荐项（direct 是内置直连，不进自定义表单） */
export const DEFAULT_TYPE = 'shadowsocks'
