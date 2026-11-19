# s-ui 后端模块功能文档

本文基于当前后端源码梳理各模块职责、核心类型/函数、依赖关系与配置项。

## 模块总览

| 模块 | 包路径 | 职责 |
|---|---|---|
| 配置管理 | `internal/config` | 加载面板配置（文件/环境变量），提供数据目录与数据库路径工具 |
| 数据库层 | `internal/db` | 基于 GORM + SQLite 的模型定义、迁移、CRUD、关联关系维护 |
| 会话管理 | `internal/session` | 基于 `scs` 的会话管理与 SQLite 会话存储 |
| HTTP API | `internal/api` | 路由注册、中间件、请求参数校验、JSON/SSE 响应 |
| 核心管理 | `internal/core` | sing-box 进程生命周期、配置生成/校验、订阅生成、统计同步、更新回滚 |
| 统计协议 | `internal/statsproto` | V2Ray Stats gRPC 协议（protobuf）定义与 Go 代码 |
| 启动入口 | `cmd/server` | 服务启动编排：配置、DB、会话、路由、定时任务、HTTP server |

---

## 配置管理（`internal/config`）

- **职责说明**
  - 加载运行配置（优先文件，支持环境变量覆盖）。
  - 在首次启动时自动生成默认配置文件与随机 `session_secret`。
  - 提供数据目录、数据库路径等通用辅助函数。
- **核心类型与函数**
  - `type Config`
    - `Addr`
    - `SessionSecret`
    - `DataDir`
    - `SingboxConfigPath`
    - `SingboxBinaryPath`
  - `LoadConfig() (*Config, error)`：加载配置（文件模式/环境变量模式）。
  - `EnsureDir(path string) error`：确保目录存在。
  - `DBPath(dataDir string) string`：返回 `s-ui.db` 文件路径。
- **依赖关系**
  - 仅依赖标准库（`os`、`filepath`、`json`、`crypto/rand`）。
  - 被 `cmd/server`、`internal/api`、`internal/core` 使用。
- **配置项**
  - `CONFIG_PATH`：配置文件路径（为空时走环境变量模式）。
  - `ADDR`：监听地址，默认 `:8080`。
  - `DATA_DIR`：数据目录，默认 `./data`。
  - `SINGBOX_CONFIG_PATH`：sing-box 配置文件路径。
  - `SINGBOX_BINARY_PATH`：sing-box 二进制路径。

## 数据库层（`internal/db`）

- **职责说明**
  - 初始化 SQLite 连接与自动迁移。
  - 管理 `Admin`、`Inbound`、`Certificate`、`User` 模型及关联。
  - 提供用户/入站/证书/管理员的业务查询与增删改。
  - 负责订阅 token 自动补齐与生成逻辑。
- **核心类型与函数**
  - 全局数据库句柄：`var DB *gorm.DB`
  - `Init(path string) error`：打开 SQLite、执行 `AutoMigrate`、补齐订阅 token。
  - 管理员：
    - `type Admin`
    - `HasAdmin()`
    - `CreateAdmin()`
    - `GetAdminByUsername()`
  - 入站：
    - `type Inbound`
    - `ListInbounds(sort string)`
    - `GetInboundByID()`
    - `GetInboundsByIDs()`
    - `CreateInbound()` / `UpdateInbound()` / `DeleteInbound()`
    - `GetInboundByTag()` / `InboundExistsByTag()`
  - 证书：
    - `type Certificate`
    - `ListCertificates()` / `GetCertificateByID()`
    - `CreateCertificate()` / `UpdateCertificate()` / `DeleteCertificate()`
    - `InboundsReferencingCert(certID uint)`
  - 用户：
    - `type User`
    - `GenerateSubscriptionToken()`
    - `ListUsers(keyword string)`
    - `GetUserByID()` / `GetUserByName()` / `GetUserBySubscriptionToken()`
    - `CreateUser()` / `UpdateUser()` / `DeleteUser()`
    - `ReplaceUserInbounds()`
    - `GetUsersForInbound(inboundID uint)`
- **依赖关系**
  - 依赖 `gorm.io/gorm`、`github.com/glebarez/sqlite`、`gorm.io/datatypes`。
  - 被 `internal/api` 与 `internal/core` 广泛依赖。
- **配置项**
  - 无独立环境变量，数据库路径由 `config.DBPath(cfg.DataDir)` 提供。

## 会话管理（`internal/session`）

- **职责说明**
  - 创建 `scs.SessionManager` 并绑定 SQLite 存储。
  - 管理会话 cookie 策略与生命周期。
  - 定期清理过期会话记录。
- **核心类型与函数**
  - 常量：`SessionKeyUserID = "user_id"`
  - `NewManager(db *gorm.DB, secure bool) (*scs.SessionManager, error)`
  - `type sqlStore`（`Find` / `Commit` / `Delete` / `startCleanup` / `deleteExpired`）
  - `NewSQLStore(db *sql.DB) *sqlStore`
- **依赖关系**
  - 依赖 `github.com/alexedwards/scs/v2` 和 `database/sql`。
  - 被 `cmd/server` 初始化，并在 `internal/api` 中通过 `SessionManager` 使用。
- **配置项**
  - `secure` 参数来自入口层 `FORCE_HTTPS` 环境变量判断结果。

## HTTP API（`internal/api`）

- **职责说明**
  - 注册所有路由（认证、核心控制、入站、用户、证书、统计、订阅）。
  - 提供认证中间件（`RequireAuth`、`RequireSetupMiddleware`）。
  - 负责请求解析、参数校验、错误映射与响应序列化（JSON/SSE）。
- **核心类型与函数**
  - 路由入口：
    - `Routes(staticFS fs.FS, sm *scs.SessionManager, cfg *config.Config) chi.Router`
    - `spaHandler(staticFS fs.FS) http.HandlerFunc`
  - 认证：
    - `SetupHandler` / `LoginHandler` / `MeHandler` / `LogoutHandler`
    - `RequireAuth`
    - `RequireSetupMiddleware`
  - 核心控制：
    - `StatusHandler` / `VersionsHandler`
    - `StartHandler` / `StopHandler` / `RestartHandler`
    - `LogsHandler`
    - `ConfigHandler` / `ConfigFileHandler`
    - `UpdateHandler` / `UpdateStreamHandler` / `RollbackHandler`
  - 入站管理：
    - `ListInboundsHandler` / `GetInboundHandler`
    - `CreateInboundHandler` / `UpdateInboundHandler` / `DeleteInboundHandler`
  - 用户管理：
    - `ListUsersHandler` / `GetUserHandler`
    - `CreateUserHandler` / `UpdateUserHandler` / `DeleteUserHandler`
    - `BatchUsersHandler`
    - `ResetSubscriptionHandler`
  - 证书管理：
    - `ListCertificatesHandler` / `GetCertificateHandler`
    - `CreateCertificateHandler` / `UpdateCertificateHandler` / `DeleteCertificateHandler`
  - 统计与订阅：
    - `StatsSummaryHandler`
    - `SubscriptionHandler`
- **依赖关系**
  - 上游依赖：`internal/config`、`internal/core`、`internal/db`、`scs`、`chi`。
  - 被 `cmd/server` 调用并挂载到 HTTP 服务。
- **配置项**
  - `SUB_URL_PREFIX`：用户订阅 URL 生成前缀（`/api/users` 响应中使用）。

## 核心管理（`internal/core`）

- **职责说明**
  - 生成并应用 sing-box 配置（先校验后原子替换）。
  - 管理 sing-box 进程（启动/停止/重启/状态机/错误语义化）。
  - 生成订阅内容（Base64 与 Clash YAML）。
  - 对接 V2Ray gRPC 统计并回写数据库。
  - 管理 sing-box 核心更新、进度广播与回滚。
- **核心类型与函数**
  - 配置应用：
    - `ApplyConfig(configPath string, configJSON []byte, pm *ProcessManager) error`
    - `type ConfigGenerator` + `Generate()`
  - 进程管理：
    - `type ProcessManager`
    - `NewProcessManagerFromConfig` / `NewProcessManagerWithBinary`
    - `Start` / `Stop` / `Restart` / `IsRunning` / `Check` / `Version`
    - `type ProcessError`、`type ProcessErrorCode`（语义化错误）
  - 生命周期状态：
    - `type CoreState`
    - `type LastFailureContext`
    - `type LifecycleSnapshot`
    - `ResolveCoreState` / `ActionMatrix`
  - 统计同步：
    - `type StatsClient`
    - `NewStatsClient`
    - `FetchAndPersist`
    - `Close`
  - 订阅生成：
    - `BuildUserinfoHeader`
    - `GetNodeLinks`
    - `GenerateBase64`
    - `GenerateClash`
  - 更新与进度：
    - `type CoreUpdater`
    - `ListReleases` / `Update` / `UpdateWithProgress` / `Rollback`
    - `type UpdateProgressState`
    - `GlobalUpdateProgressState` / `Begin` / `Publish` / `Finish` / `Subscribe` / `Snapshot`
- **依赖关系**
  - 依赖 `internal/db`、`internal/config`、`internal/statsproto`。
  - 依赖系统进程与网络（`exec`、GitHub API、gRPC）。
  - 被 `internal/api` 与 `cmd/server` 使用。
- **配置项**
  - `V2RAY_API_ENABLED`：启用配置生成中的 v2ray_api block（`true` 生效）。
  - `V2RAY_API_LISTEN`：v2ray API gRPC 监听地址，默认 `127.0.0.1:8080`。
  - `SINGBOX_BINARY_PATH`：更新与回滚目标二进制路径（为空则更新/回滚不可用）。

## 统计协议（`internal/statsproto`）

- **职责说明**
  - 提供与 V2Ray stats command 兼容的 protobuf 协议类型与 gRPC 客户端接口。
- **核心类型与函数**
  - 协议文件：`command.proto`
  - 关键消息：
    - `QueryStatsRequest`
    - `Stat`
    - `QueryStatsResponse`
  - 服务定义：
    - `service StatsService`
    - `rpc QueryStats(QueryStatsRequest) returns (QueryStatsResponse)`
  - 生成代码文件：
    - `command.pb.go`
    - `command_grpc.pb.go`
- **依赖关系**
  - 被 `internal/core/stats.go` 使用（`statsproto.StatsServiceClient`）。
- **配置项**
  - 无独立配置；地址由 `V2RAY_API_LISTEN` 在入口层提供。

## 启动入口（`cmd/server`）

- **职责说明**
  - 编排整体初始化流程并启动 HTTP 服务。
  - 在可选条件下启动统计定时任务。
  - 绑定会话中间件与 setup 重定向中间件。
- **核心类型与函数**
  - `main()`
    - `config.LoadConfig`
    - `db.Init`
    - `session.NewManager`
    - `api.Routes`
    - `sm.LoadAndSave(api.RequireSetupMiddleware(sm)(r))`
    - `http.ListenAndServe`
- **依赖关系**
  - 依赖 `internal/config`、`internal/db`、`internal/session`、`internal/api`、`internal/core`。
  - 依赖前端静态资源嵌入包 `web.FS`。
- **配置项**
  - `V2RAY_API_ENABLED`：是否启用统计抓取定时任务。
  - `V2RAY_API_LISTEN`：统计 gRPC 地址。
  - `V2RAY_STATS_INTERVAL`：统计抓取周期（秒），默认 60。
  - `FORCE_HTTPS`：会话 Cookie `Secure` 开关（`true/1` 生效）。

---

## 数据库模型与表结构

> 说明：`sessions` 为会话存储表，使用 `database/sql` 手工建表；其余由 GORM `AutoMigrate` 管理。

### `admins`

| 字段 | 类型/约束 | 说明 |
|---|---|---|
| `id` | `uint`, PK | 主键 |
| `username` | `string`, unique, size 50, not null | 管理员用户名 |
| `password_hash` | `string`, size 255, not null | bcrypt 哈希 |
| `created_at` | `time.Time` | 创建时间 |
| `deleted_at` | `gorm.DeletedAt`, indexed | 软删除时间 |

### `users`

| 字段 | 类型/约束 | 说明 |
|---|---|---|
| `id` | `uint`, PK | 主键 |
| `name` | `string`, size 100, not null | 用户名 |
| `remark` | `string`, size 255 | 备注 |
| `uuid` | `string`, size 36, unique | VLESS UUID |
| `password` | `string`, size 255 | Hysteria2 密码 |
| `subscription_token` | `string`, size 32, unique | 订阅 token（用于 `/sub/{token}`） |
| `traffic_limit` | `int64`, default 0 | 流量上限（字节，0 表示不限） |
| `traffic_used` | `int64`, default 0 | 已用流量（字节） |
| `traffic_uplink` | `int64`, default 0 | 上行流量（字节） |
| `traffic_downlink` | `int64`, default 0 | 下行流量（字节） |
| `expire_at` | `*time.Time` | 过期时间，`nil` 表示不过期 |
| `enabled` | `bool`, default true | 是否启用 |
| `created_at` | `time.Time` | 创建时间 |
| `updated_at` | `time.Time` | 更新时间 |

补充：用户与入站通过中间表 `user_inbounds` 建立多对多关系。

### `inbounds`

| 字段 | 类型/约束 | 说明 |
|---|---|---|
| `id` | `uint`, PK | 主键 |
| `tag` | `string`, unique, not null | 入站标识 |
| `protocol` | `string`, not null | 协议（`vless` / `hysteria2`） |
| `listen` | `string`, default `::` | 监听地址 |
| `listen_port` | `uint`, not null | 监听端口 |
| `config_json` | `datatypes.JSON`, text | 协议扩展配置 |
| `traffic_uplink` | `int64`, default 0 | 上行流量（字节） |
| `traffic_downlink` | `int64`, default 0 | 下行流量（字节） |
| `created_at` | `time.Time` | 创建时间 |
| `updated_at` | `time.Time` | 更新时间 |

### `certificates`

| 字段 | 类型/约束 | 说明 |
|---|---|---|
| `id` | `uint`, PK | 主键 |
| `name` | `string`, not null | 证书名称（展示用途） |
| `fullchain_path` | `string`, not null | fullchain 文件路径 |
| `privkey_path` | `string`, not null | 私钥文件路径 |
| `created_at` | `time.Time` | 创建时间 |
| `updated_at` | `time.Time` | 更新时间 |

### `sessions`

| 字段 | 类型/约束 | 说明 |
|---|---|---|
| `token` | `TEXT`, PK | 会话 token |
| `data` | `BLOB` | 会话序列化数据 |
| `expiry` | `REAL` | 过期时间（SQLite `julianday`） |

