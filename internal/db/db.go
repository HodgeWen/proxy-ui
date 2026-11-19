// Package db 提供 SQLite 连接与 GORM 全量模型、启动自动迁移。
package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开（或创建）SQLite 数据库并执行自动迁移，库文件所在目录不存在时自动创建。
func Open(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	if err := g.AutoMigrate(
		&Admin{}, &Setting{}, &Inbound{}, &Outbound{}, &PolicyGroup{}, &Rule{},
		&SubscriptionToken{}, &InboundTraffic{}, &AIProvider{}, &Certificate{},
	); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return g, nil
}
