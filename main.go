// 面板入口：解析启动参数、初始化数据库并启动 HTTP 服务。
package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"proxy-ui/internal/db"
	"proxy-ui/internal/server"
)

// Version 面板版本号，构建脚本可经 ldflags 注入。
var Version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dataDir := flag.String("data", "data", "数据目录（数据库、内核、证书等）")
	dbPath := flag.String("db", "", "数据库文件路径，默认 <data>/panel.db")
	flag.Parse()

	if *dbPath == "" {
		*dbPath = filepath.Join(*dataDir, "panel.db")
	}
	gormDB, err := db.Open(*dbPath)
	if err != nil {
		slog.Error("初始化数据库失败", "err", err)
		os.Exit(1)
	}
	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		slog.Error("加载前端资源失败", "err", err)
		os.Exit(1)
	}

	engine := server.New(server.Options{DB: gormDB, Dist: dist, Version: Version})
	slog.Info("面板启动", "addr", *addr, "version", Version, "db", *dbPath)
	if err := http.ListenAndServe(*addr, engine); err != nil {
		slog.Error("HTTP 服务退出", "err", err)
		os.Exit(1)
	}
}
