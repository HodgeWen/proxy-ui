// Package db 提供 SQLite 连接与 GORM 全量模型、启动自动迁移。
package db

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaVersion 当前库结构版本（写在 SQLite 的 user_version 上）。
//
// 只做「版本相符直接用、不符整体重建」，不做迁移：面板不做旧数据兼容，
// 结构一变就让用户以空库重新配置，避免半迁移状态留下难查的脏数据。
const schemaVersion = 1

// Open 打开（或创建）SQLite 数据库并执行自动迁移，库文件所在目录不存在时自动创建。
//
// 库文件为旧 schema（既有表或视图但结构版本不符）时删除全部表与视图并重建空库，日志给出文件路径。
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
	if err := ensureSchema(g, path); err != nil {
		return nil, err
	}
	if err := g.AutoMigrate(
		&Admin{}, &Setting{}, &Inbound{}, &InboundUser{}, &Outbound{}, &PolicyGroup{}, &Rule{},
		&RuleSet{}, &RuleSetItem{}, &SubscriptionToken{}, &SubscriptionNode{}, &UserTraffic{}, &TokenTraffic{},
		&AIProvider{}, &Certificate{},
	); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return g, nil
}

// ensureSchema 校验库结构版本：全新库标记为当前版本，版本不符的旧库整体删除后重建空库。
func ensureSchema(g *gorm.DB, path string) error {
	var version int
	if err := g.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return fmt.Errorf("读取数据库结构版本失败: %w", err)
	}
	if version == schemaVersion {
		return nil
	}
	hasObjects, err := hasUserObjects(g)
	if err != nil {
		return err
	}
	if !hasObjects {
		// 新库（或空文件）：直接打上当前版本，随后由 AutoMigrate 建表
		return setSchemaVersion(g)
	}
	slog.Warn("检测到旧版数据库结构，已删除并重建为空库（旧数据不做迁移，请重新配置面板）",
		"db", path, "oldVersion", version, "newVersion", schemaVersion)
	if err := dropUserTables(g); err != nil {
		return fmt.Errorf("重建数据库失败: %w", err)
	}
	if err := setSchemaVersion(g); err != nil {
		return err
	}
	slog.Warn("空库已重建完成", "db", path, "newVersion", schemaVersion)
	return nil
}

// hasUserObjects 判断库里是否已有业务对象（sqlite_ 前缀为 SQLite 内部表，不参与判断）。
func hasUserObjects(g *gorm.DB) (bool, error) {
	var names []string
	if err := g.Raw(
		"SELECT name FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'",
	).Scan(&names).Error; err != nil {
		return false, fmt.Errorf("读取数据库表失败: %w", err)
	}
	return len(names) > 0, nil
}

// dropUserTables 删除全部业务对象：视图用 DROP VIEW、表用 DROP TABLE（索引随表一并消失）。
func dropUserTables(g *gorm.DB) error {
	var objects []struct {
		Type string
		Name string
	}
	if err := g.Raw(
		"SELECT type, name FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'" +
			" ORDER BY CASE type WHEN 'view' THEN 0 ELSE 1 END",
	).Scan(&objects).Error; err != nil {
		return fmt.Errorf("读取数据库表失败: %w", err)
	}
	for _, obj := range objects {
		stmt := "DROP TABLE IF EXISTS"
		if obj.Type == "view" {
			stmt = "DROP VIEW IF EXISTS"
		}
		// 对象名取自 sqlite_master，仍按标识符引号包裹，避免特殊字符破坏语句
		if err := g.Exec(fmt.Sprintf("%s %q", stmt, obj.Name)).Error; err != nil {
			return fmt.Errorf("删除旧%s %s 失败: %w", obj.Type, obj.Name, err)
		}
	}
	return nil
}

// setSchemaVersion 写入当前库结构版本。
func setSchemaVersion(g *gorm.DB) error {
	if err := g.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)).Error; err != nil {
		return fmt.Errorf("写入数据库结构版本失败: %w", err)
	}
	return nil
}
