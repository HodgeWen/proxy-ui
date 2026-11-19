package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"proxy-ui/internal/db"
)

// setupReq 首次设置请求。
type setupReq struct {
	Username string `json:"username" binding:"required,min=3,max=64"` // 用户名
	Password string `json:"password" binding:"required,min=8,max=72"` // 密码（8-72 字节）
}

// HandleSetup 首次设置：创建唯一管理员并自动登录；已初始化后重复调用被拒。
func (a *API) HandleSetup(c *gin.Context) {
	if a.Initialized() {
		Fail(c, http.StatusConflict, "面板已初始化，不可重复设置")
		return
	}
	var req setupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "密码处理失败")
		return
	}
	admin := db.Admin{Username: req.Username, PasswordHash: string(hash)}
	if err := a.db.Create(&admin).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "创建管理员失败")
		return
	}
	a.setSession(c)
	OK(c, gin.H{"username": admin.Username})
}
