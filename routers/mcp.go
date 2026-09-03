// Package routers mcp.go
package routers

import (
	"hot_keyword/services"
	"io"

	"github.com/kataras/iris/v12"
)

// HandleMCPRequest 处理标准 MCP JSON-RPC 2.0 传输协议请求
func HandleMCPRequest(ctx iris.Context) {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		ctx.JSON(iris.Map{
			"jsonrpc": "2.0",
			"error": iris.Map{
				"code":    -32700,
				"message": "读取请求内容失败: " + err.Error(),
			},
		})
		return
	}

	mcpService := services.NewMCPService()
	respBytes, err := mcpService.HandleJSONRPC(body)
	if err != nil {
		ctx.JSON(iris.Map{
			"jsonrpc": "2.0",
			"error": iris.Map{
				"code":    -32603,
				"message": "内部处理异常: " + err.Error(),
			},
		})
		return
	}

	ctx.ContentType("application/json")
	_, _ = ctx.Write(respBytes)
}
