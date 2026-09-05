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
		// 获取指定页面的 SDUI 统一响应信封 (面向微信小程序客户端公开下发)
		pageParty.Get("/{page_id:string}", GetDynamicPageHandler)
	}
}

// GetDynamicPageHandler 服务端驱动页面协议下发控制器 (统一响应信封、ETag 缓存、草稿隔离与登录鉴权隔离)
func GetDynamicPageHandler(ctx iris.Context) {
	pageID := ctx.Params().Get("page_id")
	if pageID == "" {
		pageID = "home"
	}

	appID, tenantErr := middleware.RequireTenantAppID(ctx)
	if tenantErr != nil {
		ctx.StatusCode(http.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": tenantErr.Error()})
		return
	}

	// 收集 URL Query 参数供受控绑定消费
	queryParams := make(map[string]string)
	for k, v := range ctx.URLParams() {
		queryParams[k] = v
	}

	// 检查当前访问者是否携带有效登录凭证 (严格校验会话存活态与多租户隔离)
	isAuthenticated := false
	authHeader := ctx.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if _, _, _, err := jwtToken.ValidateTokenSessionAndTenant(tokenStr, appID); err == nil {
			isAuthenticated = true
		}
	}

	srv := services.NewSDUIService()
	envelope, err := srv.GetPublishedDynamicPageEnvelope(appID, pageID, queryParams, isAuthenticated)
	if err != nil {
		logger.JM.Warnf("获取动态页面协议失败: %v", err)
		ctx.StatusCode(http.StatusNotFound)
		_ = ctx.JSON(iris.Map{
			"code": 404,
			"msg":  "获取动态页面失败: " + err.Error(),
		})
		return
	}

	// 若页面声明全页受保，且未提供有效登录凭证，服务端隔离返回 401 拦截
	if envelope.Page.RequireAuth && !isAuthenticated {
		ctx.StatusCode(http.StatusUnauthorized)
		_ = ctx.JSON(envelope)
		return
	}

	// 支持 ETag 条件缓存，避免重复传输相同 Revision 的页面协议
	clientETag := ctx.GetHeader("If-None-Match")
	if clientETag != "" && clientETag == envelope.Cache.ETag {
		ctx.StatusCode(http.StatusNotModified)
		return
	}

	ctx.Header("ETag", envelope.Cache.ETag)
	ctx.Header("Cache-Control", "public, max-age=30")

	// 按照统一信封结构直接返回
	_ = ctx.JSON(envelope)
}
