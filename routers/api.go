// Package routers api.go
package routers

import (
	"hot_keyword/db"
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"hot_keyword/routers/middleware"
	"hot_keyword/sdk"
	"hot_keyword/services"
	"hot_keyword/validator"

	"github.com/23233/ggg/sv"
	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"

	"time"
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

	// 注册受控业务动作执行接口 (如游戏礼包兑换码防超发领取)
	api.Post("/action/execute", ExecuteActionHandler)

	// Protected routes
	userParty := api.Party("/user")
	userParty.Use(jwtToken.SidAndJwtMiddleware, jwtToken.TokenToUserUidMiddleware)
	userParty.Get("/info", GetNewInfo)
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

// WeCodeGetInfo 小程序通过wx.login获取code后 获取openid (兼容老版接口，支持多租户隔离)
func WeCodeGetInfo(ctx iris.Context) {
	req := ctx.Values().Get(sv.GlobalContextKey).(*validator.WeCodeReq)
	appID := middleware.GetTenantAppID(ctx)

	var err error
	sessionRsp, err := sdk.MiniSdk.Code2Session(ctx, req.Code)
	if err != nil || sessionRsp.Errcode != 0 {
		ut.IrisErrLog(ctx, err, "获取用户信息异常")
		return
	}
	// 通过 app_id + openid 获取或注册用户
	var user = models.User{}

	// 获取或创建
	err = db.Mysql.Where("app_id = ? AND wechat_openid = ?", appID, sessionRsp.Openid).Assign(models.User{
		AppID:        appID,
		WechatOpenID: sessionRsp.Openid,
		NickName:     "微信用户" + ut.RandomStr(6),
		AvatarType:   0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}).FirstOrCreate(&user).Error
	if err != nil {
		ut.IrisErrLog(ctx, err, "获取用户信息异常")
		return
	}

	token := jwtToken.GenJwtToken(user.WechatOpenID)
	_ = ctx.JSON(iris.Map{
		"code": 0,
		"data": iris.Map{
			"token": token,
			"user":  user.SimpleUser(),
		},
	})
	return
}

// GetNewInfo 获取最新信息
func GetNewInfo(ctx iris.Context) {
	userModel := ctx.Values().Get(jwtToken.JwtUserModel).(*models.User)
	_ = ctx.JSON(iris.Map{
		"code": 0,
		"data": userModel.SimpleUser(),
	})
}
