// Package services protocol_validator.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/models"
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

// 合法原子积木类型白名单
var allowedBlockTypes = map[string]bool{
	"media_hero":    true,
	"resource_card": true,
	"action_button": true,
	"notice":        true,
	"game_card":     true,
	"form":          true,
	"episode_list":  true,
	"item_grid":     true,
	"timeline":      true,
}

// 合法原子动作类型白名单
var allowedActionTypes = map[string]bool{
	"copy_text":              true,
	"navigate_page":          true,
	"open_channels_activity": true,
	"open_mini_program":      true,
	"request_data":           true,
	"open_webview":           true,
	"preview_image":          true,
	"toast":                  true,
	"refresh":                true,
	"require_auth":           true,
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
					report.IsValid = false
					report.Errors = append(report.Errors, fmt.Sprintf("%s 跨小程序跳转缺少目标 target_app_id", pathPrefix))
					report.Suggestions = append(report.Suggestions, "请配置跳转目标小程序的 AppID")
				}
			case "request_data":
				endpoint, _ := payload["endpoint"].(string)
				if endpoint == "" {
					report.Warnings = append(report.Warnings, fmt.Sprintf("%s request_data 未声明已登记的 endpoint，将尝试默认端点", pathPrefix))
				}
			}
		}
	}

	if len(report.Errors) > 0 {
		report.IsValid = false
	}

	return report
}
