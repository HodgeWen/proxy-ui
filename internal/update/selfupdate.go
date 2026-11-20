package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// panelRepo 面板自身 GitHub 仓库。
const panelRepo = "HodgeWen/proxy-ui"

// SelfUpdater 面板自身更新：查询面板 Releases、下载替换当前二进制并重启进程。
type SelfUpdater struct {
	ghClient                        // GitHub Releases 访问底座（请求与下载校验共享）
	version  string                 // 当前面板版本（构建时 ldflags 注入，dev 表示未注入）
	exePath  string                 // 当前面板二进制路径（空表示运行时探测，测试可替换）
	restart  func(exe string) error // 换入新二进制的重启实现（默认 syscall.Exec，测试可替换）
}

// NewSelfUpdate 构造面板自身更新器；version 为构建注入的当前面板版本。
func NewSelfUpdate(version string) *SelfUpdater {
	return &SelfUpdater{ghClient: newGHClient(), version: version, restart: execRestart}
}

// CurrentVersion 当前面板版本号。
func (u *SelfUpdater) CurrentVersion() string { return u.version }

// ListVersions 查询面板可用版本列表（各大版本线最新稳定版降序，另附更新预发布线的尝鲜项）。
func (u *SelfUpdater) ListVersions(ctx context.Context) ([]Version, error) {
	return u.listVersions(ctx, panelRepo, u.version)
}

// Apply 下载指定版本面板二进制（version 空=最新稳定版）替换当前可执行文件，返回版本 tag。
// 只替换不重启：进程重启由调用方在响应发出后经 Restart 触发。
func (u *SelfUpdater) Apply(ctx context.Context, version string) (string, error) {
	rels, err := u.fetchReleases(ctx, panelRepo)
	if err != nil {
		return "", err
	}
	rel, ok := pickRelease(rels, version)
	if !ok {
		return "", fmt.Errorf("版本 %q 不存在或无稳定版本可选", version)
	}
	name := panelAssetName()
	asset := findAsset(rel, name)
	if asset == nil {
		return "", fmt.Errorf("版本 %s 未提供 %s 安装包", rel.TagName, name)
	}
	exe, err := u.executable()
	if err != nil {
		return "", err
	}
	// 临时文件与二进制同目录，保证 rename 原子替换运行中的可执行文件
	tmpPath, err := u.downloadFile(ctx, asset.BrowserDownloadURL, asset.Digest, filepath.Dir(exe))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", fmt.Errorf("设置面板可执行权限失败: %w", err)
	}
	if err := os.Rename(tmpPath, exe); err != nil {
		return "", fmt.Errorf("替换面板二进制失败: %w", err)
	}
	return rel.TagName, nil
}

// Restart 换入更新后的二进制重启面板进程（unix 下同 PID，systemd 服务不中断）。
func (u *SelfUpdater) Restart() error {
	exe, err := u.executable()
	if err != nil {
		return err
	}
	return u.restart(exe)
}

// executable 当前面板二进制路径（优先测试注入值）。
func (u *SelfUpdater) executable() (string, error) {
	if u.exePath != "" {
		return u.exePath, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("定位面板二进制失败: %w", err)
	}
	return exe, nil
}

// panelAssetName 当前面板平台对应的 release 资产名（与 build.sh 产物一致：proxy-ui-<os>-<arch>）。
func panelAssetName() string {
	return fmt.Sprintf("proxy-ui-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// execRestart 以新二进制换入当前进程：继承原进程 PID 与启动参数。
func execRestart(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
