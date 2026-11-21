package api

import (
	"crypto/rand"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"proxy-ui/internal/db"
)

// setupCodeLen 设置码长度。
const setupCodeLen = 8

// setupCodeCharset 设置码字符集：32 个字符、无 I/L/O/U 等易混淆字符；
// 长度恰为 2 的幂，随机字节直接取模映射无偏。
const setupCodeCharset = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newSetupCode 用 crypto/rand 生成一次性设置码。
func newSetupCode() string {
	buf := make([]byte, setupCodeLen)
	if _, err := rand.Read(buf); err != nil {
		panic("生成设置码失败: " + err.Error()) // 系统熵源不可用，无法安全完成首次设置
	}
	code := make([]byte, setupCodeLen)
	for i, b := range buf {
		code[i] = setupCodeCharset[int(b)%len(setupCodeCharset)]
	}
	return string(code)
}

// setupReq 首次设置请求。
type setupReq struct {
	Username  string `json:"username" binding:"required,min=3,max=64"` // 用户名
	Password  string `json:"password" binding:"required,min=8,max=72"` // 密码（8-72 字节）
	SetupCode string `json:"setupCode" binding:"required"`             // 一次性设置码（面板启动日志中查看）
}

// HandleSetup 首次设置：校验一次性设置码后创建唯一管理员并自动登录；已初始化后重复调用被拒。
func (a *API) HandleSetup(c *gin.Context) {
	if a.Initialized() {
		Fail(c, http.StatusConflict, "面板已初始化，不可重复设置")
		return
	}
	ip := c.ClientIP()
	if !a.limiter.allow(ip) {
		Fail(c, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}
	var req setupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if !a.verifySetupCode(req.SetupCode) {
		a.limiter.recordFail(ip)
		Fail(c, http.StatusUnauthorized, "设置码错误，请核对服务器启动日志")
		return
	}
	// 并发竞态保护：占位成功者独占建管理员，其余并发请求按已初始化拒绝
	if !a.claimSetup() {
		Fail(c, http.StatusConflict, "面板已初始化，不可重复设置")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.releaseSetup()
		Fail(c, http.StatusInternalServerError, "密码处理失败")
		return
	}
	admin := db.Admin{Username: req.Username, PasswordHash: string(hash)}
	if err := a.db.Create(&admin).Error; err != nil {
		a.releaseSetup()
		Fail(c, http.StatusInternalServerError, "创建管理员失败")
		return
	}
	a.clearSetupCode()
	a.limiter.reset(ip)
	a.setSession(c)
	OK(c, gin.H{"username": admin.Username})
}

// verifySetupCode 常数时间比较设置码（忽略大小写）；码已消费或未生成时一律拒绝。
func (a *API) verifySetupCode(code string) bool {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	return a.setupCode != "" &&
		subtle.ConstantTimeCompare([]byte(strings.ToUpper(code)), []byte(a.setupCode)) == 1
}

// clearSetupCode 使设置码失效（设置成功后调用）。
func (a *API) clearSetupCode() {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.setupCode = ""
}

// claimSetup 原子占位首次设置，确保并发请求至多一个创建管理员。
func (a *API) claimSetup() bool {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	if a.setupClaimed {
		return false
	}
	a.setupClaimed = true
	return true
}

// releaseSetup 建管理员失败时释放占位，允许重试。
func (a *API) releaseSetup() {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.setupClaimed = false
}
