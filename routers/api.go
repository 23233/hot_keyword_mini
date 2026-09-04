// Package routers api.go
package routers

import (
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"hot_keyword/validator"
	"os"
	"strconv"
	"strings"

	"github.com/23233/ggg/sv"
	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"
)

func registerAPIRoutes(party iris.Party) {
	api := party.Party("/api/v1")

	// 自动识别与注入多小程序租户 AppID
	api.Use(middleware.TenantMiddleware)

	// 注册双 Token 微信登录与会话管理 API 路由
	RegisterAuthRoutes(api)

	// 注册 SDUI 动态组件页面下发 API 路由
	RegisterSDUIRoutes(api)

	// 兼容旧版：通过 code 获取 openid
	api.Post("/code", sv.Run(new(validator.WeCodeReq)), WeCodeGetInfo)

	// 注册短剧相关 API 路由
	RegisterDramaRoutes(api)

	// 注册可视化管理后台 API 路由
	RegisterAdminRoutes(party)

	// 注册微信分享卡片实时渲染公开接口
	api.Get("/share/card", RenderShareCardHandler)

	// 注册 SDUI 页面/草稿规范化无头截图端点 (与 MCP sdui.page.screenshot 保持严格视觉基线与哈希一致)
	api.Get("/sdui/screenshot", RenderSDUIScreenshotHandler)

	// 注册受控业务动作执行接口 (如游戏礼包兑换码防超发领取)
	api.Post("/action/execute", ExecuteActionHandler)
	api.Post("/payment/orders", CreatePaymentOrderHandler)
	api.Post("/payment/notify/{app_id:string}", PaymentNotifyHandler)
	api.Get("/payment/orders/{out_trade_no:string}", PaymentOrderStatusHandler)

	// 注册登录 WebView 一次性票据接口
	api.Post("/webview/ticket", CreateWebViewTicketHandler)
	api.Get("/webview/ticket/consume", ConsumeWebViewTicketHandler)

	// 注册 AI Model Context Protocol (MCP) 编排端点 (受 MCP 专属认证与多租户权限中间件保护)
	api.Post("/mcp", middleware.MCPAuthMiddleware, HandleMCPRequest)

	// Protected routes
	userParty := api.Party("/user")
	userParty.Use(jwtToken.SidAndJwtMiddleware, jwtToken.TokenToUserUidMiddleware)
	userParty.Get("/info", GetNewInfo)
}

// RenderSDUIScreenshotHandler 动态渲染 SDUI 页面或草稿规范化截图图片流 (与 MCP sdui.page.screenshot 完全同构对应)
// 对草稿截图实施时效签名与访问凭证强制校验，杜绝未发布草稿视觉内容被匿名公开遍历泄露
func RenderSDUIScreenshotHandler(ctx iris.Context) {
	appID := ctx.URLParam("app_id")
	if appID == "" {
		appID = middleware.GetTenantAppID(ctx)
	}
	pageID := ctx.URLParam("page_id")
	if pageID == "" {
		pageID = "home"
	}
	isDraft := ctx.URLParam("draft") == "true"

	// 1. 安全门禁拦截: 若请求草稿截图，强制验证访问签名或管理员/MCP授权凭证
	if isDraft {
		isAuthorized := false

		// 方式 A: 校验时效签名 (HMAC-SHA256: sign + expires + hash)
		sign := ctx.URLParam("sign")
		expiresStr := ctx.URLParam("expires")
		hash := ctx.URLParam("hash")
		if sign != "" && expiresStr != "" {
			if expires, err := strconv.ParseInt(expiresStr, 10, 64); err == nil {
				if services.ValidateScreenshotSignature(appID, pageID, hash, expires, sign) {
					isAuthorized = true
				}
			}
		}

		// 方式 B: 校验管理员 JWT 令牌 (Authorization: Bearer <admin_token>)
		if !isAuthorized {
			authHeader := ctx.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				adminToken := strings.TrimPrefix(authHeader, "Bearer ")
				adminAuthService := services.NewAdminAuthService()
				if _, err := adminAuthService.ParseAdminToken(adminToken); err == nil {
					isAuthorized = true
				}
			}
		}

		// 方式 C: 校验 MCP 通信密钥 (X-MCP-Auth-Key)
		if !isAuthorized {
			mcpKey := ctx.GetHeader("X-MCP-Auth-Key")
			expectedKey := os.Getenv("MCP_AUTH_KEY")
			if expectedKey != "" && mcpKey == expectedKey {
				isAuthorized = true
			}
		}

		// 门禁阻断: 无合法签名与凭据严禁访问未发布草稿
		if !isAuthorized {
			ctx.StatusCode(iris.StatusForbidden)
			_ = ctx.JSON(iris.Map{
				"code": 403,
				"msg":  "【安全拦截】未授权访问未发布草稿截图，请提供有效时效访问签名或管理鉴权凭证",
			})
			return
		}
	}

	shareCardService := services.NewShareCardService()
	var pngBytes []byte
	var err error
	if isDraft {
		pngBytes, err = shareCardService.RenderDraftShareCard(appID, pageID, "app_message")
	} else {
		pngBytes, err = shareCardService.RenderShareCard(appID, pageID, "app_message")
	}

	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		_, _ = ctx.WriteString(err.Error())
		return
	}

	ctx.Header("Content-Type", "image/png")
	if isDraft {
		// 草稿截图严禁公共 CDN 缓存
		ctx.Header("Cache-Control", "private, no-cache, no-store, must-revalidate")
	} else {
		ctx.Header("Cache-Control", "public, max-age=300")
	}
	_, _ = ctx.Write(pngBytes)
}

// RenderShareCardHandler 动态渲染微信分享卡片图片流
func RenderShareCardHandler(ctx iris.Context) {
	appID := ctx.URLParam("app_id")
	if appID == "" {
		appID = middleware.GetTenantAppID(ctx)
	}
	pageID := ctx.URLParam("page_id")
	cardType := ctx.URLParam("type")
	if cardType == "" {
		cardType = "app_message"
	}

	shareCardService := services.NewShareCardService()
	pngBytes, err := shareCardService.RenderShareCard(appID, pageID, cardType)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		_, _ = ctx.WriteString(err.Error())
		return
	}

	ctx.Header("Content-Type", "image/png")
	ctx.Header("Cache-Control", "public, max-age=1800")
	_, _ = ctx.Write(pngBytes)
}

// WeCodeGetInfo 小程序通过 wx.login 获取 code 后登录 (兼容旧版 /code 路由，全面接入双 Token 体系与多租户会话管理)
func WeCodeGetInfo(ctx iris.Context) {
	req := ctx.Values().Get(sv.GlobalContextKey).(*validator.WeCodeReq)
	appID := middleware.GetTenantAppID(ctx)

	authService := services.NewAuthService()
	loginRes, err := authService.WechatLogin(ctx.Request().Context(), appID, req.Code)

	if err != nil {
		ut.IrisErrLog(ctx, err, "通过 code 获取会话失败")
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{
			"code": 401,
			"msg":  "微信授权登录失败: " + err.Error(),
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": iris.Map{
			// 兼容旧版前端字段
			"token": loginRes.AccessToken,
			"user":  loginRes.User,
			// 全面扩展双 Token 标准字段
			"access_token":       loginRes.AccessToken,
			"access_expires_at":  loginRes.AccessExpiresAt,
			"refresh_token":      loginRes.RefreshToken,
			"refresh_expires_at": loginRes.RefreshExpiresAt,
			"session_id":         loginRes.SessionID,
		},
	})
}

// GetNewInfo 获取最新信息
func GetNewInfo(ctx iris.Context) {
	userModel := ctx.Values().Get(jwtToken.JwtUserModel).(*models.User)
	_ = ctx.JSON(iris.Map{
		"code": 0,
		"data": userModel.SimpleUser(),
	})
}
