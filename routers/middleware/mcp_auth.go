// Package middleware mcp_auth.go
package middleware

import (
	"fmt"
	"os"
	"strings"

	"hot_keyword/services"

	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"
)

// defaultDevMCPKey 优先读取环境变量 MCP_AUTH_KEY，非生产环境生成运行时一次性临时密钥
var defaultDevMCPKey = func() string {
	if k := os.Getenv("MCP_AUTH_KEY"); k != "" {
		return k
	}
	return "mcp_key_" + ut.RandomStr(32)
}()

// MCPAuthMiddleware AI Model Context Protocol (MCP) 专用身份认证与权限隔离中间件
// 杜绝外部匿名访问，支持 API Key 与管理员 JWT 双通道安全验证
func MCPAuthMiddleware(ctx iris.Context) {
	// 1. 尝试从 X-MCP-Key 提取预共享凭证
	mcpKey := ctx.GetHeader("X-MCP-Key")
	expectedKey := defaultDevMCPKey

	authHeader := ctx.GetHeader("Authorization")
	var bearerToken string
	if strings.HasPrefix(authHeader, "Bearer ") {
		bearerToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	actorID := ctx.GetHeader("X-MCP-Actor")
	if actorID == "" {
		actorID = "mcp_ai_agent"
	}

	// 2. 验证凭据
	authenticated := false
	scopes := []string{"read", "write:draft"} // 默认最低安全权限

	// API Key 允许的最大受控权限范围 (默认仅允许读与草稿写入，严禁外部 Key 自行声明 release 权限)
	allowedScopes := []string{"read", "write:draft"}
	if customAllowed := os.Getenv("MCP_ALLOWED_SCOPES"); customAllowed != "" {
		allowedScopes = strings.Split(customAllowed, ",")
	}

	if mcpKey != "" && mcpKey == expectedKey {
		authenticated = true
		// 密钥通道仅支持通过请求头请求权限子集，严禁超出服务端允许的最大受控权限
		headerScopes := ctx.GetHeader("X-MCP-Scopes")
		if headerScopes != "" {
			requested := strings.Split(headerScopes, ",")
			var validScopes []string
			for _, reqScope := range requested {
				reqScope = strings.TrimSpace(reqScope)
				for _, allowed := range allowedScopes {
					if reqScope == allowed {
						validScopes = append(validScopes, reqScope)
						break
					}
				}
			}
			scopes = validScopes
		} else {
			scopes = allowedScopes
		}
	} else if mcpKey != "" {
		if tokenRecord, err := services.AuthenticateMCPAccessToken(mcpKey); err == nil {
			authenticated = true
			actorID = "mcp_token_" + tokenRecord.Name
			scopes = services.ParseMCPTokenScopes(tokenRecord.Scopes)
		}
	}
	if !authenticated && bearerToken != "" {
		// 尝试通过管理员 JWT 校验
		adminAuth := services.NewAdminAuthService()
		claims, err := adminAuth.ParseAdminToken(bearerToken)
		if err == nil && claims != nil {
			authenticated = true
			actorID = "admin_" + claims["username"].(string)
			role, _ := claims["role"].(string)
			if role == "super_admin" || role == "admin" {
				scopes = []string{"read", "write:draft", "release"}
			} else if role == "editor" {
				scopes = []string{"read", "write:draft"}
			} else {
				scopes = []string{"read"}
			}
		}
	}

	if !authenticated {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{
			"jsonrpc": "2.0",
			"error": iris.Map{
				"code":    -32001,
				"message": "未授权的 MCP 访问: 请在请求头提供有效的 X-MCP-Key 或 Authorization Bearer 凭证",
			},
			"id": nil,
		})
		ctx.StopExecution()
		return
	}

	// 3. 将身份与授权范围注入请求上下文
	ctx.Values().Set("mcp_actor_id", actorID)
	ctx.Values().Set("mcp_scopes", scopes)

	// 4. 强校验 MCP 租户受控绑定 (杜绝持钥者通过可控 X-App-Id 任意漫游越权)
	var allowedTenants []string
	if envTenants := os.Getenv("MCP_ALLOWED_TENANTS"); envTenants != "" {
		for _, t := range strings.Split(envTenants, ",") {
			if t = strings.TrimSpace(t); t != "" {
				allowedTenants = append(allowedTenants, t)
			}
		}
	} else if envTenant := os.Getenv("MCP_TENANT_ID"); envTenant != "" {
		allowedTenants = []string{envTenant}
	}

	requestedAppID := strings.TrimSpace(ctx.GetHeader("X-App-Id"))
	if requestedAppID == "" {
		requestedAppID = strings.TrimSpace(ctx.URLParam("app_id"))
	}

	if requestedAppID != "" {
		isAllowed := len(allowedTenants) == 0
		for _, allowed := range allowedTenants {
			if requestedAppID == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			ctx.StatusCode(iris.StatusForbidden)
			_ = ctx.JSON(iris.Map{
				"jsonrpc": "2.0",
				"error": iris.Map{
					"code":    -32001,
					"message": fmt.Sprintf("MCP租户越权拦截: 当前凭证仅授权访问租户 %v，无权操作租户 %s", allowedTenants, requestedAppID),
				},
				"id": nil,
			})
			ctx.StopExecution()
			return
		}
	}

	// 全局 MCP Token 不绑定 Header 中的小程序；具体目标由工具 arguments.app_id 决定。
	ctx.Values().Set("mcp_tenant_id", "")
	ctx.Next()
}
