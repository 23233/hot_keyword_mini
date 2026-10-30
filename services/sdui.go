// Package services sdui.go
package services

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/23233/ggg/logger"
	"github.com/23233/ggg/ut"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	err := db.DB.Where("app_id = ? AND page_id = ?", appID, pageID).First(&page).Error
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

	// 1. 读取租户配置，解析当前主页、兜底页和应急发布模式。
	app := s.loadMiniAppConfig(appID)
	fallbackPageID := "home"
	if app != nil && strings.TrimSpace(app.FallbackPageID) != "" {
		fallbackPageID = strings.TrimSpace(app.FallbackPageID)
	}
	requestedPageID := pageID
	if pageID == "home" && app != nil && app.CurrentPage != "" && app.CurrentPage != "home" {
		requestedPageID = app.CurrentPage
	}
	// 应急模式：忽略请求页，全站强制切到租户兜底页。
	if app != nil && app.ReleaseMode == "fallback" {
		requestedPageID = fallbackPageID
	}

	// 2. 状态门禁：请求页不可用时按兜底页、home、维护页逐级降级，公开路径永不直接 404。
	rawPage, resolvedMode := s.resolveSafePage(appID, requestedPageID, fallbackPageID)

	// 3. 组装基础信封
	envelope, err := s.AssembleEnvelope(rawPage, queryParams, resolvedMode)
	if err != nil {
		return nil, err
	}
	if resolvedMode == "client_published" {
		envelope.Fallback.PageID = fallbackPageID
	} else {
		envelope.Fallback.PageID = rawPage.PageID
	}
	// AI 破甲首页和文章导航只下发摘要与权限结果，正文仍由文章详情接口按会员等级裁剪。
	applyPageEnvelopeEnrichers(envelope, appID, isAuthenticated, userID)
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

	// 7. 分享落地访问策略：接收者不满足 auth/membership/owner 时清空积木与衍生数据，只下发提示策略。
	s.attachViewerIdentity(envelope, appID, isAuthenticated, userID)
	s.applyShareAccess(envelope, rawPage, queryParams, isAuthenticated, userID)

	// 8. ETag 必须覆盖查询参数、客户端能力、登录态和访问者摘要，避免不同用户/客户端错误命中 304。
	envelope.Cache.ETag = computePersonalizedETag(rawPage, queryParams, clientCapabilities, isAuthenticated, envelope.Viewer)

	return envelope, nil
}

// computePersonalizedETag 生成与页面版本、查询参数、客户端能力、登录态和访问者摘要绑定的 ETag。
// 任何一项变化都会产生不同 ETag，防止公共缓存或条件请求把个性化响应串给其他访问者。
func computePersonalizedETag(page *models.DynamicPage, query map[string]string, capabilities string, authenticated bool, viewer map[string]interface{}) string {
	if page == nil {
		return ""
	}
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var queryBuilder strings.Builder
	for _, key := range keys {
		queryBuilder.WriteString(key)
		queryBuilder.WriteByte('=')
		queryBuilder.WriteString(query[key])
		queryBuilder.WriteByte('&')
	}
	viewerJSON := ""
	if len(viewer) > 0 {
		if encoded, err := json.Marshal(viewer); err == nil {
			viewerJSON = string(encoded)
		}
	}
	signature := fmt.Sprintf("%s|%d|%d|%s|%s|%t|%s",
		page.PageID, page.Revision, page.UpdatedAt.Unix(), queryBuilder.String(), capabilities, authenticated, viewerJSON)
	sum := sha256.Sum256([]byte(signature))
	return fmt.Sprintf(`W/"%s-%d-%x"`, page.PageID, page.Revision, sum[:8])
}

// loadMiniAppConfig 读取租户配置；数据库不可用或租户不存在时返回 nil。
func (s *SDUIService) loadMiniAppConfig(appID string) *models.MiniApp {
	if db.DB == nil || strings.TrimSpace(appID) == "" {
		return nil
	}
	var app models.MiniApp
	if err := db.DB.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil
	}
	return &app
}

// resolveSafePage 按请求页、租户兜底页、home、维护页逐级解析可用页面。
// 公开路径不允许直接 404，任何不可用页面都必须安全降级。
func (s *SDUIService) resolveSafePage(appID, requestedPageID, fallbackPageID string) (*models.DynamicPage, string) {
	if page, ok := s.fetchUsablePage(appID, requestedPageID); ok {
		return page, "client_published"
	}
	candidates := make([]string, 0, 2)
	if fallbackPageID != "" && fallbackPageID != requestedPageID {
		candidates = append(candidates, fallbackPageID)
	}
	if requestedPageID != "home" && fallbackPageID != "home" {
		candidates = append(candidates, "home")
	}
	for _, candidate := range candidates {
		if page, ok := s.fetchUsablePage(appID, candidate); ok {
			logger.JM.Warnf("页面 %s 不可用，租户 %s 已降级到兜底页 %s", requestedPageID, appID, candidate)
			return page, "fallback"
		}
	}
	logger.JM.Warnf("租户 %s 的页面 %s 与兜底页均不可用，返回维护页", appID, requestedPageID)
	return buildMaintenancePage(appID), "maintenance"
}

// fetchUsablePage 判断页面是否处于可公开访问状态。
func (s *SDUIService) fetchUsablePage(appID, pageID string) (*models.DynamicPage, bool) {
	if strings.TrimSpace(pageID) == "" {
		return nil, false
	}
	page, err := s.GetRawPage(appID, pageID)
	if err != nil {
		return nil, false
	}
	if page.Status != "published" || page.Hidden {
		return nil, false
	}
	if page.ExpiresAt != nil && page.ExpiresAt.Before(time.Now()) {
		return nil, false
	}
	return page, true
}

// buildMaintenancePage 生成不依赖数据库的最小维护页，保证公开路径始终有可渲染内容。
func buildMaintenancePage(appID string) *models.DynamicPage {
	blocks := []models.BlockItem{
		{ID: "maintenance_notice", Type: "notice", Props: map[string]interface{}{"text": "服务维护中，请稍后重试", "icon": "🛠"}},
		{ID: "maintenance_empty", Type: "empty", Props: map[string]interface{}{"state": "offline", "title": "服务维护中", "desc": "页面暂时不可用，请稍后重新加载"}},
		{ID: "maintenance_refresh", Type: "action_button", Props: map[string]interface{}{"text": "重新加载"}, Action: &models.BlockAction{Type: "refresh"}},
	}
	encoded, _ := json.Marshal(blocks)
	now := time.Now()
	return &models.DynamicPage{
		AppID: appID, PageID: "maintenance", Revision: 1, Status: "published", Hidden: false,
		Title: "服务维护中", BusinessType: "custom", Intent: "watch", Theme: "light_clean", AccentColor: "#0A84FF",
		ShareConfig: "{}", Blocks: string(encoded), Keyword: "服务维护中",
		Source: "system_fallback", CampaignID: "system_fallback", CreatedAt: now, UpdatedAt: now,
	}
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
	applyPageEnvelopeEnrichers(envelope, appID, false, 0)
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
	applyPageEnvelopeEnrichers(envelope, draft.AppID, false, 0)
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
	entityMap := AssemblePageEntity(rawPage.AppID, rawPage.BusinessType, rawPage.Keyword, queryParams)
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
	s.attachCapabilityViewer(envelope, rawPage.AppID)
	s.rebuildEnvelopeLayoutIR(rawPage, envelope)

	return envelope, nil
}

// attachCapabilityViewer 将租户已启用的公共能力写入 viewer，供 check_capability 动作使用。
// 只下发能力键，不包含任何密钥或配置详情。
func (s *SDUIService) attachCapabilityViewer(envelope *models.PageResponseEnvelope, appID string) {
	if envelope == nil || db.DB == nil || strings.TrimSpace(appID) == "" {
		return
	}
	var app models.MiniApp
	if err := db.DB.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return
	}
	matrix, err := CapabilityMatrixForApp(&app)
	if err != nil {
		return
	}
	enabled := make([]string, 0, len(matrix))
	if len(matrix) == 0 {
		// 兼容尚未配置能力矩阵的旧租户：与发布门禁一致，视为全部已实现能力可用。
		for _, definition := range ListCapabilityDefinitions() {
			if definition.ImplementationStatus == "ready" {
				enabled = append(enabled, definition.Key)
			}
		}
	} else {
		for key, entry := range matrix {
			if entry.State == "enabled" {
				enabled = append(enabled, key)
			}
		}
	}
	sort.Strings(enabled)
	if envelope.Viewer == nil {
		envelope.Viewer = map[string]interface{}{}
	}
	envelope.Viewer["capabilities"] = enabled
}

// resolveWebViewActions 递归解析页面所有动作链中的 WebView 登记键。
func resolveWebViewActions(appID string, blocks []models.BlockItem) error {
	var resolveAction func(*models.BlockAction) error
	resolveAction = func(action *models.BlockAction) error {
		if action == nil {
			return nil
		}
		if action.Type == "open_webview" || action.Type == "open_policy" {
			urlKey, _ := action.Payload["url_key"].(string)
			// open_policy 未配置 url_key 时使用微信原生隐私协议入口，无需解析登记地址。
			if strings.TrimSpace(urlKey) != "" {
				payload, err := ResolveWebViewPayload(appID, action.Payload)
				if err != nil {
					return err
				}
				action.Payload = payload
			}
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

	return db.DB.Create(&page).Error
}

// ListApps 获取所有已注册的小程序应用列表
func (s *SDUIService) ListApps() ([]models.MiniApp, error) {
	var apps []models.MiniApp
	err := db.DB.Order("created_at asc").Find(&apps).Error
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
	err := db.DB.Where("app_id = ?", app.AppID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			app.CreatedAt = time.Now()
			app.UpdatedAt = time.Now()
			if app.CurrentPage == "" {
				app.CurrentPage = "home"
			}
			if err := db.DB.Create(app).Error; err != nil {
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
	if err := db.DB.Model(&existing).Updates(updateMap).Error; err != nil {
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

// DeleteApp 删除小程序租户；存在页面、草稿、模板、用户、商品或会员等级数据时拒绝删除，防止产生孤儿数据。
func (s *SDUIService) DeleteApp(appID string) error {
	if strings.TrimSpace(appID) == "" {
		return errors.New("小程序 AppID 不能为空")
	}
	var app models.MiniApp
	if err := db.DB.Where("app_id = ?", appID).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("小程序不存在")
		}
		return err
	}
	guards := []struct {
		label string
		model any
	}{
		{"动态页面", &models.DynamicPage{}},
		{"页面草稿", &models.DynamicPageDraft{}},
		{"页面模板", &models.DynamicPageTemplate{}},
		{"用户", &models.User{}},
		{"商品", &models.Product{}},
		{"会员等级", &models.MembershipLevel{}},
	}
	for _, guard := range guards {
		var count int64
		if err := db.DB.Model(guard.model).Where("app_id = ?", appID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("该小程序仍有 %d 条%s数据，请先清理后再删除", count, guard.label)
		}
	}
	if err := db.DB.Where("app_id = ?", appID).Delete(&models.MiniApp{}).Error; err != nil {
		return err
	}
	sdk.InvalidateMiniSdk(appID)
	return nil
}

// ListPages 获取指定小程序下的全部动态页面列表
func (s *SDUIService) ListPages(appID string) ([]models.DynamicPage, error) {
	if appID == "" {
		return nil, errors.New("AppID 不能为空")
	}

	var pages []models.DynamicPage
	err := db.DB.Where("app_id = ?", appID).Order("updated_at desc").Find(&pages).Error
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
	return s.savePageWithAudit(page, operator, remark, expectedRevision, true)
}

// SaveSystemSeedPage 仅供系统初始化页面族使用：仍校验协议、能力、WebView、端点和 SKU，
// 但跳过导航目标页存在性检查，避免同一批互链页面按顺序发布时被相互阻断。
func (s *SDUIService) SaveSystemSeedPage(page *models.DynamicPage, operator, remark string, expectedRevision int) error {
	return s.savePageWithAudit(page, operator, remark, expectedRevision, false)
}

func (s *SDUIService) savePageWithAudit(page *models.DynamicPage, operator, remark string, expectedRevision int, checkNavigationTargets bool) error {
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
		report = validateDynamicPageForRelease(page, checkNavigationTargets)
	} else {
		report = ValidateDynamicPage(page)
	}
	if !report.IsValid {
		return fmt.Errorf("动态页面协议强校验未通过，阻断持久化: %s", strings.Join(report.Errors, "; "))
	}

	if db.DB == nil {
		return nil
	}

	// 2. 在单个原子事务中执行页面记录更新与版本历史快照沉淀
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var newRevision int
		var existing models.DynamicPage

		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("app_id = ? AND page_id = ?", page.AppID, page.PageID).First(&existing).Error
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
			updateQuery := tx.Model(&existing)
			if expectedRevision > 0 {
				updateQuery = updateQuery.Where("id = ? AND revision = ?", existing.ID, expectedRevision)
			}
			result := updateQuery.Updates(updateData)
			if result.Error != nil {
				return fmt.Errorf("更新页面协议失败: %w", result.Error)
			}
			if expectedRevision > 0 && result.RowsAffected != 1 {
				return errors.New("发布版本冲突：页面已被其他操作更新")
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

	if db.DB == nil {
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
	err := db.DB.Where("app_id = ? AND page_id = ?", appID, pageID).First(&draft).Error
	if err == nil {
		return &draft, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 草稿不存在，从已发布页面派生一份初始草稿
		var published models.DynamicPage
		if pErr := db.DB.Where("app_id = ? AND page_id = ?", appID, pageID).First(&published).Error; pErr == nil {
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
			_ = db.DB.Create(&draft).Error
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
		_ = db.DB.Create(&draft).Error
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
	if db.DB == nil {
		return nil, errors.New("数据库未初始化")
	}
	var draft models.DynamicPageDraft
	if err := db.DB.Where("app_id = ? AND page_id = ?", appID, pageID).First(&draft).Error; err != nil {
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

	if db.DB == nil {
		return nil
	}

	var existing models.DynamicPageDraft
	err := db.DB.Where("app_id = ? AND page_id = ?", draft.AppID, draft.PageID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expectedRevision > 0 {
				return errors.New("草稿不存在，无法按指定 revision 保存")
			}
			draft.Revision = 1
			draft.Status = "draft"
			draft.CreatedAt = time.Now()
			draft.UpdatedAt = time.Now()
			return db.DB.Create(draft).Error
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

	updateQuery := db.DB.Model(&existing)
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
	if db.DB != nil {
		_ = db.DB.Model(&models.DynamicPageDraft{}).
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
	err := db.DB.Where("app_id = ? AND page_id = ?", appID, pageID).
		Order("revision desc").
		Find(&revs).Error
	return revs, err
}

// RollbackPageRevision 将页面原子回滚至指定的历史 revision 版本并派生新 revision 生效 (全事务化保护并完整恢复页面语义)
func (s *SDUIService) RollbackPageRevision(appID, pageID string, targetRevision int) (*models.DynamicPage, error) {
	if appID == "" || pageID == "" || targetRevision <= 0 {
		return nil, errors.New("参数不完整")
	}

	if db.DB == nil {
		return nil, errors.New("无数据库环境，无法执行回滚")
	}

	var current models.DynamicPage

	err := db.DB.Transaction(func(tx *gorm.DB) error {
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
		if err := validateActionTargetsForRelease(&models.DynamicPage{AppID: appID, PageID: pageID}, capabilityBlocks, true); err != nil {
			return fmt.Errorf("历史版本动作目标发布门禁未通过: %w", err)
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

	if db.DB == nil {
		return errors.New("数据库未初始化")
	}
	var page models.DynamicPage
	if err := db.DB.Where("app_id = ? AND page_id = ? AND status = ? AND hidden = ?", appID, pageID, "published", false).First(&page).Error; err != nil {
		return errors.New("只有已发布且未隐藏的页面才能设为当前主页")
	}
	return db.DB.Model(&models.MiniApp{}).
		Where("app_id = ?", appID).
		Updates(map[string]interface{}{
			"current_page": pageID,
			"updated_at":   time.Now(),
		}).Error
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

// attachViewerIdentity 为所有页面补全登录态、用户 ID 与会员等级，供分享访问策略使用。
func (s *SDUIService) attachViewerIdentity(envelope *models.PageResponseEnvelope, appID string, authenticated bool, userID int64) {
	if envelope == nil {
		return
	}
	if envelope.Viewer == nil {
		envelope.Viewer = map[string]interface{}{}
	}
	envelope.Viewer["logged_in"] = authenticated
	if userID > 0 {
		envelope.Viewer["user_id"] = userID
	}
	if _, exists := envelope.Viewer["membership_level"]; !exists {
		level := 0
		if authenticated && userID > 0 && db.DB != nil {
			if membership, err := NewMembershipService().GetMembership(appID, userID); err == nil && membership != nil && membership.ExpiresAt.After(time.Now()) {
				level = membership.Level
			}
		}
		envelope.Viewer["membership_level"] = level
	}
}

// applyShareAccess 对分享落地页执行访问策略；未通过时清空页面内容并下发结构化提示。
func (s *SDUIService) applyShareAccess(envelope *models.PageResponseEnvelope, rawPage *models.DynamicPage, query map[string]string, authenticated bool, userID int64) {
	if envelope == nil {
		return
	}
	from := strings.TrimSpace(query["from"])
	if from != "share" && from != "timeline" {
		return
	}
	mode := ""
	access := (*models.ShareAccess)(nil)
	if envelope.Page.ShareConfig != nil && envelope.Page.ShareConfig.Access != nil {
		access = envelope.Page.ShareConfig.Access
		mode = strings.ToLower(strings.TrimSpace(access.Mode))
	}
	if mode == "" {
		// 业务 Provider 可声明默认分享模式（如付费文章默认会员）；否则受保护页回退登录。
		if defaultMode := DefaultShareMode(envelope.Page.BusinessType); defaultMode != "" {
			mode = defaultMode
		} else if envelope.Page.RequireAuth {
			mode = "auth"
		}
	}
	if mode == "" || mode == "public" {
		return
	}

	level := 0
	switch value := envelope.Viewer["membership_level"].(type) {
	case float64:
		level = int(value)
	case int:
		level = value
	case int64:
		level = int(value)
	}
	blocked := false
	action := "home"
	switch mode {
	case "auth":
		blocked = !authenticated
		action = "login"
	case "membership":
		blocked = !authenticated || level <= 0
		if authenticated {
			action = "membership"
		} else {
			action = "login"
		}
	case "owner":
		sharedFrom := strings.TrimSpace(query["share_from"])
		blocked = !authenticated || sharedFrom == "" || sharedFrom != strconv.FormatInt(userID, 10)
		action = "home"
	default:
		return
	}
	if !blocked {
		return
	}

	// 拦截时必须清空积木与衍生数据，避免仅靠客户端隐藏造成内容泄露。
	envelope.Page.Blocks = []models.BlockItem{}
	envelope.Data = map[string]interface{}{}
	s.rebuildEnvelopeLayoutIR(rawPage, envelope)

	title, message, actionText := "", "", ""
	membershipPageID := "membership"
	if access != nil {
		title = strings.TrimSpace(access.Title)
		message = strings.TrimSpace(access.Message)
		actionText = strings.TrimSpace(access.ActionText)
		if strings.TrimSpace(access.MembershipPageID) != "" {
			membershipPageID = strings.TrimSpace(access.MembershipPageID)
		}
	}
	switch action {
	case "login":
		title = firstNonEmpty(title, "该分享需要登录查看")
		message = firstNonEmpty(message, "为保护分享内容，请先完成微信授权后查看。")
		actionText = firstNonEmpty(actionText, "微信授权登录")
	case "membership":
		title = firstNonEmpty(title, "会员专属分享")
		message = firstNonEmpty(message, "该内容由会员分享，开通会员后可查看。")
		actionText = firstNonEmpty(actionText, "开通会员")
	default:
		title = firstNonEmpty(title, "该分享暂不可查看")
		message = firstNonEmpty(message, "分享内容受访问权限保护，请按提示操作。")
		actionText = firstNonEmpty(actionText, "返回首页")
	}
	envelope.Viewer["share_guard"] = map[string]interface{}{
		"mode": mode, "title": title, "message": message,
		"action": action, "action_text": actionText, "membership_page_id": membershipPageID,
	}
}

// firstNonEmpty 返回第一个非空字符串，用于分享提示默认文案。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
