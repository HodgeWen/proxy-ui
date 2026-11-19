package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// envelope 统一响应结构。
type envelope struct {
	Success bool   `json:"success"` // 是否成功
	Msg     string `json:"msg"`     // 提示信息
	Data    any    `json:"data"`    // 数据载荷
}

// enabledReq 通用启停请求。
type enabledReq struct {
	Enabled bool `json:"enabled"` // 目标状态
}

// OK 输出统一成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Success: true, Msg: "ok", Data: data})
}

// Fail 输出统一失败响应（HTTP 状态码 + envelope msg）。
func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, envelope{Success: false, Msg: msg})
}

// idParam 从路径参数解析资源 ID；非法时已输出 400 并返回 false。
func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Fail(c, http.StatusBadRequest, "参数错误: 非法的 ID")
		return 0, false
	}
	return uint(id), true
}
