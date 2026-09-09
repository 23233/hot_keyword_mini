// Package system migrate.go
package system

import (
	"crypto/sha256"
	"encoding/hex"
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
		&models.UserSession{},
		&models.GameRedeemPackage{},
		&models.GameRedeemRecord{},
		&models.AdminUser{},
		&models.MCPAccessToken{},
		&models.WebViewTicket{},
		&models.Product{},
		&models.PaymentOrder{},
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
