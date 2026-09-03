// Package services protocol_patch.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
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

// PatchDynamicPage 对指定动态页面应用一组原子 JSON 补丁操作并持久化更新
func PatchDynamicPage(appID, pageID string, ops []PatchOp) (*models.DynamicPage, error) {
	if appID == "" || pageID == "" {
		return nil, errors.New("app_id 与 page_id 不能为空")
	}

	if len(ops) == 0 {
		return nil, errors.New("补丁操作列表不能为空")
	}

	var page *models.DynamicPage
	sduiService := NewSDUIService()

	if db.Mysql != nil {
		p, err := sduiService.GetRawPage(appID, pageID)
		if err != nil {
			return nil, fmt.Errorf("未找到目标页面: %w", err)
		}
		page = p
	} else {
		// 内存模式兜底
		page = &models.DynamicPage{
			AppID:        appID,
			PageID:       pageID,
			Title:        "默认页面",
			BusinessType: "drama",
			Theme:        "dark_glass",
			AccentColor:  "#FF9F0A",
			Blocks:       "[]",
			Status:       "published",
		}
	}

	var blocks []models.BlockItem
	if page.Blocks != "" {
		_ = json.Unmarshal([]byte(page.Blocks), &blocks)
	}

	// 逐项应用补丁
	for idx, op := range ops {
		switch op.Op {
		case "replace":
			switch op.Path {
			case "/title":
				if s, ok := op.Value.(string); ok {
					page.Title = s
				}
			case "/theme":
				if s, ok := op.Value.(string); ok {
					page.Theme = s
				}
			case "/accent_color":
				if s, ok := op.Value.(string); ok {
					page.AccentColor = s
				}
			case "/business_type":
				if s, ok := op.Value.(string); ok {
					page.BusinessType = s
				}
			case "/status":
				if s, ok := op.Value.(string); ok {
					page.Status = s
				}
			case "/blocks":
				rawJSON, err := json.Marshal(op.Value)
				if err == nil {
					page.Blocks = string(rawJSON)
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
	page.Blocks = string(updatedBlocksJSON)

	// 若数据库可用，持久化保存并沉淀版本快照
	if db.Mysql != nil {
		if err := sduiService.SavePage(page); err != nil {
			return nil, fmt.Errorf("持久化打补丁失败: %w", err)
		}
	}

	return page, nil
}
