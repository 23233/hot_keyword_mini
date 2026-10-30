// Package routers health.go
package routers

import (
	"context"
	"hot_keyword/db"
	"net/http"
	"time"

	"github.com/kataras/iris/v12"
)

// HealthHandler 提供无鉴权的存活与数据库连通性探针，供容器编排和负载均衡使用。
// 响应只包含健康状态，不暴露版本、连接串或任何凭据。
func HealthHandler(ctx iris.Context) {
	dbStatus := "ok"
	if db.DB == nil {
		dbStatus = "unavailable"
	} else if sqlDB, err := db.DB.DB(); err != nil {
		dbStatus = "unavailable"
	} else {
		pingCtx, cancel := context.WithTimeout(ctx.Request().Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(pingCtx); err != nil {
			dbStatus = "error"
		}
	}

	status := http.StatusOK
	serviceStatus := "ok"
	if dbStatus != "ok" {
		status = http.StatusServiceUnavailable
		serviceStatus = "degraded"
	}
	ctx.StatusCode(status)
	_ = ctx.JSON(iris.Map{
		"status": serviceStatus,
		"db":     dbStatus,
	})
}
