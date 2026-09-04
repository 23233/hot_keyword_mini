// Package middleware admin_auth.go
package middleware

import (
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

// AdminAuthMiddleware 保护管理后台 API 接口，强校验管理员专属凭证
func AdminAuthMiddleware(ctx iris.Context) {
	path := ctx.Path()
	// 放行公开的管理员登录认证接口
	if strings.HasSuffix(path, "/admin/auth/login") {
		ctx.Next()
		return
	}

	// 1. 优先从 Authorization 头提取 Bearer Token
	authHeader := ctx.GetHeader("Authorization")
	var tokenStr string
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 2. 备选：从 Cookie 中提取 admin_token
	if tokenStr == "" {
		tokenStr = ctx.GetCookie("admin_token")
	}

	if tokenStr == "" {
		ctx.StatusCode(iris.StatusUnauthorized)
		ctx.JSON(iris.Map{
			"code": 401,
			"msg":  "未授权访问：请先登录管理员账户",
		})
		ctx.StopExecution()
		return
	}

	// 3. 校验 Token 有效性
	authService := services.NewAdminAuthService()
	claims, err := authService.ParseAdminToken(tokenStr)
	if err != nil {
		ctx.StatusCode(iris.StatusUnauthorized)
		ctx.JSON(iris.Map{
			"code": 401,
			"msg":  "凭证已过期或无效，请重新登录",
		})
		ctx.StopExecution()
		return
	}

	// 4. 将管理员身份上下文注入请求环境
	ctx.Values().Set("admin_claims", claims)
	if adminID, ok := claims["admin_id"].(float64); ok {
		ctx.Values().Set("admin_id", int64(adminID))
	}
	ctx.Values().Set("admin_username", claims["username"])
	ctx.Values().Set("admin_role", claims["role"])

	ctx.Next()
}

// RequireAdminRole 校验管理员是否具备指定的一组权限角色之一 (如 super_admin / editor / viewer)
func RequireAdminRole(allowedRoles ...string) iris.Handler {
	return func(ctx iris.Context) {
		userRole := ctx.Values().GetString("admin_role")
		if userRole == "" {
			claims, ok := ctx.Values().Get("admin_claims").(map[string]interface{})
			if ok {
				if r, exists := claims["role"].(string); exists {
					userRole = r
				}
			}
		}

		for _, role := range allowedRoles {
			if userRole == role {
				ctx.Next()
				return
			}
		}

		ctx.StatusCode(iris.StatusForbidden)
		ctx.JSON(iris.Map{
			"code": 403,
			"msg":  "权限不足：当前角色无权执行该操作",
		})
		ctx.StopExecution()
	}
}

// ViewerReadOnlyMiddleware 对只读角色 (viewer) 实施写操作拦截门禁
func ViewerReadOnlyMiddleware(ctx iris.Context) {
	method := ctx.Method()
	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		ctx.Next()
		return
	}

	userRole := ctx.Values().GetString("admin_role")
	if userRole == "viewer" {
		ctx.StatusCode(iris.StatusForbidden)
		ctx.JSON(iris.Map{
			"code": 403,
			"msg":  "只读观察者角色禁止执行修改、删除或发布操作",
		})
		ctx.StopExecution()
		return
	}

	ctx.Next()
}
