// Package routers index.go
package routers

import (
	"hot_keyword/routers/middleware"

	"github.com/iris-contrib/middleware/csrf"
	"github.com/kataras/iris/v12"
)

func RegisterRouters(party iris.Party) {
	// 添加路由

	// 纯web
	web := party.Party("/")

	// csrf
	cs := csrf.Protect(
		[]byte("94629587235212521125211252125212"),
		csrf.CookieName("csrfToken"),
	)

	web.Use(cs)
	web.Use(middleware.I18nMiddleware)
	web.Use(middleware.InjectViewData)

	web.Get("/", func(ctx iris.Context) {
		_ = ctx.View("index.html")
	})

	// 注册 API 路由
	registerAPIRoutes(party)

	// 管理后台可视化工作台页面 (含手机模拟器与实时编辑器)
	party.Get("/admin", func(ctx iris.Context) {
		_ = ctx.View("admin.html")
	})

}
