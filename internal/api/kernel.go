package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/singbox"
)

// kernelStatus 组合内核状态与已安装版本。
func (a *API) kernelStatus(c *gin.Context) singbox.Status {
	st := a.core.Status()
	st.Version = a.core.Version(c.Request.Context())
	return st
}

// HandleKernelStatus GET /api/kernel/status 内核运行状态。
func (a *API) HandleKernelStatus(c *gin.Context) {
	OK(c, a.kernelStatus(c))
}

// HandleKernelLogs GET /api/kernel/logs?limit=200 内核日志。
func (a *API) HandleKernelLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	OK(c, a.core.Logs(limit))
}

// HandleKernelStart POST /api/kernel/start 启动内核。
func (a *API) HandleKernelStart(c *gin.Context) {
	if err := a.core.Start(); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, a.kernelStatus(c))
}

// HandleKernelStop POST /api/kernel/stop 停止内核。
func (a *API) HandleKernelStop(c *gin.Context) {
	if err := a.core.Stop(); err != nil {
		Fail(c, http.StatusInternalServerError, "停止内核失败: "+err.Error())
		return
	}
	OK(c, a.kernelStatus(c))
}

// HandleKernelRestart POST /api/kernel/restart 重启内核。
func (a *API) HandleKernelRestart(c *gin.Context) {
	if err := a.core.Restart(); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, a.kernelStatus(c))
}

// HandleKernelVersions GET /api/kernel/versions 可用版本列表（GitHub Releases）。
func (a *API) HandleKernelVersions(c *gin.Context) {
	versions, err := a.updates.ListVersions(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, gin.H{"versions": versions})
}

// installReq 安装内核请求。
type installReq struct {
	Version string `json:"version"` // 目标版本 tag；空表示最新稳定版
}

// HandleKernelInstall POST /api/kernel/install 下载并安装内核（同步等待完成）。
func (a *API) HandleKernelInstall(c *gin.Context) {
	var req installReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if a.core.Status().Running {
		Fail(c, http.StatusConflict, "内核运行中，请先停止后再安装")
		return
	}
	tag, err := a.updates.Install(c.Request.Context(), req.Version)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	a.core.InvalidateVersion()
	OK(c, gin.H{"version": tag, "status": a.kernelStatus(c)})
}

// HandleMirrorGet GET /api/kernel/mirror 下载镜像前缀设置。
func (a *API) HandleMirrorGet(c *gin.Context) {
	OK(c, gin.H{"prefix": a.updates.MirrorPrefix()})
}

// mirrorReq 镜像前缀设置请求。
type mirrorReq struct {
	Prefix string `json:"prefix"` // 镜像前缀，空表示直连
}

// HandleMirrorSet PUT /api/kernel/mirror 保存下载镜像前缀设置。
func (a *API) HandleMirrorSet(c *gin.Context) {
	var req mirrorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	prefix, err := a.updates.SetMirrorPrefix(req.Prefix)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"prefix": prefix})
}
