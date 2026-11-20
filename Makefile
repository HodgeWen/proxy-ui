# proxy-ui 开发快捷命令
# 后端监听地址（改它时前端代理会经 API_PROXY 联动到同一地址）
DEV_ADDR ?= 127.0.0.1:8080
# 开发实例数据目录（独立于生产 data/，首次进入设置向导）
DEV_DATA ?= .devdata
# 内核以内置库形式编入面板：以下构建标签必须带上，缺 with_v2ray_api 会导致按用户流量统计失效
BUILD_TAGS ?= with_v2ray_api,with_quic,with_utls,with_acme,with_clash_api,with_gvisor

help: ## 显示可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

dev: ## 同时起前后端（热更新，访问 http://localhost:5173，Ctrl-C 一起停）
	@mkdir -p $(DEV_DATA)
	@echo ">> 后端 http://$(DEV_ADDR)  前端 http://localhost:5173"
	@trap 'kill 0' EXIT INT TERM; \
	go run -tags $(BUILD_TAGS) . --addr $(DEV_ADDR) --data $(DEV_DATA) & \
	cd web && API_PROXY=http://$(DEV_ADDR) bun run dev & \
	wait

dev-api: ## 只起 Go 后端
	@mkdir -p $(DEV_DATA)
	go run -tags $(BUILD_TAGS) . --addr $(DEV_ADDR) --data $(DEV_DATA)

dev-web: ## 只起前端 dev server（/api 代理到 DEV_ADDR）
	cd web && API_PROXY=http://$(DEV_ADDR) bun run dev

run: web-build ## 交付形态：构建前端后以单二进制运行（go:embed 产物）
	go run -tags $(BUILD_TAGS) . --addr $(DEV_ADDR) --data $(DEV_DATA)

web-build: ## 构建前端（含 vue-tsc 类型检查）
	cd web && bun run build

build: ## 发布构建：前端 + linux/darwin 交叉编译（含内置内核）
	./scripts/build.sh

test: ## 运行全部 Go 测试（含内置内核标签）
	go test -tags $(BUILD_TAGS) ./...

clean: ## 清理开发数据目录
	rm -rf $(DEV_DATA)

.PHONY: help dev dev-api dev-web run web-build build test clean
