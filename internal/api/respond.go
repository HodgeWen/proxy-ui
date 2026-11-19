package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// envelope 统一响应结构。
type envelope struct {
	Success bool   `json:"success"` // 是否成功
	Msg     string `json:"msg"`     // 提示信息
	Data    any    `json:"data"`    // 数据载荷
}

// OK 输出统一成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Success: true, Msg: "ok", Data: data})
}

// Fail 输出统一失败响应（HTTP 状态码 + envelope msg）。
func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, envelope{Success: false, Msg: msg})
}
