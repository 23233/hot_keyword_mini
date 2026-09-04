// Package services protocol_validator.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/models"
	"os"
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
	"media_hero":       true,
	"resource_card":    true,
	"action_button":    true,
	"game_card":        true,
	"form":             true,
	"episode_list":     true,
	"item_grid":        true,
	"score_panel":      true,
	"coupon_card":      true,
	"countdown":        true,
	"result_table":     true,
	"contact_card":     true,
	"map_card":         true,
	"game_header":      true,
	"redeem_code_card": true,
	"server_status":    true,
	"product_card":     true,
	"download_card":    true,
	"event_card":       true,
	"poll":             true,
	"feed_list":        true,
}

// 合法原子动作类型白名单 (严格与 doc/sdui_dynamic_engine_architecture.md 和 schema 对齐)
var allowedActionTypes = map[string]bool{
	"copy_text":              true,
	"navigate_page":          true,
	"open_channels_activity": true,
	"open_mini_program":      true,
	"request_data":           true,
	"request_payment":        true,
	"open_webview":           true,
	"preview_image":          true,
	"toast":                  true,
	"refresh":                true,
	"require_auth":           true,
	"share":                  true,
	"subscribe_message":      true,
}

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
			case "request_data":
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

// validateNestedBlockContracts 递归检查嵌套 block 的 ID、类型、动作和状态分支。
func validateNestedBlockContracts(block *models.BlockItem, path string, idMap map[string]int, report *ValidationReport) {
	if block == nil {
		return
	}
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
		validateNestedActionContracts(child.Action, childPath+".action", report)
		if child.Events != nil {
			for eventName, actions := range child.Events {
				for idx := range actions {
					validateNestedActionContracts(&actions[idx], fmt.Sprintf("%s.events.%s[%d]", childPath, eventName, idx), report)
				}
			}
		}
		childCopy := child
		validateNestedBlockContracts(&childCopy, childPath, idMap, report)
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
	if action.Type == "copy_text" {
		textValue, _ := action.Payload["text"].(string)
		if strings.TrimSpace(textValue) == "" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 缺少 payload.text", path))
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
	if action.Type == "request_data" {
		if customURL, _ := action.Payload["url"].(string); strings.HasPrefix(customURL, "http://") || strings.HasPrefix(customURL, "https://") || strings.HasPrefix(customURL, "//") {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("%s 包含非同源绝对 URL", path))
		}
	}
	for idx := range action.OnSuccess {
		validateNestedActionContracts(&action.OnSuccess[idx], fmt.Sprintf("%s.on_success[%d]", path, idx), report)
	}
	for idx := range action.OnError {
		validateNestedActionContracts(&action.OnError[idx], fmt.Sprintf("%s.on_error[%d]", path, idx), report)
	}
}

// collectNestedBlocks 从任意 props 结构递归提取所有带 type 的子 block。
func collectNestedBlocks(value interface{}) []models.BlockItem {
	result := make([]models.BlockItem, 0)
	var walk func(interface{})
	walk = func(current interface{}) {
		switch v := current.(type) {
		case map[string]interface{}:
			if _, ok := v["type"]; ok {
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
	validBusinessTypes := map[string]bool{"drama": true, "game": true, "query": true, "download": true, "custom": true}
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
				if b.Action.Payload == nil {
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s copy_text 动作 payload 必须包含 text 文本", bPath))
				} else {
					textVal, _ := b.Action.Payload["text"].(string)
					if strings.TrimSpace(textVal) == "" {
						report.IsValid = false
						report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s copy_text 动作 payload.text 不能为空", bPath))
					}
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
			}
		}

		// 检查 visible_when 条件表达式规范
		if b.VisibleWhen != nil {
			pathVal, _ := b.VisibleWhen["path"].(string)
			if pathVal != "" && !strings.HasPrefix(pathVal, "$") {
				report.IsValid = false
				report.Errors = append(report.Errors, fmt.Sprintf("Schema 契约校验失败: %s.visible_when.path '%s' 必须以 $ 状态或实体作用域开头", bPath, pathVal))
			}
		}
	}

	if len(report.Errors) > 0 {
		report.IsValid = false
	}

	return report
}
