// Package middleware tenant.go
package middleware

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kataras/iris/v12"
)

const (
	// TenantContextKey 上下文中存储小程序租户 AppID 的键名
	TenantContextKey = "app_id"
)

var wechatRefererAppID = regexp.MustCompile(`(?i)servicewechat\.com/([a-z0-9_-]{1,64})/`)

// TenantMiddleware 多小程序多租户隔离识别中间件。
// 优先从微信请求的 servicewechat.com Referer 或官方 AppID 头解析租户，不依赖客户端自定义 X-App-Id。
func TenantMiddleware(ctx iris.Context) {
	appID := strings.TrimSpace(ctx.GetHeader("X-WX-AppID"))
	if appID == "" {
		appID = strings.TrimSpace(ctx.GetHeader("X-WX-AppId"))
	}
	if appID == "" {
		if matches := wechatRefererAppID.FindStringSubmatch(ctx.GetHeader("Referer")); len(matches) == 2 {
			appID = matches[1]
		}
	}
	// 管理端、MCP 和分享截图等内部接口仍允许显式 query 选择租户。
	if appID == "" {
		appID = strings.TrimSpace(ctx.URLParam("app_id"))
	}

	ctx.Values().Set(TenantContextKey, appID)
	ctx.Next()
}

// GetTenantAppID 从 Iris 请求上下文中快速安全获取当前租户 AppID
func GetTenantAppID(ctx iris.Context) string {
	if val := ctx.Values().GetString(TenantContextKey); val != "" {
		return val
	}
	return ""
}

// RequireTenantAppID 确保需要租户隔离的公开接口不会静默落到默认小程序。
func RequireTenantAppID(ctx iris.Context) (string, error) {
	appID := GetTenantAppID(ctx)
	if appID == "" {
		return "", fmt.Errorf("无法从微信 Referer 或官方 AppID 标识解析小程序租户")
	}
	return appID, nil
}
