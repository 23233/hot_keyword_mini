// Package services sdui.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"time"

	"github.com/23233/ggg/logger"
	"github.com/23233/ggg/ut"
	"gorm.io/gorm"
)

// SDUIService 服务端驱动动态组件引擎服务
type SDUIService struct{}

// NewSDUIService 创建 SDUI 服务实例
func NewSDUIService() *SDUIService {
	return &SDUIService{}
}

// GetRawPage 根据 AppID 和 PageID 查询数据库中的原始动态页面记录
func (s *SDUIService) GetRawPage(appID, pageID string) (*models.DynamicPage, error) {
	if appID == "" {
		appID = sdk.WechatMiniAppId
	}
	if pageID == "" {
		pageID = "home"
	}

	var page models.DynamicPage
	err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&page).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 若查询 home 且尚无记录，自举创建一次
			if pageID == "home" {
				logger.JM.Infof("检测到应用 %s 尚未配置 home 页面，开始自举生成...", appID)
				_ = s.seedDefaultPage(appID)
				if err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&page).Error; err == nil {
					return &page, nil
				}
			}
			return nil, fmt.Errorf("页面不存在: app_id=%s, page_id=%s", appID, pageID)
		}
		return nil, err
	}

	return &page, nil
}

// GetDynamicPageEnvelope 获取组装后的 SDUI 统一响应信封
func (s *SDUIService) GetDynamicPageEnvelope(appID, pageID string, queryParams map[string]string) (*models.PageResponseEnvelope, error) {
	rawPage, err := s.GetRawPage(appID, pageID)
	if err != nil {
		return nil, err
	}

	// 0. 时效性与下架检测: 若已过期或下架，自动切换至租户兜底安全页面
	fallbackMode := "static_safe"
	isExpired := rawPage.ExpiresAt != nil && rawPage.ExpiresAt.Before(time.Now())
	isOffline := rawPage.Status == "offline"

	if (isExpired || isOffline) && rawPage.PageID != "home" {
		var app models.MiniApp
		fallbackID := "home"
		if db.Mysql != nil {
			if err := db.Mysql.Where("app_id = ?", rawPage.AppID).First(&app).Error; err == nil && app.FallbackPageID != "" {
				fallbackID = app.FallbackPageID
			}
		}
		if fallbackPage, err := s.GetRawPage(appID, fallbackID); err == nil {
			rawPage = fallbackPage
			fallbackMode = "expired_fallback"
		}
	}

	// 1. 反序列化 Blocks 列表
	var blocks []models.BlockItem
	if rawPage.Blocks != "" {
		if err := json.Unmarshal([]byte(rawPage.Blocks), &blocks); err != nil {
			logger.JM.Warnf("解析页面 %s 积木 JSON 失败: %v", rawPage.PageID, err)
			blocks = []models.BlockItem{}
		}
	}

	// 2. 反序列化 ShareConfig
	var shareConfig *models.PageShareConfig
	if rawPage.ShareConfig != "" {
		var sc models.PageShareConfig
		if err := json.Unmarshal([]byte(rawPage.ShareConfig), &sc); err == nil {
			shareConfig = &sc
		}
	}

	// 3. 构建 DynamicPageDTO
	pageDTO := models.DynamicPageDTO{
		PageID:       rawPage.PageID,
		Revision:     rawPage.Revision,
		Status:       rawPage.Status,
		Title:        rawPage.Title,
		BusinessType: rawPage.BusinessType,
		Intent:       rawPage.Intent,
		Theme:        rawPage.Theme,
		AccentColor:  rawPage.AccentColor,
		RequireAuth:  rawPage.RequireAuth,
		ShareConfig:  shareConfig,
		Blocks:       blocks,
	}

	// 4. 数据装配与附加实体
	dataPayload := make(map[string]interface{})
	dataPayload["keyword"] = rawPage.Keyword
	dataPayload["business_type"] = rawPage.BusinessType
	if queryParams != nil {
		dataPayload["query"] = queryParams
	}

	// 5. 组装信封元数据
	requestID := fmt.Sprintf("req_%s_%d", ut.RandomStr(8), time.Now().UnixNano())
	etag := fmt.Sprintf("W/\"%s-%d-%d\"", rawPage.PageID, rawPage.Revision, rawPage.UpdatedAt.Unix())

	envelope := &models.PageResponseEnvelope{
		ProtocolVersion: "1.1",
		SchemaVersion:   3,
		RequestID:       requestID,
		Page:            pageDTO,
		Data:            dataPayload,
		CapabilitiesRequired: []string{
			"video",
			"clipboard",
		},
		Cache: models.EnvelopeCache{
			ETag:   etag,
			MaxAge: 30,
		},
		Fallback: models.EnvelopeFallback{
			PageID: "home",
			Mode:   fallbackMode,
		},
	}

	return envelope, nil
}

// seedDefaultPage 自举生成特定小程序的默认 SDUI 首页
func (s *SDUIService) seedDefaultPage(appID string) error {
	blocksJSON := `[
		{
			"id": "block_hero_default",
			"type": "media_hero",
			"props": {
				"title": "猴王下山",
				"subtitle": "第 1 集试看 · 爆火全网",
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
			"id": "block_resource_default",
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
		}
	]`

	shareJSON := `{
		"default_image_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80",
		"friend": {
			"enabled": true,
			"title": "猴王下山全集免费看",
			"path": "/pages/index/index?page_id=home",
			"image_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80"
		},
		"timeline": {
			"enabled": true,
			"title": "猴王下山全集免费看",
			"query": "page_id=home&from=timeline",
			"image_url": "https://images.unsplash.com/photo-1578632767115-351597cf2477?w=800&q=80"
		}
	}`

	page := models.DynamicPage{
		AppID:        appID,
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

	return db.Mysql.Create(&page).Error
}

// ListApps 获取所有已注册的小程序应用列表
func (s *SDUIService) ListApps() ([]models.MiniApp, error) {
	var apps []models.MiniApp
	err := db.Mysql.Order("created_at asc").Find(&apps).Error
	if err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		// 自举确保默认小程序存在
		defaultApp := models.MiniApp{
			AppID:          sdk.WechatMiniAppId,
			AppSecret:      sdk.WechatMiniSecret,
			AppName:        "猴王下山短剧",
			CurrentPage:    "home",
			ReleaseMode:    "normal",
			FallbackPageID: "home",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		_ = db.Mysql.Create(&defaultApp).Error
		apps = append(apps, defaultApp)
	}
	return apps, nil
}

// SaveApp 保存或新增小程序配置
func (s *SDUIService) SaveApp(app *models.MiniApp) error {
	if app == nil || app.AppID == "" {
		return errors.New("小程序 AppID 不能为空")
	}

	var existing models.MiniApp
	err := db.Mysql.Where("app_id = ?", app.AppID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			app.CreatedAt = time.Now()
			app.UpdatedAt = time.Now()
			if app.CurrentPage == "" {
				app.CurrentPage = "home"
			}
			return db.Mysql.Create(app).Error
		}
		return err
	}

	// 存在则更新
	updateMap := map[string]interface{}{
		"app_name":     app.AppName,
		"current_page": app.CurrentPage,
		"release_mode": app.ReleaseMode,
		"updated_at":   time.Now(),
	}
	if app.AppSecret != "" {
		updateMap["app_secret"] = app.AppSecret
	}
	return db.Mysql.Model(&existing).Updates(updateMap).Error
}

// ListPages 获取指定小程序下的全部动态页面列表
func (s *SDUIService) ListPages(appID string) ([]models.DynamicPage, error) {
	if appID == "" {
		appID = sdk.WechatMiniAppId
	}

	var pages []models.DynamicPage
	err := db.Mysql.Where("app_id = ?", appID).Order("updated_at desc").Find(&pages).Error
	if err != nil {
		return nil, err
	}

	// 若为空且为默认小程序，自举初始化一次 home 页面
	if len(pages) == 0 {
		_ = s.seedDefaultPage(appID)
		_ = db.Mysql.Where("app_id = ?", appID).Find(&pages).Error
	}

	return pages, nil
}

// SavePage 保存或发布动态页面协议 (版本号自增 revision++，并沉淀不可篡改的快照)
func (s *SDUIService) SavePage(page *models.DynamicPage) error {
	if page == nil || page.AppID == "" || page.PageID == "" {
		return errors.New("AppID 与 PageID 不能为空")
	}

	var newRevision int
	var existing models.DynamicPage
	err := db.Mysql.Where("app_id = ? AND page_id = ?", page.AppID, page.PageID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			page.Revision = 1
			page.CreatedAt = time.Now()
			page.UpdatedAt = time.Now()
			if err := db.Mysql.Create(page).Error; err != nil {
				return err
			}
			newRevision = 1
		} else {
			return err
		}
	} else {
		newRevision = existing.Revision + 1
		updateData := map[string]interface{}{
			"title":         page.Title,
			"business_type": page.BusinessType,
			"intent":        page.Intent,
			"theme":         page.Theme,
			"accent_color":  page.AccentColor,
			"require_auth":  page.RequireAuth,
			"share_config":  page.ShareConfig,
			"blocks":        page.Blocks,
			"keyword":       page.Keyword,
			"status":        page.Status,
			"revision":      newRevision,
			"updated_at":    time.Now(),
		}
		if page.ExpiresAt != nil {
			updateData["expires_at"] = page.ExpiresAt
		}
		if err := db.Mysql.Model(&existing).Updates(updateData).Error; err != nil {
			return err
		}
	}

	// 沉淀版本历史快照
	snapshot := models.DynamicPageRevision{
		AppID:        page.AppID,
		PageID:       page.PageID,
		Revision:     newRevision,
		Title:        page.Title,
		BusinessType: page.BusinessType,
		Theme:        page.Theme,
		AccentColor:  page.AccentColor,
		Blocks:       page.Blocks,
		ShareConfig:  page.ShareConfig,
		Remark:       "受控发布版本",
		CreatedBy:    "admin",
		CreatedAt:    time.Now(),
	}
	_ = db.Mysql.Create(&snapshot).Error

	return nil
}

// ListPageRevisions 获取指定页面的全部历史版本快照列表
func (s *SDUIService) ListPageRevisions(appID, pageID string) ([]models.DynamicPageRevision, error) {
	if appID == "" || pageID == "" {
		return nil, errors.New("appID 与 pageID 不能为空")
	}

	var revs []models.DynamicPageRevision
	err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).
		Order("revision desc").
		Find(&revs).Error
	return revs, err
}

// RollbackPageRevision 将页面原子回滚至指定的历史 revision 版本并派生新 revision 生效
func (s *SDUIService) RollbackPageRevision(appID, pageID string, targetRevision int) (*models.DynamicPage, error) {
	if appID == "" || pageID == "" || targetRevision <= 0 {
		return nil, errors.New("参数不完整")
	}

	var targetRev models.DynamicPageRevision
	err := db.Mysql.Where("app_id = ? AND page_id = ? AND revision = ?", appID, pageID, targetRevision).First(&targetRev).Error
	if err != nil {
		return nil, fmt.Errorf("未找到版本 revision %d 的快照: %w", targetRevision, err)
	}

	var current models.DynamicPage
	if err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&current).Error; err != nil {
		return nil, err
	}

	newRev := current.Revision + 1
	updateData := map[string]interface{}{
		"title":         targetRev.Title,
		"business_type": targetRev.BusinessType,
		"theme":         targetRev.Theme,
		"accent_color":  targetRev.AccentColor,
		"blocks":        targetRev.Blocks,
		"share_config":  targetRev.ShareConfig,
		"status":        "published",
		"revision":      newRev,
		"updated_at":    time.Now(),
	}

	if err := db.Mysql.Model(&current).Updates(updateData).Error; err != nil {
		return nil, err
	}

	// 记录回滚产生的新快照
	newRevSnapshot := models.DynamicPageRevision{
		AppID:        appID,
		PageID:       pageID,
		Revision:     newRev,
		Title:        targetRev.Title,
		BusinessType: targetRev.BusinessType,
		Theme:        targetRev.Theme,
		AccentColor:  targetRev.AccentColor,
		Blocks:       targetRev.Blocks,
		ShareConfig:  targetRev.ShareConfig,
		Remark:       fmt.Sprintf("原子回滚至版本 v%d", targetRevision),
		CreatedBy:    "admin",
		CreatedAt:    time.Now(),
	}
	_ = db.Mysql.Create(&newRevSnapshot).Error

	_ = db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&current)
	return &current, nil
}

// SetCurrentPage 将指定动态页面设为该小程序的当前线上激活主页
func (s *SDUIService) SetCurrentPage(appID, pageID string) error {
	if appID == "" || pageID == "" {
		return errors.New("AppID 与 PageID 不能为空")
	}

	return db.Mysql.Model(&models.MiniApp{}).
		Where("app_id = ?", appID).
		Updates(map[string]interface{}{
			"current_page": pageID,
			"updated_at":   time.Now(),
		}).Error
}

