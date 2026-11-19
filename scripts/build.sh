#!/usr/bin/env bash
# 构建脚本：存在 web 前端包时先构建前端，再交叉编译 linux/amd64 与 darwin/arm64 单文件二进制（禁 CGO）。
set -euo pipefail
cd "$(dirname "$0")/.."

# 前端构建：web 包存在时执行
if [ -f web/package.json ]; then
    command -v bun >/dev/null 2>&1 || { echo "错误：web/package.json 存在但未安装 bun"; exit 1; }
    (cd web && bun install && bun run build)
fi

# 版本号经 ldflags 注入 main.Version（git tag 描述；无 git 环境回退 dev）
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
LDFLAGS="-X main.Version=${VERSION}"

mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o bin/proxy-ui-linux-amd64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o bin/proxy-ui-darwin-arm64 .

echo "构建完成（版本 ${VERSION}）："
ls -lh bin/proxy-ui-linux-amd64 bin/proxy-ui-darwin-arm64
