# proxy-ui

面向个人自用的 sing-box 代理面板：Go（后端）+ Vue3（前端）单二进制，管理入站/出站/全局规则，向各类客户端导出订阅，并内置 sing-box 内核下载、流量统计与 AI 助手。

- 单文件二进制（linux/amd64、darwin/arm64，禁 CGO），SQLite 落盘，无外部依赖
- 面板自身只起 HTTP，TLS 交给反向代理
- 面板与 sing-box 内核均支持面板内在线更新（GitHub Releases，可配镜像前缀）

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

1. 「设置 → 内核」：下载安装 sing-box 内核（网络受限时先配置下载镜像前缀），启动内核
2. 「入站」页创建入站（推荐协议：VLESS+Reality、Hysteria2、TUIC v5、AnyTLS；经典协议：Trojan、VMess+WS+TLS、Shadowsocks-2022、Direct）
3. 「订阅」页生成匿名 token 订阅链接，扫码或复制到客户端
4. 客户端按 User-Agent 自动适配格式：sing-box 客户端拿到 sing-box 配置，clash/mihomo 客户端拿到 clash(mihomo) 配置；也可用 `?target=sing-box|clash` 参数显式指定

## 在线更新

「设置 → 面板更新」：

- **面板**：检查新版本后一键更新，面板会下载替换自身二进制并自动重启（systemd 下进程同 PID 换入，服务不中断；数据保留）
- **内核**：一键更新到最新稳定版 sing-box（内核运行中需先停止）
- GitHub 访问受限时，在「设置 → 内核 → 下载镜像」配置镜像前缀（如 `https://ghproxy.example`），面板与内核的所有 GitHub 请求（版本查询与下载）均经该前缀代理

## 从源码构建

需要 Go 1.26+ 与 [bun](https://bun.sh)：

```bash
bash scripts/build.sh   # 前端构建 + linux/amd64、darwin/arm64 双平台交叉编译，产物在 bin/
```

版本号经 `git describe` 注入；开发调试：根目录 `go run .` + `cd web && bun run dev`（Vite 代理 API）。

## 许可证

个人自用，未附开源许可证。
