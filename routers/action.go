// Package routers action.go
package routers

import (
	"hot_keyword/services"

	"github.com/kataras/iris/v12"
)

// ExecuteActionReq 受控业务动作执行请求
type ExecuteActionReq struct {
	// 已登记的受控端点名称 (如 game.redeem)
	Endpoint string `json:"endpoint"`
	// 业务参数载荷
	Payload map[string]interface{} `json:"payload"`
	// 客户端幂等键
	IdempotencyKey string `json:"idempotency_key"`
}

// ExecuteActionHandler 受控动作执行统一入口 (杜绝开放网络代理)
func ExecuteActionHandler(ctx iris.Context) {
	appID := ctx.Values().GetString("app_id")
	if appID == "" {
		appID = ctx.GetHeader("X-App-Id")
	}
	if appID == "" {
		appID = "wx516563cfe994bbc6"
	}

	var req ExecuteActionReq
	if err := ctx.ReadJSON(&req); err != nil || req.Endpoint == "" {
		ctx.JSON(iris.Map{"code": 400, "msg": "参数错误，endpoint 不能为空"})
		return
	}

	// 尝试从登录态上下文中提取真实 open_id
	openID := ctx.Values().GetString("open_id")

	actionService := services.NewActionEndpointService()
	res, err := actionService.ExecuteActionEndpoint(appID, openID, req.Endpoint, req.Payload, req.IdempotencyKey)
	if err != nil {
		ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
		return
	}

	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "操作成功",
		"data": res,
	})
}
