package db

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestOpenRebuildsLegacySchema 旧 schema 库（既有表但结构版本不符）启动时整体删除并重建为空库。
func TestOpenRebuildsLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacyDB(t, path, 0)

	g, err := Open(path)
	if err != nil {
		t.Fatalf("打开并重建库失败: %v", err)
	}

	// 旧表与旧数据都不在了
	var oldCount int64
	if err := g.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'old_inbounds'").
		Scan(&oldCount).Error; err != nil {
		t.Fatalf("查询旧表失败: %v", err)
	}
	if oldCount != 0 {
		t.Error("旧 schema 的表应已被删除")
	}
	// 新库结构完整可用
	var inboundCount int64
	if err := g.Model(&Inbound{}).Count(&inboundCount).Error; err != nil {
		t.Fatalf("新库表不可用: %v", err)
	}
	if inboundCount != 0 {
		t.Errorf("重建后应是空库, got %d 行", inboundCount)
	}
	// 结构版本已标记为当前版本（再次打开不会重复重建）
	var version int
	if err := g.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		t.Fatalf("读取版本失败: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("结构版本 = %d, want %d", version, schemaVersion)
	}
}

// TestOpenRebuildsLegacySchemaWithView 旧 schema 库含视图时也能整体重建（视图需 DROP VIEW，不能当表删）。
func TestOpenRebuildsLegacySchemaWithView(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-view.db")

	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开旧库失败: %v", err)
	}
	if err := legacy.Exec("CREATE TABLE old_inbounds (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("建旧表失败: %v", err)
	}
	if err := legacy.Exec("CREATE VIEW old_inbound_names AS SELECT name FROM old_inbounds").Error; err != nil {
		t.Fatalf("建旧视图失败: %v", err)
	}
	if err := legacy.Exec("PRAGMA user_version = 0").Error; err != nil {
		t.Fatalf("写旧版本号失败: %v", err)
	}
	sqlDB, _ := legacy.DB()
	_ = sqlDB.Close()

	g, err := Open(path)
	if err != nil {
		t.Fatalf("旧库含视图时重建失败: %v", err)
	}

	var leftover int64
	if err := g.Raw(
		"SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table', 'view') AND name IN ('old_inbounds', 'old_inbound_names')",
	).Scan(&leftover).Error; err != nil {
		t.Fatalf("查询旧对象失败: %v", err)
	}
	if leftover != 0 {
		t.Error("旧 schema 的表与视图都应已被删除")
	}
	var inboundCount int64
	if err := g.Model(&Inbound{}).Count(&inboundCount).Error; err != nil {
		t.Fatalf("新库表不可用: %v", err)
	}
	var version int
	if err := g.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		t.Fatalf("读取版本失败: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("结构版本 = %d, want %d", version, schemaVersion)
	}
}

// TestOpenKeepsExistingData 结构版本相符的库再次打开时不动数据。
func TestOpenKeepsExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep.db")
	g, err := Open(path)
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	if err := g.Create(&Inbound{Name: "keep-in", Protocol: "vless", Listen: "::", ListenPort: 12345}).Error; err != nil {
		t.Fatalf("写入入站失败: %v", err)
	}
	sqlDB, _ := g.DB()
	_ = sqlDB.Close()

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("重开库失败: %v", err)
	}
	var count int64
	if err := reopened.Model(&Inbound{}).Count(&count).Error; err != nil {
		t.Fatalf("查询入站失败: %v", err)
	}
	if count != 1 {
		t.Errorf("同版本库重开不应丢数据, got %d 行", count)
	}
}

// legacyDB 造一个「旧版」库：有表、有数据、user_version 记为指定版本。
func legacyDB(t *testing.T, path string, version int) {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开旧库失败: %v", err)
	}
	if err := g.Exec("CREATE TABLE old_inbounds (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("建旧表失败: %v", err)
	}
	if err := g.Exec("INSERT INTO old_inbounds (name) VALUES ('legacy')").Error; err != nil {
		t.Fatalf("写旧数据失败: %v", err)
	}
	if err := g.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)).Error; err != nil {
		t.Fatalf("写旧版本号失败: %v", err)
	}
	sqlDB, _ := g.DB()
	_ = sqlDB.Close()
}

// TestOpenBacksUpLegacyDB 重建旧库前先把库文件备份到同目录（文件名带旧版本号），备份数据可读。
func TestOpenBacksUpLegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.db")
	legacyDB(t, path, 1)

	if _, err := Open(path); err != nil {
		t.Fatalf("打开并重建库失败: %v", err)
	}

	backup := path + ".bak-1"
	bak, err := gorm.Open(sqlite.Open(backup), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("备份文件应可作为 SQLite 打开: %v", err)
	}
	var rows int64
	if err := bak.Raw("SELECT COUNT(*) FROM old_inbounds WHERE name = 'legacy'").Scan(&rows).Error; err != nil {
		t.Fatalf("查询备份失败: %v", err)
	}
	sqlDB, _ := bak.DB()
	_ = sqlDB.Close()
	if rows != 1 {
		t.Errorf("备份应保留旧数据, got %d 行", rows)
	}
}

// TestOpenPragmaDSN 打开的库启用 WAL 与 busy_timeout（缓解并发读写冲突）。
func TestOpenPragmaDSN(t *testing.T) {
	g, err := Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	var mode string
	if err := g.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatalf("读取 journal_mode 失败: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	var timeout int
	if err := g.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil {
		t.Fatalf("读取 busy_timeout 失败: %v", err)
	}
	if timeout != 10000 {
		t.Errorf("busy_timeout = %d, want 10000", timeout)
	}
}
