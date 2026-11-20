# proxy-ui

面向个人自用的 sing-box 代理面板：Go（后端）+ Vue3（前端）单二进制，**内核内置**（sing-box 1.14.4 编进面板，无需下载），管理入站与入站用户、出站、规则集与全局规则，按订阅用户分发独立节点并支持流量配额与到期，内置 DNS 配置、BBR 一键优化、按用户流量统计与 AI 助手。

- 单文件二进制（linux/amd64、darwin/arm64，禁 CGO），SQLite 落盘，无外部依赖
- 面板自身只起 HTTP，TLS 交给反向代理
- 内核与面板同进程，面板自身支持在线更新

## 下载

从 [GitHub Releases](https://github.com/HodgeWen/proxy-ui/releases) 下载对应平台产物：

```bash
# linux/amd64
curl -LO https://github.com/HodgeWen/proxy-ui/releases/latest/download/proxy-ui-linux-amd64
# darwin/arm64 (Apple Silicon)
curl -LO https://github.com/HodgeWen/proxy-ui/releases/latest/download/proxy-ui-darwin-arm64
```

## 安装（linux systemd）

```bash
chmod +x proxy-ui-linux-amd64
sudo ./scripts/install.sh install ./proxy-ui-linux-amd64   # 安装、启动并设置开机自启
sudo ./scripts/install.sh status                            # 查看服务状态
sudo ./scripts/install.sh uninstall                         # 卸载（默认保留数据目录）
```

默认监听 `:8080`，数据目录 `/var/lib/proxy-ui`（数据库、内核、证书等），可用环境变量覆盖：`PORT=9000 DATA_DIR=/srv/proxy-ui sudo -E ./scripts/install.sh install ./proxy-ui-linux-amd64`。升级时重新执行 `install` 即可（服务会自动重启加载新二进制）。

手动运行（macOS / 临时调试，无需 systemd）：

```bash
./proxy-ui-darwin-arm64 -addr :8080 -data ./data
```

启动参数：`-addr` 监听地址（默认 `:8080`）、`-data` 数据目录（默认 `data`）、`-db` 数据库路径（默认 `<data>/panel.db`）。

## 反向代理 TLS

面板只监听 HTTP，HTTPS 建议交由反代终止。Nginx 示例：

```nginx
server {
    listen 443 ssl;
    server_name panel.example.com;

    ssl_certificate     /etc/letsencrypt/live/panel.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/panel.example.com/privkey.pem;

    # 订阅端点走流式/纯文本响应，关闭代理缓冲
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
    }
}
```

Caddy 示例（自动签发证书）：

```caddy
panel.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

## 首次向导

1. 浏览器打开 `https://panel.example.com`（首次访问自动进入设置向导）
2. 创建管理员账号与密码（无任何默认账号口令；单管理员）
3. 登录后进入仪表盘

## 客户端订阅

1. 「入站」页创建入站：选协议、监听地址与端口（端口留空自动分配随机空闲端口），配置 TLS/证书
2. 「订阅」页新建订阅用户，勾选要下发的入站 —— 面板会在该入站内为此人创建一个独立身份（用户名 `订阅备注-入站名`），各用户共享入站端口但凭据互不相同
3. 点该用户的「查看订阅」拿链接或扫码；客户端按 User-Agent 自动适配格式：sing-box 客户端拿到 sing-box 配置，clash/mihomo 客户端拿到 clash(mihomo) 配置；也可用 `?format=sing-box|clash` 显式指定。行内「预览」可在 sing-box / mihomo 之间切换查看该用户将拿到的真实配置（忽略配额与到期，只要有启用中的节点）
4. 任何改动保存后立即生效：面板启动时内核即自动运行，之后每次保存入站 / 用户 / 出站 / 规则 / DNS / 证书都会按新配置重建内核；新配置起不来时会自动回到上一份可用配置，并在保存处给出失败原因（数据已保存，修正后再保存即可）

入站下也可以手动建用户（不绑定订阅、无订阅链接），适合临时给某人单独凭据。

### 为什么按用户生成身份
配额与到期要能真正生效，就必须能按用户统计流量。面板把 sing-box 作为库编进进程并启用 `v2ray_api`，读取 `user>>><用户名>>>traffic` 计数器，因此每个用户都能精确计费；共享入站端口不会让统计混淆。

### 流量配额与到期
订阅用户可设流量配额（上+下行）与到期时间。达到配额或到期后，该用户在所有入站里的身份会被暂停并从内核配置移除，订阅端点返回 403；「清零流量」或延长到期后自动恢复。

## DNS

「设置 → DNS」可配置服务端与订阅共用的 DNS：选择预设（国内外分流 / 全 DoH / 本地）或自定义服务器（UDP/TCP/DoT/DoH/DoH3/本地）与按域名的分流规则。启用后同时写入服务端配置的 `dns` 段与 `route.default_domain_resolver`，并随订阅下发到 sing-box 客户端；mihomo 客户端拿到等价的 `dns:` 段。

## BBR 加速

「设置 → 系统优化」展示当前 TCP 拥塞控制算法。在 Linux 且面板以 root 运行时，可一键写入 `/etc/sysctl.d/99-proxy-ui-bbr.conf` 并执行 `sysctl --system` 开启 BBR；其它环境提供可复制的等价命令。

协议层拥塞算法按协议给最佳默认：Hysteria2 用 `bbr_profile=standard`、TUIC 用 `bbr`；TCP 类协议（VLESS/Trojan/VMess/Shadowsocks）走系统 TCP 拥塞控制，因此 BBR 对它们同样有效。

## AI 助手

顶栏「AI 助手」按钮打开接近全屏的对话弹框（标题栏可最大化铺满视口）：对话、工具调用确认与供应商 / 模型 / 推理等级切换都在弹框内完成；关闭弹框即重置当前对话，下次打开从新会话开始。

供应商在「设置 → AI 供应商」维护，支持任意 OpenAI 兼容供应商（OpenAI、DeepSeek、Moonshot、本地 vLLM 等）：新增供应商（base-url + API Key，Key 只存服务端、对话经面板代理转发）后，可从供应商 `/models` 接口拉取并多选导入模型，也可手工添加（如 `gpt-6.2`、`deepseek-v4.1-flash`）；每个模型可维护推理等级列表与默认等级。

## 内核

sing-box **1.14.4 已编进面板**，无需下载也无法更换版本；升级内核随面板版本一起发布。内核随面板自动运行，配置保存即热应用，无需手动启停；「设置 → 内核」只读查看内核状态与日志。

> 构建时必须带 `with_v2ray_api` 等标签（`scripts/build.sh` 与 `Makefile` 已固定），否则按用户流量统计失效、配额形同虚设。

## 面板更新

「设置 → 面板更新」检查新版本后一键更新：面板下载替换自身二进制并自动重启（systemd 下进程同 PID 换入，服务不中断；数据保留）。

## 从源码构建

需要 Go 1.26+ 与 [bun](https://bun.sh)：

```bash
bash scripts/build.sh   # 前端构建 + linux/amd64、darwin/arm64 双平台交叉编译（含内置内核），产物在 bin/
```

版本号经 `git describe` 注入；开发调试用 Makefile：`make dev` 一条命令同起前后端（Vite 热更新，访问 http://localhost:5173，开发数据在 `.devdata/`，`make help` 看全部命令）。注意 `make dev` / `make test` 已内置必需的内核构建标签；若直接 `go run .` 或 `go build`，需自行加 `-tags "with_v2ray_api,with_quic,with_utls,with_acme,with_clash_api,with_gvisor"`。

## 许可证

个人自用，未附开源许可证。
