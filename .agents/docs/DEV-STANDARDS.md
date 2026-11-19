# 开发规范

## 命名

- Go：包名小写单词；文件 snake_case；导出符号驼峰并附中文注释；测试文件 `*_test.go` 与被测文件同包
- 前端：组件文件 `PascalCase.vue`；composable `useXxx`；目录/普通文件 kebab-case；类型 PascalCase

## 目录与代码结构

- Go 在仓库根：业务模块一律放 `internal/<module>/`，禁止在根散落 .go 文件（入口 `main.go`、`embed.go` 除外）
- API handler 按资源分文件放在 `internal/api/`，路由统一在 `internal/server/` 装配
- 前端在 `web/` 独立 npm 包：`src/api/`（接口封装）、`src/stores/`（Pinia）、`src/views/`（页面）、`src/components/`、`src/router/`

## 代码风格

- Go：gofmt 格式化；注释、commit message、文档中文
- 前端：TypeScript；接入 eslint/prettier 前以现有代码风格为准

## 测试

- 必须写单测的模块：`internal/singbox`（配置生成）、`internal/subscription`（订阅/规则导出）、`internal/update`（更新逻辑）——改动即补测试
- handler、前端组件暂不强制

- 局部验证：`go test ./internal/<pkg>/...`；前端 `cd web && bun run build`
- 全量：`go test ./... && cd web && bun run build`（默认只在 CI / 发版前跑）

## 接口

- REST + JSON；统一响应 envelope（`success` / `msg` / `data`），错误用 HTTP 状态码 + envelope msg
- 订阅端点例外：匿名 token 鉴权，直接返回对应格式配置内容

## 数据与存储

- SQLite（glebarez + GORM），库文件单文件落盘；表结构变更走 GORM 启动时自动迁移，字段加注释

## 日志

- Go 侧用 `log/slog`；sing-box 子进程输出由面板收集，可在页面查看

## 版本与发布

- 语义化版本 + git tag；前端先构建、dist 嵌入后出单二进制；发 GitHub Releases；面板内支持自更新与内核更新

## 明确禁止

- 禁用 CGO（保证交叉编译单文件）
- 不引入 sing-box 之外的代理内核
- 前端不引入 `@veltra/*`（ultra-ui）之外的组件库
- 不做多租户、套餐、订阅计费
