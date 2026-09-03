// File: routers/middleware/view_data.go
package middleware

import (
	"github.com/iris-contrib/middleware/csrf"
	"github.com/kataras/iris/v12"
)

// InjectViewData 是一个中间件，用于向所有视图注入通用数据
func InjectViewData(ctx iris.Context) {
	// 注入 CSRF Token
	ctx.ViewData("csrf_token", csrf.Token(ctx))

	ctx.Next()
}
