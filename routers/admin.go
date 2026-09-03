// Package routers admin.go
package routers

import (
	"encoding/json"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/services"
	"strings"

	"github.com/kataras/iris/v12"
)

// AdminUpdateConfigRequest 保存页面配置请求参数
type AdminUpdateConfigRequest struct {
	// 展示模式: immersive_video / episode_grid / direct_portal / gallery_matrix / webview
	DisplayMode string `json:"display_mode"`
	// Webview模式目标跳转链接
	WebviewUrl string `json:"webview_url"`
	// 小程序页面主标题
	PageTitle string `json:"page_title"`
	// 小程序页面副标题
	PageSubtitle string `json:"page_subtitle"`
	// 顶部横幅公告
	Announcement string `json:"announcement"`
	// 分享标题
	ShareTitle string `json:"share_title"`
	// 分享描述
	ShareDesc string `json:"share_desc"`
	// 分享封面图
	ShareCover string `json:"share_cover"`
	// 看后续承接渠道列表
	ActionChannels []models.ActionChannel `json:"action_channels"`
	// 悬浮按钮配置
	FloatingButton *models.FloatingButton `json:"floating_button"`
}

// AdminUpdateDramaRequest 保存短剧信息请求参数
type AdminUpdateDramaRequest struct {
	// 剧集ID
	ID int64 `json:"id"`
	// 短剧标题
	Title string `json:"title"`
	// 宣传副标题
	Subtitle string `json:"subtitle"`
	// 竖版封面海报
	CoverUrl string `json:"cover_url"`
	// 横版宣传Banner
	BannerUrl string `json:"banner_url"`
	// 总集数
	TotalEpisodes int `json:"total_episodes"`
	// 评分
	Rating float64 `json:"rating"`
	// 热度指数
	HotScore int64 `json:"hot_score"`
	// 分类标签
	Tags string `json:"tags"`
	// 剧情简介
	Description string `json:"description"`
	// 爆款看点
	Highlights string `json:"highlights"`
	// 默认播放模式 (direct_video / channels_embedded / channels_video / web_view / none)
	PlayMode string `json:"play_mode"`
	// 微信视频号ID (内嵌与跳转通用参数)
	FinderUserName string `json:"finder_user_name"`
	// 微信视频号动态ID (内嵌与跳转通用参数)
	ChannelsFeedID string `json:"channels_feed_id"`
	// 外部网页播放链接
	WebUrl string `json:"web_url"`
}

// SetCurrentPageReq 设置激活主页请求
type SetCurrentPageReq struct {
	// 小程序 AppID
	AppID string `json:"app_id"`
	// 目标主页 PageID
	PageID string `json:"page_id"`
}

// ApplyTemplateReq 套用行业模板请求入参
type ApplyTemplateReq struct {
	// 行业模板 ID (如 tpl_drama_standard, tpl_game_redeem)
	TemplateID string `json:"template_id"`
	// 所属小程序 AppID
	AppID string `json:"app_id"`
	// 目标页面 PageID
	PageID string `json:"page_id"`
	// 目标页面主标题 (可选，默认为模板预设名称)
	Title string `json:"title"`
}

// GenerateShareCardReq 一键生成分享卡片请求入参
type GenerateShareCardReq struct {
	// 所属小程序 AppID
	AppID string `json:"app_id"`
	// 目标页面 PageID
	PageID string `json:"page_id"`
	// 部署服务 Host 域名 (可选，为空自动读取当前 host)
	Host string `json:"host"`
}

// RollbackPageReq 页面版本回滚请求
type RollbackPageReq struct {
	// 所属小程序 AppID
	AppID string `json:"app_id"`
	// 目标页面 PageID
	PageID string `json:"page_id"`
	// 目标历史修订版本号 (如 2)
	TargetRevision int `json:"target_revision"`
}

// PatchPageReq 协议打补丁请求入参
type PatchPageReq struct {
	// 所属小程序 AppID
	AppID string `json:"app_id"`
	// 目标页面 PageID
	PageID string `json:"page_id"`
	// 补丁操作集
	Ops []services.PatchOp `json:"ops"`
}

// RegisterAdminRoutes 注册管理后台相关路由
func RegisterAdminRoutes(party iris.Party) {
	adminParty := party.Party("/api/v1/admin")
	dramaService := services.NewDramaService()
	sduiService := services.NewSDUIService()

	// 1. 获取当前管理后台完整数据
	adminParty.Get("/config", func(ctx iris.Context) {
		homeData, err := dramaService.GetHomeData("")
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": err.Error()})
			return
		}
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": homeData,
		})
	})

	// 2. 保存更新页面与渠道配置
	adminParty.Post("/config", func(ctx iris.Context) {
		var req AdminUpdateConfigRequest
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求参数无效: " + err.Error()})
			return
		}

		drama, err := dramaService.GetDefaultDrama()
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "获取短剧数据失败: " + err.Error()})
			return
		}

		channelsJSON, _ := json.Marshal(req.ActionChannels)

		updateMap := map[string]interface{}{
			"display_mode":    req.DisplayMode,
			"webview_url":     req.WebviewUrl,
			"page_title":      req.PageTitle,
			"page_subtitle":   req.PageSubtitle,
			"announcement":    req.Announcement,
			"share_title":     req.ShareTitle,
			"share_desc":      req.ShareDesc,
			"share_cover":     req.ShareCover,
			"action_channels": string(channelsJSON),
		}

		err = db.Mysql.Model(&models.PageConfig{}).
			Where("drama_id = ?", drama.ID).
			Updates(updateMap).Error
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "更新配置失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{"code": 0, "msg": "配置已成功保存并同步到线上"})
	})

	// 3. 保存更新短剧主信息及视频号核心参数
	adminParty.Post("/drama", func(ctx iris.Context) {
		var req AdminUpdateDramaRequest
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "参数格式错误: " + err.Error()})
			return
		}

		updateData := map[string]interface{}{
			"title":            req.Title,
			"subtitle":         req.Subtitle,
			"cover_url":        req.CoverUrl,
			"banner_url":       req.BannerUrl,
			"total_episodes":   req.TotalEpisodes,
			"rating":           req.Rating,
			"hot_score":        req.HotScore,
			"tags":             req.Tags,
			"description":      req.Description,
			"highlights":       req.Highlights,
			"play_mode":        req.PlayMode,
			"finder_user_name": req.FinderUserName,
			"channels_feed_id": req.ChannelsFeedID,
			"web_url":          req.WebUrl,
		}

		err := db.Mysql.Model(&models.Drama{}).Where("id = ?", req.ID).Updates(updateData).Error
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "更新短剧信息失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{"code": 0, "msg": "短剧信息及视频号参数已更新"})
	})

	// 4. 多小程序管理: 获取小程序列表
	adminParty.Get("/apps", func(ctx iris.Context) {
		apps, err := sduiService.ListApps()
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "获取小程序列表失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "success", "data": apps})
	})

	// 5. 多小程序管理: 新增或更新小程序配置
	adminParty.Post("/apps", func(ctx iris.Context) {
		var app models.MiniApp
		if err := ctx.ReadJSON(&app); err != nil || app.AppID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "小程序参数不合法"})
			return
		}
		if err := sduiService.SaveApp(&app); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "保存小程序失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "小程序配置已保存"})
	})

	// 6. 动态页面管理: 获取指定小程序的全部页面
	adminParty.Get("/pages", func(ctx iris.Context) {
		appID := ctx.URLParam("app_id")
		pages, err := sduiService.ListPages(appID)
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "获取页面列表失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "success", "data": pages})
	})

	// 7. 动态页面管理: 获取单个页面原始协议
	adminParty.Get("/page", func(ctx iris.Context) {
		appID := ctx.URLParam("app_id")
		pageID := ctx.URLParam("page_id")
		page, err := sduiService.GetRawPage(appID, pageID)
		if err != nil {
			ctx.JSON(iris.Map{"code": 404, "msg": "未找到指定页面: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "success", "data": page})
	})

	// 8. 动态页面管理: 保存并发布动态页面协议 (版本号自增)
	adminParty.Post("/page", func(ctx iris.Context) {
		var page models.DynamicPage
		if err := ctx.ReadJSON(&page); err != nil || page.AppID == "" || page.PageID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "页面参数不合法"})
			return
		}
		if err := sduiService.SavePage(&page); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "保存动态页面失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "页面协议已成功发布"})
	})

	// 9. 动态页面管理: 设为线上当前激活主页
	adminParty.Post("/page/set_current", func(ctx iris.Context) {
		var req SetCurrentPageReq
		if err := ctx.ReadJSON(&req); err != nil || req.AppID == "" || req.PageID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求参数不完整"})
			return
		}
		if err := sduiService.SetCurrentPage(req.AppID, req.PageID); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "设置主页失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "已成功设为线上主页"})
	})

	// 10. 行业模板库: 获取可用模板列表
	adminParty.Get("/templates", func(ctx iris.Context) {
		bType := ctx.URLParam("business_type")
		templates := services.GetGlobalTemplateRegistry().ListTemplates(bType)
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": templates,
		})
	})

	// 11. 行业模板库: 一键套用模板至页面草稿
	adminParty.Post("/templates/apply", func(ctx iris.Context) {
		var req ApplyTemplateReq
		if err := ctx.ReadJSON(&req); err != nil || req.TemplateID == "" || req.AppID == "" || req.PageID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "请完整提供 template_id, app_id 与 page_id"})
			return
		}

		page, err := services.GetGlobalTemplateRegistry().ApplyTemplateToPage(req.TemplateID, req.AppID, req.PageID, req.Title)
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "套用模板失败: " + err.Error()})
			return
		}

		if err := sduiService.SavePage(page); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "保存派生页面失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "模板套用成功，已生成页面协议",
			"data": page,
		})
	})

	// 12. 视觉分享: 一键为页面生成标准微信分享卡片并回写更新配置
	adminParty.Post("/page/generate_share_card", func(ctx iris.Context) {
		var req GenerateShareCardReq
		if err := ctx.ReadJSON(&req); err != nil || req.AppID == "" || req.PageID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求参数不完整"})
			return
		}

		host := req.Host
		if host == "" {
			host = ctx.Host()
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				scheme := "http://"
				if ctx.Request().TLS != nil {
					scheme = "https://"
				}
				host = scheme + host
			}
		}

		shareCardService := services.NewShareCardService()
		if err := shareCardService.AutoUpdatePageShareConfig(req.AppID, req.PageID, host); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "生成分享卡片失败: " + err.Error()})
			return
		}

		cardURL := fmt.Sprintf("%s/api/v1/share/card?app_id=%s&page_id=%s&type=app_message", host, req.AppID, req.PageID)
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "微信 5:4 分享卡片已自动生成并保存",
			"data": iris.Map{
				"card_url": cardURL,
			},
		})
	})

	// 13. 版本管理: 获取指定动态页面的历史版本快照
	adminParty.Get("/page/revisions", func(ctx iris.Context) {
		appID := ctx.URLParam("app_id")
		pageID := ctx.URLParam("page_id")
		revs, err := sduiService.ListPageRevisions(appID, pageID)
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "获取版本快照列表失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": revs,
		})
	})

	// 14. 版本管理: 一键原子回滚至历史版本
	adminParty.Post("/page/rollback", func(ctx iris.Context) {
		var req RollbackPageReq
		if err := ctx.ReadJSON(&req); err != nil || req.AppID == "" || req.PageID == "" || req.TargetRevision <= 0 {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求参数不完整"})
			return
		}

		newPage, err := sduiService.RollbackPageRevision(req.AppID, req.PageID, req.TargetRevision)
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "回滚失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  fmt.Sprintf("页面已成功原子回滚至版本 v%d，新版本已发布生效", req.TargetRevision),
			"data": newPage,
		})
	})

	// 15. 自动化编排: 协议强校验器 (Validate)
	adminParty.Post("/page/validate", func(ctx iris.Context) {
		var page models.DynamicPage
		if err := ctx.ReadJSON(&page); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "协议反序列化失败: " + err.Error()})
			return
		}

		report := services.ValidateDynamicPage(&page)
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "校验完成",
			"data": report,
		})
	})

	// 16. 自动化编排: 受控 JSON Patch 局部更新
	adminParty.Post("/page/patch", func(ctx iris.Context) {
		var req PatchPageReq
		if err := ctx.ReadJSON(&req); err != nil || req.AppID == "" || req.PageID == "" || len(req.Ops) == 0 {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求入参不合法"})
			return
		}

		patchedPage, err := services.PatchDynamicPage(req.AppID, req.PageID, req.Ops)
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "应用补丁失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "补丁已成功应用并发布",
			"data": patchedPage,
		})
	})
}
