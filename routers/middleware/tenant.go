// Package middleware tenant.go
package middleware

import (
	"strings"

	"github.com/kataras/iris/v12"
)

const (
	// TenantContextKey 上下文中存储小程序租户 AppID 的键名
	TenantContextKey = "app_id"
	// DefaultAppID 默认小程序 AppID (用于未携带 Header 时的平滑兜底)
	DefaultAppID = "wx516563cfe994bbc6"
)

// TenantMiddleware 多小程序多租户隔离识别中间件
// 自动从请求头 X-App-Id 或 Query app_id 提取租户信息并写入请求上下文
func TenantMiddleware(ctx iris.Context) {
	appID := strings.TrimSpace(ctx.GetHeader("X-App-Id"))
	if appID == "" {
		appID = strings.TrimSpace(ctx.URLParam("app_id"))
	}
	if appID == "" {
		appID = DefaultAppID
	}

	ctx.Values().Set(TenantContextKey, appID)
	ctx.Next()
}

// GetTenantAppID 从 Iris 请求上下文中快速安全获取当前租户 AppID
func GetTenantAppID(ctx iris.Context) string {
	if val := ctx.Values().GetString(TenantContextKey); val != "" {
		return val
	}
	return DefaultAppID
}
