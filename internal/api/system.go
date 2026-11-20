package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
)

// bbrSysctlPath BBR 参数落盘位置（sysctl.d 风格，重启后仍生效）。
const bbrSysctlPath = "/etc/sysctl.d/99-proxy-ui-bbr.conf"

// systemStatusView 系统优化状态视图。
type systemStatusView struct {
	Platform       string   `json:"platform"`       // 运行平台：linux / darwin / windows
	Supported      bool     `json:"supported"`      // 是否可在面板内一键开启（仅 Linux + root）
	Privileged     bool     `json:"privileged"`     // 当前进程是否有 root 权限
	BBREnabled     bool     `json:"bbrEnabled"`     // 是否已启用 BBR
	CongestionCtl  string   `json:"congestionCtl"`  // 当前 TCP 拥塞控制算法
	AvailableCC    []string `json:"availableCc"`    // 内核支持的拥塞控制算法
	AvailableQDisc []string `json:"availableQdisc"` // 内核支持的队列规则
	Reason         string   `json:"reason"`         // 不可一键开启的原因
	ManualCommand  string   `json:"manualCommand"`  // 手动执行命令（不可一键开启时给用户复制）
}

// bbrManualCommand 手动开启 BBR 的等价命令。
const bbrManualCommand = "cat > /etc/sysctl.d/99-proxy-ui-bbr.conf <<'EOF'\nnet.core.default_qdisc=fq\nnet.ipv4.tcp_congestion_control=bbr\nEOF\nsysctl --system"

// HandleSystemStatus GET /api/system/status 系统优化状态（拥塞控制与队列规则）。
func (a *API) HandleSystemStatus(c *gin.Context) {
	OK(c, readSystemStatus())
}

// readSystemStatus 采集系统拥塞控制与 BBR 状态。
func readSystemStatus() systemStatusView {
	view := systemStatusView{
		Platform:      runtime.GOOS,
		Privileged:    os.Geteuid() == 0,
		ManualCommand: bbrManualCommand,
	}
	view.CongestionCtl = readProcLine("/proc/sys/net/ipv4/tcp_congestion_control")
	view.AvailableCC = splitSpace(readProcLine("/proc/sys/net/ipv4/tcp_available_congestion_control"))
	view.AvailableQDisc = splitSpace(readProcLine("/proc/sys/net/core/default_qdisc"))
	if cc := strings.TrimSpace(view.CongestionCtl); cc != "" {
		view.BBREnabled = cc == "bbr"
	}
	// 有些内核把可用算法写在 tcp_allowed_congestion_control 里，仅当上面为空时兜底
	if len(view.AvailableCC) == 0 {
		view.AvailableCC = splitSpace(readProcLine("/proc/sys/net/ipv4/tcp_allowed_congestion_control"))
	}
	switch {
	case runtime.GOOS != "linux":
		view.Reason = "仅 Linux 支持面板内一键开启 BBR；其它系统请参考上方手动命令"
	case !view.Privileged:
		view.Reason = "面板进程不是 root，无法写入 sysctl；可用 root 执行上方手动命令，或让面板以 root 运行"
	case !contains(view.AvailableCC, "bbr"):
		view.Reason = "当前内核未编译 BBR 模块（可尝试 modprobe tcp_bbr 后重试）"
	default:
		view.Supported = true
	}
	return view
}

// bbrReq BBR 开关请求。
type bbrReq struct {
	Enable bool `json:"enable"` // true 开启 / false 关闭（恢复 cubic + fq_codel）
}

// HandleBBREnable POST /api/system/bbr 一键开启/关闭 BBR（写 sysctl.d 并立即生效）。
func (a *API) HandleBBREnable(c *gin.Context) {
	var req bbrReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if err := applyBBR(req.Enable); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	// BBR 是系统级设置，不影响内核配置；热应用会比对出配置未变而直接返回
	if !a.applyOrFail(c) {
		return
	}
	OK(c, readSystemStatus())
}

// applyBBR 写入 sysctl 配置并执行 sysctl --system 使其立即生效。
func applyBBR(enable bool) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("仅 Linux 支持在面板内修改 BBR 设置")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("面板进程不是 root，请在服务器上以 root 执行: %s", bbrManualCommand)
	}
	content := "net.core.default_qdisc=fq\nnet.ipv4.tcp_congestion_control=bbr\n"
	if !enable {
		content = "net.core.default_qdisc=fq_codel\nnet.ipv4.tcp_congestion_control=cubic\n"
	}
	if err := os.MkdirAll(filepath.Dir(bbrSysctlPath), 0o755); err != nil {
		return fmt.Errorf("创建 sysctl.d 目录失败: %w", err)
	}
	if err := os.WriteFile(bbrSysctlPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", bbrSysctlPath, err)
	}
	if out, err := exec.Command("sysctl", "--system").CombinedOutput(); err != nil {
		return fmt.Errorf("sysctl --system 执行失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// readProcLine 读取 /proc 下单行配置，失败返回空字符串。
func readProcLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// splitSpace 按空白拆分字符串（空串返回 nil）。
func splitSpace(s string) []string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}
	return fields
}
