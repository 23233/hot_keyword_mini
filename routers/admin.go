// Package routers admin.go
package routers

import (
	"encoding/json"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/routers/middleware"
	"hot_keyword/services"
	"net/url"
	"strings"
	"time"

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

// SavePageAdminReq 管理后台保存/发布页面请求结构
type SavePageAdminReq struct {
	models.DynamicPage
	// 是否发布到线上 (默认为 false 存为草稿，true 需 release/admin 权限和人工显式确认)
	Publish bool `json:"publish"`
	// 人工显式确认标记 (发布时必填 true)
	Confirmed bool `json:"confirmed"`
	// 期望版本号 (乐观锁 CAS 比对，若大于 0 必须与当前库内版本一致)
	ExpectedRevision int `json:"expected_revision"`
	// 发布审计备注
	Remark string `json:"remark"`
}

// PublishPageDraftReq 管理后台显式发布草稿请求入参
type PublishPageDraftReq struct {
	// 所属小程序 AppID
	AppID string `json:"app_id"`
	// 目标页面 PageID
	PageID string `json:"page_id"`
	// 人工显式确认标记 (必填 true)
	Confirmed bool `json:"confirmed"`
	// 期望版本号 (乐观锁 CAS 比对)
	ExpectedRevision int `json:"expected_revision"`
	// 发布审计备注
	Remark string `json:"remark"`
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

// AdminPresignedUploadReq 管理后台图片直传 COS 请求参数。
type AdminPresignedUploadReq struct {
	// 所属小程序 AppID
	AppID string `json:"appId"`
	// 兼容使用下划线命名的调用方
	LegacyAppID string `json:"app_id"`
	// 原始文件名
	FileName string `json:"fileName"`
	// 文件大小（字节）
	FileSize int64 `json:"fileSize"`
	// 图片 MIME 类型
	ContentType string `json:"contentType"`
	// 资源分类
	OwnerType string `json:"ownerType"`
	// 资源所属业务 ID（预留，当前对象键使用随机 UUID）
	OwnerID int64 `json:"ownerId"`
}

// RegisterAdminRoutes 注册管理后台相关路由
func RegisterAdminRoutes(party iris.Party) {
	adminParty := party.Party("/api/v1/admin")

	// 挂载管理员认证拦截中间件 (保护所有后续敏感接口，放行 /auth/login)
	adminParty.Use(middleware.AdminAuthMiddleware)
	// 挂载只读角色写操作防御门禁 (viewer 角色禁止变更数据)
	adminParty.Use(middleware.ViewerReadOnlyMiddleware)

	// 挂载管理员登录与账户生命周期 CRUD 路由
	RegisterAdminUserRoutes(adminParty)
	RegisterMCPTokenRoutes(adminParty)

	dramaService := services.NewDramaService()
	sduiService := services.NewSDUIService()

	// 管理后台图片统一通过预签名 PUT 直传 COS，业务数据只保存 CDN 地址。
	adminParty.Post("/files/presigned-upload-url", func(ctx iris.Context) {
		var req AdminPresignedUploadReq
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": "图片上传参数不完整"})
			return
		}
		if strings.TrimSpace(req.AppID) == "" {
			req.AppID = req.LegacyAppID
		}
		if strings.TrimSpace(req.AppID) == "" || strings.TrimSpace(req.FileName) == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "图片上传参数不完整"})
			return
		}
		result, err := services.PrepareCOSUpload(ctx.Request().Context(), services.COSUploadRequest{AppID: req.AppID, FileName: req.FileName, FileSize: req.FileSize, ContentType: req.ContentType, OwnerType: req.OwnerType})
		if err != nil {
			ctx.JSON(iris.Map{"code": 400, "msg": err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": iris.Map{
				"presignedUrl":       result.PresignedURL,
				"finalCosFileUrl":    result.FinalCosFileURL,
				"presigned_url":      result.PresignedURL,
				"final_cos_file_url": result.FinalCosFileURL,
				"fileKey":            result.FileKey,
				"contentType":        result.ContentType,
				"uploadHeaders":      result.UploadHeaders,
				"expiresIn":          result.ExpiresIn,
			},
		})
	})

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
		var input struct {
			AppID              string `json:"app_id"`
			AppSecret          string `json:"app_secret"`
			AppName            string `json:"app_name"`
			CurrentPage        string `json:"current_page"`
			ReleaseMode        string `json:"release_mode"`
			FallbackPageID     string `json:"fallback_page_id"`
			CosCdnUrl          string `json:"cos_cdn_url"`
			PaymentMchID       string `json:"payment_mch_id"`
			PaymentMchSerialNo string `json:"payment_mch_serial_no"`
			PaymentAPIv3Key    string `json:"payment_api_v3_key"`
			PaymentPrivateKey  string `json:"payment_private_key"`
		}
		if err := ctx.ReadJSON(&input); err != nil || input.AppID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "小程序参数不合法"})
			return
		}
		if input.CosCdnUrl != "" {
			cdnURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(input.CosCdnUrl), "/"))
			if err != nil || cdnURL.Scheme != "https" || cdnURL.Host == "" || cdnURL.RawQuery != "" || cdnURL.Fragment != "" {
				ctx.JSON(iris.Map{"code": 400, "msg": "图片 CDN 必须是无查询参数的 HTTPS 根地址"})
				return
			}
			input.CosCdnUrl = strings.TrimRight(strings.TrimSpace(input.CosCdnUrl), "/")
		}
		app := models.MiniApp{AppID: input.AppID, AppSecret: input.AppSecret, AppName: input.AppName, CurrentPage: input.CurrentPage, ReleaseMode: input.ReleaseMode, FallbackPageID: input.FallbackPageID, CosCdnUrl: input.CosCdnUrl, PaymentMchID: input.PaymentMchID, PaymentMchSerialNo: input.PaymentMchSerialNo, PaymentAPIv3Key: input.PaymentAPIv3Key, PaymentPrivateKey: input.PaymentPrivateKey}
		if err := sduiService.SaveApp(&app); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "保存小程序失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "小程序配置已保存"})
	})

	// 商品管理：商品和金额只由后台配置，客户端不得传入金额。
	adminParty.Get("/products", func(ctx iris.Context) {
		appID := strings.TrimSpace(ctx.URLParam("app_id"))
		var products []models.Product
		if err := db.Mysql.Where("app_id = ?", appID).Order("id asc").Find(&products).Error; err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "获取商品失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "data": products})
	})
	adminParty.Post("/products", func(ctx iris.Context) {
		var product models.Product
		if err := ctx.ReadJSON(&product); err != nil || product.AppID == "" || product.SKU == "" || product.Name == "" || product.PriceFen <= 0 {
			ctx.JSON(iris.Map{"code": 400, "msg": "商品参数不完整或金额无效"})
			return
		}
		if product.Status == "" {
			product.Status = models.ProductStatusActive
		}
		if product.Status != models.ProductStatusActive && product.Status != models.ProductStatusInactive {
			ctx.JSON(iris.Map{"code": 400, "msg": "商品状态无效"})
			return
		}
		product.CreatedAt = time.Now()
		product.UpdatedAt = time.Now()
		if err := db.Mysql.Where("app_id = ? AND sku = ?", product.AppID, product.SKU).Assign(map[string]interface{}{"name": product.Name, "description": product.Description, "price_fen": product.PriceFen, "status": product.Status, "updated_at": product.UpdatedAt}).FirstOrCreate(&product).Error; err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "保存商品失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "data": product})
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

	// 8. 动态页面管理: 获取单个页面草稿箱协议
	adminParty.Get("/page/draft", func(ctx iris.Context) {
		appID := ctx.URLParam("app_id")
		pageID := ctx.URLParam("page_id")
		draft, err := sduiService.GetRawDraft(appID, pageID)
		if err != nil {
			ctx.JSON(iris.Map{"code": 404, "msg": "未找到指定草稿: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "success", "data": draft})
	})

	// 9. 动态页面管理: 保存草稿协议或执行受控发布 (受角色鉴权与人工确认门禁严格约束)
	adminParty.Post("/page", func(ctx iris.Context) {
		var req SavePageAdminReq
		if err := ctx.ReadJSON(&req); err != nil || req.AppID == "" || req.PageID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "页面参数不合法"})
			return
		}

		operator := ctx.Values().GetString("admin_username")
		if operator == "" {
			operator = "admin"
		}

		// A. 默认流程: 存为草稿 (无发布风险，普通 editor 可执行)
		if !req.Publish {
			draft := models.DynamicPageDraft{
				AppID:        req.AppID,
				PageID:       req.PageID,
				Revision:     req.Revision,
				Status:       "draft",
				Title:        req.Title,
				BusinessType: req.BusinessType,
				Intent:       req.Intent,
				Theme:        req.Theme,
				AccentColor:  req.AccentColor,
				RequireAuth:  req.RequireAuth,
				ShareConfig:  req.ShareConfig,
				Blocks:       req.Blocks,
				Keyword:      req.Keyword,
				Source:       req.Source,
				CampaignID:   req.CampaignID,
				ExpiresAt:    req.ExpiresAt,
				UpdatedBy:    operator,
			}
			if err := sduiService.SaveDraftWithAudit(&draft, operator, req.ExpectedRevision); err != nil {
				ctx.JSON(iris.Map{"code": 500, "msg": "保存草稿失败: " + err.Error()})
				return
			}
			ctx.JSON(iris.Map{"code": 0, "msg": "页面协议已成功保存至草稿箱", "data": draft})
			return
		}

		// B. 发布流程: 执行双重严格门禁 (角色权限 + 显式人工确认 + 乐观锁 CAS)
		userRole := ctx.Values().GetString("admin_role")
		if userRole != "super_admin" && userRole != "admin" {
			ctx.StatusCode(iris.StatusForbidden)
			ctx.JSON(iris.Map{"code": 403, "msg": "【权限不足】当前账户角色无 release 权限，禁止直接发布线上页面，请提交管理员审批"})
			return
		}

		if !req.Confirmed {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{"code": 400, "msg": "【发布门禁拦截】必须由人工审核通过并在参数中显式确认 confirmed: true 方可发布上线"})
			return
		}

		targetPage := req.DynamicPage
		targetPage.Status = "published"
		if err := sduiService.SavePageWithAudit(&targetPage, operator, req.Remark, req.ExpectedRevision); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "发布动态页面失败: " + err.Error()})
			return
		}
		ctx.JSON(iris.Map{"code": 0, "msg": "页面协议已成功发布上线", "data": targetPage})
	})

	// 10. 动态页面管理: 显式发布草稿箱页面至线上 (受 release 角色权限与 confirmed 门禁保护)
	adminParty.Post("/page/publish", middleware.RequireAdminRole("super_admin", "admin"), func(ctx iris.Context) {
		var req PublishPageDraftReq
		if err := ctx.ReadJSON(&req); err != nil || req.AppID == "" || req.PageID == "" {
			ctx.JSON(iris.Map{"code": 400, "msg": "请求参数不合法"})
			return
		}

		if !req.Confirmed {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{"code": 400, "msg": "【发布门禁拦截】必须人工确认发布 confirmed: true"})
			return
		}

		operator := ctx.Values().GetString("admin_username")
		if operator == "" {
			operator = "admin"
		}

		publishedPage, err := sduiService.PublishDraft(req.AppID, req.PageID, operator, req.Remark)
		if err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "发布草稿失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "草稿已成功发布生效",
			"data": publishedPage,
		})
	})

	// 11. 动态页面管理: 设为线上当前激活主页
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

	// 12. 行业模板库: 获取可用模板列表
	adminParty.Get("/templates", func(ctx iris.Context) {
		bType := ctx.URLParam("business_type")
		templates := services.GetGlobalTemplateRegistry().ListTemplates(bType)
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "success",
			"data": templates,
		})
	})

	// 13. 行业模板库: 一键套用模板至页面草稿箱 (严格仅存草稿，杜绝直接发布线上表)
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

		operator := ctx.Values().GetString("admin_username")
		if operator == "" {
			operator = "admin"
		}

		// 严格安全合规: 套用模板仅派生保存至草稿箱，绝不直接写线上表
		draft := models.DynamicPageDraft{
			AppID:        page.AppID,
			PageID:       page.PageID,
			Status:       "draft",
			Title:        page.Title,
			BusinessType: page.BusinessType,
			Intent:       page.Intent,
			Theme:        page.Theme,
			AccentColor:  page.AccentColor,
			RequireAuth:  page.RequireAuth,
			ShareConfig:  page.ShareConfig,
			Blocks:       page.Blocks,
			Keyword:      page.Keyword,
			Source:       page.Source,
			CampaignID:   page.CampaignID,
			ExpiresAt:    page.ExpiresAt,
			UpdatedBy:    operator,
		}

		if err := sduiService.SaveDraftWithAudit(&draft, operator, 0); err != nil {
			ctx.JSON(iris.Map{"code": 500, "msg": "套用模板保存草稿失败: " + err.Error()})
			return
		}

		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "模板套用成功，已保存至草稿箱，请核验无误后提交发布",
			"data": draft,
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
