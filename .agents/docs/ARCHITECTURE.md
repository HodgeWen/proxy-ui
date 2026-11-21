# 架构

## 业务架构

面向个人自用的代理面板：sing-box 内核**以内置库形式编进面板**，管理入站、出站、规则集与全局路由规则、DNS 与系统优化，入站用户由订阅用户勾选生成，并按订阅用户分发节点。无套餐、计费、多租户概念，单管理员使用。

核心域与流程：

1. **内核内置且随面板自运行**：`github.com/sagernet/sing-box v1.14.4` 编进面板进程（不再下载外部二进制），带 `with_v2ray_api` 标签，因此可按**用户**统计流量。面板启动即拉起内核（无可用入站时以最小配置运行）；此后面板内**任何写操作保存即热应用**——聚合配置 → 校验 → 重建实例，失败自动回滚上一份可用配置并在该次保存处报错。「应用配置」「启动 / 停止 / 重启」概念与接口整体删除。详见 `.agents/docs/SPEC-core-embedded.md`
2. **入站与用户**：入站是真实监听（协议 + 监听地址 + 端口 + 连接层参数：TLS/Reality/ECH/混淆/masquerade/带宽/拥塞算法/多路复用/TCP Fast Open）；端口默认随机空闲端口（**不默认 443**）。字段与 sing-box `option` 包一一对应；单字段说明留在 `web/src/inbound-schema.ts`，默认值经 `GET /api/inbounds/template/:protocol` 取自后端单一来源。入站内可挂多个用户，各自凭据、**共享入站端口**
3. **订阅用户**：每用户独立匿名 token、流量配额与到期；勾选某入站即在该入站内创建该人的身份。订阅链接走面板同源子路径 `/sub/:token`（按客户端访问地址自适应）+ 二维码，按客户端返回 sing-box 或 clash/mihomo 格式（含 DNS 段）；管理端可按订阅用户预览双格式真实导出（忽略配额与到期，与匿名端点共用同一组装与渲染）
4. **配额执行**：达到配额或到期 → 该用户在各入站的身份被停用并从内核配置移除（配额核对循环发现状态变化时走同一条热应用路径），订阅端点 403
5. **规则体系（两层）**：规则 = 匹配对象（域名 / IP / 规则集）× 匹配方式（精确 / 后缀 / 关键字 / 正则 / 网段）；规则集 = 一个 tag + 多条内容项，每条内容携带 sing-box(.srs) 与 mihomo(.mrs/.yaml) 两个源，同一规则集只启用一条
6. **DNS**：一份配置双端生效——写入服务端 `dns` 段与 `route.default_domain_resolver`，同时随订阅下发（sing-box `dns` 段、mihomo `dns:` 段）
7. **面板更新**：面板自身二进制从 GitHub Releases 更新（二进制替换 + 同 PID 重启）；内核随面板版本走
8. **AI 助手**：设置页配置多供应商 → 每供应商多模型 → 每模型多推理等级（模型 id 跨供应商全局唯一）；模型可从供应商 `/models` 拉取导入或手工添加；对话按供应商代理（OpenAI 兼容 SSE），前端经 `GET /api/ai/transport` 元数据喂 `@veltra/ai` 的 `createOpenAITransport`；AI 能以工具（function calling）方式调用面板接口
9. **证书**：一级导航「证书」页统一管理 TLS 证书——一键自签（域名 + 有效期）、上传/粘贴 PEM（面板存储 managed）、引用服务器路径（path）三种来源；列表解析并展示域名、到期时间与剩余天数，入站 TLS 可直接选引用，被引用的证书删除被拒；不做 ACME

参考形态：alireza0/s-ui（入站内多用户 + 每用户一条订阅 + 按用户名统计）。

## 技术架构

单进程架构：一个 Go 二进制同时承载内核实例、嵌入的 Vue SPA、REST API 与订阅端点。

- **web（Vue3 SPA）**：`@veltra/desktop`（ultra-ui）组件 + `@veltra/ai`（AI 助手对话组件），构建产物经 `go:embed` 打进二进制
- **api（Gin）**：REST + JSON 统一响应；会话鉴权（动态 Secure Cookie、改密吊销全部会话），首次设置一次性设置码（启动日志下发）+ 登录/设置按 IP 失败限流；单管理员
- **subscription**：匿名 token 鉴权的 GET 订阅端点，按 User-Agent/查询参数输出 sing-box / clash(mihomo) 配置；另供 api 以订阅用户 ID 做面板内预览（跳过停用/配额/到期校验，共用同一组装与渲染）
- **singbox**：配置生成（入站+多用户/出站/路由/规则集/DNS）、**进程内内核实例生命周期**（`box.New`/`Start`/`Close`；`Apply` 配置未变则跳过重建、失败回滚 `lastGood`、统计监听端口按实例动态分配）、按用户流量采集（v2ray 统计 gRPC；差分内存聚合、每 5 秒单事务批量落库，失败重试不丢数，flush 后对有流量 token 即时配额核对）
- **update**：面板自身的 GitHub Releases 版本查询、下载校验与二进制替换重启
- **ai**：多供应商与多模型配置存取（key 不回显、模型 id 跨供应商全局唯一）、上游 `/models` 拉取、`@veltra/ai` transport 元数据构造、按供应商路由的 OpenAI 兼容 SSE 对话代理与工具注册表
- **db**：SQLite 单文件（glebarez 纯 Go 驱动 + GORM，连接带 busy_timeout 与 WAL），存面板配置、账号、订阅用户/节点绑定与累计流量；启动时按 `PRAGMA user_version` 校验结构版本，旧 schema 库先自动备份为 `<db>.bak-<旧版本>` 再整体删除重建为空库（不做旧数据迁移）

> **为什么内核必须内置**：官方 Release 二进制未编入 `with_v2ray_api`，拿不到按用户流量；Clash API 的连接元数据也没有用户字段。只有把内核作为库编进来并启用该标签，共享入站下的按用户配额才成立。

架构形态与 `PROJECT.md` 一致：前后端分离（独立前端 + REST API），部署为单二进制。

### 技术栈

| 层 | 选型 | 备注 |
| --- | --- | --- |
| 语言 / runtime | Go 1.26；TypeScript（Node 26 + bun） | 后端 Go，前端 TS |
| 内核 | `github.com/sagernet/sing-box v1.14.4`（库内嵌） | 构建标签固定，见 Makefile |
| 框架 | Gin；Vue3 + Vue Router + Pinia + `@veltra/desktop` + `@veltra/ai` | ultra-ui npm 包名为 `@veltra/*` |
| 数据 | SQLite（glebarez/sqlite + GORM） | 纯 Go 无 CGO |
| 构建 / 包管理 | go modules；bun + Vite | 前端 dist 经 go:embed 嵌入 |
| 测试 | go test（需带内核构建标签） | `make test` |
| 部署 | 单二进制 + GitHub Releases + 面板自更新 | 交叉编译，禁 CGO |

## 未决

- CI 已接入（`.github/workflows/ci.yml`：前端构建 + `make test` + 双平台编译；`scripts/build.sh` 产出 checksums.txt、dirty 树默认拒绝构建）；发版 job 与变更日志仍交由 `release` 技能接入时定
