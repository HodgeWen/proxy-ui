# Phase 8: Deployment & Production - Context

**Gathered:** 2026-02-12
**Status:** Ready for planning

<domain>
## Phase Boundary

用户可通过 Docker、bash 脚本或单二进制三种方式部署面板。HTTPS 由用户通过反向代理自行处理，面板本身只提供 HTTP 服务。面板监听端口可配置。

</domain>

<decisions>
## Implementation Decisions

### HTTPS 实现方式
- 面板本身不内置 TLS/ACME，HTTPS 通过反向代理（Nginx/Caddy）处理
- 面板默认 HTTP，监听端口可配置
- HTTP 跳转、证书管理等均由反向代理负责，面板不涉及
- 提供 Nginx/Caddy 反向代理配置示例（文档/README 级别）

### Docker 部署体验
- 单容器包含 s-ui + sing-box，不做多容器拆分
- 数据持久化使用 bind mount 挂载主机目录（非 named volume）
- 配置传入方式以配置文件为主，挂载 config.json
- 容器内 s-ui 与 sing-box 通过 localhost 通信

### 安装脚本行为
- 脚本只安装 s-ui 二进制，sing-box 由用户自行安装或通过面板内核管理下载
- 创建 systemd service（s-ui.service），开机自启
- 交互式安装：提示用户输入端口、管理员密码等
- 支持完整生命周期子命令：install / update / uninstall

### 单二进制发布
- 前端资源通过 Go embed 编译时嵌入到二进制中
- 只支持 Linux 平台：amd64 + arm64 两个架构
- 使用 GitHub Actions CI：push tag 自动触发构建和 Release 发布
- 首次运行时自动生成默认配置文件（无需手动创建 config.json）

### Claude's Discretion
- Dockerfile 基础镜像选择和多阶段构建细节
- 安装脚本中的错误处理和回退逻辑
- GitHub Actions workflow 的具体 step 设计
- docker-compose.yml 的端口映射和卷挂载具体路径
- 反向代理配置示例的具体内容

</decisions>

<specifics>
## Specific Ideas

- 安装脚本风格参考常见的 `curl | bash` 一键安装体验（类似 x-ui、3x-ui 等项目）
- Docker 部署追求 `docker compose up -d` 一条命令即可运行
- 二进制发布物命名规范：`s-ui-linux-amd64`、`s-ui-linux-arm64`

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-deployment-production*
*Context gathered: 2026-02-12*
