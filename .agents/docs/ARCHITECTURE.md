# 架构

## 业务架构

面向个人自用的代理面板：以 sing-box 为唯一内核，管理入站（服务端协议监听）与出站配置、全局路由规则，并向各类客户端导出订阅。无套餐、计费、多租户概念，单管理员使用。

核心域与流程：

1. **内核管理**：面板生成 sing-box 配置、拉起/停止内核子进程、展示运行状态与流量
2. **配置管理**：入站/出站/全局规则的 CRUD；推荐协议排前、默认配置预填、高级项默认折叠、弱安全协议提示、前置依赖（如 TLS）快捷跳转
3. **订阅导出**：以订阅用户组织（每用户独立匿名 token，可停用/重置/删除），订阅链接走面板同源子路径 `/sub/:token`（按客户端访问地址自适应，无需配置服务器地址）+ 二维码；按客户端返回 sing-box 或 clash/mihomo 格式；订阅内含故障转移、固定代理等策略组与广告拦截（anti-AD 规则源，客户端自动下载更新，服务端同份规则双端生效）
4. **在线更新**：面板自身二进制与 sing-box 内核均在面板内点击更新（GitHub Releases）
5. **AI 助手**：供应商/模型/api-key/推理强度在设置中配置；AI 能以工具（function calling）方式调用面板全部接口，辅助配置与解惑

参考形态：alireza0/s-ui（核心功能）、mllt992/qing-zhou（订阅规则生成）。

## 技术架构

单进程架构：一个 Go 二进制同时提供嵌入的 Vue SPA、REST API、订阅端点，并以子进程方式管理 sing-box 内核。

- **web（Vue3 SPA）**：`@veltra/desktop`（ultra-ui）组件 + `@veltra/ai`（AI 助手对话组件），构建产物经 `go:embed` 打进二进制
- **api（Gin）**：REST + JSON 统一响应；会话鉴权，单管理员
- **subscription**：匿名 token 鉴权的 GET 订阅端点，按 User-Agent/查询参数输出 sing-box / clash(mihomo) 配置（二维码由前端 etiket 生成）
- **singbox / update / ai**：业务模块，被 api 编排；singbox 负责配置生成、子进程生命周期与流量/连接统计采集（Clash API 实时采样 + 按入站累计落库）；update 负责内核与面板自身的 GitHub Releases 版本查询、下载与安装（面板更新含二进制替换与同 PID 重启）；ai 负责供应商配置存取（key 不回显）、OpenAI 兼容 SSE 对话代理与工具注册表（面板工具由 api 注册，执行回落业务模块）
- **db**：SQLite 单文件（glebarez 纯 Go 驱动 + GORM），存面板配置、账号与按入站累计流量；启动时自动迁移

架构形态与 `PROJECT.md` 一致：前后端分离（独立前端 + REST API），部署为单二进制。

### 技术栈

| 层 | 选型 | 备注 |
| --- | --- | --- |
| 语言 / runtime | Go 1.26；TypeScript（Node 26 + bun） | 后端 Go，前端 TS |
| 框架 | Gin；Vue3 + Vue Router + Pinia + `@veltra/desktop` + `@veltra/ai` | ultra-ui npm 包名为 `@veltra/*` |
| 数据 | SQLite（glebarez/sqlite + GORM） | 纯 Go 无 CGO |
| 构建 / 包管理 | go modules；bun + Vite | 前端 dist 经 go:embed 嵌入 |
| 测试 | go test | 核心逻辑模块必须；前端暂不强制 |
| 部署 | 单二进制 + GitHub Releases + 面板内在线更新 | 交叉编译，禁 CGO |

## 未决

- 发版流水线（GitHub Actions / goreleaser / 变更日志）交由 `release` 技能接入时定
