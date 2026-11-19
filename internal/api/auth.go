package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"proxy-ui/internal/db"
)

// loginReq 登录请求。
type loginReq struct {
	Username string `json:"username" binding:"required"` // 用户名
	Password string `json:"password" binding:"required"` // 密码
}

// HandleLogin 管理员登录，成功后写入会话 Cookie。
func (a *API) HandleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var admin db.Admin
	err := a.db.Where("username = ?", req.Username).First(&admin).Error
	if err == nil {
		err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password))
	}
	if err != nil {
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	a.setSession(c)
	OK(c, gin.H{"username": admin.Username})
}

// HandleLogout 注销当前会话。
func (a *API) HandleLogout(c *gin.Context) {
	if token, err := c.Cookie(SessionCookie); err == nil {
		a.sessions.Remove(token)
	}
	c.SetCookie(SessionCookie, "", -1, "/", "", false, true)
	OK(c, nil)
}

// HandleMe 返回当前管理员信息。
func (a *API) HandleMe(c *gin.Context) {
	var admin db.Admin
	if err := a.db.First(&admin).Error; err != nil {
		Fail(c, http.StatusUnauthorized, "管理员不存在")
		return
	}
	OK(c, gin.H{"username": admin.Username, "createdAt": admin.CreatedAt})
}

// changePasswordReq 修改密码请求。
type changePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`              // 原密码
	NewPassword string `json:"newPassword" binding:"required,min=8,max=72"` // 新密码（8-72 字节）
}

// HandleChangePassword 校验原密码后修改管理员密码。
func (a *API) HandleChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	var admin db.Admin
	if err := a.db.First(&admin).Error; err != nil {
		Fail(c, http.StatusUnauthorized, "管理员不存在")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.OldPassword)) != nil {
		Fail(c, http.StatusBadRequest, "原密码错误")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "密码处理失败")
		return
	}
	if err := a.db.Model(&admin).Update("password_hash", string(hash)).Error; err != nil {
		Fail(c, http.StatusInternalServerError, "保存密码失败")
		return
	}
	OK(c, nil)
}
