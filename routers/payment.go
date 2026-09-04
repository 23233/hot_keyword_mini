// Package routers payment.go
package routers

import (
	"hot_keyword/db"
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

type createPaymentOrderRequest struct {
	SKU            string `json:"sku"`
	IdempotencyKey string `json:"idempotency_key"`
}

// PaymentOrderStatusHandler 查询当前用户在本小程序下的支付订单状态。
func PaymentOrderStatusHandler(ctx iris.Context) {
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	authHeader := ctx.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "请先完成微信登录"})
		return
	}
	_, user, _, err := jwtToken.ValidateTokenSessionAndTenant(strings.TrimPrefix(authHeader, "Bearer "), appID)
	if err != nil || user == nil {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "登录态无效，请重新授权"})
		return
	}
	tradeNo := strings.TrimSpace(ctx.URLParam("out_trade_no"))
	if tradeNo == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "订单号不能为空"})
		return
	}
	order, err := services.NewPaymentService().GetOrderStatus(ctx.Request().Context(), appID, user.ID, tradeNo)
	if err != nil {
		ctx.StatusCode(iris.StatusNotFound)
		_ = ctx.JSON(iris.Map{"code": 404, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": order})
}

// CreatePaymentOrderHandler 创建商品支付订单并返回 wx.requestPayment 参数。
func CreatePaymentOrderHandler(ctx iris.Context) {
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	authHeader := ctx.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "请先完成微信登录"})
		return
	}
	_, user, claims, err := jwtToken.ValidateTokenSessionAndTenant(strings.TrimPrefix(authHeader, "Bearer "), appID)
	if err != nil || user == nil {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "登录态无效，请重新授权"})
		return
	}
	openID := user.WechatOpenID
	if openID == "" {
		openID, _ = claims["openId"].(string)
	}
	var req createPaymentOrderRequest
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.SKU) == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "商品 SKU 不能为空"})
		return
	}
	order, params, err := services.NewPaymentService().CreateJSAPIOrder(ctx.Request().Context(), appID, user.ID, openID, req.SKU, req.IdempotencyKey)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{"order": order, "payment": params}})
}

// PaymentNotifyHandler 接收微信支付通知并执行验签、解密和幂等更新。
func PaymentNotifyHandler(ctx iris.Context) {
	// 微信支付通知不携带自定义租户头，必须通过通知 URL 路径识别 AppID。
	appID := strings.TrimSpace(ctx.URLParam("app_id"))
	if appID == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": "FAIL", "message": "支付通知地址缺少 app_id"})
		return
	}
	var app models.MiniApp
	if db.Mysql.Where("app_id = ?", appID).First(&app).Error != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": "FAIL", "message": "商户配置不存在"})
		return
	}
	if err := services.NewPaymentService().ApplyNotify(ctx.Request().Context(), &app, ctx.Request()); err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": "FAIL", "message": err.Error()})
		return
	}
	ctx.StatusCode(iris.StatusNoContent)
}
