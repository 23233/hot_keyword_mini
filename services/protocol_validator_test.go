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

// TestProtocolValidation_CopyTextVariants 测试 copy_text 动作的多形态合规校验 (支持 text/content/path/对象绑定)
func TestProtocolValidation_CopyTextVariants(t *testing.T) {
	// 1. 使用 path 字段动态绑定 (如官方游戏模板中的 $result.code)
	pageWithPath := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "复制测试-Path",
		BusinessType: "game",
		Blocks: `[
			{
				"id": "btn_copy_path",
				"type": "action_button",
				"props": { "text": "复制" },
				"action": {
					"type": "copy_text",
					"payload": {
						"path": "$result.code",
						"toast": "兑换码已复制"
					}
				}
			}
		]`,
	}
	report := ValidateDynamicPage(pageWithPath)
	if !report.IsValid {
		t.Fatalf("copy_text 携带 path 载荷应当校验通过，但报错误: %v", report.Errors)
	}

	// 2. 使用 content 字段
	pageWithContent := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "复制测试-Content",
		BusinessType: "drama",
		Blocks: `[
			{
				"id": "btn_copy_content",
				"type": "action_button",
				"props": { "text": "复制网盘" },
				"action": {
					"type": "copy_text",
					"payload": {
						"content": "https://pan.quark.cn/s/test 提取码: 8888"
					}
				}
			}
		]`,
	}
	report = ValidateDynamicPage(pageWithContent)
	if !report.IsValid {
		t.Fatalf("copy_text 携带 content 载荷应当校验通过，但报错误: %v", report.Errors)
	}

	// 3. 使用 text 为对象形式绑定 { "path": "$item.code" }
	pageWithObj := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "复制测试-Object",
		BusinessType: "game",
		Blocks: `[
			{
				"id": "btn_copy_obj",
				"type": "action_button",
				"props": { "text": "复制对象" },
				"action": {
					"type": "copy_text",
					"payload": {
						"text": { "path": "$item.code" }
					}
				}
			}
		]`,
	}
	report = ValidateDynamicPage(pageWithObj)
	if !report.IsValid {
		t.Fatalf("copy_text 携带对象形式 text 应当校验通过，但报错误: %v", report.Errors)
	}

	// 4. 完全缺失复制文本/路径应当拦截
	pageEmpty := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "复制测试-空",
		BusinessType: "game",
		Blocks: `[
			{
				"id": "btn_copy_empty",
				"type": "action_button",
				"props": { "text": "复制空" },
				"action": {
					"type": "copy_text",
					"payload": {}
				}
			}
		]`,
	}
	report = ValidateDynamicPage(pageEmpty)
	if report.IsValid {
		t.Fatalf("空 payload 的 copy_text 动作应当被校验拦截判定为非法")
	}
}

// TestProtocolValidation_RequestPayment 测试 request_payment 动作合规与缺少 sku 拦截
func TestProtocolValidation_RequestPayment(t *testing.T) {
	// 1. 合规支付动作配置
	validPaymentPage := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "支付测试",
		BusinessType: "drama",
		Blocks: `[
			{
				"id": "btn_pay_1",
				"type": "action_button",
				"props": { "text": "立即开通" },
				"action": {
					"type": "request_payment",
					"payload": {
						"sku": "vip_monthly_001"
					}
				}
			}
		]`,
	}
	report := ValidateDynamicPage(validPaymentPage)
	if !report.IsValid {
		t.Fatalf("合规 request_payment 应当校验通过，但报错误: %v", report.Errors)
	}

	// 2. 缺失 sku 应当拦截
	invalidPaymentPage := &models.DynamicPage{
		AppID:        "wx516563cfe994bbc6",
		PageID:       "home",
		Title:        "支付缺失SKU测试",
		BusinessType: "drama",
		Blocks: `[
			{
				"id": "btn_pay_2",
				"type": "action_button",
				"props": { "text": "立即开通" },
				"action": {
					"type": "request_payment",
					"payload": {}
				}
			}
		]`,
	}
	report = ValidateDynamicPage(invalidPaymentPage)
	if report.IsValid {
		t.Fatalf("缺失 sku 的 request_payment 应当被拦截判定为非法")
	}
}
