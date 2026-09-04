// Package routers action.go
package routers

import (
	"hot_keyword/jwtToken"
	"hot_keyword/services"
	"strings"

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
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "无法识别当前小程序租户"})
		return
	}

	var req ExecuteActionReq
	if err := ctx.ReadJSON(&req); err != nil || req.Endpoint == "" {
		ctx.JSON(iris.Map{"code": 400, "msg": "参数错误，endpoint 不能为空"})
		return
	}

	// 尝试从登录态上下文中提取真实 open_id，未提取到时严格解析并校验 Authorization Header
	openID := ctx.Values().GetString("open_id")
	if openID == "" {
		authHeader := ctx.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			_, user, claims, err := jwtToken.ValidateTokenSessionAndTenant(tokenStr, appID)
			if err == nil {
				if user != nil && user.WechatOpenID != "" {
					openID = user.WechatOpenID
				} else if oid, ok := claims["openId"].(string); ok {
					openID = oid
				}
			}
		}
	}

	// 敏感端点要求前置登录态校验
	if req.Endpoint == "game.redeem" && openID == "" {
		ctx.StatusCode(iris.StatusUnauthorized)
		ctx.JSON(iris.Map{
			"code": 401,
			"msg":  "执行兑换动作前必须完成微信登录授权",
		})
		return
	}

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
