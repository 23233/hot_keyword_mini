// Package services sdui.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"strings"
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
		return nil, errors.New("AppID 不能为空")
	}
	if pageID == "" {
		pageID = "home"
	}

	var page models.DynamicPage
	err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&page).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("页面不存在: app_id=%s, page_id=%s", appID, pageID)
		}
		return nil, err
	}

	return &page, nil
}

// GetPublishedDynamicPageEnvelope 获取面向普通微信客户端的已发布动态页面信封 (严格隔离草稿、下架及未登录数据)
func (s *SDUIService) GetPublishedDynamicPageEnvelope(appID, pageID string, queryParams map[string]string, isAuthenticated bool) (*models.PageResponseEnvelope, error) {
	if appID == "" {
		return nil, errors.New("AppID 不能为空")
	}
	if pageID == "" {
		pageID = "home"
	}

	// 1. 若客户端请求主页 home，优先检查该租户 mini_apps 的 current_page 设置
	actualPageID := pageID
	if pageID == "home" && db.Mysql != nil {
		var app models.MiniApp
		if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err == nil && app.CurrentPage != "" && app.CurrentPage != "home" {
			// 若当前激活的主页存在且为已发布状态，优先使用
			var activePage models.DynamicPage
			if err := db.Mysql.Where("app_id = ? AND page_id = ? AND status = 'published'", appID, app.CurrentPage).First(&activePage).Error; err == nil {
				actualPageID = app.CurrentPage
			}
		}
	}

	rawPage, err := s.GetRawPage(appID, actualPageID)
	if err != nil {
		return nil, err
	}

	// 2. 状态门禁隔离: 客户端仅允许获取 published 状态，草稿和已下架必须被隔离拦截
	if rawPage.Status != "published" {
		// 尝试降级至兜底安全主页
		if actualPageID != "home" {
			if homePage, hErr := s.GetRawPage(appID, "home"); hErr == nil && homePage.Status == "published" {
				rawPage = homePage
			} else {
				return nil, errors.New("目标页面处于草稿或已下架状态，无法对外提供访问")
			}
		} else {
			return nil, errors.New("目标页面处于草稿或已下架状态，无法对外提供访问")
		}
	}

	// 3. 过期检测
	if rawPage.ExpiresAt != nil && rawPage.ExpiresAt.Before(time.Now()) {
		if actualPageID != "home" {
			if homePage, hErr := s.GetRawPage(appID, "home"); hErr == nil && homePage.Status == "published" {
				rawPage = homePage
			} else {
				return nil, errors.New("页面已过期失效")
			}
		} else {
			return nil, errors.New("页面已过期失效")
		}
	}

	// 4. 组装基础信封
	envelope, err := s.AssembleEnvelope(rawPage, queryParams, "client_published")
	if err != nil {
		return nil, err
	}

	// 5. 服务端受保页隔离: 若页面声明 require_auth 且用户尚未认证通过，清空 Blocks 避免敏感泄露
	if rawPage.RequireAuth && !isAuthenticated {
		envelope.Page.Blocks = []models.BlockItem{}
		// 移除敏感附加数据
		delete(envelope.Data, "keyword")
	}

	return envelope, nil
}

// GetDynamicPageEnvelope 获取组装后的 SDUI 统一响应信封 (供管理后台线上版本预览使用)
func (s *SDUIService) GetDynamicPageEnvelope(appID, pageID string, queryParams map[string]string) (*models.PageResponseEnvelope, error) {
	rawPage, err := s.GetRawPage(appID, pageID)
	if err != nil {
		return nil, err
	}

	return s.AssembleEnvelope(rawPage, queryParams, "admin_preview")
}

// GetDynamicDraftEnvelope 获取组装后的草稿 SDUI 统一响应信封 (供管理后台草稿箱和 AI MCP 编排即时预览使用)
func (s *SDUIService) GetDynamicDraftEnvelope(appID, pageID string, queryParams map[string]string) (*models.PageResponseEnvelope, error) {
	draft, err := s.GetRawDraft(appID, pageID)
	if err != nil {
		return nil, err
	}

	tempPage := &models.DynamicPage{
		AppID:        draft.AppID,
		PageID:       draft.PageID,
		Revision:     draft.Revision,
		Status:       draft.Status,
		Title:        draft.Title,
		BusinessType: draft.BusinessType,
		Intent:       draft.Intent,
		Theme:        draft.Theme,
		AccentColor:  draft.AccentColor,
		RequireAuth:  draft.RequireAuth,
		ShareConfig:  draft.ShareConfig,
		Blocks:       draft.Blocks,
		Keyword:      draft.Keyword,
		Source:       draft.Source,
		CampaignID:   draft.CampaignID,
		ExpiresAt:    draft.ExpiresAt,
	}

	return s.AssembleEnvelope(tempPage, queryParams, "draft_preview")
}

// AssembleEnvelope 统一组装响应信封 (共享 DTO 转换、Blocks 反序列化与 ETag 计算)
func (s *SDUIService) AssembleEnvelope(rawPage *models.DynamicPage, queryParams map[string]string, mode string) (*models.PageResponseEnvelope, error) {
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

	// 5. 计算同构布局中间表示 LayoutIR (两端共同消费基线)
	irContext := make(map[string]interface{})
	irContext["entity"] = dataPayload
	irContext["$entity"] = dataPayload
	if queryParams != nil {
		irContext["query"] = queryParams
		irContext["$query"] = queryParams
	}
	device := DefaultDeviceParams()
	layoutIR, _ := BuildPageLayoutIRWithContext(rawPage, device, "normal", irContext)

	// 6. 组装信封元数据
	requestID := fmt.Sprintf("req_%s_%d", ut.RandomStr(8), time.Now().UnixNano())
	etag := fmt.Sprintf("W/\"%s-%d-%d\"", rawPage.PageID, rawPage.Revision, rawPage.UpdatedAt.Unix())

	envelope := &models.PageResponseEnvelope{
		ProtocolVersion: "1.1",
		SchemaVersion:   3,
		RequestID:       requestID,
		Page:            pageDTO,
		Data:            dataPayload,
		LayoutIR:        layoutIR,
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
			Mode:   mode,
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
			if err := db.Mysql.Create(app).Error; err != nil {
				return err
			}
			sdk.InvalidateMiniSdk(app.AppID)
			return nil
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
	if app.PaymentMchID != "" {
		updateMap["payment_mch_id"] = app.PaymentMchID
	}
	if app.PaymentMchSerialNo != "" {
		updateMap["payment_mch_serial_no"] = app.PaymentMchSerialNo
	}
	if app.PaymentAPIv3Key != "" {
		updateMap["payment_api_v3_key"] = app.PaymentAPIv3Key
	}
	if app.PaymentPrivateKey != "" {
		updateMap["payment_private_key"] = app.PaymentPrivateKey
	}
	if err := db.Mysql.Model(&existing).Updates(updateMap).Error; err != nil {
		return err
	}
	if app.AppSecret != "" {
		sdk.InvalidateMiniSdk(app.AppID)
	}
	if app.PaymentMchID != "" || app.PaymentMchSerialNo != "" || app.PaymentAPIv3Key != "" || app.PaymentPrivateKey != "" {
		InvalidatePaymentClient(app.AppID)
	}
	return nil
}

// ListPages 获取指定小程序下的全部动态页面列表
func (s *SDUIService) ListPages(appID string) ([]models.DynamicPage, error) {
	if appID == "" {
		return nil, errors.New("AppID 不能为空")
	}

	var pages []models.DynamicPage
	err := db.Mysql.Where("app_id = ?", appID).Order("updated_at desc").Find(&pages).Error
	if err != nil {
		return nil, err
	}

	return pages, nil
}

// SavePage 保存或发布动态页面协议 (兼容接口，内部代理至 SavePageWithAudit)
func (s *SDUIService) SavePage(page *models.DynamicPage) error {
	return s.SavePageWithAudit(page, "admin", "受控发布版本快照", 0)
}

// SavePageWithAudit 保存或发布动态页面协议 (支持操作人审计、发布备注与版本乐观锁 CAS 强一致性)
func (s *SDUIService) SavePageWithAudit(page *models.DynamicPage, operator, remark string, expectedRevision int) error {
	if page == nil || page.AppID == "" || page.PageID == "" {
		return errors.New("AppID 与 PageID 不能为空")
	}

	if operator == "" {
		operator = "admin"
	}
	if remark == "" {
		remark = "受控发布版本快照"
	}

	// 1. 强制协议合法性与安全规则强校验
	report := ValidateDynamicPage(page)
	if !report.IsValid {
		return fmt.Errorf("动态页面协议强校验未通过，阻断持久化: %s", strings.Join(report.Errors, "; "))
	}

	if db.Mysql == nil {
		return nil
	}

	// 2. 在单个原子事务中执行页面记录更新与版本历史快照沉淀
	return db.Mysql.Transaction(func(tx *gorm.DB) error {
		var newRevision int
		var existing models.DynamicPage

		err := tx.Where("app_id = ? AND page_id = ?", page.AppID, page.PageID).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				page.Revision = 1
				page.CreatedAt = time.Now()
				page.UpdatedAt = time.Now()
				if err := tx.Create(page).Error; err != nil {
					return fmt.Errorf("创建新页面记录失败: %w", err)
				}
				newRevision = 1
			} else {
				return err
			}
		} else {
			// 乐观锁 CAS 并发控制: 若传入 expectedRevision (或 page.Revision > 0)，比对是否被并发修改
			if expectedRevision == 0 && page.Revision > 0 {
				expectedRevision = page.Revision
			}
			if expectedRevision > 0 && existing.Revision != expectedRevision {
				return fmt.Errorf("发布版本冲突 (乐观锁 CAS 拦截): 期望基于版本 v%d 发布，当前线上已为 v%d，请拉取最新版本后再试", expectedRevision, existing.Revision)
			}

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
				"source":        page.Source,
				"campaign_id":   page.CampaignID,
				"status":        page.Status,
				"revision":      newRevision,
				"updated_at":    time.Now(),
			}
			if page.ExpiresAt != nil {
				updateData["expires_at"] = page.ExpiresAt
			}
			if err := tx.Model(&existing).Updates(updateData).Error; err != nil {
				return fmt.Errorf("更新页面协议失败: %w", err)
			}
			page.Revision = newRevision
		}

		// 3. 沉淀版本历史快照 (包含所有核心语义字段与真实操作人/备注)
		snapshot := models.DynamicPageRevision{
			AppID:        page.AppID,
			PageID:       page.PageID,
			Revision:     newRevision,
			Title:        page.Title,
			BusinessType: page.BusinessType,
			Intent:       page.Intent,
			Theme:        page.Theme,
			AccentColor:  page.AccentColor,
			RequireAuth:  page.RequireAuth,
			Blocks:       page.Blocks,
			ShareConfig:  page.ShareConfig,
			Keyword:      page.Keyword,
			Source:       page.Source,
			CampaignID:   page.CampaignID,
			ExpiresAt:    page.ExpiresAt,
			Remark:       remark,
			CreatedBy:    operator,
			CreatedAt:    time.Now(),
		}
		if err := tx.Create(&snapshot).Error; err != nil {
			return fmt.Errorf("沉淀版本快照失败，事务回滚: %w", err)
		}

		return nil
	})
}

// GetRawDraft 根据 AppID 和 PageID 查询草稿；若草稿不存在，基于线上已发布版本派生草稿，确保编辑不影响线上
func (s *SDUIService) GetRawDraft(appID, pageID string) (*models.DynamicPageDraft, error) {
	if appID == "" {
		return nil, errors.New("AppID 不能为空")
	}
	if pageID == "" {
		pageID = "home"
	}

	if db.Mysql == nil {
		return &models.DynamicPageDraft{
			AppID:        appID,
			PageID:       pageID,
			Status:       "draft",
			Title:        "草稿页面",
			BusinessType: "drama",
			Blocks:       "[]",
		}, nil
	}

	var draft models.DynamicPageDraft
	err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&draft).Error
	if err == nil {
		return &draft, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 草稿不存在，从已发布页面派生一份初始草稿
		var published models.DynamicPage
		if pErr := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&published).Error; pErr == nil {
			draft = models.DynamicPageDraft{
				AppID:        published.AppID,
				PageID:       published.PageID,
				Revision:     published.Revision,
				Status:       "draft",
				Title:        published.Title,
				BusinessType: published.BusinessType,
				Intent:       published.Intent,
				Theme:        published.Theme,
				AccentColor:  published.AccentColor,
				RequireAuth:  published.RequireAuth,
				ShareConfig:  published.ShareConfig,
				Blocks:       published.Blocks,
				Keyword:      published.Keyword,
				Source:       published.Source,
				CampaignID:   published.CampaignID,
				ExpiresAt:    published.ExpiresAt,
				UpdatedBy:    "system",
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			_ = db.Mysql.Create(&draft).Error
			return &draft, nil
		}

		// 线上无此页面时创建空白草稿，不写入任何默认业务数据。
		draft = models.DynamicPageDraft{
			AppID:        appID,
			PageID:       pageID,
			Revision:     1,
			Status:       "draft",
			Title:        "未命名草稿",
			BusinessType: "custom",
			Theme:        "dark_glass",
			AccentColor:  "#FF9F0A",
			Blocks:       "[]",
			UpdatedBy:    "admin",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		_ = db.Mysql.Create(&draft).Error
		return &draft, nil
	}

	return nil, err
}

// SaveDraft 保存草稿协议 (严格只更新 dynamic_page_drafts，默认以 draft.Revision 作为 CAS 乐观锁期望版本)
func (s *SDUIService) SaveDraft(draft *models.DynamicPageDraft) error {
	expectedRev := 0
	if draft != nil {
		expectedRev = draft.Revision
	}
	operator := "admin"
	if draft != nil && draft.UpdatedBy != "" {
		operator = draft.UpdatedBy
	}
	return s.SaveDraftWithAudit(draft, operator, expectedRev)
}

// SaveDraftWithAudit 保存草稿协议 (支持操作人审计与 expectedRevision 乐观锁 CAS 并发控制)
func (s *SDUIService) SaveDraftWithAudit(draft *models.DynamicPageDraft, operator string, expectedRevision int) error {
	if draft == nil || draft.AppID == "" || draft.PageID == "" {
		return errors.New("草稿 AppID 与 PageID 不能为空")
	}

	if operator == "" {
		operator = "admin"
	}
	draft.UpdatedBy = operator

	// 协议强校验
	tempPage := models.DynamicPage{
		AppID:        draft.AppID,
		PageID:       draft.PageID,
		Revision:     draft.Revision,
		Status:       "draft",
		Title:        draft.Title,
		BusinessType: draft.BusinessType,
		Intent:       draft.Intent,
		Theme:        draft.Theme,
		AccentColor:  draft.AccentColor,
		RequireAuth:  draft.RequireAuth,
		ShareConfig:  draft.ShareConfig,
		Blocks:       draft.Blocks,
		Keyword:      draft.Keyword,
		Source:       draft.Source,
		CampaignID:   draft.CampaignID,
		ExpiresAt:    draft.ExpiresAt,
	}
	report := ValidateDynamicPage(&tempPage)
	if !report.IsValid {
		return fmt.Errorf("草稿协议强校验未通过: %s", strings.Join(report.Errors, "; "))
	}

	if db.Mysql == nil {
		return nil
	}

	var existing models.DynamicPageDraft
	err := db.Mysql.Where("app_id = ? AND page_id = ?", draft.AppID, draft.PageID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			draft.Revision = 1
			draft.Status = "draft"
			draft.CreatedAt = time.Now()
			draft.UpdatedAt = time.Now()
			return db.Mysql.Create(draft).Error
		}
		return err
	}

	// 乐观锁 CAS 并发控制: 若期望版本大于 0，比对是否被并发修改
	if expectedRevision > 0 && existing.Revision != expectedRevision {
		return fmt.Errorf("草稿并发保存冲突 (乐观锁 CAS 拦截): 期望基于版本 v%d 保存，当前库内已被更新至 v%d，请先拉取最新草稿", expectedRevision, existing.Revision)
	}

	newRevision := existing.Revision + 1
	draft.Revision = newRevision
	draft.Status = "draft"
	draft.UpdatedAt = time.Now()

	return db.Mysql.Model(&existing).Updates(map[string]interface{}{
		"revision":      newRevision,
		"status":        "draft",
		"title":         draft.Title,
		"business_type": draft.BusinessType,
		"intent":        draft.Intent,
		"theme":         draft.Theme,
		"accent_color":  draft.AccentColor,
		"require_auth":  draft.RequireAuth,
		"share_config":  draft.ShareConfig,
		"blocks":        draft.Blocks,
		"keyword":       draft.Keyword,
		"source":        draft.Source,
		"campaign_id":   draft.CampaignID,
		"expires_at":    draft.ExpiresAt,
		"updated_by":    operator,
		"updated_at":    time.Now(),
	}).Error
}

// PublishDraft 发布指定草稿至线上 dynamic_pages 并沉淀不可篡改的 Revision 快照
func (s *SDUIService) PublishDraft(appID, pageID, operator, remark string) (*models.DynamicPage, error) {
	if appID == "" || pageID == "" {
		return nil, errors.New("AppID 与 PageID 不能为空")
	}

	if operator == "" {
		operator = "admin"
	}
	if remark == "" {
		remark = "从草稿受控发布上线"
	}

	draft, err := s.GetRawDraft(appID, pageID)
	if err != nil {
		return nil, fmt.Errorf("获取草稿失败: %w", err)
	}

	// 转化为 DynamicPage 并调用 SavePageWithAudit 沉淀真实操作人与备注
	targetPage := models.DynamicPage{
		AppID:        draft.AppID,
		PageID:       draft.PageID,
		Status:       "published", // 显式置为发布状态
		Title:        draft.Title,
		BusinessType: draft.BusinessType,
		Intent:       draft.Intent,
		Theme:        draft.Theme,
		AccentColor:  draft.AccentColor,
		RequireAuth:  draft.RequireAuth,
		ShareConfig:  draft.ShareConfig,
		Blocks:       draft.Blocks,
		Keyword:      draft.Keyword,
		Source:       draft.Source,
		CampaignID:   draft.CampaignID,
		ExpiresAt:    draft.ExpiresAt,
	}

	if err := s.SavePageWithAudit(&targetPage, operator, remark, 0); err != nil {
		return nil, fmt.Errorf("发布页面失败: %w", err)
	}

	// 同步更新草稿状态为 published
	if db.Mysql != nil {
		_ = db.Mysql.Model(&models.DynamicPageDraft{}).
			Where("app_id = ? AND page_id = ?", appID, pageID).
			Updates(map[string]interface{}{
				"status":     "published",
				"updated_by": operator,
				"updated_at": time.Now(),
			}).Error
	}

	return &targetPage, nil
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

// RollbackPageRevision 将页面原子回滚至指定的历史 revision 版本并派生新 revision 生效 (全事务化保护并完整恢复页面语义)
func (s *SDUIService) RollbackPageRevision(appID, pageID string, targetRevision int) (*models.DynamicPage, error) {
	if appID == "" || pageID == "" || targetRevision <= 0 {
		return nil, errors.New("参数不完整")
	}

	if db.Mysql == nil {
		return nil, errors.New("无数据库环境，无法执行回滚")
	}

	var current models.DynamicPage

	err := db.Mysql.Transaction(func(tx *gorm.DB) error {
		var targetRev models.DynamicPageRevision
		if err := tx.Where("app_id = ? AND page_id = ? AND revision = ?", appID, pageID, targetRevision).First(&targetRev).Error; err != nil {
			return fmt.Errorf("未找到版本 revision %d 的快照: %w", targetRevision, err)
		}

		if err := tx.Where("app_id = ? AND page_id = ?", appID, pageID).First(&current).Error; err != nil {
			return err
		}

		newRev := current.Revision + 1
		updateData := map[string]interface{}{
			"title":         targetRev.Title,
			"business_type": targetRev.BusinessType,
			"intent":        targetRev.Intent,
			"theme":         targetRev.Theme,
			"accent_color":  targetRev.AccentColor,
			"require_auth":  targetRev.RequireAuth,
			"blocks":        targetRev.Blocks,
			"share_config":  targetRev.ShareConfig,
			"keyword":       targetRev.Keyword,
			"source":        targetRev.Source,
			"campaign_id":   targetRev.CampaignID,
			"expires_at":    targetRev.ExpiresAt,
			"status":        "published",
			"revision":      newRev,
			"updated_at":    time.Now(),
		}

		if err := tx.Model(&current).Updates(updateData).Error; err != nil {
			return fmt.Errorf("回滚更新动态页面失败: %w", err)
		}

		// 记录回滚产生的新版本快照 (完整保留全部 6 个核心字段与回滚审计说明)
		newRevSnapshot := models.DynamicPageRevision{
			AppID:        appID,
			PageID:       pageID,
			Revision:     newRev,
			Title:        targetRev.Title,
			BusinessType: targetRev.BusinessType,
			Intent:       targetRev.Intent,
			Theme:        targetRev.Theme,
			AccentColor:  targetRev.AccentColor,
			RequireAuth:  targetRev.RequireAuth,
			Blocks:       targetRev.Blocks,
			ShareConfig:  targetRev.ShareConfig,
			Keyword:      targetRev.Keyword,
			Source:       targetRev.Source,
			CampaignID:   targetRev.CampaignID,
			ExpiresAt:    targetRev.ExpiresAt,
			Remark:       fmt.Sprintf("原子回滚至历史版本 v%d", targetRevision),
			CreatedBy:    "admin",
			CreatedAt:    time.Now(),
		}
		if err := tx.Create(&newRevSnapshot).Error; err != nil {
			return fmt.Errorf("创建回滚快照失败，事务回滚: %w", err)
		}

		current.Revision = newRev
		current.Blocks = targetRev.Blocks
		current.Title = targetRev.Title
		current.BusinessType = targetRev.BusinessType
		current.Intent = targetRev.Intent
		current.RequireAuth = targetRev.RequireAuth
		current.Keyword = targetRev.Keyword
		current.Source = targetRev.Source
		current.CampaignID = targetRev.CampaignID
		current.ExpiresAt = targetRev.ExpiresAt
		current.Status = "published"
		return nil
	})

	if err != nil {
		return nil, err
	}

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
