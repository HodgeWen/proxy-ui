# AGENTS.md — s-ui

sing-box 管理面板，Go 后端 + React 前端单体仓库。

## 常用命令

```bash
# 依赖安装
make deps                    # 前后端依赖一起装
cd web && bun install        # 仅前端

# 开发
cd web && bun run dev        # 前端 dev server（代理 /api → localhost:8080）
go run ./cmd/server          # 后端

# 构建
make build                   # 前端 → 后端（前端产物嵌入 Go 二进制）
make build-release           # 交叉编译 linux amd64 + arm64

# 代码检查
cd web && bun run lint       # ESLint

# Docker
docker compose up --build
```

## 技术栈

| 层 | 技术 | 版本 |
|---|------|------|
| 前端框架 | React | 19 |
| 语言 | TypeScript | 5.9 |
| 构建 | Vite | 7 |
| 样式 | Tailwind CSS | 4 |
| UI 库 | shadcn/ui (new-york, neutral, lucide) | — |
| 表单 | react-hook-form + zod + @hookform/resolvers | — |
| 数据请求 | @tanstack/react-query | 5 |
| 路由 | react-router-dom | 7 |
| 包管理 | bun | — |
| 后端 | Go | 1.25 |
| HTTP 路由 | chi/v5 | — |
| ORM | GORM + SQLite | — |
| 会话 | scs/v2 | — |
| 协议 | gRPC + protobuf | — |

## 目录结构

```
├── cmd/server/             # Go 入口 main.go
├── internal/               # Go 内部包（不对外暴露）
│   ├── api/                #   HTTP 路由与处理器
│   ├── config/             #   配置加载
│   ├── core/               #   sing-box 进程管理与统计
│   ├── db/                 #   GORM 数据库层
│   ├── session/            #   会话管理 (scs)
│   └── statsproto/         #   gRPC 统计协议
├── web/                    # 前端项目
│   └── src/
│       ├── components/     #   按业务域拆分
│       │   ├── ui/         #     shadcn/ui 基础组件（勿手动修改）
│       │   ├── layout/     #     AppLayout, Sidebar
│       │   ├── users/      #     用户管理
│       │   ├── inbounds/   #     入站管理
│       │   └── certificates/ #   证书管理
│       ├── hooks/          #   自定义 hooks
│       ├── lib/            #   工具函数、常量
│       └── pages/          #   页面级组件（与路由 1:1）
├── data/                   # 运行时数据（SQLite、配置）
├── docs/                   # 项目文档
├── Makefile                # 构建入口
├── Dockerfile              # 多阶段构建
└── docker-compose.yml
```

## 代码规范

### 通用

- Commit 格式：`type(scope): subject`，type 为 feat/fix/chore/docs/test/refactor。
- 代码、命名使用英文；注释和文档可用中文。

### 前端 (web/)

- **组件**：函数组件 + named export，PascalCase 文件名（如 `UserTable.tsx`）。
- **样式**：Tailwind utility classes，用 `cn()` 合并 class。禁止内联 style 和独立 CSS 文件。
- **路径别名**：`@/*` 映射 `src/*`。
- **API 调用**：直接 `fetch`，带 `credentials: "include"`。无独立 API 层。
- **状态管理**：服务端状态用 React Query；本地 UI 状态用 `useState`。
- **表单**：react-hook-form + zod schema 验证。
- **新增 UI 组件**：优先通过 `bunx shadcn@latest add <component>` 添加。`components/ui/` 下的文件由 shadcn 生成，避免手动修改。
- **Lint**：ESLint flat config，无 Prettier。提交前执行 `bun run lint`。
- **测试**：暂未配置。

### 后端 (Go)

- 遵循 Go 标准项目布局，业务代码在 `internal/`。
- HTTP handler 在 `internal/api/`，数据库操作在 `internal/db/`。
- 前端构建产物通过 `embed.go` 嵌入二进制，无需独立静态文件服务。

## 架构要点

- 前后端分离开发，统一部署：前端 `web/dist` 被 embed 进 Go 二进制。
- 路由鉴权：React Router loader 调用 `/api/me` 检查登录状态。
- 主题：ThemeProvider (Context + localStorage)，支持亮/暗模式。
- 开发时前端 Vite dev server 通过 proxy 转发 `/api` 到后端 `localhost:8080`。
