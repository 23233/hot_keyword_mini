// Package routers sdui.go
package routers

import (
	"hot_keyword/jwtToken"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"net/http"
	"strings"

	"github.com/23233/ggg/logger"
	"github.com/kataras/iris/v12"
)

// RegisterSDUIRoutes 注册 SDUI 动态组件页面下发路由
func RegisterSDUIRoutes(party iris.Party) {
	pageParty := party.Party("/page")
	{
		// 获取默认主页 SDUI 统一响应信封 (面向无参数路径请求，优雅兜底至 home)
		pageParty.Get("/", GetDynamicPageHandler)
		// 获取指定页面的 SDUI 统一响应信封 (面向微信小程序客户端公开下发)
		pageParty.Get("/{page_id:string}", GetDynamicPageHandler)
	}
}

// GetDynamicPageHandler 服务端驱动页面协议下发控制器 (统一响应信封、ETag 缓存、草稿隔离与登录鉴权隔离)
func GetDynamicPageHandler(ctx iris.Context) {
	clientVersion := strings.TrimSpace(ctx.GetHeader("X-SDUI-Version"))
	if clientVersion != "" && clientVersion != "1.1" {
		ctx.StatusCode(http.StatusUpgradeRequired)
		_ = ctx.JSON(iris.Map{
			"code":             http.StatusUpgradeRequired,
			"msg":              "客户端 SDUI 协议版本不兼容，请升级至 1.1",
			"protocol_version": "1.1",
		})
		return
	}

	pageID := strings.TrimSpace(ctx.Params().Get("page_id"))
	if pageID == "" {
		pageID = strings.TrimSpace(ctx.URLParam("page_id"))
	}
	if pageID == "" {
		pageID = "home"
	}

	appID, tenantErr := middleware.RequireTenantAppID(ctx)
	if tenantErr != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": tenantErr.Error()})
		return
	}

	// 收集 URL Query 参数供受控绑定消费；URLParams 只包含路径参数，不能替代 Query。
	queryParams := make(map[string]string)
	for key, values := range ctx.Request().URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// 检查当前访问者是否携带有效登录凭证 (严格校验会话存活态与多租户隔离)
	isAuthenticated := false
	viewerUserID := int64(0)
	authHeader := ctx.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if _, user, _, err := jwtToken.ValidateTokenSessionAndTenant(tokenStr, appID); err == nil && user != nil {
			isAuthenticated = true
			viewerUserID = user.ID
		}
	}

	clientCaps := strings.TrimSpace(ctx.GetHeader("X-Client-Capabilities"))
	srv := services.NewSDUIService()
	envelope, err := srv.GetPublishedDynamicPageEnvelopeWithCapabilitiesAndViewer(appID, pageID, queryParams, isAuthenticated, clientCaps, viewerUserID)
	if err != nil {
		logger.JM.Warnf("获取动态页面协议失败: %v", err)
		ctx.Header("Cache-Control", "no-store")
		ctx.StatusCode(http.StatusNotFound)
		_ = ctx.JSON(iris.Map{
			"code": 404,
			"msg":  "获取动态页面失败: " + err.Error(),
		})
		return
	}

	// 个性化或受保页面必须先声明缓存策略，401/304 也不能被公共缓存复用。
	privateCache := isAuthenticated || envelope.Page.RequireAuth ||
		envelope.Page.BusinessType == "ai_breakthrough" || envelope.Page.BusinessType == "ai_article" ||
		envelope.Page.PageID == "maintenance"
	ctx.Header("ETag", envelope.Cache.ETag)
	if privateCache {
		ctx.Header("Vary", "X-WX-AppID, X-SDUI-Version, X-Client-Capabilities, Authorization")
		ctx.Header("Cache-Control", "private, no-store, max-age=0")
	} else {
		// 匿名公共页面才允许短时公共缓存，且必须按租户和客户端能力拆分变体。
		ctx.Header("Vary", "X-WX-AppID, X-SDUI-Version, X-Client-Capabilities")
		ctx.Header("Cache-Control", "public, max-age=30")
	}

	// 若页面声明全页受保，且未提供有效登录凭证，服务端隔离返回 401 拦截
	if envelope.Page.RequireAuth && !isAuthenticated {
		ctx.StatusCode(http.StatusUnauthorized)
		_ = ctx.JSON(envelope)
		return
	}

	// 支持 ETag 条件缓存；ETag 已包含查询参数、能力、登录态和访问者摘要。
	clientETag := ctx.GetHeader("If-None-Match")
	if clientETag != "" && clientETag == envelope.Cache.ETag {
		ctx.StatusCode(http.StatusNotModified)
		return
	}

	// 按照统一信封结构直接返回
	_ = ctx.JSON(envelope)
}
