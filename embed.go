package main

import "embed"

// distFS 内嵌的前端构建产物（web/dist，由 scripts/build.sh 或 cd web && bun run build 生成）。
//
//go:embed all:web/dist
var distFS embed.FS
