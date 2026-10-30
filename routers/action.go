// Package routers action.go
package routers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hot_keyword/jwtToken"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

// stableGuestKey 基于租户与真实客户端地址派生稳定的匿名访客标识 (前缀 guest_ 表示未登录身份)
func stableGuestKey(ctx iris.Context, appID string) string {
	sum := sha256.Sum256([]byte(appID + "|" + middleware.ClientIP(ctx)))
	return "guest_" + hex.EncodeToString(sum[:])[:24]
}

// ExecuteActionReq 受控业务动作执行请求
type ExecuteActionReq struct {
	// 已登记的受控端点名称 (如 game.redeem)
	Endpoint string `json:"endpoint"`
	// 业务参数载荷
	Payload map[string]interface{} `json:"payload"`
	// 客户端幂等键
	IdempotencyKey string `json:"idempotency_key"`
}

// ExecuteActionHandler 受控动作执行统一入口 (杜绝开放网络代理)
func ExecuteActionHandler(ctx iris.Context) {
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}

	var req ExecuteActionReq
	if err := ctx.ReadJSON(&req); err != nil || req.Endpoint == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": "参数错误，endpoint 不能为空"})
		return
	}

	// 尝试从登录态上下文中提取真实 open_id，未提取到时严格解析并校验 Authorization Header
	openID := ctx.Values().GetString("open_id")
	if openID == "" {
		authHeader := ctx.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			_, user, claims, err := jwtToken.ValidateTokenSessionAndTenant(tokenStr, appID)
			if err == nil {
				if user != nil && user.WechatOpenID != "" {
					openID = user.WechatOpenID
				} else if oid, ok := claims["openId"].(string); ok {
					openID = oid
				}
			}
		}
	}

	// 匿名请求派生稳定访客指纹：同一来源地址在限领/频控判定中保持一致身份，
	// 防止每次请求随机 guest_ 标识绕过"同一访客限领一次"与频控校验。
	if openID == "" {
		openID = stableGuestKey(ctx, appID)
	}

	actionService := services.NewActionEndpointService()
	// 运行时能力门禁：公开动作端点必须尊重租户能力矩阵，不能只依赖发布门禁。
	requiredCapabilities := services.CapabilityKeysForEndpoint(req.Endpoint, req.Payload)
	if allowed, capabilityErr := services.EnsureAppCapability(appID, requiredCapabilities); capabilityErr != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		_ = ctx.JSON(iris.Map{"code": 500, "error_code": "CAPABILITY_CHECK_FAILED", "msg": "能力状态检查失败"})
		return
	} else if !allowed {
		ctx.StatusCode(iris.StatusForbidden)
		_ = ctx.JSON(iris.Map{
			"code": 403, "error_code": "CAPABILITY_DISABLED",
			"msg": "当前小程序未启用该能力", "required_capabilities": requiredCapabilities,
		})
		return
	}
	res, err := actionService.ExecuteActionEndpoint(appID, openID, req.Endpoint, req.Payload, req.IdempotencyKey)

	if err != nil {
		status, errorCode := actionErrorStatus(err)
		ctx.StatusCode(status)
		_ = ctx.JSON(iris.Map{"code": status, "error_code": errorCode, "msg": err.Error()})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "操作成功",
		"data": res,
	})
}

// actionErrorStatus 将领域错误映射为稳定的 HTTP 状态与机器可读错误码。
// 客户端应优先使用 error_code 做分支，HTTP 状态仅用于网关和通用重试策略。
func actionErrorStatus(err error) (int, string) {
	var domainErr *services.DomainError
	if errors.As(err, &domainErr) {
		switch domainErr.Code {
		case "UNAUTHORIZED":
			return iris.StatusUnauthorized, domainErr.Code
		case "FORBIDDEN":
			return iris.StatusForbidden, domainErr.Code
		case "NOT_FOUND":
			return iris.StatusNotFound, domainErr.Code
		case "CONFLICT", "IDEMPOTENCY_CONFLICT", "PAYMENT_REUSED", "PAYMENT_MISMATCH":
			return iris.StatusConflict, domainErr.Code
		case "INVALID_ARGUMENT", "INVALID_STATE", "PAYMENT_REQUIRED", "INSUFFICIENT_BALANCE":
			return iris.StatusBadRequest, domainErr.Code
		case "UNAVAILABLE":
			return iris.StatusServiceUnavailable, domainErr.Code
		default:
			return iris.StatusBadRequest, domainErr.Code
		}
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "授权登录") || strings.Contains(message, "登录后"):
		return iris.StatusUnauthorized, "UNAUTHORIZED"
	case strings.Contains(message, "不存在"):
		return iris.StatusNotFound, "NOT_FOUND"
	case strings.Contains(message, "领完") || strings.Contains(message, "库存"):
		return iris.StatusConflict, "OUT_OF_STOCK"
	case strings.Contains(message, "不能为空") || strings.Contains(message, "非法") || strings.Contains(message, "未登记") || strings.Contains(message, "无效"):
		return iris.StatusBadRequest, "INVALID_ARGUMENT"
	default:
		return iris.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
