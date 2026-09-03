// Package routers sdui.go
package routers

import (
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"net/http"

	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"
)

// RegisterSDUIRoutes 注册 SDUI 动态组件页面下发路由
func RegisterSDUIRoutes(party iris.Party) {
	pageParty := party.Party("/page")
	{
		// 获取指定页面的 SDUI 统一响应信封
		pageParty.Get("/{page_id:string}", GetDynamicPageHandler)
	}
}

// GetDynamicPageHandler 服务端驱动页面协议下发控制器 (统一响应信封、ETag 缓存与多租户自适应)
func GetDynamicPageHandler(ctx iris.Context) {
	pageID := ctx.Params().Get("page_id")
	if pageID == "" {
		pageID = "home"
	}

	appID := middleware.GetTenantAppID(ctx)

	// 收集 URL Query 参数供受控绑定消费
	queryParams := make(map[string]string)
	for k, v := range ctx.URLParams() {
		queryParams[k] = v
	}

	srv := services.NewSDUIService()
	envelope, err := srv.GetDynamicPageEnvelope(appID, pageID, queryParams)
	if err != nil {
		ut.IrisErrLog(ctx, err, "获取动态页面协议失败")
		_ = ctx.JSON(iris.Map{
			"code": 404,
			"msg":  "获取动态页面失败: " + err.Error(),
		})
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
