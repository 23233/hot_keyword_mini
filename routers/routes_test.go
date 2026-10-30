// Package routers routes_test.go
package routers

import (
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
)

// TestActionRouteKeepsTenantMiddleware 回归 Iris 子 Party 复用父中间件切片导致的串链问题。
// 动作端点必须是「全局中间件 + 租户中间件 + 动作处理器」，不能被管理员中间件拦截，也不能丢失租户识别。
func TestActionRouteKeepsTenantMiddleware(t *testing.T) {
	app := iris.New()
	RegisterRouters(app)
	if err := app.Build(); err != nil {
		t.Fatalf("构建路由失败: %v", err)
	}

	for _, route := range app.GetRoutes() {
		if route.Method != "POST" || route.Path != "/api/v1/action/execute" {
			continue
		}
		names := make([]string, 0, len(route.Handlers))
		for _, handler := range route.Handlers {
			names = append(names, context.HandlerName(handler))
		}
		joined := strings.Join(names, ",")
		if !strings.Contains(joined, "TenantMiddleware") {
			t.Fatalf("动作端点缺少租户中间件: %s", joined)
		}
		if strings.Contains(joined, "AdminAuthMiddleware") || strings.Contains(joined, "ViewerReadOnlyMiddleware") {
			t.Fatalf("管理员中间件串入公开动作端点: %s", joined)
		}
		return
	}
	t.Fatal("未找到 POST /api/v1/action/execute 路由")
}
