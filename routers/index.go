// Package routers index.go
package routers

import (
	"hot_keyword/routers/middleware"
	"os"
	"strings"

	"github.com/23233/ggg/logger"
	"github.com/23233/ggg/ut"
	"github.com/iris-contrib/middleware/csrf"
	"github.com/kataras/iris/v12"
)

// csrfSecretKey 读取 CSRF Cookie 签名密钥；未配置时使用进程内随机密钥，禁止在仓库写死。
func csrfSecretKey() []byte {
	if value := strings.TrimSpace(os.Getenv("CSRF_SECRET")); value != "" {
		return []byte(value)
	}
	logger.JM.Warnf("环境变量 CSRF_SECRET 未配置，已生成进程内随机 CSRF 密钥")
	return []byte("sdui_csrf_" + ut.RandomStr(24))
}

func RegisterRouters(party iris.Party) {
	// 添加路由

	// CSRF 保护作为 web 子 Party 的首个中间件传入，确保中间件切片独立。
	cs := csrf.Protect(
		csrfSecretKey(),
		csrf.CookieName("csrfToken"),
	)

	// 纯web
	web := party.Party("/", cs)
	web.Use(middleware.I18nMiddleware)
	web.Use(middleware.InjectViewData)

	web.Get("/", func(ctx iris.Context) {
		_ = ctx.View("index.html")
	})

	// 容器编排与负载均衡使用的无鉴权健康探针。
	party.Get("/healthz", HealthHandler)

	// 注册 API 路由
	registerAPIRoutes(party)

	// 管理后台可视化工作台页面 (含手机模拟器与实时编辑器)
	party.Get("/admin", func(ctx iris.Context) {
		_ = ctx.View("admin.html")
	})

}
