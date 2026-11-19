# s-ui HTTP API 文档

本文覆盖 `internal/api/routes.go` 中注册的全部后端端点，按功能域分组描述请求与响应规格。

## 通用说明

- Base URL: `/`
- 会话认证：基于 Cookie（`scs`），需要登录的接口在下方标注为“需登录”。
- 响应类型：
  - JSON 接口默认 `Content-Type: application/json`
  - 订阅接口返回 `text/plain; charset=utf-8`
  - SSE 接口返回 `text/event-stream`
- 错误响应：
  - 大多数接口使用 `http.Error(...)`，返回纯文本错误消息。
  - 部分核心接口返回结构化 JSON 错误（见对应接口说明）。

---

## 认证域（Auth）

### `GET /api/health`

- **认证要求**：公开
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - Body: `ok`（纯文本）
- **错误响应**：无业务错误分支

### `GET /api/me`

- **认证要求**：需登录（通过会话存在性判断）
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"ok":true}`
- **错误响应**
  - `401 Unauthorized`：`unauthorized`

### `POST /api/setup`

- **认证要求**：公开（仅首次无管理员时可用）
- **请求参数（JSON Body）**
  - `username: string`（长度 3-50）
  - `password: string`（最少 8 位）
  - `confirm: string`（需与 `password` 一致）
- **成功响应**
  - `200 OK`
  - `{"ok":"true"}`
- **错误响应**
  - `400 Bad Request`
    - `invalid request`
    - `admin already exists`
    - `username must be 3-50 characters`
    - `password must be at least 8 characters`
    - `password and confirm must match`
  - `405 Method Not Allowed`：`method not allowed`
  - `500 Internal Server Error`：`server error`

### `POST /api/login`

- **认证要求**：公开
- **请求参数（JSON Body）**
  - `username: string`
  - `password: string`
  - `remember: bool`（当前后端未使用该字段控制会话行为）
- **成功响应**
  - `200 OK`
  - `{"ok":"true"}`
- **错误响应**
  - `400 Bad Request`：`invalid request`
  - `401 Unauthorized`：`用户名或密码错误`
  - `405 Method Not Allowed`：`method not allowed`
  - `500 Internal Server Error`：`server error`

### `POST /api/logout`

- **认证要求**：公开（即使未登录也会销毁当前会话上下文）
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"ok":"true"}`
- **错误响应**
  - `405 Method Not Allowed`：`method not allowed`

---

## 核心管理域（Core）

> 以下接口全部挂载在 `/api/core/*` 且统一使用 `RequireAuth`，均为“需登录”。

### `GET /api/core/status`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - JSON:
    - `running: bool`
    - `state: "not_installed" | "stopped" | "running" | "error"`
    - `actions: string[]`
    - `lastError?: { message, occurredAt, stage, source }`
    - `version: string`
    - `binaryPath: string`
    - `configPath: string`
- **错误响应**
  - `401 Unauthorized`：`unauthorized`

### `GET /api/core/versions`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"releases":[{"tag":"v1.x.x","version":"1.x.x","prerelease":false}]}`
- **错误响应**
  - `401 Unauthorized`：`unauthorized`
  - `500 Internal Server Error`：`{"error":"..."}`（JSON）

### `POST /api/core/start`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"ok":true}`
- **错误响应**
  - `401 Unauthorized`：`unauthorized`
  - `404 Not Found`（结构化 JSON）
    - `{"code":"CORE_CONFIG_NOT_FOUND","message":"config file not found","detail":"..."}`
  - `400 Bad Request`（结构化 JSON）
    - `{"code":"CORE_NOT_INSTALLED",...}`
  - `409 Conflict`（结构化 JSON）
    - `{"code":"CORE_ALREADY_RUNNING",...}`
  - `500 Internal Server Error`（结构化 JSON）
    - `{"code":"CORE_START_FAILED",...}` 或 `CORE_INTERNAL_ERROR`
  - `405 Method Not Allowed`：`method not allowed`

### `POST /api/core/stop`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"ok":true}`
- **错误响应**
  - `401 Unauthorized`
  - `409 Conflict`（结构化 JSON）
    - `{"code":"CORE_ALREADY_STOPPED",...}`
  - `500 Internal Server Error`（结构化 JSON）
    - `{"code":"CORE_STOP_FAILED",...}` 或 `CORE_INTERNAL_ERROR`
  - `405 Method Not Allowed`

### `POST /api/core/restart`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"ok":true}`
- **错误响应**
  - `401 Unauthorized`
  - `404 Not Found`：`CORE_CONFIG_NOT_FOUND`（结构化 JSON）
  - `400/409/500`：`ProcessError` 映射后的结构化 JSON
  - `405 Method Not Allowed`

### `GET /api/core/logs`

- **认证要求**：需登录
- **请求参数（Query）**
  - `lines?: number`（正整数；默认 `200`；最大 `2000`）
- **成功响应**
  - `200 OK`
  - `{"path":".../sing-box.log","count":N,"entries":["line1","line2", ...]}`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`（结构化 JSON）
    - `{"code":"CORE_INVALID_LOG_LINES",...}`
  - `404 Not Found`（结构化 JSON）
    - `{"code":"CORE_LOG_NOT_FOUND",...}`
  - `500 Internal Server Error`（结构化 JSON）
    - `{"code":"CORE_LOG_READ_FAILED",...}`
  - `405 Method Not Allowed`

### `POST /api/core/config`

- **认证要求**：需登录
- **请求参数（JSON Body）**
  - 原始 sing-box 配置 JSON（`json.RawMessage`）
- **成功响应**
  - `200 OK`
  - `{"ok":"true"}`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid JSON`（纯文本）
    - `{"error":"..."}`（配置校验失败）
  - `500 Internal Server Error`
    - `failed to create config dir`（纯文本）
  - `405 Method Not Allowed`

### `GET /api/core/config-file`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - Body: 配置文件原始 JSON 内容
- **错误响应**
  - `401 Unauthorized`
  - `404 Not Found`：`{"error":"config file not found"}`
  - `500 Internal Server Error`：纯文本错误

### `POST /api/core/update`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `202 Accepted`
  - `{"status":"accepted"}`
  - 实际更新在后台 goroutine 执行，进度通过 SSE 查看。
- **错误响应**
  - `401 Unauthorized`
  - `409 Conflict`（结构化 JSON）
    - `{"code":"CORE_UPDATE_CONFLICT","message":"core update already in progress","detail":""}`
  - `405 Method Not Allowed`

### `GET /api/core/update/stream`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `Content-Type: text/event-stream`
  - SSE `data:` 事件体 JSON:
    - `inProgress: bool`
    - `percent: number`
    - `version: string`
    - `error?: string`
    - `updatedAt: string (RFC3339)`
- **错误响应**
  - `401 Unauthorized`
  - `500 Internal Server Error`：`stream unsupported`
  - `405 Method Not Allowed`

### `POST /api/core/rollback`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"ok":true}`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`（JSON）
    - `{"error":"暂无备份可回滚"}`
    - `{"error":"请设置 SINGBOX_BINARY_PATH 以启用核心更新"}`
  - `500 Internal Server Error`（JSON）
    - `{"error":"..."}`
  - `405 Method Not Allowed`

---

## 统计域（Stats）

### `GET /api/stats/summary`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"inbound_count":number,"user_count":number,"active_user_count":number,"total_uplink":number,"total_downlink":number}`
- **错误响应**
  - `401 Unauthorized`

---

## 入站域（Inbounds）

> 以下接口全部为“需登录”。

### `GET /api/inbounds`

- **认证要求**：需登录
- **请求参数（Query）**
  - `sort?: "traffic_asc" | "traffic_desc"`（为空时按 `created_at DESC`）
- **成功响应**
  - `200 OK`
  - `{"data":[inboundItem]}`
  - `inboundItem` 字段：
    - `id`
    - `tag`
    - `protocol`
    - `listen`
    - `listen_port`
    - `tls_type`（`none | tls | reality`）
    - `transport_type`
    - `user_count`（当前固定为 `0`）
    - `traffic_uplink`
    - `traffic_downlink`
    - `created_at`（RFC3339 格式字符串）
- **错误响应**
  - `401 Unauthorized`
  - `500 Internal Server Error`（纯文本）

### `GET /api/inbounds/{id}`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `200 OK`
  - `inboundDetail`（`inboundItem` + `config_json`）
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`：`invalid id`
  - `404 Not Found`：`not found`

### `POST /api/inbounds`

- **认证要求**：需登录
- **请求参数（JSON Body）**
  - `tag: string`（必填，唯一）
  - `protocol: string`（必填）
  - `listen: string`（可空，空时后端默认 `::`）
  - `listen_port: number`
  - `config_json: object`
- **成功响应**
  - `201 Created`
  - Body: `inboundItem`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid JSON`
    - `tag and protocol required`
    - `tag already exists`
    - `{"error":"..."}`（配置校验失败）
  - `500 Internal Server Error`
    - 数据库/配置生成/目录创建失败（纯文本）

### `PUT /api/inbounds/{id}`

- **认证要求**：需登录
- **请求参数**
  - Path: `id: uint`
  - Body:
    - `tag: string`（必填；若变化需唯一）
    - `protocol: string`（必填）
    - `listen: string`（可空，空时默认 `::`）
    - `listen_port: number`
    - `config_json: object`
- **成功响应**
  - `200 OK`
  - Body: `inboundItem`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid id`
    - `invalid JSON`
    - `tag and protocol required`
    - `tag already exists`
    - `{"error":"..."}`（配置校验失败）
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`：数据库或配置生成失败

### `DELETE /api/inbounds/{id}`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `204 No Content`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid id`
    - `{"error":"..."}`（配置应用失败）
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`

---

## 证书域（Certificates）

> 以下接口全部为“需登录”。

### `GET /api/certs`

- **认证要求**：需登录
- **请求参数**：无
- **成功响应**
  - `200 OK`
  - `{"data":[{"id","name","fullchain_path","privkey_path","created_at"}]}`
- **错误响应**
  - `401 Unauthorized`
  - `500 Internal Server Error`

### `GET /api/certs/{id}`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `200 OK`
  - `{"id","name","fullchain_path","privkey_path","created_at"}`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`：`invalid id`
  - `404 Not Found`：`not found`

### `POST /api/certs`

- **认证要求**：需登录
- **请求参数（JSON Body）**
  - `name: string`
  - `fullchain_path: string`（必填，去首尾空格后不能为空）
  - `privkey_path: string`（必填，去首尾空格后不能为空）
- **成功响应**
  - `201 Created`
  - Body: `certItem`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid JSON`
    - `fullchain_path and privkey_path required`
  - `500 Internal Server Error`

### `PUT /api/certs/{id}`

- **认证要求**：需登录
- **请求参数**
  - Path: `id: uint`
  - Body:
    - `name: string`
    - `fullchain_path: string`（必填）
    - `privkey_path: string`（必填）
- **成功响应**
  - `200 OK`
  - Body: `certItem`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid id`
    - `invalid JSON`
    - `fullchain_path and privkey_path required`
    - `{"error":"..."}`（配置应用失败）
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`

### `DELETE /api/certs/{id}`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `204 No Content`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid id`
    - `{"error":"证书正在被以下入站使用: ..."}`
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`

---

## 用户域（Users）

> 以下接口全部为“需登录”。

### `GET /api/users`

- **认证要求**：需登录
- **请求参数（Query）**
  - `q?: string`（按 `name`/`remark` 模糊检索）
- **成功响应**
  - `200 OK`
  - `{"data":[userItem]}`
  - `userItem` 字段：
    - `id`
    - `name`
    - `remark`
    - `uuid`
    - `password`
    - `traffic_limit`
    - `traffic_used`
    - `traffic_uplink`
    - `traffic_downlink`
    - `expire_at: string | null`（RFC3339）
    - `enabled`
    - `created_at`
    - `inbound_ids: number[]`
    - `inbound_tags: string[]`
    - `subscription_url`
- **错误响应**
  - `401 Unauthorized`
  - `500 Internal Server Error`

### `GET /api/users/{id}`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `200 OK`
  - Body: `userItem`（包含额外字段 `subscription_nodes: [{name, link}]`）
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`：`invalid id`
  - `404 Not Found`：`not found`

### `POST /api/users`

- **认证要求**：需登录
- **请求参数（JSON Body）**
  - `name: string`（必填）
  - `remark: string`
  - `inbound_ids: number[]`
  - `traffic_limit?: number`
  - `expire_at?: string`（RFC3339）
- **成功响应**
  - `201 Created`
  - Body: `userItem`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid JSON`
    - `name required`
    - `invalid expire_at format`
    - `{"error":"..."}`（配置应用失败）
  - `500 Internal Server Error`

### `PUT /api/users/{id}`

- **认证要求**：需登录
- **请求参数**
  - Path: `id: uint`
  - Body:
    - `name: string`（必填）
    - `remark: string`
    - `inbound_ids: number[]`
    - `traffic_limit?: number`
    - `expire_at?: string`（RFC3339）
- **成功响应**
  - `200 OK`
  - Body: `userItem`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid id`
    - `invalid JSON`
    - `name required`
    - `invalid expire_at format`
    - `{"error":"..."}`（配置应用失败）
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`

### `DELETE /api/users/{id}`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `204 No Content`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid id`
    - `{"error":"..."}`（配置应用失败）
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`

### `POST /api/users/batch`

- **认证要求**：需登录
- **请求参数（JSON Body）**
  - `action: "delete" | "enable" | "disable" | "reset_traffic"`
  - `ids: number[]`（非空）
- **成功响应**
  - `200 OK`
  - `{"ok":"true"}`
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`
    - `invalid JSON`
    - `action required`
    - `ids must be non-empty`
    - `invalid action`
    - `{"error":"..."}`（配置应用失败）
  - `500 Internal Server Error`

### `POST /api/users/{id}/reset-subscription`

- **认证要求**：需登录
- **请求参数（Path）**
  - `id: uint`
- **成功响应**
  - `200 OK`
  - `{"subscription_url":"/sub/<token>"}` 或带前缀 URL
- **错误响应**
  - `401 Unauthorized`
  - `400 Bad Request`：`invalid id`
  - `404 Not Found`：`not found`
  - `500 Internal Server Error`

---

## 订阅域（Subscription）

### `GET /sub/{token}`

- **认证要求**：公开
- **请求参数**
  - Path: `token: string`
  - Query:
    - `format=clash` 时返回 Clash YAML
  - Header:
    - `User-Agent` 含 `clash` 时也返回 Clash YAML
- **成功响应**
  - `200 OK`
  - `Content-Type: text/plain; charset=utf-8`
  - Header: `subscription-userinfo`
    - `upload=<bytes>; download=<bytes>; total=<bytes?>; expire=<unix?>`
  - Body:
    - 默认：Base64 编码的多行节点链接
    - Clash：YAML 内容
- **错误响应**
  - `403 Forbidden`
    - 用户被禁用 / 已过期 / 超流量
  - `404 Not Found`
    - token 无效或用户不存在
  - `500 Internal Server Error`
    - 订阅内容生成失败

---

## 路由覆盖校验结果

已与 `internal/api/routes.go` 逐项对照，本文覆盖全部注册端点：

- 认证与健康：`/api/health`、`/api/me`、`/api/setup`、`/api/login`、`/api/logout`
- 核心管理：`/api/core/*` 共 11 个
- 统计：`/api/stats/summary`
- 入站：`/api/inbounds` 与 `/{id}` 共 5 个
- 证书：`/api/certs` 与 `/{id}` 共 5 个
- 用户：`/api/users` 及批量/重置订阅共 7 个
- 订阅：`/sub/{token}`

