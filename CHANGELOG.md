# Changelog

## 0.1.2 (2026-11-22)

### 修复

- 修复 Clash（mihomo）订阅在 TUN 模式及国内网络环境下因缺少 `fake-ip`、`proxy-server-nameserver` / `direct-nameserver`、远程加密 DNS（DoH / DoH3 / DoT）未走代理、规则集未经代理下载及 `ipcidr` 规则集缺少 `no-resolve` 导致客户端无法连通的问题
- 修复 sing-box 客户端订阅在启用远程加密 DNS 时默认域名解析器与规则集下载直连导致解析死锁或下载失败的问题
- 修复 Clash 客户端导入订阅时因 `Content-Disposition` 引号解析导致订阅名称异常显示的问题
- 修复已配置 AI 供应商后打开「AI 助手」首次误报「尚未配置可用的 AI 供应商」并要求手动刷新的问题；AI 弹框改为立即弹出并按需并发加载对话模块与配置
- 修复新建入站、订阅用户等弹框及侧栏页面切换需等待接口响应完成后才显示的问题，改为先展示弹框与页面再异步渲染数据并补齐加载状态与防闪烁处理
- 修复顶栏版本号重复显示前缀（如 `vv0.1.1`）的问题

## 0.1.1 (2026-11-22)

### 修复

- 修复 Clash（mihomo）订阅中 `dns.nameserver-policy` 被渲染为列表导致客户端校验报错 `pipeline must contain YAML mapping` 的问题，并将域名后缀规则改为 `+.` 匹配及过滤非 IP 类型的 `default-nameserver`
- 修复客户端导入订阅后名称显示为 `config.json` / `config.yaml` 的问题：改用订阅用户名称输出 `Content-Disposition` 与 `Profile-Title` 响应头，并附带 `Subscription-Userinfo` 流量与到期信息
- 修复使用自签 TLS 证书的入站在客户端订阅中未下发跳过证书校验参数（`insecure` / `skip-cert-verify`）导致 TLS 握手失败的问题
- 修复 VLESS + Reality 修改握手目标站点或省略握手端口时服务端与客户端 SNI 不一致、端口缺失的问题
- 修复无自定义出站时应用规则模板后客户端 `proxy` 策略组仅剩 `DIRECT`、未包含订阅节点的问题
- 修复 sing-box 客户端订阅缺少 TUN 入站与基础路由规则（嗅探、DNS 劫持、私有 IP 直连）以及默认下发旧版客户端不识别的 `bbr_profile` 导致配置不可用的问题
- 修复 Clash 订阅中 VMess 节点缺少必填的 `alterId` / `cipher` 字段及代理节点默认未开启 `udp` 的问题

## 0.1.0 (2026-11-21)

### 新增

- 新增单文件二进制发布（`linux/amd64`、`darwin/arm64`，禁 CGO），内嵌 Vue3 管理面板与 SQLite 持久化，提供 `-addr`、`-data`、`-db` 启动参数及 Linux systemd 安装脚本
- 新增内置 sing-box `1.14.4` 内核运行支持：随面板自动启动，任一配置保存即热应用，新配置校验或启动失败时自动回滚至上一可用配置并提示原因
- 新增七协议入站管理：支持 VLESS（Reality）、Hysteria2、TUIC v5、AnyTLS、Trojan、VMess 与 Shadowsocks-2022，端口留空自动分配随机空闲端口，支持 TLS、Reality、ECH、混淆、拥塞控制、多路复用、TCP Fast Open 与弱配置警示
- 新增出站、规则集、策略组与全局路由管理：支持按域名、IP 与规则集配置分流规则，规则集同时维护 sing-box（`.srs`）与 mihomo（`.mrs` / `.yaml`）源，提供兜底出站与预设模板
- 新增订阅用户、按用户流量统计与配额到期管理：提供 `/sub/:token` 匿名订阅端点与二维码，按 `User-Agent` 或 `?format=sing-box|clash` 返回 sing-box 或 Clash（mihomo）配置并支持面板内双格式预览；为订阅用户生成独立入站凭据并精确统计实时速率与累计流量，超配额或到期自动停用身份并使订阅返回 `403`
- 新增双端共用 DNS 配置：支持预设与自定义 DNS 服务器（UDP / TCP / DoT / DoH / DoH3 / 本地）与按域名分流规则，同时写入服务端配置并随订阅下发
- 新增 TLS 证书管理：支持一键自签（域名与 IP SAN）、上传或粘贴 PEM、引用服务器本地路径，展示到期倒计时并拦截删除被入站引用的证书
- 新增 AI 助手：支持配置多个 OpenAI 兼容供应商、拉取 `/models` 或手动维护模型及推理等级，提供流式对话与面板工具调用
- 新增面板在线自更新：从 GitHub Releases 查询版本列表，校验资产 SHA-256 摘要后原子替换当前二进制并同 PID 重启

### 安全

- 新增首次设置向导一次性设置码（仅输出至启动日志，无默认账号密码）、单管理员会话鉴权、修改密码自动吊销全部历史会话，以及登录与首次设置端点按 IP 失败限流
