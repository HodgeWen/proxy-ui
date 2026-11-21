package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"proxy-ui/internal/db"
)

// ginCtx 构造带路径参数与 JSON 请求体的 gin 测试上下文（handler 级测试共用）。
func ginCtx(method, target, body string, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = params
	return c, w
}

// TestHandleRuleSetUpdateRejectsBuiltinName 重命名不能改成内置规则集名：
// 内置默认源按名字补齐，被冒名后该内置规则集永远补不回来。
func TestHandleRuleSetUpdateRejectsBuiltinName(t *testing.T) {
	a := newInboundTestAPI(t)
	set, err := a.createRuleSet(ruleSetReq{Name: "custom-set", Description: "自定义"})
	if err != nil {
		t.Fatalf("创建规则集失败: %v", err)
	}
	target := "/api/rule-sets/" + strconv.Itoa(int(set.ID))
	params := gin.Params{{Key: "id", Value: strconv.Itoa(int(set.ID))}}

	for _, builtin := range []string{"anti-ad", "geosite-cn", "geoip-cn"} {
		c, w := ginCtx(http.MethodPut, target, `{"name":"`+builtin+`"}`, params)
		a.HandleRuleSetUpdate(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("重命名为内置名 %s 应返回 400, got %d: %s", builtin, w.Code, w.Body.String())
		}
		var got db.RuleSet
		if err := a.db.First(&got, set.ID).Error; err != nil {
			t.Fatal(err)
		}
		if got.Name != "custom-set" {
			t.Fatalf("重命名被拒后名称不应变化, got %q", got.Name)
		}
	}

	// 普通重命名不受挡板影响
	c, w := ginCtx(http.MethodPut, target, `{"name":"renamed"}`, params)
	a.HandleRuleSetUpdate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("普通重命名应成功, got %d: %s", w.Code, w.Body.String())
	}
	var got db.RuleSet
	if err := a.db.First(&got, set.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Name != "renamed" {
		t.Errorf("重命名后名称 = %q, want renamed", got.Name)
	}

	// 内置规则集保持同名更新说明不受挡板影响
	builtin := db.RuleSet{Name: "anti-ad", Description: "旧说明", Builtin: true}
	if err := a.db.Create(&builtin).Error; err != nil {
		t.Fatal(err)
	}
	builtinTarget := "/api/rule-sets/" + strconv.Itoa(int(builtin.ID))
	c, w = ginCtx(http.MethodPut, builtinTarget, `{"name":"anti-ad","description":"新说明"}`,
		gin.Params{{Key: "id", Value: strconv.Itoa(int(builtin.ID))}})
	a.HandleRuleSetUpdate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("内置规则集同名更新应成功, got %d: %s", w.Code, w.Body.String())
	}
	var updated db.RuleSet
	if err := a.db.First(&updated, builtin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Description != "新说明" {
		t.Errorf("说明应已更新, got %q", updated.Description)
	}
}
