// Package services protocol_patch.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/models"
	"strings"
	"time"
)

// PatchOp 局部受控打补丁操作
type PatchOp struct {
	// 操作类型: replace / add_block / remove_block
	Op string `json:"op"`
	// 操作目标路径 (如 /title, /theme, /accent_color, /status, /blocks)
	Path string `json:"path"`
	// 替换或新增的值
	Value interface{} `json:"value"`
}

// PatchDynamicPageDraft 对指定页面的草稿应用一组原子 JSON 补丁操作并持久化到草稿表
// 强制限定在草稿态进行，绝不允许直接修改线上已发布版本
func PatchDynamicPageDraft(appID, pageID string, ops []PatchOp) (*models.DynamicPageDraft, error) {
	return PatchDynamicPageDraftWithRevision(appID, pageID, 0, ops)
}

// PatchDynamicPageDraftWithRevision 基于调用方已读取的草稿版本执行原子补丁。
func PatchDynamicPageDraftWithRevision(appID, pageID string, expectedRevision int, ops []PatchOp) (*models.DynamicPageDraft, error) {
	return PatchDynamicPageDraftWithRevisionBy(appID, pageID, expectedRevision, "admin", ops)
}

// PatchDynamicPageDraftWithRevisionBy 使用指定操作人基于 revision 执行原子补丁。
func PatchDynamicPageDraftWithRevisionBy(appID, pageID string, expectedRevision int, operator string, ops []PatchOp) (*models.DynamicPageDraft, error) {
	if appID == "" || pageID == "" {
		return nil, errors.New("app_id 与 page_id 不能为空")
	}

	if len(ops) == 0 {
		return nil, errors.New("补丁操作列表不能为空")
	}

	sduiService := NewSDUIService()
	draft, err := sduiService.GetRawDraft(appID, pageID)
	if err != nil {
		return nil, fmt.Errorf("未找到或创建草稿失败: %w", err)
	}
	if expectedRevision > 0 && draft.Revision != expectedRevision {
		return nil, fmt.Errorf("草稿版本冲突: 期望 v%d，当前为 v%d，请重新调用 page.get", expectedRevision, draft.Revision)
	}
	if strings.TrimSpace(operator) == "" {
		operator = "admin"
	}
	draft.UpdatedBy = operator

	// 强制处于草稿状态，防止越权发布
	draft.Status = "draft"

	var blocks []models.BlockItem
	if draft.Blocks != "" {
		_ = json.Unmarshal([]byte(draft.Blocks), &blocks)
	}

	// 逐项应用补丁
	for idx, op := range ops {
		if op.Op == "add_block" || strings.HasPrefix(op.Path, "/blocks") {
			raw, err := json.Marshal(op.Value)
			if err != nil {
				return nil, fmt.Errorf("补丁[%d] 无法编码: %w", idx, err)
			}
			if err := ValidateSDUIStyleJSON(raw); err != nil {
				return nil, fmt.Errorf("补丁[%d]: %w", idx, err)
			}
		}
		switch op.Op {
		case "replace":
			switch op.Path {
			case "/title":
				s, ok := op.Value.(string)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] /title 必须是字符串", idx)
				}
				draft.Title = s
			case "/theme":
				s, ok := op.Value.(string)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] /theme 必须是字符串", idx)
				}
				draft.Theme = s
			case "/accent_color":
				s, ok := op.Value.(string)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] /accent_color 必须是字符串", idx)
				}
				draft.AccentColor = s
			case "/business_type":
				s, ok := op.Value.(string)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] /business_type 必须是字符串", idx)
				}
				draft.BusinessType = s
			case "/intent", "/keyword", "/source", "/campaign_id":
				value, ok := op.Value.(string)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] %s 必须是字符串", idx, op.Path)
				}
				switch op.Path {
				case "/intent":
					draft.Intent = value
				case "/keyword":
					draft.Keyword = value
				case "/source":
					draft.Source = value
				case "/campaign_id":
					draft.CampaignID = value
				}
			case "/require_auth":
				value, ok := op.Value.(bool)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] /require_auth 必须是布尔值", idx)
				}
				draft.RequireAuth = value
			case "/share_config":
				if value, ok := op.Value.(string); ok {
					draft.ShareConfig = value
				} else {
					rawValue, err := json.Marshal(op.Value)
					if err != nil {
						return nil, fmt.Errorf("补丁[%d] /share_config 格式无效: %w", idx, err)
					}
					draft.ShareConfig = string(rawValue)
				}
			case "/expires_at":
				if op.Value == nil {
					draft.ExpiresAt = nil
					break
				}
				value, ok := op.Value.(string)
				if !ok {
					return nil, fmt.Errorf("补丁[%d] /expires_at 必须是 RFC3339 字符串或 null", idx)
				}
				parsed, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return nil, fmt.Errorf("补丁[%d] /expires_at 必须是 RFC3339 时间: %w", idx, err)
				}
				draft.ExpiresAt = &parsed
			case "/status":
				// 禁止外部通过 patch 绕过发布权限直接改成 published
				return nil, errors.New("禁止通过 patch 修改 status，草稿状态由 sdui.page.create、sdui.page.publish 统一管理")
			case "/blocks":
				rawJSON, err := json.Marshal(op.Value)
				if err != nil {
					return nil, fmt.Errorf("补丁[%d] /blocks 格式无效: %w", idx, err)
				}
				var nextBlocks []models.BlockItem
				if err := json.Unmarshal(rawJSON, &nextBlocks); err != nil {
					return nil, fmt.Errorf("补丁[%d] /blocks 必须是 BlockItem 数组: %w", idx, err)
				}
				draft.Blocks = string(rawJSON)
				blocks = nextBlocks
			default:
				if strings.HasPrefix(op.Path, "/blocks/") {
					blockID := strings.TrimPrefix(op.Path, "/blocks/")
					if blockID == "" || strings.Contains(blockID, "/") {
						return nil, fmt.Errorf("补丁[%d] blocks 路径必须使用 /blocks/{block_id}", idx)
					}
					rawBlock, err := json.Marshal(op.Value)
					if err != nil {
						return nil, fmt.Errorf("补丁[%d] block 格式无效: %w", idx, err)
					}
					var replacement models.BlockItem
					if err := json.Unmarshal(rawBlock, &replacement); err != nil || strings.TrimSpace(replacement.Type) == "" {
						return nil, fmt.Errorf("补丁[%d] block value 必须是完整 BlockItem", idx)
					}
					replacement.ID = blockID
					found := false
					for blockIndex := range blocks {
						if blocks[blockIndex].ID == blockID {
							blocks[blockIndex] = replacement
							found = true
							break
						}
					}
					if !found {
						return nil, fmt.Errorf("补丁[%d] 未找到积木 ID: %s", idx, blockID)
					}
				} else {
					return nil, fmt.Errorf("补丁[%d] 不支持的目标路径: %s", idx, op.Path)
				}
			}

		case "add_block":
			rawBlock, err := json.Marshal(op.Value)
			if err != nil {
				return nil, fmt.Errorf("补丁[%d] add_block 格式无效: %w", idx, err)
			}
			var item models.BlockItem
			if err := json.Unmarshal(rawBlock, &item); err != nil {
				return nil, fmt.Errorf("补丁[%d] 积木解析失败: %w", idx, err)
			}
			blocks = append(blocks, item)

		case "remove_block":
			removeID, ok := op.Value.(string)
			if !ok || removeID == "" {
				return nil, fmt.Errorf("补丁[%d] remove_block 必须指定积木 ID 字符串", idx)
			}
			newBlocks := make([]models.BlockItem, 0, len(blocks))
			removed := false
			for _, b := range blocks {
				if b.ID != removeID {
					newBlocks = append(newBlocks, b)
				} else {
					removed = true
				}
			}
			if !removed {
				return nil, fmt.Errorf("补丁[%d] 未找到积木 ID: %s", idx, removeID)
			}
			blocks = newBlocks

		default:
			return nil, fmt.Errorf("补丁[%d] 不支持的操作类型: %s", idx, op.Op)
		}
	}

	// 重新写回序列化积木
	updatedBlocksJSON, _ := json.Marshal(blocks)
	draft.Blocks = string(updatedBlocksJSON)

	// 持久化保存至草稿表 (绝对不修改线上 dynamic_pages 表)
	if err := sduiService.SaveDraftWithAudit(draft, operator, expectedRevision); err != nil {
		return nil, fmt.Errorf("持久化草稿补丁失败: %w", err)
	}

	return draft, nil
}

// PatchDynamicPage 兼容接口，内部安全打补丁至草稿
func PatchDynamicPage(appID, pageID string, ops []PatchOp) (*models.DynamicPage, error) {
	draft, err := PatchDynamicPageDraft(appID, pageID, ops)
	if err != nil {
		return nil, err
	}
	return &models.DynamicPage{
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
	}, nil
}
