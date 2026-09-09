// Package routers ai_breakthrough_admin.go
package routers

import (
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/services"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
)

// RegisterAIBreakthroughAdminRoutes 注册 AI 破甲内容和会员配置后台接口。
func RegisterAIBreakthroughAdminRoutes(adminParty iris.Party) {
	articleService := services.NewArticleService()
	membershipService := services.NewMembershipService()
	commentService := services.NewCommentService()

	adminParty.Get("/article-categories", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		var rows []models.ArticleCategory
		if err := db.Mysql.Where("app_id = ?", appID).Order("sort asc, id asc").Find(&rows).Error; err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": rows})
	})
	adminParty.Post("/article-categories", func(ctx iris.Context) {
		var row models.ArticleCategory
		if err := ctx.ReadJSON(&row); err != nil || row.AppID == "" || row.Slug == "" || row.Name == "" {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "栏目参数不完整"})
			return
		}
		if err := articleService.SaveCategory(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": row})
	})
	adminParty.Put("/article-categories/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		var input models.ArticleCategory
		if appID == "" || id <= 0 || ctx.ReadJSON(&input) != nil || strings.TrimSpace(input.Slug) == "" || strings.TrimSpace(input.Name) == "" {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "栏目参数无效"})
			return
		}
		status := strings.TrimSpace(input.Status)
		if status == "" {
			status = "active"
		}
		if status != "active" && status != "inactive" {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "栏目状态必须为 active 或 inactive"})
			return
		}
		result := db.Mysql.Model(&models.ArticleCategory{}).Where("app_id = ? AND id = ?", appID, id).Updates(map[string]interface{}{"slug": strings.TrimSpace(input.Slug), "name": strings.TrimSpace(input.Name), "summary": input.Summary, "sort": input.Sort, "status": status, "updated_at": time.Now()})
		if result.Error != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": "栏目不存在"})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "栏目已更新"})
	})
	adminParty.Delete("/article-categories/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		result := db.Mysql.Model(&models.ArticleCategory{}).Where("app_id = ? AND id = ?", appID, id).Update("status", "inactive")
		if result.Error != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": "栏目不存在"})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "栏目已停用"})
	})

	adminParty.Get("/membership/levels", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		var rows []models.MembershipLevel
		if err := db.Mysql.Where("app_id = ?", appID).Order("level asc").Find(&rows).Error; err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": rows})
	})
	adminParty.Post("/membership/levels", func(ctx iris.Context) {
		var row models.MembershipLevel
		if err := ctx.ReadJSON(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "会员等级参数不完整"})
			return
		}
		if err := membershipService.SavePlan(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": row})
	})
	adminParty.Put("/membership/levels/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		var row models.MembershipLevel
		if appID == "" || id <= 0 || db.Mysql.Where("app_id = ? AND id = ?", appID, id).First(&row).Error != nil || ctx.ReadJSON(&row) != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "会员等级不存在或参数无效"})
			return
		}
		row.ID, row.AppID = id, appID
		if err := membershipService.SavePlan(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": row})
	})
	adminParty.Delete("/membership/levels/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		var plan models.MembershipLevel
		if appID == "" || id <= 0 || db.Mysql.Where("app_id = ? AND id = ?", appID, id).First(&plan).Error != nil {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": "会员等级不存在"})
			return
		}
		result := db.Mysql.Model(&plan).Updates(map[string]interface{}{"status": "inactive", "updated_at": time.Now()})
		if result.Error != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": "会员等级不存在"})
			return
		}
		db.Mysql.Model(&models.Product{}).Where("app_id = ? AND sku = ?", appID, plan.SKU).Update("status", models.ProductStatusInactive)
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "会员等级已停用"})
	})

	adminParty.Get("/articles", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		var rows []models.Article
		if err := db.Mysql.Where("app_id = ?", appID).Order("id desc").Find(&rows).Error; err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": rows})
	})
	adminParty.Post("/articles", func(ctx iris.Context) {
		var row models.Article
		if err := ctx.ReadJSON(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "文章参数不完整"})
			return
		}
		if err := articleService.SaveArticle(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": row})
	})
	adminParty.Put("/articles/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		var row models.Article
		if id <= 0 || appID == "" || db.Mysql.Where("app_id = ? AND id = ?", appID, id).First(&row).Error != nil || ctx.ReadJSON(&row) != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "文章不存在或参数无效"})
			return
		}
		row.ID, row.AppID = id, appID
		if err := articleService.SaveArticle(&row); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": row})
	})
	adminParty.Delete("/articles/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		result := db.Mysql.Model(&models.Article{}).Where("app_id = ? AND id = ?", appID, id).Updates(map[string]interface{}{"status": "archived", "updated_at": time.Now()})
		if result.Error != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			ctx.StatusCode(404)
			_ = ctx.JSON(iris.Map{"code": 404, "msg": "文章不存在"})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "文章已下架"})
	})

	adminParty.Post("/ai-breakthrough/seed", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		if appID == "" {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "app_id 不能为空"})
			return
		}
		if err := services.NewMembershipService().SeedDefaultPlans(appID); err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		if err := articleService.SeedDefaultContent(appID); err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "AI 破甲默认内容已初始化"})
	})

	adminParty.Get("/article-comments", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		articleID, _ := strconv.ParseInt(ctx.URLParam("article_id"), 10, 64)
		var rows []models.ArticleComment
		query := db.Mysql.Where("app_id = ?", appID)
		if articleID > 0 {
			query = query.Where("article_id = ?", articleID)
		}
		if err := query.Order("id desc").Find(&rows).Error; err != nil {
			ctx.StatusCode(500)
			_ = ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "data": rows})
	})
	adminParty.Put("/article-comments/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		var input struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if appID == "" || id <= 0 || ctx.ReadJSON(&input) != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": "评论审核参数无效"})
			return
		}
		if err := commentService.Moderate(appID, id, input.Status, input.Reason); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "评论状态已更新"})
	})
	adminParty.Delete("/article-comments/{id:int64}", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		id, _ := strconv.ParseInt(ctx.Params().Get("id"), 10, 64)
		if err := commentService.Moderate(appID, id, "deleted", "管理员删除"); err != nil {
			ctx.StatusCode(400)
			_ = ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}
		_ = ctx.JSON(iris.Map{"code": 0, "msg": "评论已删除"})
	})
}
