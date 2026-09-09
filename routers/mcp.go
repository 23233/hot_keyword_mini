// Package routers mcp.go
package routers

import (
	"hot_keyword/services"
	"io"

	"github.com/kataras/iris/v12"
)

// HandleMCPRequest 处理标准 MCP JSON-RPC 2.0 传输协议请求
func HandleMCPRequest(ctx iris.Context) {
	const maxMCPRequestBytes = 4 * 1024 * 1024
	if ctx.Request().ContentLength > maxMCPRequestBytes {
		ctx.StatusCode(413)
		_ = ctx.JSON(iris.Map{"jsonrpc": "2.0", "error": iris.Map{"code": -32600, "message": "MCP 请求体不能超过 4MB"}, "id": nil})
		return
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request().Body, maxMCPRequestBytes+1))
	if err != nil {
		ctx.JSON(iris.Map{
			"jsonrpc": "2.0",
			"id":      nil,
			"error": iris.Map{
				"code":    -32700,
				"message": "读取请求内容失败: " + err.Error(),
			},
		})
		return
	}
	if len(body) > maxMCPRequestBytes {
		ctx.StatusCode(413)
		_ = ctx.JSON(iris.Map{"jsonrpc": "2.0", "error": iris.Map{"code": -32600, "message": "MCP 请求体不能超过 4MB"}, "id": nil})
		return
	}

	mcpService := services.NewMCPService()
	actorID := ctx.Values().GetStringDefault("mcp_actor_id", "mcp_anonymous")
	tenantID := ctx.Values().GetStringDefault("mcp_tenant_id", "")
	var scopes []string
	if rawScopes := ctx.Values().Get("mcp_scopes"); rawScopes != nil {
		if s, ok := rawScopes.([]string); ok {
			scopes = s
		}
	}

	respBytes, err := mcpService.HandleJSONRPCWithContext(actorID, tenantID, scopes, body)
	if err != nil {
		ctx.JSON(iris.Map{
			"jsonrpc": "2.0",
			"id":      nil,
			"error": iris.Map{
				"code":    -32603,
				"message": "内部处理异常: " + err.Error(),
			},
		})
		return
	}
	if len(respBytes) == 0 {
		ctx.StatusCode(202)
		return
	}

	ctx.ContentType("application/json")
	_, _ = ctx.Write(respBytes)
}
