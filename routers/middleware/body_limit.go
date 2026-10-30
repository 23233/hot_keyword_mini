// Package middleware body_limit.go
package middleware

import (
	"net/http"

	"github.com/kataras/iris/v12"
)

// BodyLimit 拒绝超过上限的请求体，并给流式请求加读取上限，防止超大 JSON 拖垮服务。
func BodyLimit(maxBytes int64) iris.Handler {
	return func(ctx iris.Context) {
		if maxBytes <= 0 {
			ctx.Next()
			return
		}
		if ctx.Request().ContentLength > maxBytes {
			ctx.Header("Cache-Control", "no-store")
			ctx.StatusCode(http.StatusRequestEntityTooLarge)
			_ = ctx.JSON(iris.Map{"code": 413, "msg": "请求体过大"})
			return
		}
		if ctx.Request().Body != nil {
			ctx.Request().Body = http.MaxBytesReader(ctx.ResponseWriter(), ctx.Request().Body, maxBytes)
		}
		ctx.Next()
	}
}
