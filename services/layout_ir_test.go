// Package services layout_ir_test.go
package services

import (
	"hot_keyword/models"
	"testing"
)

// TestDefaultDeviceParams_WechatBaseline 验证默认设备与微信开发者工具 iPhone 12/13 Pro 基准一致。
func TestDefaultDeviceParams_WechatBaseline(t *testing.T) {
	device := DefaultDeviceParams()
	if device.Name != "iPhone 12/13 Pro" || device.Width != 390 || device.Height != 844 || device.DPR != 3 {
		t.Fatalf("默认设备基准异常: %+v", device)
	}
	legacy := ResolveDeviceParams("iphone_16_pro")
	if legacy.Width != 393 || legacy.Height != 852 {
		t.Fatalf("旧设备兼容别名异常: %+v", legacy)
	}
}

// TestLayoutIR_DataBindingAndRepeat 测试同构 IR 的受控数据绑定求值与 Repeat 展开
func TestLayoutIR_DataBindingAndRepeat(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "home",
		Title:        "同构测试",
		BusinessType: "custom",
		Blocks: `[
			{
				"id": "block_img",
				"type": "image",
				"props": {
					"image_url": {"path": "$entity.cover_url"},
					"aspect_ratio": "16:9"
				}
			},
			{
				"id": "block_repeat_item",
				"type": "text",
				"repeat": {
					"items": [
						{"name": "选项A"},
						{"name": "选项B"},
						{"name": "选项C"}
					]
				},
				"props": {
					"text": {"path": "$item.name"}
				}
			}
		]`,
	}

	context := map[string]interface{}{
		"entity": map[string]interface{}{
			"cover_url": "https://img.example.com/cover.jpg",
		},
	}

	device := DefaultDeviceParams()
	ir, err := BuildPageLayoutIRWithContext(page, device, "normal", context)
	if err != nil {
		t.Fatalf("生成同构 IR 失败: %v", err)
	}

	// 1 image block + 3 repeat text blocks = 4 nodes
	if len(ir.Nodes) != 4 {
		t.Fatalf("预期展开后应有 4 个布局节点，实际得到 %d 个", len(ir.Nodes))
	}

	// 检查第一张图片的数据绑定解析与真实高度
	imgNode := ir.Nodes[0]
	if imgNode.Type != "image" {
		t.Fatalf("第 1 个节点类型应为 image，实际为 %s", imgNode.Type)
	}
	expectedImgH := int(float64(device.Width-32) * 9.0 / 16.0)
	if imgNode.BoundingBox.Height != expectedImgH {
		t.Fatalf("16:9 图片自适应高度应为 %d，实际为 %d", expectedImgH, imgNode.BoundingBox.Height)
	}

	// 检查 Repeat 展开后的文本
	nodeA := ir.Nodes[1]
	if nodeA.ID != "block_repeat_item_0" || nodeA.TextSummary != "选项A" {
		t.Fatalf("Repeat 第 0 项解析异常: ID=%s, TextSummary=%s", nodeA.ID, nodeA.TextSummary)
	}
	nodeB := ir.Nodes[2]
	if nodeB.ID != "block_repeat_item_1" || nodeB.TextSummary != "选项B" {
		t.Fatalf("Repeat 第 1 项解析异常: ID=%s, TextSummary=%s", nodeB.ID, nodeB.TextSummary)
	}
	nodeC := ir.Nodes[3]
	if nodeC.ID != "block_repeat_item_2" || nodeC.TextSummary != "选项C" {
		t.Fatalf("Repeat 第 2 项解析异常: ID=%s, TextSummary=%s", nodeC.ID, nodeC.TextSummary)
	}
}

// TestLayoutIR_ConditionEvaluation 测试受控条件求值 visible_when
func TestLayoutIR_ConditionEvaluation(t *testing.T) {
	ctx := map[string]interface{}{
		"entity": map[string]interface{}{
			"is_vip": true,
			"score":  88,
			"role":   "admin",
		},
		"query": map[string]interface{}{
			"channel": "search",
		},
	}

	// 1. eq
	condEq := map[string]interface{}{
		"eq": []interface{}{
			map[string]interface{}{"path": "$entity.role"},
			"admin",
		},
	}
	if !EvaluateCondition(condEq, ctx) {
		t.Fatalf("role == admin 应为 true")
	}

	// 2. and + gte
	condAnd := map[string]interface{}{
		"and": []interface{}{
			map[string]interface{}{
				"eq": []interface{}{
					map[string]interface{}{"path": "$entity.is_vip"},
					true,
				},
			},
			map[string]interface{}{
				"gte": []interface{}{
					map[string]interface{}{"path": "$entity.score"},
					60,
				},
			},
		},
	}
	if !EvaluateCondition(condAnd, ctx) {
		t.Fatalf("VIP 且 score >= 60 应为 true")
	}

	// 3. not
	condNot := map[string]interface{}{
		"not": map[string]interface{}{
			"eq": []interface{}{
				map[string]interface{}{"path": "$entity.role"},
				"guest",
			},
		},
	}
	if !EvaluateCondition(condNot, ctx) {
		t.Fatalf("not guest 应为 true")
	}
}

// TestLayoutIR_BlockStateVariants 测试块级 loading/empty/error 状态切换
func TestLayoutIR_BlockStateVariants(t *testing.T) {
	block := models.BlockItem{
		ID:   "hero_main",
		Type: "media_hero",
		Props: map[string]interface{}{
			"title": "默认展示标题",
		},
		Loading: &models.BlockItem{
			ID:   "hero_loading",
			Type: "skeleton",
			Props: map[string]interface{}{
				"rows": 4,
			},
		},
		Empty: &models.BlockItem{
			ID:   "hero_empty",
			Type: "empty",
			Props: map[string]interface{}{
				"title": "暂无资源",
			},
		},
	}

	// normal 态
	resolvedNormal := resolveBlockStateVariant(block, "normal")
	if resolvedNormal.Type != "media_hero" {
		t.Fatalf("normal 态应为 media_hero")
	}

	// loading 态
	resolvedLoading := resolveBlockStateVariant(block, "loading")
	if resolvedLoading.Type != "skeleton" {
		t.Fatalf("loading 态应切换为 skeleton 骨架屏，实际为 %s", resolvedLoading.Type)
	}

	// empty 态
	resolvedEmpty := resolveBlockStateVariant(block, "empty")
	if resolvedEmpty.Type != "empty" {
		t.Fatalf("empty 态应切换为 empty 块，实际为 %s", resolvedEmpty.Type)
	}
}

// TestLayoutIR_NestedChildrenPreserveBinding 验证容器子块递归生成且不会提前消费 $item 绑定。
func TestLayoutIR_NestedChildrenPreserveBinding(t *testing.T) {
	page := &models.DynamicPage{
		PageID: "nested",
		Title:  "嵌套布局",
		Blocks: `[{
			"id":"container",
			"type":"container",
			"props":{"children":[{
				"id":"child_text",
				"type":"text",
				"repeat":{"items":[{"name":"甲"},{"name":"乙"}]},
				"props":{"text":{"path":"$item.name"}}
			}]}
		}]`,
	}
	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成嵌套 IR 失败: %v", err)
	}
	if len(ir.Nodes) != 1 || len(ir.Nodes[0].Children) != 2 {
		t.Fatalf("容器应递归生成 2 个子节点，实际顶层=%d 子节点=%d", len(ir.Nodes), len(ir.Nodes[0].Children))
	}
	if ir.Nodes[0].Children[0].TextSummary != "甲" || ir.Nodes[0].Children[1].TextSummary != "乙" {
		t.Fatalf("子节点 $item 绑定丢失: %+v", ir.Nodes[0].Children)
	}
}
