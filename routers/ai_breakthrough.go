// Package routers ai_breakthrough.go
package routers

import (
	"errors"
	"hot_keyword/db"
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
)

// RegisterAIBreakthroughRoutes 注册 AI 破甲资讯、会员和评论公开接口。
func RegisterAIBreakthroughRoutes(api iris.Party) {
	articleService := services.NewArticleService()
	membershipService := services.NewMembershipService()
	commentService := services.NewCommentService()

	api.Get("/article-categories", func(ctx iris.Context) {
		appID, err := middleware.RequireTenantAppID(ctx)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		var categories []models.ArticleCategory
		if err := db.Mysql.Where("app_id = ? AND status = ?", appID, "active").Order("sort asc, id asc").Find(&categories).Error; err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": categories})
	})

	api.Get("/articles", func(ctx iris.Context) {
		appID, err := middleware.RequireTenantAppID(ctx)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		level, _ := currentMembershipLevel(ctx, appID)
		categoryID, _ := strconv.ParseInt(ctx.URLParam("category_id"), 10, 64)
		limit, _ := strconv.Atoi(ctx.URLParam("limit"))
		offset, _ := strconv.Atoi(ctx.URLParam("offset"))
		items, err := articleService.ListPublishedForViewer(appID, categoryID, limit, offset, level, currentUserID(ctx, appID))
		if err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": items})
	})

	api.Get("/articles/{article_id:uint64}", func(ctx iris.Context) {
		appID, err := middleware.RequireTenantAppID(ctx)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		id, _ := strconv.ParseInt(ctx.Params().Get("article_id"), 10, 64)
		level, _ := currentMembershipLevel(ctx, appID)
		item, err := articleService.GetPublishedForViewer(appID, id, level, currentUserID(ctx, appID))
		if err != nil {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": "文章不存在"})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": item})
	})

	api.Get("/membership/plans", func(ctx iris.Context) {
		appID, err := middleware.RequireTenantAppID(ctx)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		plans, err := membershipService.ListPlans(appID)
		if err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": plans})
	})

	api.Get("/membership/me", requireUser, func(ctx iris.Context) {
		appID := middleware.GetTenantAppID(ctx)
		user := ctx.Values().GetString("article_user_id")
		userID, _ := strconv.ParseInt(user, 10, 64)
		membership, err := membershipService.GetMembership(appID, userID)
		if err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": membership})
	})

	api.Post("/membership/orders", requireUser, func(ctx iris.Context) {
		appID := middleware.GetTenantAppID(ctx)
		userID, _ := strconv.ParseInt(ctx.Values().GetString("article_user_id"), 10, 64)
		openID := ctx.Values().GetString("article_open_id")
		var req struct {
			SKU            string `json:"sku"`
			Level          int    `json:"level"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "会员套餐参数无效"})
			return
		}
		if req.SKU == "" && req.Level > 0 {
			var plan models.MembershipLevel
			if err := db.Mysql.Where("app_id = ? AND level = ? AND status = ?", appID, req.Level, "active").First(&plan).Error; err != nil {
				ctx.StatusCode(404)
				_ = ctx.JSON(iris.Map{"code": 404, "msg": "会员等级不存在"})
				return
			}
			req.SKU = plan.SKU
		}
		order, payment, err := services.NewPaymentService().CreateJSAPIOrder(ctx.Request().Context(), appID, userID, openID, strings.TrimSpace(req.SKU), req.IdempotencyKey)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": iris.Map{"order": order, "payment": payment}})
	})

	api.Get("/articles/{article_id:uint64}/comments", func(ctx iris.Context) {
		appID, err := middleware.RequireTenantAppID(ctx)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		articleID, _ := strconv.ParseInt(ctx.Params().Get("article_id"), 10, 64)
		limit, _ := strconv.Atoi(ctx.URLParam("limit"))
		offset, _ := strconv.Atoi(ctx.URLParam("offset"))
		items, err := commentService.ListRoots(appID, articleID, limit, offset)
		if err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": items})
	})

	api.Get("/comments/{comment_id:uint64}/replies", func(ctx iris.Context) {
		appID, err := middleware.RequireTenantAppID(ctx)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		rootID, _ := strconv.ParseInt(ctx.Params().Get("comment_id"), 10, 64)
		limit, _ := strconv.Atoi(ctx.URLParam("limit"))
		offset, _ := strconv.Atoi(ctx.URLParam("offset"))
		items, err := commentService.ListReplies(appID, rootID, limit, offset)
		if err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": items})
	})

	api.Post("/articles/{article_id:uint64}/comments", requireUser, func(ctx iris.Context) {
		appID := middleware.GetTenantAppID(ctx)
		articleID, _ := strconv.ParseInt(ctx.Params().Get("article_id"), 10, 64)
		user, err := currentUser(ctx, appID)
		if err != nil {
			ctx.StatusCode(401)
			_ = ctx.JSON(iris.Map{"code": 401, "msg": err.Error()})
			return
		}
		var req services.CommentCreateInput
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "评论参数无效"})
			return
		}
		req.ArticleID = articleID
		comment, err := commentService.Create(ctx.Request().Context(), appID, user, req)
		if err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": comment})
	})

	// 微信图片审核异步回调；文本审核为同步结果，无需回调。
	api.Post("/content-audit/callback/{app_id:string}", func(ctx iris.Context) {
		var req struct {
			TraceID string `json:"trace_id"`
			Result  struct {
				Suggest string `json:"suggest"`
				Label   int    `json:"label"`
			} `json:"result"`
		}
		appID := strings.TrimSpace(ctx.Params().Get("app_id"))
		readErr := ctx.ReadJSON(&req)
		if appID == "" || readErr != nil || req.TraceID == "" {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "审核回调参数无效"})
			return
		}
		if err := commentService.CompleteImageAudit(appID, req.TraceID, req.Result.Suggest, req.Result.Label); err != nil {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0})
	})
}

func requireUser(ctx iris.Context) {
	appID, err := middleware.RequireTenantAppID(ctx)
	if err != nil {
		ctx.StatusCode(400)
		_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
		return
	}
	token := strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	_, user, claims, err := jwtToken.ValidateTokenSessionAndTenant(token, appID)
	if err != nil || user == nil || user.ID <= 0 {
		ctx.StatusCode(401)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "请先完成微信登录"})
		return
	}
	ctx.Values().Set("article_user_id", strconv.FormatInt(user.ID, 10))
	ctx.Values().Set("article_open_id", user.WechatOpenID)
	ctx.Values().Set("article_claims", claims)
	ctx.Next()
}

func currentUser(ctx iris.Context, appID string) (*models.User, error) {
	userID, _ := strconv.ParseInt(ctx.Values().GetString("article_user_id"), 10, 64)
	if userID <= 0 {
		return nil, errors.New("请先完成微信登录")
	}
	var user models.User
	if err := db.Mysql.Where("id = ? AND app_id = ?", userID, appID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func currentMembershipLevel(ctx iris.Context, appID string) (int, *time.Time) {
	token := strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	if token == "" {
		return 0, nil
	}
	_, user, _, err := jwtToken.ValidateTokenSessionAndTenant(token, appID)
	if err != nil || user == nil {
		return 0, nil
	}
	membership, err := services.NewMembershipService().GetMembership(appID, user.ID)
	if err != nil || membership == nil || !membership.ExpiresAt.After(time.Now()) {
		return 0, nil
	}
	return membership.Level, &membership.ExpiresAt
}

func currentUserID(ctx iris.Context, appID string) int64 {
	token := strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	if token == "" {
		return 0
	}
	_, user, _, err := jwtToken.ValidateTokenSessionAndTenant(token, appID)
	if err != nil || user == nil {
		return 0
	}
	return user.ID
}
