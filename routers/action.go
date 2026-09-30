// Package routers action.go
package routers

import (
	"crypto/sha256"
	"encoding/hex"
	"hot_keyword/jwtToken"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

// stableGuestKey 基于租户与客户端来源地址派生稳定的匿名访客标识 (前缀 guest_ 表示未登录身份)
func stableGuestKey(ctx iris.Context, appID string) string {
	sum := sha256.Sum256([]byte(appID + "|" + ctx.RemoteAddr()))
	return "guest_" + hex.EncodeToString(sum[:])[:24]
}

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
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}

	var req ExecuteActionReq
	if err := ctx.ReadJSON(&req); err != nil || req.Endpoint == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "参数错误，endpoint 不能为空"})
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

	// 匿名请求派生稳定访客指纹：同一来源地址在限领/频控判定中保持一致身份，
	// 防止每次请求随机 guest_ 标识绕过"同一访客限领一次"与频控校验。
	if openID == "" {
		openID = stableGuestKey(ctx, appID)
	}

	actionService := services.NewActionEndpointService()
	res, err := actionService.ExecuteActionEndpoint(appID, openID, req.Endpoint, req.Payload, req.IdempotencyKey)

	if err != nil {
		status := iris.StatusInternalServerError
		errMsg := err.Error()
		if strings.Contains(errMsg, "授权登录") || strings.Contains(errMsg, "登录") {
			status = iris.StatusUnauthorized
		} else if strings.Contains(errMsg, "不存在") || strings.Contains(errMsg, "领完") || strings.Contains(errMsg, "不能为空") || strings.Contains(errMsg, "未登记") {
			status = iris.StatusBadRequest
		}
		ctx.StatusCode(status)
		_ = ctx.JSON(iris.Map{"code": status, "msg": errMsg})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "操作成功",
		"data": res,
	})
}
