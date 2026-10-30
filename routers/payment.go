// Package routers payment.go
package routers

import (
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

type createPaymentOrderItem struct {
	SKU      string `json:"sku"`
	Quantity int64  `json:"quantity"`
}

type createPaymentOrderRequest struct {
	SKU            string                   `json:"sku"`
	Items          []createPaymentOrderItem `json:"items"`
	CouponID       string                   `json:"coupon_id"`
	IdempotencyKey string                   `json:"idempotency_key"`
}

// cartPaymentItems 将购物车明细转换为领域服务入参；空的 items 返回 nil。
func cartPaymentItems(items []createPaymentOrderItem) []map[string]interface{} {
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{"sku": item.SKU, "quantity": item.Quantity})
	}
	return result
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
	tradeNo := strings.TrimSpace(ctx.Params().Get("out_trade_no"))
	if tradeNo == "" {
		tradeNo = strings.TrimSpace(ctx.URLParam("out_trade_no"))
	}
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
	// 运行时能力门禁：支付能力被禁用时，直接拒绝创建支付订单。
	if allowed, capabilityErr := services.EnsureAppCapability(appID, []string{"payment"}); capabilityErr != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		_ = ctx.JSON(iris.Map{"code": 500, "error_code": "CAPABILITY_CHECK_FAILED", "msg": "能力状态检查失败"})
		return
	} else if !allowed {
		ctx.StatusCode(iris.StatusForbidden)
		_ = ctx.JSON(iris.Map{"code": 403, "error_code": "CAPABILITY_DISABLED", "msg": "当前小程序未启用支付能力"})
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
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "支付参数格式错误"})
		return
	}
	cartItems := cartPaymentItems(req.Items)
	if len(cartItems) == 0 && strings.TrimSpace(req.SKU) == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "商品 SKU 或购物车明细不能为空"})
		return
	}
	var order interface{}
	var params interface{}
	if len(cartItems) > 0 {
		order, params, err = services.NewPaymentService().CreateCartJSAPIOrderWithCoupon(ctx.Request().Context(), appID, user.ID, openID, cartItems, req.IdempotencyKey, req.CouponID)
	} else {
		order, params, err = services.NewPaymentService().CreateJSAPIOrderWithCoupon(ctx.Request().Context(), appID, user.ID, openID, req.SKU, req.IdempotencyKey, req.CouponID)
	}
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
	appID := strings.TrimSpace(ctx.Params().Get("app_id"))
	if appID == "" {
		appID = strings.TrimSpace(ctx.URLParam("app_id"))
	}
	if appID == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": "FAIL", "message": "支付通知地址缺少 app_id"})
		return
	}
	if db.DB == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		_ = ctx.JSON(iris.Map{"code": "FAIL", "message": "数据库服务未就绪"})
		return
	}
	var app models.MiniApp
	if db.DB.Where("app_id = ?", appID).First(&app).Error != nil {
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

// CreateSandboxPaymentOrderHandler 创建本地支付沙箱订单，仅允许开发环境使用。
func CreateSandboxPaymentOrderHandler(ctx iris.Context) {
	if config.Cfg != nil && config.Cfg.IsProduction() {
		ctx.StatusCode(iris.StatusNotFound)
		_ = ctx.JSON(iris.Map{"code": 404, "msg": "生产环境不开放支付沙箱"})
		return
	}
	appID, userID, openID, ok := paymentUser(ctx)
	if !ok {
		return
	}
	var req createPaymentOrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "支付参数格式错误"})
		return
	}
	cartItems := cartPaymentItems(req.Items)
	if len(cartItems) == 0 && strings.TrimSpace(req.SKU) == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "商品 SKU 或购物车明细不能为空"})
		return
	}
	var order *models.PaymentOrder
	var err error
	if len(cartItems) > 0 {
		order, err = services.NewPaymentService().CreateSandboxCartOrderWithCoupon(appID, userID, openID, cartItems, req.IdempotencyKey, req.CouponID)
	} else {
		order, err = services.NewPaymentService().CreateSandboxOrderWithCoupon(appID, userID, openID, req.SKU, req.IdempotencyKey, req.CouponID)
	}
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{"order": order, "sandbox": true}})
}

// TransitionSandboxPaymentHandler 推进本地支付沙箱订单状态。
func TransitionSandboxPaymentHandler(ctx iris.Context) {
	if config.Cfg != nil && config.Cfg.IsProduction() {
		ctx.StatusCode(iris.StatusNotFound)
		_ = ctx.JSON(iris.Map{"code": 404, "msg": "生产环境不开放支付沙箱"})
		return
	}
	appID, userID, _, ok := paymentUser(ctx)
	if !ok {
		return
	}
	var req struct {
		OutTradeNo string `json:"out_trade_no"`
		Transition string `json:"transition"`
	}
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.OutTradeNo) == "" || strings.TrimSpace(req.Transition) == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "订单号和状态不能为空"})
		return
	}
	order, err := services.NewPaymentService().ApplySandboxTransition(appID, userID, req.OutTradeNo, req.Transition)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{"order": order, "sandbox": true}})
}

// SandboxPaymentNotifyHandler 模拟微信支付通知，供本地回调验收使用。
func SandboxPaymentNotifyHandler(ctx iris.Context) {
	if config.Cfg != nil && config.Cfg.IsProduction() {
		ctx.StatusCode(iris.StatusNotFound)
		_ = ctx.JSON(iris.Map{"code": 404, "msg": "生产环境不开放支付沙箱"})
		return
	}
	appID := strings.TrimSpace(ctx.Params().Get("app_id"))
	var req struct {
		OutTradeNo string `json:"out_trade_no"`
		Success    bool   `json:"success"`
	}
	if err := ctx.ReadJSON(&req); err != nil || appID == "" || strings.TrimSpace(req.OutTradeNo) == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "支付沙箱回调参数无效"})
		return
	}
	order, err := services.NewPaymentService().ApplySandboxNotify(appID, req.OutTradeNo, req.Success)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{"order": order, "sandbox": true}})
}

func paymentUser(ctx iris.Context) (string, int64, string, bool) {
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return "", 0, "", false
	}
	authHeader := ctx.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "请先完成微信登录"})
		return "", 0, "", false
	}
	_, user, claims, err := jwtToken.ValidateTokenSessionAndTenant(strings.TrimPrefix(authHeader, "Bearer "), appID)
	if err != nil || user == nil {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "登录态无效，请重新授权"})
		return "", 0, "", false
	}
	openID := user.WechatOpenID
	if openID == "" {
		openID, _ = claims["openId"].(string)
	}
	if openID == "" {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "登录态缺少微信用户标识"})
		return "", 0, "", false
	}
	return appID, user.ID, openID, true
}
