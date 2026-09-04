// Package services protocol_patch.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/models"
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

	// 强制处于草稿状态，防止越权发布
	draft.Status = "draft"

	var blocks []models.BlockItem
	if draft.Blocks != "" {
		_ = json.Unmarshal([]byte(draft.Blocks), &blocks)
	}

	// 逐项应用补丁
	for idx, op := range ops {
		switch op.Op {
		case "replace":
			switch op.Path {
			case "/title":
				if s, ok := op.Value.(string); ok {
					draft.Title = s
				}
			case "/theme":
				if s, ok := op.Value.(string); ok {
					draft.Theme = s
				}
			case "/accent_color":
				if s, ok := op.Value.(string); ok {
					draft.AccentColor = s
				}
			case "/business_type":
				if s, ok := op.Value.(string); ok {
					draft.BusinessType = s
				}
			case "/status":
				// 禁止外部通过 patch 绕过发布权限直接改成 published
				if s, ok := op.Value.(string); ok && s == "published" {
					return nil, errors.New("禁止通过 patch 补丁将状态变更为 published，必须通过 release 权限显式调用发布接口")
				}
			case "/blocks":
				rawJSON, err := json.Marshal(op.Value)
				if err == nil {
					draft.Blocks = string(rawJSON)
					_ = json.Unmarshal(rawJSON, &blocks)
				}
			default:
				return nil, fmt.Errorf("补丁[%d] 不支持的目标路径: %s", idx, op.Path)
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
			for _, b := range blocks {
				if b.ID != removeID {
					newBlocks = append(newBlocks, b)
				}
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
	if err := sduiService.SaveDraft(draft); err != nil {
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
