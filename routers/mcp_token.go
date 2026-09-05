// Package routers mcp_token.go
package routers

import (
	"hot_keyword/routers/middleware"
	"hot_keyword/services"

	"github.com/kataras/iris/v12"
)

// RegisterMCPTokenRoutes 注册管理后台 MCP Token 管理接口。
func RegisterMCPTokenRoutes(adminParty iris.Party) {
	adminParty.Get("/mcp-tokens", func(ctx iris.Context) {
		list, err := services.ListMCPAccessTokens()
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "success", "data": list})
	})

	adminParty.Post("/mcp-tokens", middleware.RequireAdminRole("super_admin", "admin"), func(ctx iris.Context) {
		var req struct {
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		}
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "MCP Token 参数无效"})
			return
		}
		record, rawToken, err := services.CreateMCPAccessToken(req.Name, req.Scopes)
		if err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "MCP Token 已创建", "data": iris.Map{"token": rawToken, "record": record}})
	})

	adminParty.Delete("/mcp-tokens/{id:int64}", middleware.RequireAdminRole("super_admin", "admin"), func(ctx iris.Context) {
		id, _ := ctx.Params().GetInt64("id")
		if id <= 0 {
			ctx.JSON(iris.Map{"code": 400, "msg": "MCP Token ID 无效"})
			return
		}
		if err := services.DeleteMCPAccessToken(id); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "MCP Token 已删除"})
	})
}
