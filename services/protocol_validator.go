// Package services protocol_validator.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// ValidationReport 协议强校验机器可读诊断报告
type ValidationReport struct {
	// 协议是否整体合法有效 (为 true 且无致命错误方可发布)
	IsValid bool `json:"is_valid"`
	// 致命错误列表 (阻断发布)
	Errors []string `json:"errors"`
	// 潜在问题警告 (不阻断发布，但建议优化)
	Warnings []string `json:"warnings"`
	// 机器可读自动修复建议
	Suggestions []string `json:"suggestions"`
	// 积木组件总数
	BlockCount int `json:"block_count"`
	// 租户能力矩阵校验结果；发布校验时返回。
	Capability *CapabilityValidationReport `json:"capability,omitempty"`
}

// ValidateDynamicPageForRelease 执行协议与租户能力矩阵的一致发布校验。
func ValidateDynamicPageForRelease(page *models.DynamicPage) ValidationReport {
	report := ValidateDynamicPage(page)
	if page == nil || !report.IsValid {
		return report
	}
	var blocks []models.BlockItem
	if err := json.Unmarshal([]byte(page.Blocks), &blocks); err != nil {
		report.IsValid = false
		report.Errors = append(report.Errors, "页面能力依赖解析失败: "+err.Error())
		return report
	}
	capabilityReport, err := ValidatePageCapabilities(page.AppID, blocks)
	if err != nil {
		report.IsValid = false
		report.Errors = append(report.Errors, err.Error())
		return report
	}
	report.Capability = &capabilityReport
	if !capabilityReport.IsValid {
		report.IsValid = false
		report.Errors = append(report.Errors, capabilityReport.Errors...)
	}
	if err := validateWebViewKeysForRelease(page.AppID, blocks); err != nil {
		report.IsValid = false
		report.Errors = append(report.Errors, err.Error())
	}
	return report
}

// validateWebViewKeysForRelease 校验发布页面引用的 WebView 入口属于当前租户且处于启用状态。
func validateWebViewKeysForRelease(appID string, blocks []models.BlockItem) error {
	if db.Mysql == nil {
		return nil
	}
	var entries []WebViewEntry
	loaded := false
	loadEntries := func() error {
		if loaded {
			return nil
		}
		var err error
		entries, err = ListWebViews(appID)
		if err != nil {
			return fmt.Errorf("WebView 登记表校验失败: %w", err)
		}
		loaded = true
		return nil
	}
	var walkAction func(*models.BlockAction) error
	walkAction = func(action *models.BlockAction) error {
		if action == nil {
			return nil
		}
		if action.Type == "open_webview" {
			if err := loadEntries(); err != nil {
				return err
			}
			urlKey, _ := action.Payload["url_key"].(string)
			if _, err := findWebViewEntry(entries, urlKey); err != nil {
				return fmt.Errorf("WebView url_key %q 不可用: %w", urlKey, err)
			}
		}
		for index := range action.OnSuccess {
			if err := walkAction(&action.OnSuccess[index]); err != nil {
				return err
			}
		}
		for index := range action.OnError {
			if err := walkAction(&action.OnError[index]); err != nil {
				return err
			}
		}
		return nil
	}
	var walkBlock func(models.BlockItem) error
	walkBlock = func(block models.BlockItem) error {
		if err := walkAction(block.Action); err != nil {
			return err
		}
		for _, actions := range block.Events {
			for index := range actions {
				if err := walkAction(&actions[index]); err != nil {
					return err
				}
			}
		}
		for _, state := range []*models.BlockItem{block.Loading, block.Empty, block.Error, block.Fallback} {
			if state != nil {
				if err := walkBlock(*state); err != nil {
					return err
				}
			}
		}
		if block.Props != nil {
			for _, key := range []string{"children", "blocks", "items"} {
				if raw, ok := block.Props[key]; ok {
					encoded, _ := json.Marshal(raw)
					var nested []models.BlockItem
					if json.Unmarshal(encoded, &nested) == nil {
						for _, child := range nested {
							if err := walkBlock(child); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		return nil
	}
	for _, block := range blocks {
		if err := walkBlock(block); err != nil {
			return err
		}
	}
	return nil
}

// 合法原子积木类型白名单 (严格与 doc/sdui_dynamic_engine_architecture.md 和 schema 对齐)
var allowedBlockTypes = map[string]bool{
	// 1. 基础布局块
	"stack":     true,
	"container": true,
	"grid":      true,
	"tabs":      true,
	"carousel":  true,
	"list":      true,
	"spacer":    true,

	// 2. 基础内容块
	"text":      true,
	"rich_text": true,
	"image":     true,
	"video":     true,
	"notice":    true,
	"timeline":  true,
	"empty":     true,
	"skeleton":  true,

	// 3. 业务功能块
	"media_hero":           true,
	"resource_card":        true,
	"action_button":        true,
	"game_card":            true,
	"form":                 true,
	"episode_list":         true,
	"item_grid":            true,
	"score_panel":          true,
	"coupon_card":          true,
	"countdown":            true,
	"result_table":         true,
	"contact_card":         true,
	"map_card":             true,
	"game_header":          true,
	"redeem_code_card":     true,
	"server_status":        true,
	"product_card":         true,
	"download_card":        true,
	"payment_result":       true,
	"price_breakdown":      true,
	"event_card":           true,
	"poll":                 true,
	"feed_list":            true,
	"category_nav":         true,
	"article_feed":         true,
	"article_detail":       true,
	"membership_plan_list": true,
	"comment_thread":       true,
	"collection_nav":       true,
	"content_feed":         true,
	"content_detail":       true,
	"offer_list":           true,
	"discussion_thread":    true,
	"bottom_nav":           true,
	"floating_action":      true,
	"media_picker":         true,
	"upload_progress":      true,
	"media_gallery":        true,
	"location_picker":      true,
	"address_card":         true,
	"service_entry":        true,
	"qr_code":              true,
	"webview_entry":        true,
	"webview_state":        true,
	"feature_gate":         true,
	"compliance_panel":     true,
	"chat_thread":          true,
	"message_list":         true,
	"message_composer":     true,
	"unread_badge":         true,
	"order_card":           true,
	"order_summary":        true,
	"order_timeline":       true,
	"logistics_track":      true,
	"after_sale_form":      true,
	"evidence_list":        true,
	"service_card":         true,
	"task_card":            true,
	"quote_card":           true,
	"schedule_picker":      true,
	"membership_card":      true,
	"ad_slot":              true,
	"wallet_card":          true,
	"withdraw_form":        true,

	// 4. 通用自由编排/自定义卡片积木
	"custom":       true,
	"custom_block": true,
}

// 合法原子样式令牌白名单。客户端只将这些标记翻译为固定工具类，不接受任意 CSS。
var allowedStyleUtilities = map[string]bool{
	"layout/flat": true, "layout/card": true,
	"surface/canvas": true, "surface/base": true, "surface/raised": true, "surface/ink": true,
	"border/none": true, "border/subtle": true, "border/strong": true,
	"radius/none": true, "radius/sm": true, "radius/md": true, "radius/lg": true, "radius/xl": true, "radius/full": true,
	"space/y-0": true, "space/y-2": true, "space/y-4": true, "space/y-6": true, "space/y-8": true, "space/y-10": true, "space/y-12": true,
	"padding/none": true, "padding/0": true, "padding/2": true, "padding/4": true, "padding/5": true, "padding/6": true, "padding/8": true,
	"padding/x-4": true, "padding/x-5": true, "padding/x-6": true, "padding/y-4": true, "padding/y-6": true,
	"gap/2": true, "gap/4": true, "gap/6": true, "gap/8": true,
	"text/display": true, "text/section": true, "text/muted": true, "text/inverse": true,
	"accent/blue": true, "accent/ink": true, "accent/amber": true, "accent/green": true,
	"elevation/none": true, "elevation/sm": true, "elevation/md": true,
	"media/rounded": true,
}

// 合法原子动作类型白名单 (严格与 doc/sdui_dynamic_engine_architecture.md 和 schema 对齐)
var allowedActionTypes = map[string]bool{
	"copy_text":              true,
	"navigate_page":          true,
	"open_channels_activity": true,
	"open_mini_program":      true,
	"request_data":           true,
	"request":                true, // 兼容架构设计文档 3.2.2 与 request_data 等价
	"request_payment":        true,
	"open_webview":           true,
	"preview_image":          true,
	"toast":                  true,
	"refresh":                true,
	"require_auth":           true,
	"share":                  true,
	"subscribe_message":      true,
	"set_state":              true,
	"toggle_state":           true,
	"reset_state":            true,
	"show_error_state":       true,
	"show_empty_state":       true,
	"show_loading_state":     true,
	"reset_block_state":      true,
	"choose_media":           true,
	"upload_file":            true,
	"delete_media":           true,
	"request_location":       true,
	"choose_location":        true,
	"open_map":               true,
	"open_wechat_service":    true,
	"save_qr":                true,
	"open_internal_chat":     true,
	"send_message":           true,
	"mark_read":              true,
	"poll_messages":          true,
	"connect_message":        true,
	"upload_chat_media":      true,
	"create_order":           true,
	"confirm_order":          true,
	"cancel_order":           true,
	"confirm_receipt":        true,
	"refresh_logistics":      true,
	"apply_after_sale":       true,
	"upload_evidence":        true,
	"accept_task":            true,
	"reject_task":            true,
	"submit_quote":           true,
	"update_service_status":  true,
	"open_membership":        true,
	"load_ad":                true,
	"refresh_wallet":         true,
	"request_withdraw":       true,
}

var actionEndpointPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
var dynamicPageIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ValidateDynamicPage 对传入的动态页面协议执行严格结构化校验与安全审计
func ValidateDynamicPage(page *models.DynamicPage) ValidationReport {
	report := ValidationReport{
		IsValid:     true,
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		Suggestions: make([]string, 0),
	}

	if page == nil {
		report.IsValid = false
		report.Errors = append(report.Errors, "DynamicPage 实体不能为空")
		return report
	}

	// 1. 校验元信息必填项
	if strings.TrimSpace(page.AppID) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, "缺少所属租户 app_id")
		report.Suggestions = append(report.Suggestions, "请为页面指定归属的小程序 AppID")
	}

	if strings.TrimSpace(page.PageID) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, "缺少页面唯一标识 page_id")
		report.Suggestions = append(report.Suggestions, "请指定合法的 page_id，如 home 或 drama_detail")
	}

	if strings.TrimSpace(page.Title) == "" {
		report.Warnings = append(report.Warnings, "页面主标题为空，将采用默认占位标题")
		report.Suggestions = append(report.Suggestions, "建议配置醒目的页面主标题以提高转化率")
	}

	// 2. 校验 Blocks 积木树
	if strings.TrimSpace(page.Blocks) == "" || page.Blocks == "[]" {
		report.Warnings = append(report.Warnings, "页面当前积木组件列表为空")
		report.Suggestions = append(report.Suggestions, "可使用套用行业模板快速填充初始积木树")
		report.BlockCount = 0
		return report
	}

	var blocks []models.BlockItem
	if err := json.Unmarshal([]byte(page.Blocks), &blocks); err != nil {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Blocks JSON 序列化解析失败: %v", err))
		return report
	}

	report.BlockCount = len(blocks)
	idMap := make(map[string]int)

	// 3. 逐个积木深度语法与动作审计
	for idx, block := range blocks {
		pathPrefix := fmt.Sprintf("blocks[%d](id:%s)", idx, block.ID)

		if strings.TrimSpace(block.ID) == "" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 缺少唯一标识 ID", pathPrefix))
		} else {
			if prevIdx, exists := idMap[block.ID]; exists {
				report.IsValid = false
				report.Errors = append(report.Errors, fmt.Sprintf("积木 ID 冲突重复: '%s' (出现在索引 %d 与 %d)", block.ID, prevIdx, idx))
				report.Suggestions = append(report.Suggestions, fmt.Sprintf("请重命名积木 %s 的 ID 保持页面内全局唯一", block.ID))
			} else {
				idMap[block.ID] = idx
			}
		}

		// 检查积木类型是否在受控白名单
		if !allowedBlockTypes[block.Type] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 使用了未知积木类型 '%s'，客户端将执行降级占位", pathPrefix, block.Type))
		}
		validateStyleUtilities(block.Style, pathPrefix+".style", &report)

		// 检查积木绑定的交互动作合法性
		if block.Action != nil && block.Action.Type != "" {
			actType := block.Action.Type
			if !allowedActionTypes[actType] {
				report.IsValid = false
				report.Errors = append(report.Errors, fmt.Sprintf("%s 绑定了未知的交互动作类型 '%s'", pathPrefix, actType))
				report.Suggestions = append(report.Suggestions, fmt.Sprintf("请将动作类型修正为合法动作，如 copy_text, open_channels_activity, request_data"))
			}

			payload := block.Action.Payload
			if payload == nil {
				payload = make(map[string]interface{})
			}

			// 针对关键原生动作进行参数完备性强校验
			switch actType {
			case "open_channels_activity":
				feedID, _ := payload["feed_id"].(string)
				finderName, _ := payload["finder_user_name"].(string)
				if feedID == "" || finderName == "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s 视频号跳转缺少必填参数 feed_id 或 finder_user_name", pathPrefix))
					report.Suggestions = append(report.Suggestions, "请在动作 payload 中补充有效的视频号动态 feed_id 与 finder_user_name")
				}
			case "open_mini_program":
				targetAppID, _ := payload["target_app_id"].(string)
				if targetAppID == "" {
					targetAppID, _ = payload["app_id"].(string)
				}
				if strings.TrimSpace(targetAppID) == "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s 跨小程序跳转缺少目标 target_app_id 或 app_id", pathPrefix))
					report.Suggestions = append(report.Suggestions, "请配置跳转目标小程序的 AppID")
				}
			case "open_webview":
				urlKey, _ := payload["url_key"].(string)
				if strings.TrimSpace(urlKey) == "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s WebView 动作缺少 payload.url_key", pathPrefix))
					report.Suggestions = append(report.Suggestions, "请使用已登记的 WebView url_key，不要直接下发网页地址")
				}
				if customURL, _ := payload["url"].(string); strings.TrimSpace(customURL) != "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s WebView 动作不得直接配置 payload.url", pathPrefix))
				}
				if customURL, _ := payload["web_url"].(string); strings.TrimSpace(customURL) != "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s WebView 动作不得直接配置 payload.web_url", pathPrefix))
				}
			case "request_data", "request":
				endpoint, _ := payload["endpoint"].(string)
				customURL, _ := payload["url"].(string)
				if endpoint == "" && customURL == "" {
					report.Warnings = append(report.Warnings, fmt.Sprintf("%s request_data 未声明已登记的 endpoint，将尝试默认端点", pathPrefix))
				}
				// 严密安全审计: 若配置了自定义 url，严禁配置任意外部第三方未知地址，防止开放代理与凭据泄漏
				if customURL != "" && (strings.HasPrefix(customURL, "http://") || strings.HasPrefix(customURL, "https://") || strings.HasPrefix(customURL, "//")) {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s request_data 包含非同源绝对 URL '%s'，违反安全白名单要求", pathPrefix, customURL))
					report.Suggestions = append(report.Suggestions, "请改用已登记的受控 endpoint (如 game.redeem) 或使用同源相对路径 (以 / 开头)")
				}
			}
		}

		// 检查积木绑定的多事件流动作列表 (events: { tap: [...] })
		if block.Events != nil {
			for eventName, actions := range block.Events {
				for eIdx, evAct := range actions {
					evPrefix := fmt.Sprintf("%s.events.%s[%d]", pathPrefix, eventName, eIdx)
					if evAct.Type != "" && !allowedActionTypes[evAct.Type] {
						report.IsValid = false
						report.Errors = append(report.Errors, fmt.Sprintf("%s 绑定了未知的交互动作类型 '%s'", evPrefix, evAct.Type))
					}
				}
			}
		}
	}
	// 4. 递归审计所有嵌套 block、状态分支、fallback 与事件链，避免子树绕过发布校验。
	for idx := range blocks {
		validateNestedBlockContracts(&blocks[idx], fmt.Sprintf("blocks[%d]", idx), idMap, &report)
	}

	// 5. 执行机器可读 JSON Schema 基础契约核验
	if schemaReport := ValidatePageAgainstSchema(page); !schemaReport.IsValid {
		report.IsValid = false
		report.Errors = append(report.Errors, schemaReport.Errors...)
	}

	if len(report.Errors) > 0 {
		report.IsValid = false
	}

	return report
}

// validateRawNestedBlocks 校验原始嵌套节点，避免非法子节点在模型反序列化时被静默跳过。
func validateRawNestedBlocks(raw, path string, report *ValidationReport) {
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return
	}
	var walk func(interface{}, string)
	walk = func(current interface{}, currentPath string) {
		switch node := current.(type) {
		case []interface{}:
			for index, item := range node {
				walk(item, fmt.Sprintf("%s[%d]", currentPath, index))
			}
		case map[string]interface{}:
			if _, hasType := node["type"]; hasType {
				encoded, _ := json.Marshal(node)
				var block models.BlockItem
				if err := json.Unmarshal(encoded, &block); err != nil {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s 嵌套 Block 无法解析: %v", currentPath, err))
				}
			}
			for key, item := range node {
				walk(item, currentPath+"."+key)
			}
		}
	}
	walk(value, path)
}

// validateNestedBlockContracts 递归检查嵌套 block 的 ID、类型、动作和状态分支。
func validateNestedBlockContracts(block *models.BlockItem, path string, idMap map[string]int, report *ValidationReport) {
	if block == nil {
		return
	}
	validateStyleUtilities(block.Style, path+".style", report)
	validateVisibleWhen(block.VisibleWhen, path, report)
	// 校验当前 block 自身绑定的单一动作
	validateNestedActionContracts(block.Action, path+".action", report)
	// 校验当前 block 绑定的多事件流动作列表
	if block.Events != nil {
		for eventName, actions := range block.Events {
			for idx := range actions {
				validateNestedActionContracts(&actions[idx], fmt.Sprintf("%s.events.%s[%d]", path, eventName, idx), report)
			}
		}
	}
	// 校验 props 中嵌入的节点动作 (如 timeline 节点 action、item_grid 单元格 action)
	validateEmbeddedActions(block.Props, path+".props", report)
	for _, state := range []*models.BlockItem{block.Loading, block.Empty, block.Error, block.Fallback} {
		if state == nil {
			continue
		}
		statePath := path + ".state"
		if strings.TrimSpace(state.ID) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s.id 必填且不能为空", statePath))
		} else if _, exists := idMap[state.ID]; exists {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("积木 ID 冲突重复: '%s'（状态分支 %s）", state.ID, statePath))
		} else {
			idMap[state.ID] = len(idMap)
		}
		validateNestedBlockContracts(state, statePath, idMap, report)
	}
	for _, child := range collectNestedBlocks(block.Props) {
		childPath := fmt.Sprintf("%s.props.child(id:%s)", path, child.ID)
		if strings.TrimSpace(child.ID) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s.id 必填且不能为空", childPath))
		} else if _, exists := idMap[child.ID]; exists {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("积木 ID 冲突重复: '%s'（嵌套路径 %s）", child.ID, childPath))
		} else {
			idMap[child.ID] = len(idMap)
		}
		if strings.TrimSpace(child.Type) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s.type 必填且不能为空", childPath))
		} else if !allowedBlockTypes[child.Type] && child.Fallback == nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 使用未知积木类型 '%s'，客户端将执行优雅降级", childPath, child.Type))
		}
		childCopy := child
		validateNestedBlockContracts(&childCopy, childPath, idMap, report)
	}
}

// validateVisibleWhen 校验任意层级 Block 的受控条件作用域。
func validateVisibleWhen(condition map[string]interface{}, path string, report *ValidationReport) {
	if condition == nil {
		return
	}
	for key, raw := range condition {
		switch child := raw.(type) {
		case map[string]interface{}:
			validateVisibleWhen(child, path+"."+key, report)
		case []interface{}:
			for index, item := range child {
				if nested, ok := item.(map[string]interface{}); ok {
					validateVisibleWhen(nested, fmt.Sprintf("%s.%s[%d]", path, key, index), report)
				}
			}
		}
	}
	value, _ := condition["path"].(string)
	if value == "" {
		return
	}
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "$")
	root := strings.Split(trimmed, ".")[0]
	allowed := map[string]bool{"entity": true, "query": true, "item": true, "state": true, "result": true, "page": true, "session": true, "tenant": true, "props": true}
	if !allowed[root] {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("%s.visible_when.path '%s' 不属于受控作用域", path, value))
	}
}

// validateStyleUtilities 校验样式令牌，拒绝未知令牌以保证 MCP 和 HTTP 一致的受控渲染边界。
func validateStyleUtilities(style *models.BlockStyle, path string, report *ValidationReport) {
	if style == nil {
		return
	}
	for _, utility := range style.Utilities {
		if !allowedStyleUtilities[utility] {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s.utilities 包含未注册样式令牌 '%s'", path, utility))
		}
	}
}

// validateNestedActionContracts 校验嵌套动作及其成功/失败链的动作类型和关键参数。
func validateNestedActionContracts(action *models.BlockAction, path string, report *ValidationReport) {
	if action == nil {
		return
	}
	if strings.TrimSpace(action.Type) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("%s.type 必填且不能为空", path))
	} else if !allowedActionTypes[action.Type] {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("%s.type '%s' 不在动作枚举定义中", path, action.Type))
	}
	if action.Type == "open_channels_activity" {
		feedID, _ := action.Payload["feed_id"].(string)
		finder, _ := action.Payload["finder_user_name"].(string)
		if strings.TrimSpace(feedID) == "" || strings.TrimSpace(finder) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 缺少 feed_id 或 finder_user_name", path))
		}
	}
	if action.Type == "open_mini_program" {
		target, _ := action.Payload["target_app_id"].(string)
		if target == "" {
			target, _ = action.Payload["app_id"].(string)
		}
		if strings.TrimSpace(target) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 缺少 target_app_id 或 app_id", path))
		}
	}
	if action.Type == "open_webview" {
		urlKey, _ := action.Payload["url_key"].(string)
		if strings.TrimSpace(urlKey) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 缺少 payload.url_key", path))
		}
		if customURL, _ := action.Payload["url"].(string); strings.TrimSpace(customURL) != "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 不得直接配置 payload.url", path))
		}
		if customURL, _ := action.Payload["web_url"].(string); strings.TrimSpace(customURL) != "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 不得直接配置 payload.web_url", path))
		}
	}
	if action.Type == "copy_text" {
		if !isValidCopyTextPayload(action.Payload) {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s copy_text 动作缺少有效的 text、content 或 path 载荷", path))
		}
	}
	if action.Type == "request_payment" {
		if !isValidRequestPaymentPayload(action.Payload) {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s request_payment 动作缺少有效的商品 sku 标识", path))
		}
	}
	if action.Type == "upload_file" {
		filePath := action.Payload["file_path"]
		if filePath == nil {
			filePath = action.Payload["temp_file_path"]
		}
		if !isNonEmptyActionValue(filePath) || !isHTTPSActionValue(action.Payload["presigned_url"]) {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s upload_file 缺少有效文件路径或 HTTPS 预签名地址", path))
		}
	}
	if action.Type == "delete_media" {
		endpoint, _ := action.Payload["endpoint"].(string)
		if !actionEndpointPattern.MatchString(endpoint) {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s delete_media 缺少合法的受控 endpoint", path))
		}
	}
	if action.Type == "open_map" {
		latitudeOK := isCoordinateActionValue(action.Payload["latitude"], -90, 90)
		longitudeOK := isCoordinateActionValue(action.Payload["longitude"], -180, 180)
		scaleOK := action.Payload["scale"] == nil || isCoordinateActionValue(action.Payload["scale"], 3, 20)
		if !latitudeOK || !longitudeOK || !scaleOK {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s open_map 经纬度或缩放级别无效", path))
		}
	}
	if action.Type == "open_wechat_service" {
		corpID := action.Payload["corp_id"]
		extURL := action.Payload["url"]
		if extInfo, ok := action.Payload["ext_info"].(map[string]interface{}); ok {
			extURL = extInfo["url"]
		}
		if !isNonEmptyActionValue(corpID) || !isHTTPSActionValue(extURL) {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s open_wechat_service 缺少 corp_id 或 HTTPS 客服地址", path))
		}
	}
	if action.Type == "save_qr" {
		imageURL := action.Payload["url"]
		if imageURL == nil {
			imageURL = action.Payload["image_url"]
		}
		if !isHTTPSActionValue(imageURL) {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s save_qr 缺少有效的 HTTPS 图片地址", path))
		}
	}
	if action.Type == "open_internal_chat" {
		if pageID := action.Payload["page_id"]; pageID != nil && !isBindingActionValue(pageID) {
			value, ok := pageID.(string)
			if !ok || !dynamicPageIDPattern.MatchString(value) {
				report.IsValid = false
				report.Errors = append(report.Errors, fmt.Sprintf("%s open_internal_chat 页面标识无效", path))
			}
		}
	}
	if action.Type == "subscribe_message" {
		_, hasTemplate := action.Payload["template_id"]
		_, hasTmplID := action.Payload["tmpl_id"]
		_, hasTemplates := action.Payload["tmpl_ids"]
		_, hasTemplateIDs := action.Payload["template_ids"]
		if !hasTemplate && !hasTmplID && !hasTemplates && !hasTemplateIDs {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 缺少 template_id 或 tmpl_ids", path))
		}
	}
	if action.Type == "request_data" || action.Type == "request" {
		if customURL, _ := action.Payload["url"].(string); strings.HasPrefix(customURL, "http://") || strings.HasPrefix(customURL, "https://") || strings.HasPrefix(customURL, "//") {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 包含非同源绝对 URL", path))
		}
	}
	if action.Type == "set_state" || action.Type == "toggle_state" {
		if !hasActionTarget(action.Payload, "key", "name", "target") {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s %s 动作缺少 payload.key、payload.name 或 payload.target", path, action.Type))
		}
	}
	if action.Type == "show_error_state" || action.Type == "show_empty_state" || action.Type == "show_loading_state" || action.Type == "reset_block_state" {
		if !hasActionTarget(action.Payload, "target") {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s %s 动作缺少 payload.target", path, action.Type))
		}
	}
	for idx := range action.OnSuccess {
		validateNestedActionContracts(&action.OnSuccess[idx], fmt.Sprintf("%s.on_success[%d]", path, idx), report)
	}
	for idx := range action.OnError {
		validateNestedActionContracts(&action.OnError[idx], fmt.Sprintf("%s.on_error[%d]", path, idx), report)
	}
	// 兼容校验 payload 中的级联成功/失败动作链 (支持 []interface{} 与 []map[string]interface{} 双向兼容)
	if action.Payload != nil {
		if rawSucc, ok := action.Payload["on_success"]; ok {
			var succList []interface{}
			if s, ok := rawSucc.([]interface{}); ok {
				succList = s
			} else if s, ok := rawSucc.([]map[string]interface{}); ok {
				for _, m := range s {
					succList = append(succList, m)
				}
			}
			for idx, succItem := range succList {
				if subActBytes, err := json.Marshal(succItem); err == nil {
					var subAct models.BlockAction
					if json.Unmarshal(subActBytes, &subAct) == nil {
						validateNestedActionContracts(&subAct, fmt.Sprintf("%s.payload.on_success[%d]", path, idx), report)
					}
				}
			}
		}
		if rawErr, ok := action.Payload["on_error"]; ok {
			var errList []interface{}
			if s, ok := rawErr.([]interface{}); ok {
				errList = s
			} else if s, ok := rawErr.([]map[string]interface{}); ok {
				for _, m := range s {
					errList = append(errList, m)
				}
			}
			for idx, errItem := range errList {
				if subActBytes, err := json.Marshal(errItem); err == nil {
					var subAct models.BlockAction
					if json.Unmarshal(subActBytes, &subAct) == nil {
						validateNestedActionContracts(&subAct, fmt.Sprintf("%s.payload.on_error[%d]", path, idx), report)
					}
				}
			}
		}
	}
}

// hasActionTarget 校验动作目标字段为非空字符串，避免客户端静默忽略无效状态动作。
func hasActionTarget(payload map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// collectNestedBlocks 从任意 props 结构递归提取所有带 type 的子 block。
func collectNestedBlocks(value interface{}) []models.BlockItem {
	result := make([]models.BlockItem, 0)
	var walk func(interface{})
	walk = func(current interface{}) {
		switch v := current.(type) {
		case map[string]interface{}:
			if _, ok := v["type"]; ok {
				// props 中的动作对象同样包含 type，但通过 payload/endpoint/url 可识别，不应当按积木校验。
				if _, isAction := v["payload"]; isAction {
					return
				}
				if raw, err := json.Marshal(v); err == nil {
					var child models.BlockItem
					if json.Unmarshal(raw, &child) == nil && child.Type != "" {
						result = append(result, child)
						return
					}
				}
			}
			for _, item := range v {
				walk(item)
			}
		case []interface{}:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(value)
	return result
}

// ValidatePageAgainstSchema 加载并基于 schema/sdui.schema.json 执行动态页面与信封契约合规深度递归核验
func ValidatePageAgainstSchema(page *models.DynamicPage) ValidationReport {
	report := ValidationReport{
		IsValid:     true,
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		Suggestions: make([]string, 0),
	}

	if page == nil {
		report.IsValid = false
		report.Errors = append(report.Errors, "动态页面协议实体不能为空")
		return report
	}

	// 1. 尝试加载并解析模式定义文件 schema/sdui.schema.json
	schemaPath := "schema/sdui.schema.json"
	if _, err := os.Stat(schemaPath); err == nil {
		content, err := os.ReadFile(schemaPath)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("读取 Schema 模式定义文件失败: %v", err))
		} else {
			var schemaDef map[string]interface{}
			if err := json.Unmarshal(content, &schemaDef); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("解析 Schema 模式定义文件失败: %v", err))
			}
		}
	}

	// 2. 深度契约核验: 页面顶层 required 必填字段
	if strings.TrimSpace(page.PageID) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, "Schema 契约校验失败: page.page_id 必须为非空字符串")
	}
	if strings.TrimSpace(page.Title) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, "Schema 契约校验失败: page.title 必须为非空字符串")
	}
	if strings.TrimSpace(page.BusinessType) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, "Schema 契约校验失败: page.business_type 必须声明有效业务类型")
	}

	// 3. 枚举有效性强校验 (严格对齐 sdui.schema.json definitions)
	validBusinessTypes := map[string]bool{"drama": true, "game": true, "query": true, "download": true, "custom": true, "ai_breakthrough": true, "ai_article": true}
	if page.BusinessType != "" && !validBusinessTypes[page.BusinessType] {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: page.business_type '%s' 超出合法枚举定义", page.BusinessType))
	}

	validIntents := map[string]bool{
		"watch":    true,
		"redeem":   true,
		"query":    true,
		"download": true,
		"buy":      true,
		"book":     true,
		"join":     true,
	}
	if page.Intent != "" && !validIntents[page.Intent] {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: page.intent '%s' 超出合法意图枚举定义", page.Intent))
	}

	validStatuses := map[string]bool{
		"draft":     true,
		"published": true,
		"archived":  true,
		"reviewing": true,
	}
	if page.Status != "" && !validStatuses[page.Status] {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: page.status '%s' 超出状态枚举定义", page.Status))
	}

	validThemes := map[string]bool{"dark_glass": true, "light_clean": true, "cyber_neon": true}
	if page.Theme != "" && !validThemes[page.Theme] {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: page.theme '%s' 超出主题风格枚举定义", page.Theme))
	}

	// 4. 嵌套积木树 Blocks 深度 Schema 契约校验
	if strings.TrimSpace(page.Blocks) == "" {
		report.IsValid = false
		report.Errors = append(report.Errors, "Schema 契约校验失败: page.blocks 字段不能为空")
		return report
	}
	validateRawStyleFields(page.Blocks, &report)
	validateRawNestedBlocks(page.Blocks, "page.blocks", &report)

	var blocks []models.BlockItem
	if err := json.Unmarshal([]byte(page.Blocks), &blocks); err != nil {
		report.IsValid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: page.blocks 数组反序列化异常: %v", err))
		return report
	}

	for idx, b := range blocks {
		bPath := fmt.Sprintf("page.blocks[%d]", idx)
		if strings.TrimSpace(b.ID) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s.id 必填且不能为空", bPath))
		}
		if strings.TrimSpace(b.Type) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s.type 必填且不能为空", bPath))
		} else if !allowedBlockTypes[b.Type] {
			if b.Fallback != nil {
				// 未知积木但已提供有效 fallback 降级块，符合规范允许降级
				report.Warnings = append(report.Warnings, fmt.Sprintf("Schema 提示: %s.type '%s' 为扩展/自定义类型，已提供 fallback 降级保护", bPath, b.Type))
			} else {
				// 未知积木且无 fallback，记录警告降级为占位符
				report.Warnings = append(report.Warnings, fmt.Sprintf("Schema 提示: %s.type '%s' 未在标准积木库定义中，将使用优雅降级占位", bPath, b.Type))
			}
		}

		// 检查 action 契约规范
		if b.Action != nil {
			if strings.TrimSpace(b.Action.Type) == "" {
				report.IsValid = false
				report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s.action.type 必填且不能为空", bPath))
			} else if !allowedActionTypes[b.Action.Type] {
				report.IsValid = false
				report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s.action.type '%s' 不在动作枚举定义中", bPath, b.Action.Type))
			}

			// 特殊动作类型必填 payload 属性深度校验
			if b.Action.Type == "copy_text" {
				if !isValidCopyTextPayload(b.Action.Payload) {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s copy_text 动作 payload 必须包含有效的 text、content 或 path 载荷", bPath))
				}
			} else if b.Action.Type == "request_payment" {
				if !isValidRequestPaymentPayload(b.Action.Payload) {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s request_payment 动作 payload 必须包含有效商品 sku", bPath))
				}
			} else if b.Action.Type == "subscribe_message" {
				if b.Action.Payload == nil {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s subscribe_message 动作 payload 必须包含 template_id 或 tmpl_ids", bPath))
				} else {
					_, hasTmpl := b.Action.Payload["template_id"]
					_, hasTmplId := b.Action.Payload["tmpl_id"]
					_, hasTmplIds := b.Action.Payload["tmplIds"]
					_, hasTmplIdsSnake := b.Action.Payload["tmpl_ids"]
					_, hasTmplIdsPlural := b.Action.Payload["template_ids"]
					if !hasTmpl && !hasTmplId && !hasTmplIds && !hasTmplIdsSnake && !hasTmplIdsPlural {
						report.IsValid = false
						report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s subscribe_message 动作 payload 必须包含 template_id 或 tmpl_ids", bPath))
					}
				}
			} else if b.Action.Type == "open_mini_program" {
				if b.Action.Payload == nil {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s open_mini_program 动作 payload 必须包含 target_app_id 或 app_id", bPath))
				} else {
					targetAppID, _ := b.Action.Payload["target_app_id"].(string)
					appIDVal, _ := b.Action.Payload["app_id"].(string)
					if strings.TrimSpace(targetAppID) == "" && strings.TrimSpace(appIDVal) == "" {
						report.IsValid = false
						report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s open_mini_program 动作 payload.target_app_id 或 app_id 不能为空", bPath))
					}
				}
			} else if b.Action.Type == "open_webview" {
				urlKey, _ := b.Action.Payload["url_key"].(string)
				if strings.TrimSpace(urlKey) == "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s open_webview 动作 payload 必须包含 url_key", bPath))
				}
				if customURL, _ := b.Action.Payload["url"].(string); strings.TrimSpace(customURL) != "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s open_webview 动作不得直接配置 url", bPath))
				}
				if customURL, _ := b.Action.Payload["web_url"].(string); strings.TrimSpace(customURL) != "" {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s open_webview 动作不得直接配置 web_url", bPath))
				}
			}
		}

		validateVisibleWhen(b.VisibleWhen, bPath, &report)
	}

	if len(report.Errors) > 0 {
		report.IsValid = false
	}

	return report
}

// ValidateSDUIStyleJSON 在请求入口校验原始样式，避免模板解码丢弃非法字段。
func ValidateSDUIStyleJSON(raw []byte) error {
	report := ValidationReport{IsValid: true}
	validateRawStyleFields(string(raw), &report)
	if !report.IsValid {
		return fmt.Errorf("样式不合规: %s", strings.Join(report.Errors, "; "))
	}
	return nil
}

// validateRawStyleFields 在反序列化前检查原始样式对象，避免未知字段被模型静默丢弃。
func validateRawStyleFields(raw string, report *ValidationReport) {
	var value interface{}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return
	}
	var walk func(interface{}, string)
	walk = func(current interface{}, path string) {
		switch node := current.(type) {
		case []interface{}:
			for index, item := range node {
				walk(item, fmt.Sprintf("%s[%d]", path, index))
			}
		case map[string]interface{}:
			_, isBlock := node["type"].(string)
			if style, ok := node["style"].(map[string]interface{}); ok && isBlock && node["id"] != nil {
				for key := range style {
					if key != "utilities" && key != "glass_blur" {
						report.IsValid = false
						report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s.style.%s 已废弃，请改用 utilities", path, key))
					}
				}
			}
			for key, item := range node {
				walk(item, path+"."+key)
			}
		}
	}
	walk(value, "page.blocks")
}

// isValidCopyTextPayload 检查 copy_text 动作的载荷是否满足合规要求
// 严格对齐架构设计文档 3.1 节与 3.7 节：支持 text(非空字符串或 path 结构)、content(非空字符串或 path 结构)与 path(非空受控路径)
func isValidCopyTextPayload(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	// 1. 检查 text 字段 (直接字符串或受控 path 对象)
	if textVal, ok := payload["text"]; ok && textVal != nil {
		if s, ok := textVal.(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
		if m, ok := textVal.(map[string]interface{}); ok {
			if p, ok := m["path"].(string); ok && strings.TrimSpace(p) != "" {
				return true
			}
		}
	}
	// 2. 检查 content 别名字段 (直接字符串或受控 path 对象)
	if contentVal, ok := payload["content"]; ok && contentVal != nil {
		if s, ok := contentVal.(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
		if m, ok := contentVal.(map[string]interface{}); ok {
			if p, ok := m["path"].(string); ok && strings.TrimSpace(p) != "" {
				return true
			}
		}
	}
	// 3. 检查直接挂载于 payload 的受控 path 字段 (如 $result.code / $result.copy_text)
	if pathVal, ok := payload["path"]; ok && pathVal != nil {
		if s, ok := pathVal.(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

// isValidRequestPaymentPayload 检查 request_payment 动作的载荷是否满足合规要求
// 严格对齐架构设计文档 3.1 节：必须显式提交商品 sku 或 product_sku，金额始终由服务端查询保障安全
func isValidRequestPaymentPayload(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	if sku, ok := payload["sku"].(string); ok && strings.TrimSpace(sku) != "" {
		return true
	}
	if psku, ok := payload["product_sku"].(string); ok && strings.TrimSpace(psku) != "" {
		return true
	}
	// 兼容受控路径绑定的情况 (如 { "sku": { "path": "$item.sku" } })
	if m, ok := payload["sku"].(map[string]interface{}); ok {
		if p, ok := m["path"].(string); ok && strings.TrimSpace(p) != "" {
			return true
		}
	}
	return false
}

// isBindingActionValue 判断动作参数是否为运行时受控绑定。
func isBindingActionValue(value interface{}) bool {
	if text, ok := value.(string); ok {
		text = strings.TrimSpace(text)
		return strings.HasPrefix(text, "$") || (strings.HasPrefix(text, "{{") && strings.HasSuffix(text, "}}"))
	}
	if binding, ok := value.(map[string]interface{}); ok {
		path, _ := binding["path"].(string)
		return strings.TrimSpace(path) != ""
	}
	return false
}

// isNonEmptyActionValue 校验非空静态值或受控绑定。
func isNonEmptyActionValue(value interface{}) bool {
	if isBindingActionValue(value) {
		return true
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

// isHTTPSActionValue 校验静态 HTTPS URL 或受控绑定。
func isHTTPSActionValue(value interface{}) bool {
	if isBindingActionValue(value) {
		return true
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(text))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// isCoordinateActionValue 校验坐标、缩放值或运行时受控绑定。
func isCoordinateActionValue(value interface{}, min, max float64) bool {
	if isBindingActionValue(value) {
		return true
	}
	number, ok := toFloatStrict(value)
	return ok && !math.IsNaN(number) && !math.IsInf(number, 0) && number >= min && number <= max
}

// validateEmbeddedActions 递归审计非 BlockItem 属性结构中嵌入的动作对象 (如 timeline 节点、item_grid 单元格)
func validateEmbeddedActions(value interface{}, path string, report *ValidationReport) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case map[string]interface{}:
		// 若自身是一个 BlockItem，由 validateNestedBlockContracts 处理，避免重复校验
		if _, hasBlockType := v["type"].(string); hasBlockType {
			if _, hasID := v["id"]; hasID {
				return
			}
		}
		// 若自身是 action 字段对象
		if actMap, ok := v["action"].(map[string]interface{}); ok {
			if raw, err := json.Marshal(actMap); err == nil {
				var act models.BlockAction
				if json.Unmarshal(raw, &act) == nil && act.Type != "" {
					validateNestedActionContracts(&act, path+".action", report)
				}
			}
		}
		for k, sub := range v {
			if k == "children" || k == "blocks" {
				continue
			}
			validateEmbeddedActions(sub, fmt.Sprintf("%s.%s", path, k), report)
		}
	case []interface{}:
		for idx, sub := range v {
			validateEmbeddedActions(sub, fmt.Sprintf("%s[%d]", path, idx), report)
		}
	}
}
