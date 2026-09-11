// Package system migrate.go
package system

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/services"
	"log"
	"os"
	"strings"
	"time"

	"github.com/23233/ggg/ut"
	"gorm.io/gorm"
)

// Migrate 数据库迁移
func Migrate() error {

	migrateList := []any{
		&models.User{},
		&models.Drama{},
		&models.DramaEpisode{},
		&models.PageConfig{},
		&models.MiniApp{},
		&models.DynamicPage{},
		&models.DynamicPageDraft{},
		&models.DynamicPageRevision{},
		&models.DynamicPageTemplate{},
		&models.CapabilityChangeLog{},
		&models.UserSession{},
		&models.GameRedeemPackage{},
		&models.GameRedeemRecord{},
		&models.AdminUser{},
		&models.MCPAccessToken{},
		&models.WebViewTicket{},
		&models.Product{},
		&models.PaymentOrder{},
		&models.PlatformOperation{},
		&models.WalletAccount{},
		&models.ArticleCategory{},
		&models.Article{},
		&models.ArticlePurchase{},
		&models.MembershipLevel{},
		&models.UserMembership{},
		&models.ArticleComment{},
		&models.ContentAuditRecord{},
	}

	err := db.Mysql.AutoMigrate(migrateList...)
	if err != nil {
		log.Fatalf("无法自动迁移数据库: %v", err)
		return err
	}

	return nil
}

// EnsureInitialAdmin 仅负责确保管理员账户存在，不初始化任何业务种子数据。
func EnsureInitialAdmin() error {
	if db.Mysql == nil {
		return nil
	}

	var adminUser models.AdminUser
	errAdmin := db.Mysql.Where("username = ?", "admin").First(&adminUser).Error
	if errAdmin == nil && adminUser.ID != 0 {
		oldHasher := sha256.New()
		oldHasher.Write([]byte("admin123456" + adminUser.Salt))
		isWeakDefault := hex.EncodeToString(oldHasher.Sum(nil)) == adminUser.PasswordHash
		if !isWeakDefault && os.Getenv("RESET_ADMIN_PASSWORD") != "true" {
			return nil
		}

		newPwd := os.Getenv("ADMIN_INITIAL_PASSWORD")
		if newPwd == "" {
			newPwd = ut.RandomStr(16)
		}
		newSalt := "sdui_salt_" + ut.RandomStr(16)
		newHasher := sha256.New()
		newHasher.Write([]byte(newPwd + newSalt))
		if err := db.Mysql.Model(&adminUser).Updates(map[string]interface{}{
			"password_hash": hex.EncodeToString(newHasher.Sum(nil)),
			"salt":          newSalt,
			"status":        "active",
			"updated_at":    time.Now(),
		}).Error; err != nil {
			return err
		}
		log.Printf("管理员 admin 密码已重置，请妥善保存新密码: %s", newPwd)
		return nil
	}
	if errAdmin != nil && !errors.Is(errAdmin, gorm.ErrRecordNotFound) {
		return errAdmin
	}

	initialPwd := os.Getenv("ADMIN_INITIAL_PASSWORD")
	if initialPwd == "" {
		initialPwd = ut.RandomStr(16)
		log.Printf("==================================================================")
		log.Printf("【安全提醒】已自动生成初始管理员随机密码:")
		log.Printf("用户名: admin")
		log.Printf("密码:   %s", initialPwd)
		log.Printf("请务必保存并在首次登录后立即修改！")
		log.Printf("==================================================================")
	}

	salt := "sdui_salt_" + ut.RandomStr(16)
	hasher := sha256.New()
	hasher.Write([]byte(initialPwd + salt))
	superAdmin := models.AdminUser{
		Username:     "admin",
		PasswordHash: hex.EncodeToString(hasher.Sum(nil)),
		Salt:         salt,
		RealName:     "系统超级管理员",
		Role:         "super_admin",
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return db.Mysql.Create(&superAdmin).Error
}

// EnsureAIBreakthroughData 将默认小程序名称切换为 ai 破甲，并初始化三档会员与导航内容。
func EnsureAIBreakthroughData() error {
	if db.Mysql == nil {
		return nil
	}
	const appID = "wx516563cfe994bbc6"
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		app = models.MiniApp{AppID: appID, AppName: "ai破甲", CurrentPage: "home", ReleaseMode: "normal", FallbackPageID: "home", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := db.Mysql.Create(&app).Error; err != nil {
			return err
		}
	} else if err := db.Mysql.Model(&app).Updates(map[string]interface{}{"app_name": "ai破甲", "updated_at": time.Now()}).Error; err != nil {
		return err
	}
	if strings.TrimSpace(app.CapabilityMatrix) == "" {
		matrix, matrixErr := services.BuildReadyCapabilityMatrix()
		if matrixErr != nil {
			return matrixErr
		}
		if err := db.Mysql.Model(&app).Updates(map[string]interface{}{"capability_matrix": matrix, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
	}
	if err := services.NewMembershipService().SeedDefaultPlans(appID); err != nil {
		return err
	}
	if err := services.NewArticleService().SeedDefaultContent(appID); err != nil {
		return err
	}
	if err := ensureAIBreakthroughPage(appID, "home", "tpl_ai_breakthrough_portal", "ai破甲"); err != nil {
		return err
	}
	if err := ensureAIBreakthroughPage(appID, "article_detail", "tpl_ai_breakthrough_article", "AI 破甲文章"); err != nil {
		return err
	}
	return ensureAIBreakthroughPage(appID, "membership", "tpl_ai_breakthrough_membership", "AI 破甲会员中心")
}

// EnsureSDUIAcceptanceData 在开发环境同步固定的 SDUI 全量验收页面。
// 该页面只用于微信开发者工具验收，不覆盖运营页面，也不会在生产环境调用。
func EnsureSDUIAcceptanceData() error {
	if db.Mysql == nil {
		return nil
	}
	if err := ensureWebViewRegistry("wx516563cfe994bbc6"); err != nil {
		return err
	}

	const (
		appID      = "wx516563cfe994bbc6"
		pageID     = "component_lab_acceptance_20260907"
		templateID = "tpl_sdui_component_lab"
		title      = "SDUI 全量组件验收"
	)

	service := services.NewSDUIService()
	existing, err := service.GetRawPage(appID, pageID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && !strings.Contains(err.Error(), "页面不存在") {
		return err
	}

	page, err := services.NewTemplateService().ApplyTemplateToPage(templateID, appID, pageID, title)
	if err != nil {
		return err
	}
	page.Status = "published"
	page.RequireAuth = false

	if existing != nil {
		// 验收页由模板唯一维护；只有协议内容或发布元数据变化时才创建新快照。
		if existing.Status == page.Status && existing.Title == page.Title && existing.BusinessType == page.BusinessType &&
			existing.Intent == page.Intent && existing.Theme == page.Theme && existing.AccentColor == page.AccentColor &&
			existing.RequireAuth == page.RequireAuth && existing.Blocks == page.Blocks && existing.ShareConfig == page.ShareConfig &&
			existing.CampaignID == page.CampaignID {
			return nil
		}
		page.Revision = existing.Revision
	}

	return service.SavePageWithAudit(page, "system", "同步 SDUI 全量组件验收模板", 0)
}

// ensureWebViewRegistry 为本地验收租户补齐同一份受控 WebView 入口。
func ensureWebViewRegistry(appID string) error {
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return err
	}
	if strings.TrimSpace(app.WebViewRegistry) != "" {
		return nil
	}
	registry, _ := json.Marshal([]services.WebViewEntry{{URLKey: "component-lab", URL: "https://wx.a0free.com", Purpose: "SDUI 组件验收 WebView", Version: "2026.09", Enabled: true}})
	return db.Mysql.Model(&app).Updates(map[string]interface{}{"webview_registry": string(registry), "updated_at": time.Now()}).Error
}

// EnsureLocalSDUITenants 初始化两个本地验收租户及其通用能力矩阵。
func EnsureLocalSDUITenants() error {
	if db.Mysql == nil {
		return nil
	}
	type tenant struct {
		AppID string
		Name  string
	}
	tenants := []tenant{
		{AppID: "wx516563cfe994bbc6", Name: "AI破甲"},
		{AppID: "wx7a779add6a689881", Name: "设身处地游戏"},
		{AppID: "wx8b8e899d4829481a", Name: "斗草粮"},
	}
	for _, item := range tenants {
		matrix, err := services.BuildReadyCapabilityMatrix()
		if err != nil {
			return err
		}
		var app models.MiniApp
		err = db.Mysql.Where("app_id = ?", item.AppID).First(&app).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			app = models.MiniApp{AppID: item.AppID, AppName: item.Name, CurrentPage: "home", ReleaseMode: "normal", FallbackPageID: "home", CapabilityMatrix: matrix, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			if err := db.Mysql.Create(&app).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if strings.TrimSpace(app.CapabilityMatrix) == "" {
			if err := db.Mysql.Model(&app).Updates(map[string]interface{}{"capability_matrix": matrix, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		if strings.TrimSpace(app.WebViewRegistry) == "" {
			registry, _ := json.Marshal([]services.WebViewEntry{{URLKey: "component-lab", URL: "https://wx.a0free.com", Purpose: "SDUI 组件验收 WebView", Version: "2026.09", Enabled: true}})
			if err := db.Mysql.Model(&app).Updates(map[string]interface{}{"webview_registry": string(registry), "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		if err := ensureSDUIAcceptanceDraft(item.AppID, item.Name+" SDUI 组件验收"); err != nil {
			return err
		}
	}
	if err := ensureLocalGameDataAndPages("wx7a779add6a689881"); err != nil {
		return err
	}
	return ensureLocalDCLPages("wx8b8e899d4829481a")
}

// ensureLocalGameDataAndPages 初始化设身处地游戏的本地礼包数据和四个首发页面。
func ensureLocalGameDataAndPages(appID string) error {
	const packageID = "pkg_game_novice_888"
	var packageConfig models.GameRedeemPackage
	err := db.Mysql.Where("app_id = ? AND package_id = ?", appID, packageID).First(&packageConfig).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		packageConfig = models.GameRedeemPackage{
			AppID: appID, PackageID: packageID, GameID: "game_default", Title: "设身处地游戏公测礼包",
			Description: "新游公测礼包，领取后复制兑换码到游戏内使用", TotalStock: 10000, RemainingStock: 10000,
			CodePrefix: "GAME-", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := db.Mysql.Create(&packageConfig).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	nav := func(active string) models.BlockItem {
		items := []interface{}{
			map[string]interface{}{"key": "home", "label": "首页", "page_id": "home"},
			map[string]interface{}{"key": "game_center", "label": "游戏中心", "page_id": "game_center"},
			map[string]interface{}{"key": "game_redeem", "label": "兑换码", "page_id": "game_redeem"},
		}
		return models.BlockItem{ID: "game_bottom_nav", Type: "bottom_nav", Props: map[string]interface{}{"active_key": active, "items": items}}
	}
	redeemAction := func() *models.BlockAction {
		return &models.BlockAction{
			Type: "request_data",
			Payload: map[string]interface{}{
				"endpoint": "game.redeem",
				"body":     map[string]interface{}{"package_id": packageID},
				"response": map[string]interface{}{"data_path": "data", "save_as": "redeem_result"},
			},
			OnSuccess: []models.BlockAction{{Type: "copy_text", Payload: map[string]interface{}{"path": "$result.code", "toast": "兑换码已复制，请进入游戏兑换"}}},
		}
	}
	gameHeader := func(id, title, description, active string, action *models.BlockAction) models.BlockItem {
		return models.BlockItem{ID: id, Type: "game_header", Props: map[string]interface{}{
			"title": title, "version": "本地公测版", "publisher": "设身处地游戏", "tags": []interface{}{"官方礼包", "限量领取"},
			"description": description, "btn_text": "领取兑换码",
		}, Action: action}
	}
	gameCard := func(id, title string, action *models.BlockAction) models.BlockItem {
		return models.BlockItem{ID: id, Type: "game_card", Props: map[string]interface{}{
			"title": title, "subtitle": "公测礼包与游戏资料", "version": "最新公测", "package_id": packageID,
			"redeem_code": "点击领取", "copy_action": action,
		}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "game_detail", "query": map[string]interface{}{"package_id": packageID}}}}
	}
	pages := []*models.DynamicPage{
		localSeedPage(appID, "home", "设身处地游戏", "game", "watch", "dark_glass", "#0A84FF", []models.BlockItem{
			{ID: "game_notice", Type: "notice", Props: map[string]interface{}{"icon": "🎮", "text": "设身处地游戏中心，公测礼包码实时更新"}},
			gameHeader("game_home_header", "设身处地游戏", "游戏介绍、版本信息和礼包入口由后台动态配置。", "home", &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "game_center"}}),
			gameCard("game_home_card", "热门公测游戏", redeemAction()),
			{ID: "game_home_redeem", Type: "action_button", Props: map[string]interface{}{"text": "领取独家公测礼包", "badge": "匿名领取 · 领取后自动复制"}, Action: redeemAction()},
			nav("home"),
		}),
		localSeedPage(appID, "game_center", "游戏中心", "game", "watch", "dark_glass", "#0A84FF", []models.BlockItem{
			{ID: "game_center_notice", Type: "notice", Props: map[string]interface{}{"icon": "🕹️", "text": "选择游戏查看详情，礼包码按需领取"}},
			gameCard("game_center_card_1", "热门公测游戏", redeemAction()),
			gameCard("game_center_card_2", "新游体验专区", redeemAction()),
			nav("game_center"),
		}),
		localSeedPage(appID, "game_detail", "游戏详情", "game", "watch", "dark_glass", "#0A84FF", []models.BlockItem{
			gameHeader("game_detail_header", "热门公测游戏", "游戏介绍、版本说明和礼包内容统一由 SDUI 页面配置。", "game_detail", &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "game_redeem", "query": map[string]interface{}{"package_id": packageID}}}),
			{ID: "game_detail_features", Type: "item_grid", Props: map[string]interface{}{"title": "礼包内容", "items": []interface{}{map[string]interface{}{"title": "公测专属道具"}, map[string]interface{}{"title": "限量兑换资格"}, map[string]interface{}{"title": "领取后立即复制"}}}},
			{ID: "game_detail_description", Type: "rich_text", Props: map[string]interface{}{"content": "## 游戏介绍\n\n这是由后台动态维护的游戏详情页。游戏资料、版本和礼包入口都可以在管理后台调整，无需重新编译客户端。"}},
			nav("game_detail"),
		}),
		localSeedPage(appID, "game_redeem", "游戏兑换码", "game", "redeem", "cyber_neon", "#30D158", []models.BlockItem{
			gameHeader("game_redeem_header", "游戏兑换码", "点击领取后，服务端返回兑换码并自动复制到剪贴板。", "game_redeem", nil),
			{ID: "game_redeem_card", Type: "redeem_code_card", Props: map[string]interface{}{"title": "公测独家礼包", "desc": "匿名领取，领取成功后自动复制", "btn_text": "领取并复制", "code": ""}, Action: redeemAction(), Empty: &models.BlockItem{ID: "game_redeem_empty", Type: "empty", Props: map[string]interface{}{"title": "礼包暂未开放"}}, Error: &models.BlockItem{ID: "game_redeem_error", Type: "custom", Props: map[string]interface{}{"title": "领取失败", "content": "请稍后重试"}}},
			nav("game_redeem"),
		}),
	}
	for _, page := range pages {
		if err := ensureLocalSeedPage(page); err != nil {
			return err
		}
	}
	return nil
}

// ensureLocalDCLPages 初始化斗艺馆的只读首发页面族和一个默认隐藏分支。
func ensureLocalDCLPages(appID string) error {
	nav := func(active string) models.BlockItem {
		return models.BlockItem{ID: "dcl_bottom_nav", Type: "bottom_nav", Props: map[string]interface{}{"active_key": active, "items": []interface{}{
			map[string]interface{}{"key": "home", "label": "首页", "page_id": "home"},
			map[string]interface{}{"key": "encyclopedia", "label": "百科", "page_id": "encyclopedia"},
			map[string]interface{}{"key": "masters", "label": "斗师", "page_id": "masters"},
			map[string]interface{}{"key": "marketplace", "label": "市集", "page_id": "marketplace"},
		}}}
	}
	contentItems := []interface{}{
		map[string]interface{}{"key": "story", "title": "品牌故事", "summary": "品牌、器型与材质资料", "meta": "持续更新"},
		map[string]interface{}{"key": "care", "title": "养护知识", "summary": "日常保养与使用建议", "meta": "百科"},
		map[string]interface{}{"key": "masters", "title": "斗师作品", "summary": "查看斗师与作品展示", "meta": "作品库"},
	}
	pages := []*models.DynamicPage{
		localSeedPage(appID, "home", "斗艺馆", "custom", "watch", "light_clean", "#0A84FF", []models.BlockItem{
			{ID: "dcl_notice", Type: "notice", Props: map[string]interface{}{"icon": "◌", "text": "斗艺馆 · 品牌、百科、作品与服务信息"}},
			{ID: "dcl_intro", Type: "text", Props: map[string]interface{}{"title": "斗艺馆", "content": "内容和页面入口由统一 SDUI 协议动态配置。"}},
			{ID: "dcl_home_feed", Type: "content_feed", Props: map[string]interface{}{"title": "精选内容", "items": contentItems, "fields": map[string]interface{}{"key": "key", "title": "title", "summary": "summary", "meta": "meta"}}},
			nav("home"),
		}),
		localSeedPage(appID, "encyclopedia", "斗艺百科", "custom", "watch", "light_clean", "#0A84FF", []models.BlockItem{
			{ID: "dcl_encyclopedia_nav", Type: "collection_nav", Props: map[string]interface{}{"title": "资料分类", "items": []interface{}{map[string]interface{}{"key": "brand", "label": "品牌"}, map[string]interface{}{"key": "shape", "label": "器型"}, map[string]interface{}{"key": "material", "label": "材质"}}, "fields": map[string]interface{}{"key": "key", "label": "label"}}},
			{ID: "dcl_encyclopedia_feed", Type: "content_feed", Props: map[string]interface{}{"title": "百科内容", "items": contentItems, "fields": map[string]interface{}{"key": "key", "title": "title", "summary": "summary", "meta": "meta"}}},
			nav("encyclopedia"),
		}),
		localSeedPage(appID, "brand_detail", "品牌详情", "custom", "watch", "light_clean", "#0A84FF", []models.BlockItem{
			{ID: "dcl_brand_title", Type: "text", Props: map[string]interface{}{"title": "品牌故事", "content": "品牌资料、产品信息和内容说明可由后台随时调整。"}},
			{ID: "dcl_brand_body", Type: "rich_text", Props: map[string]interface{}{"content": "## 品牌资料\n\n这里是可复用的内容详情页面。后续可以通过协议追加图片、画廊、地图或客服入口，并由租户能力矩阵决定是否显示。"}},
			nav("encyclopedia"),
		}),
		localSeedPage(appID, "masters", "斗师与作品", "custom", "watch", "light_clean", "#0A84FF", []models.BlockItem{
			{ID: "dcl_masters_feed", Type: "content_feed", Props: map[string]interface{}{"title": "斗师展示", "items": []interface{}{map[string]interface{}{"key": "master-1", "title": "资深斗师", "summary": "作品与服务资料待审核", "meta": "展示中"}, map[string]interface{}{"key": "master-2", "title": "青年斗师", "summary": "作品集与服务范围", "meta": "展示中"}}, "fields": map[string]interface{}{"key": "key", "title": "title", "summary": "summary", "meta": "meta"}}},
			nav("masters"),
		}),
		localSeedPage(appID, "marketplace", "合规市集", "custom", "watch", "light_clean", "#0A84FF", []models.BlockItem{
			{ID: "dcl_market_feed", Type: "content_feed", Props: map[string]interface{}{"title": "商品展示", "items": []interface{}{map[string]interface{}{"key": "item-1", "title": "精选作品", "summary": "商品详情和合规状态由后台配置", "meta": "待开放"}, map[string]interface{}{"key": "item-2", "title": "配件专区", "summary": "分类、库存和展示状态可控", "meta": "待开放"}}, "fields": map[string]interface{}{"key": "key", "title": "title", "summary": "summary", "meta": "meta"}}},
			nav("marketplace"),
		}),
		localSeedPageHidden(appID, "service_orders", "服务订单（暂未开放）", []models.BlockItem{{ID: "dcl_service_gate", Type: "feature_gate", Props: map[string]interface{}{"title": "服务订单能力暂未开放", "content": "该分支由后台页面隐藏控制。"}}}),
	}
	for _, page := range pages {
		if err := ensureLocalSeedPage(page); err != nil {
			return err
		}
	}
	return nil
}

// localSeedPage 构造本地首发页面实体，页面协议仍使用公共 DynamicPage 模型。
func localSeedPage(appID, pageID, title, businessType, intent, theme, accent string, blocks []models.BlockItem) *models.DynamicPage {
	encoded, _ := json.Marshal(blocks)
	return &models.DynamicPage{AppID: appID, PageID: pageID, Revision: 1, Status: "published", Hidden: false, Title: title, BusinessType: businessType, Intent: intent, Theme: theme, AccentColor: accent, ShareConfig: "{}", Blocks: string(encoded), Keyword: title, Source: "local_seed", CampaignID: "local_sdui_seed", CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

// localSeedPageHidden 构造默认隐藏的本地页面分支。
func localSeedPageHidden(appID, pageID, title string, blocks []models.BlockItem) *models.DynamicPage {
	page := localSeedPage(appID, pageID, title, "custom", "watch", "light_clean", "#0A84FF", blocks)
	page.Hidden = true
	return page
}

// ensureLocalSeedPage 只创建缺失的本地种子页面，不覆盖已有运营页面。
func ensureLocalSeedPage(page *models.DynamicPage) error {
	service := services.NewSDUIService()
	_, err := service.GetRawPage(page.AppID, page.PageID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return service.SavePageWithAudit(page, "system", "初始化本地 SDUI 首发页面", 0)
}

// ensureSDUIAcceptanceDraft 为指定租户同步同一份隐藏验收草稿，供 MCP、Web 预览和微信开发者工具复用。
func ensureSDUIAcceptanceDraft(appID, title string) error {
	const (
		pageID     = "component_lab_acceptance_20260907"
		templateID = "tpl_sdui_component_lab"
	)
	page, err := services.NewTemplateService().ApplyTemplateToPage(templateID, appID, pageID, title)
	if err != nil {
		return err
	}
	draft := &models.DynamicPageDraft{
		AppID: page.AppID, PageID: page.PageID, Status: "draft", Hidden: true, Title: page.Title,
		BusinessType: page.BusinessType, Intent: page.Intent, Theme: page.Theme, AccentColor: page.AccentColor,
		RequireAuth: false, ShareConfig: page.ShareConfig, Blocks: page.Blocks, Keyword: page.Keyword,
		Source: "local_acceptance", CampaignID: "acceptance_fixture", ExpiresAt: page.ExpiresAt,
		UpdatedBy: "system",
	}
	service := services.NewSDUIService()
	existing, findErr := service.FindRawDraft(appID, pageID)
	if findErr == nil {
		if existing.Title == draft.Title && existing.Hidden == draft.Hidden && existing.BusinessType == draft.BusinessType &&
			existing.Intent == draft.Intent && existing.Theme == draft.Theme && existing.AccentColor == draft.AccentColor &&
			existing.RequireAuth == draft.RequireAuth && existing.ShareConfig == draft.ShareConfig && existing.Blocks == draft.Blocks &&
			existing.Keyword == draft.Keyword && existing.Source == draft.Source && existing.CampaignID == draft.CampaignID {
			return nil
		}
		draft.Revision = existing.Revision
		return service.SaveDraftWithAudit(draft, "system", existing.Revision)
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}
	return service.SaveDraftWithAudit(draft, "system", 0)
}

func ensureAIBreakthroughPage(appID, pageID, templateID, title string) error {
	existingRevision := 0
	if existing, err := services.NewSDUIService().GetRawPage(appID, pageID); err == nil {
		existingRevision = existing.Revision
		businessType := map[string]string{"tpl_ai_breakthrough_portal": "ai_breakthrough", "tpl_ai_breakthrough_article": "ai_article", "tpl_ai_breakthrough_membership": "ai_breakthrough"}[templateID]
		// 只迁移本项目早期生成的旧 AI 破甲页面；后台后续自定义页面不被启动初始化覆盖。
		if existing.BusinessType != businessType || existing.CampaignID != "template_derived" {
			return nil
		}
		migrated := map[string]string{"tpl_ai_breakthrough_portal": "$entity.content", "tpl_ai_breakthrough_article": "content_discussion", "tpl_ai_breakthrough_membership": "content_offers"}[templateID]
		upToDate := migrated != "" && strings.Contains(existing.Blocks, migrated) && strings.Contains(existing.Blocks, `"layout/flat"`) && existing.Theme == "light_clean"
		if templateID == "tpl_ai_breakthrough_portal" {
			upToDate = upToDate && strings.Contains(existing.Blocks, `"template_version":"3.1.0"`) && strings.Count(existing.Blocks, `"action":{"type":"navigate_page"`) >= 4
		}
		if migrated == "" || upToDate {
			return nil
		}
	}
	service := services.NewTemplateService()
	page, err := service.ApplyTemplateToPage(templateID, appID, pageID, title)
	if err != nil {
		return err
	}
	page.Status = "published"
	page.RequireAuth = false
	if existingRevision > 0 {
		page.Revision = existingRevision
	}
	return services.NewSDUIService().SavePageWithAudit(page, "system", "初始化 AI 破甲资讯模板", 0)
}

// SeedMultiTenantAndSDUIData 自举初始化多租户小程序与默认 SDUI 页面
func SeedMultiTenantAndSDUIData() error {
	defaultAppID := "wx516563cfe994bbc6"
	defaultSecret := "673ac42e8aaec2f9cfa5547e780f7658"

	// 1. 初始化默认小程序
	var count int64
	db.Mysql.Model(&models.MiniApp{}).Where("app_id = ?", defaultAppID).Count(&count)
	if count == 0 {
		defaultApp := models.MiniApp{
			AppID:          defaultAppID,
			AppSecret:      defaultSecret,
			AppName:        "ai破甲",
			CurrentPage:    "home",
			ReleaseMode:    "normal",
			FallbackPageID: "home",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := db.Mysql.Create(&defaultApp).Error; err != nil {
			return err
		}
	}

	// 2. 初始化默认 SDUI 首页 (如果不存在)
	var pageCount int64
	db.Mysql.Model(&models.DynamicPage{}).Where("app_id = ? AND page_id = ?", defaultAppID, "home").Count(&pageCount)
	if pageCount == 0 {
		blocksJSON := `[
			{
				"id": "block_hero_101",
				"type": "media_hero",
				"props": {
					"title": "猴王下山",
					"subtitle": "第 1 集试看 · 全网爆火",
					"cover_url": "",
					"video_url": "https://sample-videos.com/video321/mp4/720/big_buck_bunny_720p_1mb.mp4",
					"rating": 9.8,
					"hot_score": 998000
				},
				"style": {
					"utilities": ["space/y-4", "radius/xl", "accent/amber"],
					"glass_blur": true
				},
				"action": {
					"type": "open_channels_activity",
					"payload": {
						"feed_id": "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
						"finder_user_name": "gh_drama_official"
					}
				}
			},
			{
				"id": "block_resource_102",
				"type": "resource_card",
				"props": {
					"title": "夸克网盘极速看全集",
					"desc": "高清4K未删减版 免费自取",
					"btn_text": "一键复制网盘链接",
					"pan_name": "夸克网盘",
					"fetch_code": "hwxs88",
					"content": "https://pan.quark.cn/s/monkey_king_full_888"
				},
				"style": {
					"utilities": ["space/y-4", "radius/lg"],
					"glass_blur": true
				},
				"action": {
					"type": "copy_text",
					"payload": {
						"text": "https://pan.quark.cn/s/monkey_king_full_888 提取码: hwxs88",
						"toast": "夸克网盘链接已复制，请打开浏览器粘贴访问"
					}
				}
			},
			{
				"id": "block_btn_103",
				"type": "action_button",
				"props": {
					"text": "⚡ 立即获取 1-80 集完整大结局",
					"badge": "免费无删减"
				},
				"style": {
					"utilities": ["space/y-6", "radius/full", "accent/amber"]
				},
				"action": {
					"type": "copy_text",
					"payload": {
						"text": "关注公众号【猴王剧场】回复【猴王下山】获取完整版",
						"toast": "已复制公众号信息，微信搜一搜即可直达"
					}
				}
			}
		]`

		shareJSON := `{
			"default_image_url": "",
			"friend": {
				"enabled": true,
				"title": "猴王下山全集免费看 - 爆款都市短剧",
				"path": "/pages/index/index?page_id=home",
				"image_url": ""
			},
			"timeline": {
				"enabled": true,
				"title": "猴王下山全集免费看 - 爆款都市短剧",
				"query": "page_id=home&from=timeline",
				"image_url": ""
			}
		}`

		homePage := models.DynamicPage{
			AppID:        defaultAppID,
			PageID:       "home",
			Revision:     1,
			Status:       "published",
			Title:        "猴王下山 - 精选剧场",
			BusinessType: "drama",
			Intent:       "watch",
			Theme:        "dark_glass",
			AccentColor:  "#FF9F0A",
			RequireAuth:  false,
			ShareConfig:  shareJSON,
			Blocks:       blocksJSON,
			Keyword:      "猴王下山",
			Source:       "wechat_search",
			CampaignID:   "initial_release",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := db.Mysql.Create(&homePage).Error; err != nil {
			return err
		}
	}

	// 3. 初始化默认游戏公测礼包数据
	var pkgCount int64
	db.Mysql.Model(&models.GameRedeemPackage{}).Where("app_id = ? AND package_id = ?", defaultAppID, "pkg_game_novice_888").Count(&pkgCount)
	if pkgCount == 0 {
		defaultPkg := models.GameRedeemPackage{
			AppID:          defaultAppID,
			PackageID:      "pkg_game_novice_888",
			GameID:         "game_bullet_storm",
			Title:          "绝地突围公测独家礼包",
			Description:    "含金币*8888、高级强化石*10、公测SSR限定称号",
			TotalStock:     5000,
			RemainingStock: 5000,
			CodePrefix:     "VIP888-",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		_ = db.Mysql.Create(&defaultPkg).Error
	}

	return nil
}
