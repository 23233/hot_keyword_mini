// Package routers auth.go
package routers

import (
	"hot_keyword/jwtToken"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"

	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"
)

// WechatLoginReq 微信小程序免密登录入参
type WechatLoginReq struct {
	// 小程序 AppID (可选，公开请求为空时由微信 Referer/X-WX-AppID 识别)
	AppID string `json:"app_id"`
	// wx.login 返回的临时授权凭证 code (必填)
	Code string `json:"code"`
}

// RefreshSessionReq 刷新凭证换取双 Token 入参
type RefreshSessionReq struct {
	// 小程序 AppID (可选)
	AppID string `json:"app_id"`
	// 长期刷新凭证 refresh_token (必填)
	RefreshToken string `json:"refresh_token"`
}

// LogoutReq 用户登出入参
type LogoutReq struct {
	// 指定撤销的会话ID (可选，默认使用当前请求携带的 SessionID)
	SessionID string `json:"session_id"`
}

// RegisterAuthRoutes 注册双 Token 微信登录与会话生命周期路由
func RegisterAuthRoutes(party iris.Party) {
	authParty := party.Party("/auth")
	{
		// 微信登录换取双 Token (短期 Access Token + 长期 Refresh Token)
		authParty.Post("/wechat-login", WechatLoginHandler)
		// 刷新凭证无感续期 (支持 Token 轮换与重放防御)
		authParty.Post("/refresh", RefreshSessionHandler)
		// 查询当前会话有效状态 (需要 Bearer Token 鉴权)
		authParty.Get("/session", jwtToken.CustomJwt.Serve, jwtToken.TokenToUserUidMiddleware, GetSessionInfoHandler)
		// 主动登出并撤销当前会话
		authParty.Post("/logout", jwtToken.CustomJwt.Serve, jwtToken.TokenToUserUidMiddleware, LogoutHandler)
	}
}

// WechatLoginHandler 微信免密登录接口控制器
func WechatLoginHandler(ctx iris.Context) {
	var req WechatLoginReq
	if err := ctx.ReadJSON(&req); err != nil || req.Code == "" {
		_ = ctx.JSON(iris.Map{
			"code": 400,
			"msg":  "微信授权 code 不能为空",
		})
		return
	}

	appID, _ := middleware.RequireTenantAppID(ctx)
	if appID == "" {
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "无法识别当前小程序租户"})
		return
	}

	srv := services.NewAuthService()
	res, err := srv.WechatLogin(ctx.Request().Context(), appID, req.Code)
	if err != nil {
		ut.IrisErrLog(ctx, err, "微信登录换取会话失败")
		_ = ctx.JSON(iris.Map{
			"code": 500,
			"msg":  "微信登录失败: " + err.Error(),
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": res,
	})
}

// RefreshSessionHandler 刷新令牌换取新凭证接口控制器
func RefreshSessionHandler(ctx iris.Context) {
	var req RefreshSessionReq
	if err := ctx.ReadJSON(&req); err != nil || req.RefreshToken == "" {
		_ = ctx.JSON(iris.Map{
			"code": 400,
			"msg":  "refresh_token 不能为空",
		})
		return
	}

	appID, _ := middleware.RequireTenantAppID(ctx)
	if appID == "" {
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "无法识别当前小程序租户"})
		return
	}

	srv := services.NewAuthService()
	res, err := srv.RefreshSession(appID, req.RefreshToken)
	if err != nil {
		ut.IrisErrLog(ctx, err, "刷新会话令牌失败")
		_ = ctx.JSON(iris.Map{
			"code": 401,
			"msg":  err.Error(),
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": res,
	})
}

// GetSessionInfoHandler 查询当前会话状态控制器
func GetSessionInfoHandler(ctx iris.Context) {
	sessionID := ctx.Values().GetString(jwtToken.JwtSessionId)
	appID := ctx.Values().GetString(jwtToken.JwtAppId)
	if appID == "" {
		appID = middleware.GetTenantAppID(ctx)
	}

	srv := services.NewAuthService()
	res, err := srv.GetSessionInfo(sessionID, appID)
	if err != nil {
		_ = ctx.JSON(iris.Map{
			"code": 404,
			"msg":  err.Error(),
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": res,
	})
}

// LogoutHandler 登出接口控制器 (撤销会话)
func LogoutHandler(ctx iris.Context) {
	var req LogoutReq
	_ = ctx.ReadJSON(&req)

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = ctx.Values().GetString(jwtToken.JwtSessionId)
	}

	if sessionID != "" {
		srv := services.NewAuthService()
		_ = srv.RevokeSession(sessionID, "user_logout")
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "已成功退出登录",
	})
}
