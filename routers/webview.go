// Package routers webview.go
package routers

import (
	"hot_keyword/jwtToken"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

type createWebViewTicketRequest struct {
	URLKey string `json:"url_key"`
}

// CreateWebViewTicketHandler 为已登录小程序用户创建一次性 WebView 地址。
func CreateWebViewTicketHandler(ctx iris.Context) {
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	var req createWebViewTicketRequest
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.URLKey) == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "WebView url_key 不能为空"})
		return
	}
	authHeader := ctx.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "请先完成微信登录"})
		return
	}
	_, user, _, err := jwtToken.ValidateTokenSessionAndTenant(strings.TrimPrefix(authHeader, "Bearer "), appID)
	if err != nil || user == nil || user.ID <= 0 {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "登录态无效，请重新授权"})
		return
	}
	entry, err := services.ValidateWebViewKey(appID, req.URLKey)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	target, err := services.NewWebViewTicketService().CreateTicket(appID, user.ID, entry.URL)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{"url": target, "expires_in": 120}})
}

// ConsumeWebViewTicketHandler 原子消费 WebView 票据，供目标 H5 后端反向解析用户身份。
func ConsumeWebViewTicketHandler(ctx iris.Context) {
	ticket, err := services.NewWebViewTicketService().ConsumeTicket(ctx.URLParam("ticket"))
	if err != nil {
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": err.Error()})
		return
	}
	_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{
		"app_id":     ticket.AppID,
		"user_id":    ticket.UserID,
		"target_url": ticket.TargetURL,
	}})
}
