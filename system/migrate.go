// Package system migrate.go
package system

import (
	"crypto/sha256"
	"encoding/hex"
	"hot_keyword/db"
	"hot_keyword/models"
	"log"
	"time"

	"github.com/23233/ggg/ut"
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
		&models.DynamicPageRevision{},
		&models.UserSession{},
		&models.GameRedeemPackage{},
		&models.GameRedeemRecord{},
		&models.AdminUser{},
	}

	err := db.Mysql.AutoMigrate(migrateList...)
	if err != nil {
		log.Fatalf("无法自动迁移数据库: %v", err)
		return err
	}

	// 执行多租户与 SDUI 默认数据自举初始化
	if err := SeedMultiTenantAndSDUIData(); err != nil {
		log.Printf("初始化默认多租户与SDUI数据异常: %v", err)
	}

	return nil
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
			AppName:        "猴王下山短剧",
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
					"cover_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80",
					"video_url": "https://sample-videos.com/video321/mp4/720/big_buck_bunny_720p_1mb.mp4",
					"rating": 9.8,
					"hot_score": 998000
				},
				"style": {
					"margin_y": "16rpx",
					"border_radius": "28rpx",
					"glass_blur": true,
					"accent_color": "#FF9F0A"
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
					"margin_y": "16rpx",
					"border_radius": "24rpx",
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
					"margin_y": "24rpx",
					"border_radius": "999rpx",
					"accent_color": "#FF9F0A"
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
			"default_image_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80",
			"friend": {
				"enabled": true,
				"title": "猴王下山全集免费看 - 爆款都市短剧",
				"path": "/pages/index/index?page_id=home",
				"image_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80"
			},
			"timeline": {
				"enabled": true,
				"title": "猴王下山全集免费看 - 爆款都市短剧",
				"query": "page_id=home&from=timeline",
				"image_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80"
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

	// 4. 初始化默认超级管理员账户 (admin / admin123456)
	var adminCount int64
	db.Mysql.Model(&models.AdminUser{}).Where("username = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		salt := "sdui_salt_" + ut.RandomStr(16)
		hasher := sha256.New()
		hasher.Write([]byte("admin123456" + salt))
		pwdHash := hex.EncodeToString(hasher.Sum(nil))

		superAdmin := models.AdminUser{
			Username:     "admin",
			PasswordHash: pwdHash,
			Salt:         salt,
			RealName:     "系统超级管理员",
			Role:         "super_admin",
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		_ = db.Mysql.Create(&superAdmin).Error
	}

	return nil
}
