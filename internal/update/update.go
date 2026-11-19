package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// 设置键。
const (
	settingMirror      = "mirror_prefix" // GitHub 下载镜像前缀
	settingCoreVersion = "core_version"  // 已安装内核版本 tag
)

// Manager sing-box 内核更新管理：版本查询、镜像下载、完整性校验与安装。
type Manager struct {
	ghClient        // GitHub Releases 访问底座（镜像前缀与下载校验共享）
	dir      string // 内核安装目录
}

// New 构造内核更新管理器；dir 为内核安装目录（自动创建）。
func New(database *gorm.DB, dir string) *Manager {
	_ = os.MkdirAll(dir, 0o755)
	return &Manager{ghClient: newGHClient(database), dir: dir}
}

// CorePath 内核可执行文件路径。
func (m *Manager) CorePath() string { return filepath.Join(m.dir, "sing-box") }

// Installed 内核是否已安装。
func (m *Manager) Installed() bool {
	_, err := os.Stat(m.CorePath())
	return err == nil
}

// InstalledVersion 已安装内核版本 tag（未记录返回空）。
func (m *Manager) InstalledVersion() string { return getSetting(m.db, settingCoreVersion) }

// MirrorPrefix 读取下载镜像前缀设置（空表示直连 GitHub）。
func (m *Manager) MirrorPrefix() string { return m.mirror() }

// SetMirrorPrefix 保存镜像前缀；非空须为 http(s):// 地址，返回去掉尾部斜杠后的规范化值。
func (m *Manager) SetMirrorPrefix(prefix string) (string, error) {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if prefix != "" && !strings.HasPrefix(prefix, "http://") && !strings.HasPrefix(prefix, "https://") {
		return "", errors.New("镜像前缀须以 http:// 或 https:// 开头")
	}
	if err := setSetting(m.db, settingMirror, prefix); err != nil {
		return "", err
	}
	return prefix, nil
}

// ListVersions 查询 GitHub 可用内核版本列表，按版本号降序返回。
func (m *Manager) ListVersions(ctx context.Context) ([]Version, error) {
	return m.listVersions(ctx, repo, m.InstalledVersion())
}

// Install 下载并安装指定版本内核；version 为空时选择最新稳定版，返回安装的版本 tag。
func (m *Manager) Install(ctx context.Context, version string) (string, error) {
	rels, err := m.fetchReleases(ctx, repo)
	if err != nil {
		return "", err
	}
	rel, ok := pickRelease(rels, version)
	if !ok {
		return "", fmt.Errorf("版本 %q 不存在或无稳定版本可选", version)
	}
	name := assetName(rel.TagName)
	asset := findAsset(rel, name)
	if asset == nil {
		return "", fmt.Errorf("版本 %s 未提供 %s 安装包", rel.TagName, name)
	}

	tmpPath, err := m.downloadFile(ctx, asset.BrowserDownloadURL, asset.Digest, m.dir)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)

	coreTmp := m.CorePath() + ".new"
	if err := extractCoreTo(tmpPath, coreTmp); err != nil {
		_ = os.Remove(coreTmp)
		return "", err
	}
	if err := os.Chmod(coreTmp, 0o755); err != nil {
		_ = os.Remove(coreTmp)
		return "", fmt.Errorf("设置内核可执行权限失败: %w", err)
	}
	if err := os.Rename(coreTmp, m.CorePath()); err != nil {
		_ = os.Remove(coreTmp)
		return "", fmt.Errorf("安装内核失败: %w", err)
	}
	if err := setSetting(m.db, settingCoreVersion, rel.TagName); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// extractCoreTo 从 tar.gz 安装包中提取 sing-box 可执行文件写到 dest（gzip 解压自带 CRC 完整性校验）。
func extractCoreTo(tarGzPath, dest string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("打开安装包失败: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("安装包不是有效的 gzip 文件: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("安装包中未找到 sing-box 可执行文件")
		}
		if err != nil {
			return fmt.Errorf("读取安装包失败: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "sing-box" {
			continue
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return fmt.Errorf("写入内核文件失败: %w", err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("解压内核失败: %w", err)
		}
		return out.Close()
	}
}
