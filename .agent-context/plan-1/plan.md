# 后端文档生成

> 状态: 未执行

## 目标

根据后端 Go 源码，生成两份文档到 `docs/` 目录：
1. **模块功能文档** (`docs/modules.md`) — 描述各模块职责、核心类型、函数、模块间依赖关系
2. **API 文档** (`docs/api.md`) — 描述所有 HTTP API 端点的完整规格

## 内容

### 步骤 1：生成模块功能文档 `docs/modules.md`

覆盖以下模块，每个模块包含：职责说明、核心类型与函数列表、依赖关系、配置项

| 模块 | 包路径 | 职责 |
|------|--------|------|
| 配置管理 | `internal/config` | 加载面板配置（文件 / 环境变量） |
| 数据库层 | `internal/db` | GORM + SQLite，CRUD、迁移、模型定义 |
| 会话管理 | `internal/session` | scs 会话管理，SQLite 存储后端 |
| HTTP API | `internal/api` | 路由注册、中间件、请求/响应处理 |
| 核心管理 | `internal/core` | sing-box 进程管理、配置生成、订阅、统计、更新 |
| 统计协议 | `internal/statsproto` | gRPC V2Ray 统计协议定义 |
| 启动入口 | `cmd/server` | 初始化流程与依赖编排 |

数据库模型单独成节，列出 `admins`、`users`、`inbounds`、`certificates`、`sessions` 表结构。

### 步骤 2：生成 API 文档 `docs/api.md`

按功能域分组（认证、核心管理、入站、用户、证书、统计、订阅），每个端点包含：

- HTTP 方法与路径
- 认证要求（公开 / 需登录）
- 请求参数（Query / Path / Body + 字段类型）
- 响应格式（JSON 结构 + 状态码）
- 错误响应

### 步骤 3：交叉校验

- 确认文档中所有端点与 `internal/api/routes.go` 注册的路由一一对应
- 确认所有数据库模型字段与 `internal/db/` 定义一致

## 影响范围

## 历史补丁
