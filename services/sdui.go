// Package services sdui.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"sort"
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
			return nil, fmt.Errorf("页面不存在: app_id=%s, page_id=%s: %w", appID, pageID, err)
		}
		return nil, err
	}

	return &page, nil
}

// GetPublishedDynamicPageEnvelope 获取面向普通微信客户端的已发布动态页面信封 (严格隔离草稿、下架及未登录数据)
// GetPublishedDynamicPageEnvelope 获取面向微信小程序正式发布的动态页面统一响应信封
func (s *SDUIService) GetPublishedDynamicPageEnvelope(appID, pageID string, queryParams map[string]string, isAuthenticated bool) (*models.PageResponseEnvelope, error) {
	return s.GetPublishedDynamicPageEnvelopeWithCapabilitiesAndViewer(appID, pageID, queryParams, isAuthenticated, "", 0)
}

// GetPublishedDynamicPageEnvelopeWithCapabilities 获取面向微信小程序发布的动态页面信封 (集成客户端能力协商与块级降级)
func (s *SDUIService) GetPublishedDynamicPageEnvelopeWithCapabilities(appID, pageID string, queryParams map[string]string, isAuthenticated bool, clientCapabilities string) (*models.PageResponseEnvelope, error) {
	return s.GetPublishedDynamicPageEnvelopeWithCapabilitiesAndViewer(appID, pageID, queryParams, isAuthenticated, clientCapabilities, 0)
}

// GetPublishedDynamicPageEnvelopeWithCapabilitiesAndViewer 按访问者会员权益装配 AI 破甲信封。
func (s *SDUIService) GetPublishedDynamicPageEnvelopeWithCapabilitiesAndViewer(appID, pageID string, queryParams map[string]string, isAuthenticated bool, clientCapabilities string, userID int64) (*models.PageResponseEnvelope, error) {
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
			if err := db.Mysql.Where("app_id = ? AND page_id = ? AND status = 'published' AND hidden = ?", appID, app.CurrentPage, false).First(&activePage).Error; err == nil {
				actualPageID = app.CurrentPage
			}
		}
	}

	rawPage, err := s.GetRawPage(appID, actualPageID)
	if err != nil {
		return nil, err
	}

	// 2. 状态门禁隔离: 客户端仅允许获取 published 状态，草稿和已下架必须被隔离拦截
	if rawPage.Status != "published" || rawPage.Hidden {
		// 尝试降级至兜底安全主页
		if actualPageID != "home" {
			if homePage, hErr := s.GetRawPage(appID, "home"); hErr == nil && homePage.Status == "published" && !homePage.Hidden {
				rawPage = homePage
			} else {
				return nil, errors.New("目标页面未发布、已隐藏或已下架，无法对外提供访问")
			}
		} else {
			return nil, errors.New("目标页面未发布、已隐藏或已下架，无法对外提供访问")
		}
	}

	// 3. 过期检测
	if rawPage.ExpiresAt != nil && rawPage.ExpiresAt.Before(time.Now()) {
		if actualPageID != "home" {
			if homePage, hErr := s.GetRawPage(appID, "home"); hErr == nil && homePage.Status == "published" && !homePage.Hidden {
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
	// AI 破甲首页和文章导航只下发摘要与权限结果，正文仍由文章详情接口按会员等级裁剪。
	s.attachAIBreakthroughData(envelope, appID, isAuthenticated, userID)
	// 资讯块高度依赖动态文章和套餐数据，装配后同步刷新 IR。
	s.rebuildEnvelopeLayoutIR(rawPage, envelope)

	// 5. 客户端能力协商与块级受控降级: 若客户端申报了 X-Client-Capabilities，执行过滤与 Fallback 替换
	if strings.TrimSpace(clientCapabilities) != "" && len(envelope.Page.Blocks) > 0 {
		envelope.Page.Blocks = FilterBlocksByCapabilities(envelope.Page.Blocks, clientCapabilities)
		envelope.CapabilitiesRequired = extractRequiredCapabilities(envelope.Page.Blocks)
		// 能力协商会改变积木树，必须同步重建 IR，避免客户端优先消费旧 IR 而绕过降级结果。
		s.rebuildEnvelopeLayoutIR(rawPage, envelope)
	}

	// 6. 服务端受保页隔离: 若页面声明 require_auth 且用户尚未认证通过，清空 Blocks 避免敏感泄露
	if rawPage.RequireAuth && !isAuthenticated {
		envelope.Page.Blocks = []models.BlockItem{}
		// 未认证响应不得携带可绑定到页面上的衍生数据，避免通过 IR 或数据字段泄露内容。
		envelope.Data = map[string]interface{}{}
		s.rebuildEnvelopeLayoutIR(rawPage, envelope)
	}

	return envelope, nil
}

// attachAIBreakthroughData 将资讯导航、会员方案和访问者权益摘要附加到统一信封。
func (s *SDUIService) attachAIBreakthroughData(envelope *models.PageResponseEnvelope, appID string, authenticated bool, userID int64) {
	if envelope == nil || db.Mysql == nil || (envelope.Page.BusinessType != "ai_breakthrough" && envelope.Page.BusinessType != "ai_article") {
		return
	}
	level := 0
	var membership *models.UserMembership
	if userID > 0 {
		membership, _ = NewMembershipService().GetMembership(appID, userID)
		if membership != nil && membership.ExpiresAt.After(time.Now()) {
			level = membership.Level
		}
	}
	viewer := map[string]interface{}{"logged_in": authenticated, "membership_level": level}
	if membership != nil && membership.ExpiresAt.After(time.Now()) {
		viewer["membership_expires_at"] = membership.ExpiresAt
	}
	var categories []models.ArticleCategory
	_ = db.Mysql.Where("app_id = ? AND status = ?", appID, "active").Order("sort asc, id asc").Find(&categories).Error
	categorySlugs := make(map[int64]string, len(categories))
	categoryNames := make(map[int64]string, len(categories))
	for _, category := range categories {
		categorySlugs[category.ID] = category.Slug
		categoryNames[category.ID] = category.Name
	}
	var articles []models.Article
	_ = db.Mysql.Where("app_id = ? AND status = ?", appID, "published").Order("published_at desc, id desc").Limit(12).Find(&articles).Error
	items := make([]map[string]interface{}, 0, len(articles))
	articleService := NewArticleService()
	for _, article := range articles {
		categorySlug := categorySlugs[article.CategoryID]
		categoryName := categoryNames[article.CategoryID]
		purchased := userID > 0 && articleService.hasPurchase(appID, userID, article.ID)
		summary := toArticleSummaryForViewer(article, level, purchased)
		item := map[string]interface{}{"id": article.ID, "title": article.Title, "summary": article.Summary, "cover_url": article.CoverURL, "category_id": article.CategoryID, "category_slug": categorySlug, "category_name": categoryName, "featured": categorySlug == "featured" && !article.IsPaid && article.RequiredLevel == 0, "published_at": article.PublishedAt, "required_level": article.RequiredLevel, "is_paid": article.IsPaid, "pay_sku": article.PaySKU, "price_fen": article.PriceFen, "allow_comments": article.AllowComments, "can_read": summary.CanRead, "display_meta": summary.DisplayMeta, "access_badge": summary.AccessBadge, "key": article.ID, "image": article.CoverURL, "meta": summary.DisplayMeta, "badge": summary.AccessBadge}
		items = append(items, item)
	}
	var plans []models.MembershipLevel
	_ = db.Mysql.Where("app_id = ? AND status = ?", appID, "active").Order("level asc").Find(&plans).Error
	offers := make([]map[string]interface{}, 0, len(plans))
	for _, plan := range plans {
		offers = append(offers, map[string]interface{}{
			"key": plan.SKU, "title": plan.Name, "summary": plan.Description,
			"caption": fmt.Sprintf("有效期 %d 天", plan.DurationDays), "price": fmt.Sprintf("¥%.2f", float64(plan.PriceFen)/100), "purchase_value": plan.SKU,
		})
	}
	envelope.Viewer = viewer
	content := map[string]interface{}{"categories": categories, "items": items, "offers": offers, "articles": items, "membership_plans": plans}
	envelope.Data["content"] = content
	// 兼容旧页面协议；新模板只使用通用 content 数据键。
	envelope.Data["ai_breakthrough"] = content
}

// GetDynamicPageEnvelope 获取组装后的 SDUI 统一响应信封 (供管理后台线上版本预览使用)
func (s *SDUIService) GetDynamicPageEnvelope(appID, pageID string, queryParams map[string]string) (*models.PageResponseEnvelope, error) {
	rawPage, err := s.GetRawPage(appID, pageID)
	if err != nil {
		return nil, err
	}

	envelope, err := s.AssembleEnvelope(rawPage, queryParams, "admin_preview")
	if err != nil {
		return nil, err
	}
	s.attachAIBreakthroughData(envelope, appID, false, 0)
	s.rebuildEnvelopeLayoutIR(rawPage, envelope)
	return envelope, nil
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
		Hidden:       draft.Hidden,
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

	envelope, err := s.AssembleEnvelope(tempPage, queryParams, "draft_preview")
	if err != nil {
		return nil, err
	}
	s.attachAIBreakthroughData(envelope, draft.AppID, false, 0)
	s.rebuildEnvelopeLayoutIR(tempPage, envelope)
	return envelope, nil
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
	// WebView 动作只允许引用当前租户已登记且启用的 url_key，服务端在下发前解析为受控 HTTPS 地址。
	if err := resolveWebViewActions(rawPage.AppID, blocks); err != nil {
		return nil, fmt.Errorf("WebView 动作校验失败: %w", err)
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
		Hidden:       rawPage.Hidden,
		Title:        rawPage.Title,
		BusinessType: rawPage.BusinessType,
		Intent:       rawPage.Intent,
		Theme:        rawPage.Theme,
		AccentColor:  rawPage.AccentColor,
		RequireAuth:  rawPage.RequireAuth,
		ShareConfig:  shareConfig,
		Blocks:       blocks,
	}

	// 4. 数据装配与受控领域业务实体装配 ($entity.* 双端求值支持)
	dataPayload := make(map[string]interface{})
	dataPayload["keyword"] = rawPage.Keyword
	dataPayload["business_type"] = rawPage.BusinessType
	if queryParams != nil {
		dataPayload["query"] = queryParams
	}

	// 按业务领域分类装配真实实体，不泄露未授权敏感数据
	entityMap := s.assembleDomainEntity(rawPage.AppID, rawPage.BusinessType, rawPage.Keyword, queryParams)
	for k, v := range entityMap {
		if _, exists := dataPayload[k]; !exists {
			dataPayload[k] = v
		}
	}
	dataPayload["entity"] = entityMap

	// 5. 组装信封元数据
	requestID := fmt.Sprintf("req_%s_%d", ut.RandomStr(8), time.Now().UnixNano())
	etag := fmt.Sprintf("W/\"%s-%d-%d\"", rawPage.PageID, rawPage.Revision, rawPage.UpdatedAt.Unix())

	envelope := &models.PageResponseEnvelope{
		ProtocolVersion:      "1.1",
		SchemaVersion:        3,
		RequestID:            requestID,
		Page:                 pageDTO,
		Data:                 dataPayload,
		CapabilitiesRequired: extractRequiredCapabilities(blocks),
		Cache: models.EnvelopeCache{
			ETag:   etag,
			MaxAge: 30,
		},
		Fallback: models.EnvelopeFallback{
			PageID: "home",
			Mode:   mode,
		},
	}
	s.rebuildEnvelopeLayoutIR(rawPage, envelope)

	return envelope, nil
}

// resolveWebViewActions 递归解析页面所有动作链中的 WebView 登记键。
func resolveWebViewActions(appID string, blocks []models.BlockItem) error {
	var resolveAction func(*models.BlockAction) error
	resolveAction = func(action *models.BlockAction) error {
		if action == nil {
			return nil
		}
		if action.Type == "open_webview" {
			payload, err := ResolveWebViewPayload(appID, action.Payload)
			if err != nil {
				return err
			}
			action.Payload = payload
		}
		for index := range action.OnSuccess {
			if err := resolveAction(&action.OnSuccess[index]); err != nil {
				return err
			}
		}
		for index := range action.OnError {
			if err := resolveAction(&action.OnError[index]); err != nil {
				return err
			}
		}
		return nil
	}
	var resolveBlock func(*models.BlockItem) error
	resolveBlock = func(block *models.BlockItem) error {
		if block == nil {
			return nil
		}
		if err := resolveAction(block.Action); err != nil {
			return err
		}
		for _, actions := range block.Events {
			for index := range actions {
				if err := resolveAction(&actions[index]); err != nil {
					return err
				}
			}
		}
		for _, state := range []*models.BlockItem{block.Loading, block.Empty, block.Error, block.Fallback} {
			if err := resolveBlock(state); err != nil {
				return err
			}
		}
		if block.Props != nil {
			for _, key := range []string{"children", "blocks", "items"} {
				raw, ok := block.Props[key]
				if !ok {
					continue
				}
				encoded, _ := json.Marshal(raw)
				var children []models.BlockItem
				if json.Unmarshal(encoded, &children) != nil {
					continue
				}
				validChildren := len(children) > 0
				for index := range children {
					if children[index].ID == "" || children[index].Type == "" {
						validChildren = false
						break
					}
					if err := resolveBlock(&children[index]); err != nil {
						return err
					}
				}
				if !validChildren {
					continue
				}
				updated, _ := json.Marshal(children)
				var value []interface{}
				if json.Unmarshal(updated, &value) == nil {
					block.Props[key] = value
				}
			}
			if raw, ok := block.Props["tabs"]; ok {
				encoded, _ := json.Marshal(raw)
				var tabs []map[string]interface{}
				if json.Unmarshal(encoded, &tabs) == nil {
					for index := range tabs {
						childrenRaw, exists := tabs[index]["blocks"]
						if !exists {
							continue
						}
						childrenEncoded, _ := json.Marshal(childrenRaw)
						var children []models.BlockItem
						if json.Unmarshal(childrenEncoded, &children) != nil {
							continue
						}
						for childIndex := range children {
							if err := resolveBlock(&children[childIndex]); err != nil {
								return err
							}
						}
						updated, _ := json.Marshal(children)
						var value []interface{}
						if json.Unmarshal(updated, &value) == nil {
							tabs[index]["blocks"] = value
						}
					}
					updated, _ := json.Marshal(tabs)
					var value []interface{}
					if json.Unmarshal(updated, &value) == nil {
						block.Props["tabs"] = value
					}
				}
			}
		}
		return nil
	}
	for index := range blocks {
		if err := resolveBlock(&blocks[index]); err != nil {
			return err
		}
	}
	return nil
}

// rebuildEnvelopeLayoutIR 使用信封当前的积木树重建布局 IR，确保截图与结构对照基于同一份协议内容；小程序运行时仍消费 page.blocks。
func (s *SDUIService) rebuildEnvelopeLayoutIR(rawPage *models.DynamicPage, envelope *models.PageResponseEnvelope) {
	if rawPage == nil || envelope == nil {
		return
	}

	pageForLayout := *rawPage
	blocksJSON, err := json.Marshal(envelope.Page.Blocks)
	if err != nil {
		return
	}
	pageForLayout.Blocks = string(blocksJSON)
	context := map[string]interface{}{
		"entity":  envelope.Data,
		"$entity": envelope.Data,
	}
	if query, ok := envelope.Data["query"].(map[string]string); ok {
		context["query"] = query
		context["$query"] = query
	}
	envelope.LayoutIR, _ = BuildPageLayoutIRWithContext(&pageForLayout, DefaultDeviceParams(), "normal", context)
}

// extractRequiredCapabilities 从页面积木列表中提取必要的能力标识
func extractRequiredCapabilities(blocks []models.BlockItem) []string {
	set := map[string]bool{}
	var scanAction func(*models.BlockAction)
	scanAction = func(action *models.BlockAction) {
		if action == nil {
			return
		}
		if capability := actionCapability(action.Type); capability != "" {
			set[capability] = true
		}
		for index := range action.OnSuccess {
			scanAction(&action.OnSuccess[index])
		}
		for index := range action.OnError {
			scanAction(&action.OnError[index])
		}
	}
	var scan func(models.BlockItem)
	scan = func(block models.BlockItem) {
		if capability := capabilityBlockKeys[block.Type]; capability != "" {
			set[capability] = true
		}
		scanAction(block.Action)
		for _, actions := range block.Events {
			for index := range actions {
				scanAction(&actions[index])
			}
		}
		for _, state := range []*models.BlockItem{block.Loading, block.Empty, block.Error, block.Fallback} {
			if state != nil {
				scan(*state)
			}
		}
		if block.Props != nil {
			for _, key := range []string{"children", "blocks", "items"} {
				if raw, ok := block.Props[key]; ok {
					var nested []models.BlockItem
					if encoded, err := json.Marshal(raw); err == nil && json.Unmarshal(encoded, &nested) == nil {
						for _, child := range nested {
							scan(child)
						}
					}
				}
			}
		}
	}
	for _, block := range blocks {
		scan(block)
	}
	result := make([]string, 0, len(set))
	for capability := range set {
		result = append(result, capability)
	}
	sort.Strings(result)
	return result
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
		}
	]`

	shareJSON := `{
		"default_image_url": "",
		"friend": {
			"enabled": true,
			"title": "猴王下山全集免费看",
			"path": "/pages/index/index?page_id=home",
			"image_url": ""
		},
		"timeline": {
			"enabled": true,
			"title": "猴王下山全集免费看",
			"query": "page_id=home&from=timeline",
			"image_url": ""
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
		"app_name":    app.AppName,
		"cos_cdn_url": app.CosCdnUrl,
		"updated_at":  time.Now(),
	}
	// 主体配置表单未提交页面路由时，保留已有首页及回退策略。
	for key, value := range map[string]string{"current_page": app.CurrentPage, "release_mode": app.ReleaseMode, "fallback_page_id": app.FallbackPageID} {
		if value != "" {
			updateMap[key] = value
		}
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
	for key, value := range map[string]string{
		"tencent_map_key": app.TencentMapKey, "subscribe_template_ids": app.SubscribeTemplateIDs, "ad_unit_ids": app.AdUnitIDs,
	} {
		if strings.TrimSpace(value) != "" {
			updateMap[key] = strings.TrimSpace(value)
		}
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
	var report ValidationReport
	if page.Status == "published" {
		report = ValidateDynamicPageForRelease(page)
	} else {
		report = ValidateDynamicPage(page)
	}
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
				"hidden":        page.Hidden,
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
			Hidden:       page.Hidden,
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
				Hidden:       published.Hidden,
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

// FindRawDraft 只读取已有草稿，不会因读取请求创建或派生草稿。
func (s *SDUIService) FindRawDraft(appID, pageID string) (*models.DynamicPageDraft, error) {
	if appID == "" {
		return nil, errors.New("AppID 不能为空")
	}
	if pageID == "" {
		pageID = "home"
	}
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化")
	}
	var draft models.DynamicPageDraft
	if err := db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).First(&draft).Error; err != nil {
		return nil, err
	}
	return &draft, nil
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
		Hidden:       draft.Hidden,
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
			if expectedRevision > 0 {
				return errors.New("草稿不存在，无法按指定 revision 保存")
			}
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

	updateQuery := db.Mysql.Model(&existing)
	if expectedRevision > 0 {
		updateQuery = updateQuery.Where("id = ? AND revision = ?", existing.ID, expectedRevision)
	}
	result := updateQuery.Updates(map[string]interface{}{
		"revision":      newRevision,
		"status":        "draft",
		"hidden":        draft.Hidden,
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
	})
	if result.Error != nil {
		return result.Error
	}
	if expectedRevision > 0 && result.RowsAffected != 1 {
		return errors.New("草稿并发保存冲突: revision 已变化，请重新读取")
	}
	return nil
}

// PublishDraft 发布指定草稿至线上 dynamic_pages 并沉淀不可篡改的 Revision 快照
func (s *SDUIService) PublishDraft(appID, pageID, operator, remark string) (*models.DynamicPage, error) {
	return s.PublishDraftWithRevision(appID, pageID, operator, remark, 0)
}

// PublishDraftWithRevision 发布指定 revision 的草稿，防止人工审查后被覆盖。
func (s *SDUIService) PublishDraftWithRevision(appID, pageID, operator, remark string, expectedRevision int) (*models.DynamicPage, error) {
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
	if expectedRevision > 0 && draft.Revision != expectedRevision {
		return nil, fmt.Errorf("发布版本冲突: 期望草稿 v%d，当前为 v%d，请重新审查", expectedRevision, draft.Revision)
	}

	// 转化为 DynamicPage 并调用 SavePageWithAudit 沉淀真实操作人与备注
	targetPage := models.DynamicPage{
		AppID:        draft.AppID,
		PageID:       draft.PageID,
		Status:       "published", // 显式置为发布状态
		Hidden:       draft.Hidden,
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
		report := ValidateDynamicPage(&models.DynamicPage{AppID: appID, PageID: pageID, Title: targetRev.Title, BusinessType: targetRev.BusinessType, Intent: targetRev.Intent, Theme: targetRev.Theme, Blocks: targetRev.Blocks})
		if !report.IsValid {
			return fmt.Errorf("历史版本不符合当前协议，请迁移为新草稿后发布: %s", strings.Join(report.Errors, "; "))
		}
		var capabilityBlocks []models.BlockItem
		if err := json.Unmarshal([]byte(targetRev.Blocks), &capabilityBlocks); err != nil {
			return fmt.Errorf("历史版本能力依赖解析失败: %w", err)
		}
		var app models.MiniApp
		if err := tx.Where("app_id = ?", appID).First(&app).Error; err != nil {
			return err
		}
		matrix, err := CapabilityMatrixForApp(&app)
		if err != nil {
			return err
		}
		if capabilityReport := ValidatePageCapabilitiesForMatrix(capabilityBlocks, matrix, len(matrix) > 0); !capabilityReport.IsValid {
			return fmt.Errorf("历史版本能力发布门禁未通过: %s", strings.Join(capabilityReport.Errors, "; "))
		}

		if err := tx.Where("app_id = ? AND page_id = ?", appID, pageID).First(&current).Error; err != nil {
			return err
		}

		newRev := current.Revision + 1
		updateData := map[string]interface{}{
			"title":         targetRev.Title,
			"hidden":        targetRev.Hidden,
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
			Hidden:       targetRev.Hidden,
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
		current.Hidden = targetRev.Hidden
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

	if db.Mysql == nil {
		return errors.New("数据库未初始化")
	}
	var page models.DynamicPage
	if err := db.Mysql.Where("app_id = ? AND page_id = ? AND status = ? AND hidden = ?", appID, pageID, "published", false).First(&page).Error; err != nil {
		return errors.New("只有已发布且未隐藏的页面才能设为当前主页")
	}
	return db.Mysql.Model(&models.MiniApp{}).
		Where("app_id = ?", appID).
		Updates(map[string]interface{}{
			"current_page": pageID,
			"updated_at":   time.Now(),
		}).Error
}

// assembleDomainEntity 按业务领域分类装配受控业务实体 (支持 short drama, game package, query 等，防止数据暴露与状态污染)
func (s *SDUIService) assembleDomainEntity(appID, businessType, keyword string, queryParams map[string]string) map[string]interface{} {
	entity := make(map[string]interface{})
	if keyword != "" {
		entity["title"] = keyword
	}

	if db.Mysql == nil {
		return entity
	}

	switch businessType {
	case "drama":
		// 短剧领域实体装配: 优先根据 queryParams["id"] 查询，若未指定则查询当前热门短剧
		var drama models.Drama
		queryID := ""
		if queryParams != nil {
			queryID = queryParams["id"]
		}

		var err error
		if queryID != "" {
			err = db.Mysql.Where("id = ?", queryID).First(&drama).Error
		} else if keyword != "" {
			err = db.Mysql.Where("title = ?", keyword).First(&drama).Error
		}
		if err != nil || drama.ID == 0 {
			// 兜底查询爆款短剧：优先按热度降序取当前最热门短剧，无则回退查询种子短剧《猴王下山》
			if errFallback := db.Mysql.Order("hot_score desc, id asc").First(&drama).Error; errFallback != nil {
				_ = db.Mysql.Where("title = ?", "猴王下山").First(&drama).Error
			}
		}

		if drama.ID != 0 {
			entity["id"] = drama.ID
			entity["title"] = drama.Title
			entity["subtitle"] = drama.Subtitle
			entity["cover_url"] = drama.CoverUrl
			entity["banner_url"] = drama.BannerUrl
			entity["rating"] = drama.Rating
			entity["hot_score"] = drama.HotScore
			entity["total_episodes"] = drama.TotalEpisodes
			entity["updated_episodes"] = drama.UpdatedEpisodes
			entity["tags"] = drama.Tags
			entity["description"] = drama.Description
			entity["is_locked"] = false
		}

	case "game":
		// 游戏领域实体装配: 查询该小程序下的公测礼包信息 (未领取前不提前下发真实兑换码)
		var pkg models.GameRedeemPackage
		queryPkgID := ""
		if queryParams != nil {
			queryPkgID = queryParams["package_id"]
			if queryPkgID == "" {
				queryPkgID = queryParams["game_id"]
			}
		}

		var err error
		if queryPkgID != "" {
			err = db.Mysql.Where("app_id = ? AND (package_id = ? OR game_id = ?)", appID, queryPkgID, queryPkgID).First(&pkg).Error
		}
		if err != nil || pkg.ID == 0 {
			_ = db.Mysql.Where("app_id = ?", appID).First(&pkg).Error
		}

		if pkg.ID != 0 {
			entity["package_id"] = pkg.PackageID
			entity["game_id"] = pkg.GameID
			entity["title"] = pkg.Title
			entity["description"] = pkg.Description
			entity["total_stock"] = pkg.TotalStock
			entity["remaining_stock"] = pkg.RemainingStock
			entity["claim_status"] = "unclaimed"
		}

	case "query":
		if queryParams != nil && queryParams["query_value"] != "" {
			entity["query_value"] = queryParams["query_value"]
		}
		entity["status"] = "ready"

	case "download":
		entity["platform"] = "Android / iOS"
		entity["status"] = "verified"
	}

	return entity
}

// FilterBlocksByCapabilities 根据客户端声明的 X-Client-Capabilities 能力集对积木树执行受控协商过滤与 Fallback 降级
func FilterBlocksByCapabilities(blocks []models.BlockItem, clientCapsStr string) []models.BlockItem {
	trimmed := strings.TrimSpace(clientCapsStr)
	if trimmed == "" {
		return blocks
	}

	capSet := make(map[string]bool)
	for _, cap := range strings.Split(trimmed, ",") {
		c := strings.TrimSpace(cap)
		if c != "" {
			capSet[c] = true
		}
	}

	return filterBlockList(blocks, capSet)
}

// filterBlockList 递归过滤积木树，对客户端不支持的类型使用 Fallback 替换，若无可用 Fallback 则剔除
func filterBlockList(blocks []models.BlockItem, capSet map[string]bool) []models.BlockItem {
	result := make([]models.BlockItem, 0, len(blocks))
	for _, block := range blocks {
		current := block

		// 检查当前积木类型是否在客户端能力支持集中
		if !capSet[current.Type] {
			// 若不支持但配置了块级 fallback 且 fallback 类型被客户端支持，则优雅降级为 fallback
			if current.Fallback != nil && capSet[current.Fallback.Type] {
				current = *current.Fallback
			} else {
				// 不支持且无可用 fallback，安全剔除避免客户端渲染崩溃
				continue
			}
		}
		// 原生能力动作不受支持时，整块回退或剔除，避免保留不可点击的半成品组件。
		if !blockActionsSupported(current, capSet) {
			if current.Fallback != nil && capSet[current.Fallback.Type] && blockActionsSupported(*current.Fallback, capSet) {
				current = *current.Fallback
			} else {
				continue
			}
		}

		// 递归过滤布局容器与 Tabs 内的子积木。
		if current.Props != nil {
			newProps := make(map[string]interface{})
			for k, v := range current.Props {
				newProps[k] = v
			}

			for _, childKey := range []string{"children", "blocks", "items"} {
				if childVal, exists := newProps[childKey]; exists {
					newProps[childKey] = filterBlockListValue(childVal, capSet)
				}
			}
			if tabs, exists := newProps["tabs"]; exists {
				newProps["tabs"] = filterTabsBlocks(tabs, capSet)
			}
			current.Props = newProps
		}

		result = append(result, current)
	}
	return result
}

// blockActionsSupported 校验当前块的动作链是否依赖客户端已声明的原生能力。
func blockActionsSupported(block models.BlockItem, capSet map[string]bool) bool {
	if !actionChainSupported(block.Action, capSet) {
		return false
	}
	for _, actions := range block.Events {
		for index := range actions {
			if !actionChainSupported(&actions[index], capSet) {
				return false
			}
		}
	}
	return true
}

// actionChainSupported 递归验证动作及其成功/失败链需要的原生能力。
func actionChainSupported(action *models.BlockAction, capSet map[string]bool) bool {
	if action == nil {
		return true
	}
	if capability := actionCapability(action.Type); capability != "" && !capSet[capability] {
		return false
	}
	for index := range action.OnSuccess {
		if !actionChainSupported(&action.OnSuccess[index], capSet) {
			return false
		}
	}
	for index := range action.OnError {
		if !actionChainSupported(&action.OnError[index], capSet) {
			return false
		}
	}
	return true
}

// actionCapability 返回动作所依赖的客户端原生能力；通用路由和状态动作无需额外协商。
func actionCapability(actionType string) string {
	switch actionType {
	case "copy_text":
		return "clipboard"
	case "open_channels_activity":
		return "channels"
	case "request_payment":
		return "request_payment"
	case "subscribe_message":
		return "subscribe_message"
	case "choose_media", "upload_file", "delete_media":
		return "media_upload"
	case "request_location", "choose_location", "open_map":
		return "location"
	case "open_wechat_service", "save_qr":
		return "wechat_customer_service"
	case "open_internal_chat":
		return "internal_chat"
	case "send_message", "mark_read":
		return "internal_chat"
	case "poll_messages", "connect_message", "upload_chat_media":
		return "realtime_chat"
	case "create_order", "confirm_order", "cancel_order", "confirm_receipt":
		return "orders"
	case "refresh_logistics":
		return "logistics"
	case "apply_after_sale", "upload_evidence":
		return "after_sale"
	case "accept_task", "reject_task", "submit_quote", "update_service_status":
		return "service_orders"
	case "open_membership":
		return "membership"
	case "load_ad":
		return "ads"
	case "refresh_wallet", "request_withdraw":
		return "wallet"
	case "open_webview":
		return "webview"
	default:
		return ""
	}
}

// filterBlockListValue 兼容 JSON 反序列化后的 []interface{} 与服务端构造的 []BlockItem。
func filterBlockListValue(value interface{}, capSet map[string]bool) interface{} {
	if blocks, ok := value.([]models.BlockItem); ok {
		return filterBlockList(blocks, capSet)
	}
	interfaces, ok := value.([]interface{})
	if !ok {
		return value
	}
	var blocks []models.BlockItem
	encoded, err := json.Marshal(interfaces)
	if err != nil || json.Unmarshal(encoded, &blocks) != nil || len(blocks) != len(interfaces) {
		return value
	}
	for _, block := range blocks {
		if block.ID == "" || block.Type == "" {
			return value
		}
	}
	filtered := filterBlockList(blocks, capSet)
	encoded, _ = json.Marshal(filtered)
	var result []interface{}
	if json.Unmarshal(encoded, &result) != nil {
		return value
	}
	return result
}

// filterTabsBlocks 递归过滤每个 Tab 中的 blocks、children、items 和单个 child。
func filterTabsBlocks(value interface{}, capSet map[string]bool) interface{} {
	tabs, ok := value.([]interface{})
	if !ok {
		return value
	}
	result := make([]interface{}, 0, len(tabs))
	for _, rawTab := range tabs {
		tab, ok := rawTab.(map[string]interface{})
		if !ok {
			result = append(result, rawTab)
			continue
		}
		filteredTab := make(map[string]interface{}, len(tab))
		for key, tabValue := range tab {
			filteredTab[key] = tabValue
		}
		for _, childKey := range []string{"blocks", "children", "items"} {
			if childValue, exists := filteredTab[childKey]; exists {
				filteredTab[childKey] = filterBlockListValue(childValue, capSet)
			}
		}
		if childValue, exists := filteredTab["child"].(map[string]interface{}); exists {
			var child models.BlockItem
			if encoded, err := json.Marshal(childValue); err == nil && json.Unmarshal(encoded, &child) == nil && child.ID != "" && child.Type != "" {
				filtered := filterBlockList([]models.BlockItem{child}, capSet)
				if len(filtered) == 0 {
					delete(filteredTab, "child")
				} else if encoded, err := json.Marshal(filtered[0]); err == nil {
					var filteredChild map[string]interface{}
					if json.Unmarshal(encoded, &filteredChild) == nil {
						filteredTab["child"] = filteredChild
					}
				}
			}
		}
		result = append(result, filteredTab)
	}
	return result
}
