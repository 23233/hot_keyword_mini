// Package routers domain_admin.go
package routers

import (
	"github.com/kataras/iris/v12"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
)

// RegisterDomainAdminRoutes 后台领域操作使用管理员身份，敏感操作需要显式确认。
func RegisterDomainAdminRoutes(admin iris.Party) {
	admin.Post("/domain/execute", middleware.RequireAdminRole("super_admin", "admin"), func(ctx iris.Context) {
		var input struct {
			// 租户标识。
			AppID string `json:"app_id"`
			// 受控业务操作。
			Endpoint string `json:"endpoint"`
			// 领域参数。
			Payload map[string]interface{} `json:"payload"`
			// 幂等键。
			IdempotencyKey string `json:"idempotency_key"`
			// 人工确认。
			Confirmed bool `json:"confirmed"`
		}
		if err := ctx.ReadJSON(&input); err != nil || !input.Confirmed {
			ctx.StatusCode(400)
			ctx.JSON(iris.Map{"code": "CONFIRMATION_REQUIRED", "msg": "操作须明确确认"})
			return
		}
		result, err := services.ExecuteDomainOperation(input.AppID, 0, "admin:"+ctx.Values().GetString("admin_username"), input.Endpoint, input.Payload, input.IdempotencyKey, true)
		if err != nil {
			ctx.StatusCode(400)
			ctx.JSON(iris.Map{"code": "DOMAIN_ERROR", "msg": err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "data": result})
	})
}
