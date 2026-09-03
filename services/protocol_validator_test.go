// Package services protocol_validator_test.go
package services

import (
	"hot_keyword/models"
	"testing"
)

// TestProtocolValidation_Valid 测试合规协议校验通过
func TestProtocolValidation_Valid(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "绝地突围",
		BusinessType: "game",
		Blocks: `[
			{
				"id": "hero_1",
				"type": "media_hero",
				"props": { "title": "热播精选" },
				"action": {
					"type": "open_channels_activity",
					"payload": {
						"feed_id": "export/123",
						"finder_user_name": "sph_official"
					}
				}
			}
		]`,
	}

	report := ValidateDynamicPage(page)
	if !report.IsValid {
		t.Fatalf("合规协议应当校验通过，但报错误: %v", report.Errors)
	}
	if report.BlockCount != 1 {
		t.Fatalf("积木数量应为 1，实际为 %d", report.BlockCount)
	}
}

// TestProtocolValidation_InvalidIDConflict 测试积木 ID 重复冲突拦截
func TestProtocolValidation_InvalidIDConflict(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "重复ID测试",
		BusinessType: "drama",
		Blocks: `[
			{ "id": "dup_block", "type": "notice" },
			{ "id": "dup_block", "type": "action_button" }
		]`,
	}

	report := ValidateDynamicPage(page)
	if report.IsValid {
		t.Fatalf("重复积木 ID 应当被校验器拦截判定为非法")
	}
	if len(report.Errors) == 0 {
		t.Fatalf("应当包含明确的错误描述")
	}
}

// TestProtocolValidation_MissingActionParam 测试动作关键入参缺失拦截
func TestProtocolValidation_MissingActionParam(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "动作参数缺失测试",
		BusinessType: "drama",
		Blocks: `[
			{
				"id": "block_1",
				"type": "action_button",
				"action": {
					"type": "open_channels_activity",
					"payload": {}
				}
			}
		]`,
	}

	report := ValidateDynamicPage(page)
	if report.IsValid {
		t.Fatalf("缺失 feed_id 的视频号跳转应当被判定为非法")
	}
}

// TestProtocolPatch 测试 JSON Patch 打补丁操作
func TestProtocolPatch(t *testing.T) {
	ops := []PatchOp{
		{
			Op:    "replace",
			Path:  "/title",
			Value: "打补丁后的新标题",
		},
		{
			Op:   "add_block",
			Path: "/blocks",
			Value: map[string]interface{}{
				"id":   "patched_block_1",
				"type": "notice",
				"props": map[string]interface{}{
					"text": "补丁插入的公告",
				},
			},
		},
	}

	patchedPage, err := PatchDynamicPage("wx516563cfe994bbc6", "home", ops)
	if err != nil {
		t.Fatalf("应用补丁失败: %v", err)
	}

	if patchedPage.Title != "打补丁后的新标题" {
		t.Fatalf("补丁未能成功替换标题")
	}
}
