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

	// 1.1 Schema 接受的 path + eq 简写必须与数组形式同构。
	if !EvaluateCondition(map[string]interface{}{"path": "$entity.is_vip", "eq": true}, ctx) {
		t.Fatalf("path + eq 简写应为 true")
	}
	if EvaluateCondition(map[string]interface{}{"path": "$entity.is_vip", "eq": false}, ctx) {
		t.Fatalf("path + eq 简写的反向条件应为 false")
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

// TestLayoutIR_RepeatConditionKeepsItemContext 验证循环块在 IR 条件求值时保留各自的 $item 上下文。
func TestLayoutIR_RepeatConditionKeepsItemContext(t *testing.T) {
	page := &models.DynamicPage{
		PageID: "repeat_condition",
		Title:  "循环条件",
		Blocks: `[
			{
				"id":"repeat_text",
				"type":"text",
				"repeat":{"items":[{"name":"显示","enabled":true},{"name":"隐藏","enabled":false}]},
				"visible_when":{"eq":[{"path":"$item.enabled"},true]},
				"props":{"text":{"path":"$item.name"}}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", nil)
	if err != nil {
		t.Fatalf("生成循环条件 IR 失败: %v", err)
	}
	if len(ir.Nodes) != 2 {
		t.Fatalf("循环块应展开为 2 个节点，实际为 %d", len(ir.Nodes))
	}
	if !ir.Nodes[0].Visible || ir.Nodes[1].Visible {
		t.Fatalf("循环块条件未按各自 $item 计算: %+v", ir.Nodes)
	}
	if _, ok := ir.Nodes[0].Props["_repeat_item"]; !ok {
		t.Fatal("展开后的 IR 节点必须保留 _repeat_item，供客户端继续求值动作与条件")
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

	block.Error = &models.BlockItem{ID: "hero_error", Type: "empty"}
	resolvedExpired := resolveBlockStateVariant(block, "expired")
	if resolvedExpired.Type != "empty" {
		t.Fatalf("expired 态应切换为 error 块，实际为 %s", resolvedExpired.Type)
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

// TestLayoutIR_OmittedDollarBinding 验证省略 $ 作用域的受控路径（如 entity.title 与 {"path": "entity.cover_url"}）在 Layout IR 中能同构解析。
func TestLayoutIR_OmittedDollarBinding(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "omitted_dollar",
		Title:        "同构测试",
		BusinessType: "custom",
		Blocks: `[
			{
				"id": "block_img",
				"type": "image",
				"props": {
					"image_url": {"path": "entity.cover_url"},
					"title": "entity.title",
					"sub_title": "$entity.sub_title",
					"aspect_ratio": "16:9"
				}
			}
		]`,
	}

	context := map[string]interface{}{
		"entity": map[string]interface{}{
			"cover_url": "https://img.example.com/cover2.jpg",
			"title":     "测试标题",
			"sub_title": "测试副标题",
		},
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", context)
	if err != nil {
		t.Fatalf("生成 IR 失败: %v", err)
	}

	if len(ir.Nodes) != 1 {
		t.Fatalf("预期 1 个节点，实际: %d", len(ir.Nodes))
	}

	node := ir.Nodes[0]
	if node.Props["image_url"] != "https://img.example.com/cover2.jpg" {
		t.Fatalf("省略 $ 的 {path: entity.cover_url} 解析失败，实际: %v", node.Props["image_url"])
	}
	if node.Props["title"] != "测试标题" {
		t.Fatalf("省略 $ 的直接路径字符串 entity.title 解析失败，实际: %v", node.Props["title"])
	}
	if node.Props["sub_title"] != "测试副标题" {
		t.Fatalf("带 $ 的路径字符串 $entity.sub_title 解析失败，实际: %v", node.Props["sub_title"])
	}
}

// TestLayoutIR_ContextIsNotMutated 验证 IR 构建不会向调用方上下文写入页面元数据或作用域别名。
func TestLayoutIR_ContextIsNotMutated(t *testing.T) {
	context := map[string]interface{}{
		"entity": map[string]interface{}{"title": "外部上下文"},
	}
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "context_isolation",
		Title:  "内部页面标题",
		Blocks: `[{"id":"title","type":"text","props":{"content":{"path":"$page.title"}}}]`,
	}

	if _, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", context); err != nil {
		t.Fatalf("生成 IR 失败: %v", err)
	}
	if _, exists := context["$page"]; exists {
		t.Fatalf("IR 构建不应污染调用方上下文: $page")
	}
	if _, exists := context["page"]; exists {
		t.Fatalf("IR 构建不应污染调用方上下文: page")
	}
	if _, exists := context["$entity"]; exists {
		t.Fatalf("IR 构建不应写入作用域别名: $entity")
	}
}

// TestLayoutIR_CompareLooseEqual 测试同构宽松等值比较与空值防护
func TestLayoutIR_CompareLooseEqual(t *testing.T) {
	ctx := map[string]interface{}{
		"entity": map[string]interface{}{
			"is_active":  true,
			"status_num": 1,
			"str_num":    "1",
			"empty_val":  nil,
		},
	}

	// 1. 布尔与字符串 true 比较
	condBoolStr := map[string]interface{}{
		"eq": []interface{}{
			map[string]interface{}{"path": "$entity.is_active"},
			"true",
		},
	}
	if !EvaluateCondition(condBoolStr, ctx) {
		t.Fatalf("is_active(true) 与 'true' 应判定为相等")
	}

	// 2. 数值与数字字符串比较
	condNumStr := map[string]interface{}{
		"eq": []interface{}{
			map[string]interface{}{"path": "$entity.status_num"},
			"1",
		},
	}
	if !EvaluateCondition(condNumStr, ctx) {
		t.Fatalf("status_num(1) 与 '1' 应判定为相等")
	}

	// 3. 空值与非空值比较应为 false，不可因 sprintf 假阳性
	condNilFalse := map[string]interface{}{
		"eq": []interface{}{
			map[string]interface{}{"path": "$entity.empty_val"},
			false,
		},
	}
	if EvaluateCondition(condNilFalse, ctx) {
		t.Fatalf("nil 与 false 绝不能判定为相等")
	}

	// 4. 布尔字符串 false 必须按语义比较，不能使用 JavaScript 非空字符串真值。
	if !EvaluateCondition(map[string]interface{}{"eq": []interface{}{false, "false"}}, ctx) {
		t.Fatalf("false 与 'false' 应判定为相等")
	}

	// 5. 不可解析数值不能被降级成 0，否则会与客户端结果不一致。
	if EvaluateCondition(map[string]interface{}{"gt": []interface{}{"invalid-number", -1}}, ctx) {
		t.Fatal("不可解析数值比较应为 false")
	}
}

// TestLayoutIR_VisibleWhenAndRepeatInNode 验证 Layout IR 节点保留 VisibleWhen 与 Repeat 属性以供客户端响应式求值
func TestLayoutIR_VisibleWhenAndRepeatInNode(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_visible_repeat",
		Title:  "条件与循环",
		Blocks: `[
			{
				"id": "card_dynamic",
				"type": "custom",
				"visible_when": { "eq": [{ "path": "$state.is_open" }, true] },
				"repeat": { "items": [{"name": "A"}, {"name": "B"}] },
				"props": { "title": "动态积木" }
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成 IR 失败: %v", err)
	}

	if len(ir.Nodes) != 2 {
		t.Fatalf("预期 Repeat 展开 2 个节点，实际: %d", len(ir.Nodes))
	}

	node := ir.Nodes[0]
	if node.VisibleWhen == nil {
		t.Fatalf("IR 节点必须保留 VisibleWhen 原始表达式")
	}
}

// TestLayoutIR_EmptyRepeatHandling 验证 Repeat 数据源为空时不应下发幽灵积木
func TestLayoutIR_EmptyRepeatHandling(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_empty_repeat",
		Title:  "空循环",
		Blocks: `[
			{
				"id": "card_repeat_empty",
				"type": "custom",
				"repeat": { "path": "$entity.list" },
				"props": { "title": {"path": "$item.name"} }
			}
		]`,
	}

	// 1. 数据列表为空切片
	ctx := map[string]interface{}{
		"entity": map[string]interface{}{
			"list": []interface{}{},
		},
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", ctx)
	if err != nil {
		t.Fatalf("生成 IR 失败: %v", err)
	}

	if len(ir.Nodes) != 0 {
		t.Fatalf("Repeat 数据源为空列表时，节点数应为 0，杜绝幽灵积木，实际为: %d", len(ir.Nodes))
	}
}

// TestLayoutIR_GridMultiColumnsLayout 验证网格 (Grid) 积木在多列排布下的精确点阵与高度计算
func TestLayoutIR_GridMultiColumnsLayout(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_grid_layout",
		Title:  "网格布局测试",
		Blocks: `[
			{
				"id": "grid_container",
				"type": "grid",
				"props": {
					"columns": 2,
					"gap": "16rpx",
					"children": [
						{"id": "cell_1", "type": "action_button", "props": {"text": "按钮1"}},
						{"id": "cell_2", "type": "action_button", "props": {"text": "按钮2"}},
						{"id": "cell_3", "type": "action_button", "props": {"text": "按钮3"}},
						{"id": "cell_4", "type": "action_button", "props": {"text": "按钮4"}}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成网格 IR 失败: %v", err)
	}

	if len(ir.Nodes) != 1 {
		t.Fatalf("预期顶层 1 个网格节点，实际: %d", len(ir.Nodes))
	}

	gridNode := ir.Nodes[0]
	if len(gridNode.Children) != 4 {
		t.Fatalf("网格应包含 4 个子单元格，实际: %d", len(gridNode.Children))
	}

	// 验证列 0 和列 1 的 X 坐标差值大于 0
	cell0 := gridNode.Children[0]
	cell1 := gridNode.Children[1]
	cell2 := gridNode.Children[2]

	if cell1.BoundingBox.X <= cell0.BoundingBox.X {
		t.Fatalf("第 2 个 cell 的 X 坐标 (%d) 应大于第 1 个 cell (%d)", cell1.BoundingBox.X, cell0.BoundingBox.X)
	}
	// 验证第 1 行与第 2 行的 Y 坐标换行递增
	if cell2.BoundingBox.Y <= cell0.BoundingBox.Y {
		t.Fatalf("第 3 个 cell (第二行) 的 Y 坐标 (%d) 应大于第 1 个 cell (%d)", cell2.BoundingBox.Y, cell0.BoundingBox.Y)
	}
	// 验证单元格宽度并非全宽
	if cell0.BoundingBox.Width >= gridNode.BoundingBox.Width {
		t.Fatalf("两列网格的子项宽度 (%d) 应小于父容器宽度 (%d)", cell0.BoundingBox.Width, gridNode.BoundingBox.Width)
	}
}

// TestLayoutIR_StackOverlapLayout 验证堆叠重叠 (Overlap/ZStack) 模式下子节点原点重合与高度取最大值
func TestLayoutIR_StackOverlapLayout(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_overlap_layout",
		Title:  "重叠堆叠测试",
		Blocks: `[
			{
				"id": "stack_overlap",
				"type": "stack",
				"props": {
					"mode": "overlap",
					"children": [
						{"id": "bg_image", "type": "image", "props": {"height": 200}},
						{"id": "float_btn", "type": "action_button", "props": {"text": "浮动按钮"}}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成重叠 IR 失败: %v", err)
	}

	stackNode := ir.Nodes[0]
	if len(stackNode.Children) != 2 {
		t.Fatalf("应包含 2 个重叠子节点，实际: %d", len(stackNode.Children))
	}

	child0 := stackNode.Children[0]
	child1 := stackNode.Children[1]

	// 重叠模式下 X 坐标相同
	if child0.BoundingBox.X != child1.BoundingBox.X {
		t.Fatalf("重叠子节点的 X 坐标应一致，实际: %d vs %d", child0.BoundingBox.X, child1.BoundingBox.X)
	}
}

// TestLayoutIR_StackRowLayout 验证横排布局 (Row) 下水平推进与单行最大高度计算
func TestLayoutIR_StackRowLayout(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_row_layout",
		Title:  "横排布局测试",
		Blocks: `[
			{
				"id": "stack_row",
				"type": "container",
				"props": {
					"direction": "row",
					"children": [
						{"id": "btn_left", "type": "action_button", "props": {"text": "左"}},
						{"id": "btn_right", "type": "action_button", "props": {"text": "右"}}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成横排 IR 失败: %v", err)
	}

	rowNode := ir.Nodes[0]
	if len(rowNode.Children) != 2 {
		t.Fatalf("应包含 2 个横排子节点，实际: %d", len(rowNode.Children))
	}

	left := rowNode.Children[0]
	right := rowNode.Children[1]

	if right.BoundingBox.X <= left.BoundingBox.X {
		t.Fatalf("右侧按钮 X 坐标 (%d) 应大于左侧按钮 (%d)", right.BoundingBox.X, left.BoundingBox.X)
	}
}

// TestLayoutIR_NestedContainerYAccumulation 验证多层嵌套容器内兄弟积木按真实展开高度累加 Y 坐标，杜绝穿模重叠
func TestLayoutIR_NestedContainerYAccumulation(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_nested_y",
		Blocks: `[
			{
				"id": "outer_container",
				"type": "container",
				"props": {
					"direction": "column",
					"children": [
						{
							"id": "inner_card_1",
							"type": "custom",
							"props": {
								"image_url": "https://cdn.example.com/a.jpg",
								"title": "卡片1",
								"content": "正文说明",
								"btn_text": "操作1"
							}
						},
						{
							"id": "inner_card_2",
							"type": "action_button",
							"props": {
								"text": "操作2"
							}
						}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成嵌套容器 IR 失败: %v", err)
	}

	outer := ir.Nodes[0]
	if len(outer.Children) != 2 {
		t.Fatalf("应包含 2 个子节点，实际: %d", len(outer.Children))
	}

	child1 := outer.Children[0]
	child2 := outer.Children[1]

	// child2 的 Y 坐标必须严格大于等于 child1 的底部 (Y + Height)
	if child2.BoundingBox.Y < child1.BoundingBox.Y+child1.BoundingBox.Height {
		t.Fatalf("子节点2 Y 坐标 (%d) 发生了重叠穿模，应大于等于子节点1 底部 (%d)",
			child2.BoundingBox.Y, child1.BoundingBox.Y+child1.BoundingBox.Height)
	}
}

// TestLayoutIR_TabsLayoutWithHeader 验证 Tabs 积木顶部 TabBar 标签栏避让与高度计算
func TestLayoutIR_TabsLayoutWithHeader(t *testing.T) {
	page := &models.DynamicPage{
		AppID:  "wx_test",
		PageID: "test_tabs_header",
		Blocks: `[
			{
				"id": "tabs_1",
				"type": "tabs",
				"props": {
					"tabs": [
						{
							"key": "tab_1",
							"title": "精选",
							"blocks": [
								{
									"id": "btn_in_tab",
									"type": "action_button",
									"props": { "text": "标签内按钮" }
								}
							]
						}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成 Tabs IR 失败: %v", err)
	}

	tabsNode := ir.Nodes[0]
	if len(tabsNode.Children) != 1 {
		t.Fatalf("Tabs 应包含 1 个展开子节点，实际: %d", len(tabsNode.Children))
	}

	child := tabsNode.Children[0]
	// 子节点的 Y 坐标必须避开顶部标签栏 (46px)
	if child.BoundingBox.Y < tabsNode.BoundingBox.Y+46 {
		t.Fatalf("Tabs 子节点 Y 坐标 (%d) 未正确避让顶部 TabBar (应 >= %d)",
			child.BoundingBox.Y, tabsNode.BoundingBox.Y+46)
	}
}

// TestLayoutIR_TabsUseConfiguredActiveTab 验证 Tabs 的 IR 与客户端 default_active_key 选中规则一致。
func TestLayoutIR_TabsUseConfiguredActiveTab(t *testing.T) {
	page := &models.DynamicPage{
		PageID: "tabs_active",
		Title:  "标签选择",
		Blocks: `[
			{
				"id":"tabs_active_block",
				"type":"tabs",
				"props":{
					"default_active_key":"second",
					"tabs":[
						{"key":"first","title":"第一栏","blocks":[{"id":"first_text","type":"text","props":{"text":"第一栏内容"}}]},
						{"key":"second","title":"第二栏","blocks":[{"id":"second_text","type":"text","props":{"text":"第二栏内容"}}]}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", nil)
	if err != nil {
		t.Fatalf("生成 Tabs IR 失败: %v", err)
	}
	if len(ir.Nodes) != 1 || len(ir.Nodes[0].Children) != 1 {
		t.Fatalf("Tabs IR 子节点数量异常: %+v", ir.Nodes)
	}
	if ir.Nodes[0].Children[0].ID != "second_text" {
		t.Fatalf("Tabs IR 应展开默认选中的第二栏，实际为 %s", ir.Nodes[0].Children[0].ID)
	}
}

// TestLayoutIR_TabStateKeyIsStructural 验证 Tab 的 state key 不会被误解析为 $state 作用域。
func TestLayoutIR_TabStateKeyIsStructural(t *testing.T) {
	page := &models.DynamicPage{
		PageID: "tabs_state_key",
		Title:  "标签结构标识",
		Blocks: `[{"id":"tabs","type":"tabs","props":{"tabs":[{"key":"layout","title":"布局","blocks":[]},{"key":"state","title":"状态","blocks":[{"id":"state_text","type":"text","props":{"text":"状态内容"}}]}]}}]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{"state": map[string]interface{}{"enabled": true}})
	if err != nil {
		t.Fatalf("生成 Tabs IR 失败: %v", err)
	}
	if len(ir.Nodes) != 1 {
		t.Fatalf("Tabs IR 节点数量异常: %d", len(ir.Nodes))
	}
	tabs, ok := ir.Nodes[0].Props["tabs"].([]interface{})
	if !ok || len(tabs) != 2 {
		t.Fatalf("Tabs 配置丢失: %#v", ir.Nodes[0].Props["tabs"])
	}
	stateTab, ok := tabs[1].(map[string]interface{})
	if !ok || stateTab["key"] != "state" {
		t.Fatalf("Tab state key 被错误解析: %#v", tabs[1])
	}
}

// TestLayoutIR_InlineInterpolation 验证字符串内嵌 {{...}} 插值解析与数据流转
func TestLayoutIR_InlineInterpolation(t *testing.T) {
	ctx := map[string]interface{}{
		"$entity": map[string]interface{}{
			"username": "短剧宗师",
			"score":    99,
		},
		"result": map[string]interface{}{
			"code": "VIP666",
		},
	}

	res1 := ResolveBindingValue("欢迎玩家 {{entity.username}}，您的评分为 {{entity.score}} 分！", ctx)
	expected1 := "欢迎玩家 短剧宗师，您的评分为 99 分！"
	if res1 != expected1 {
		t.Fatalf("内嵌插值解析不匹配，预期: %s，实际: %v", expected1, res1)
	}

	res2 := ResolveBindingValue("兑换码: {{$result.code}}", ctx)
	expected2 := "兑换码: VIP666"
	if res2 != expected2 {
		t.Fatalf("兑换码插值解析不匹配，预期: %s，实际: %v", expected2, res2)
	}
}

// TestCalculateAdaptiveBlockHeightTypedSlices 验证真实 Go 切片绑定能驱动资讯与会员块高度。
func TestCalculateAdaptiveBlockHeightTypedSlices(t *testing.T) {
	feedHeight, _ := CalculateAdaptiveBlockHeight(&models.BlockItem{Type: "article_feed"}, map[string]interface{}{
		"items_path": []map[string]interface{}{{"id": 1}, {"id": 2}, {"id": 3}},
		"layout":     "list",
		"limit":      8,
	}, 358)
	if feedHeight != 258 {
		t.Fatalf("文章流应按真实绑定条目计算高度，实际: %d", feedHeight)
	}

	planHeight, _ := CalculateAdaptiveBlockHeight(&models.BlockItem{Type: "membership_plan_list"}, map[string]interface{}{
		"items_path": []models.MembershipLevel{{Level: 1}, {Level: 2}, {Level: 3}},
	}, 358)
	if planHeight != 276 {
		t.Fatalf("会员套餐应按真实绑定条目计算高度，实际: %d", planHeight)
	}
}

// TestLayoutIR_StackOverlapAlignSelfAndJustifySelf 测试 Overlap 模式下两趟扫描自适应对齐与尺寸计算
func TestLayoutIR_StackOverlapAlignSelfAndJustifySelf(t *testing.T) {
	page := &models.DynamicPage{
		PageID:   "test_overlap_align",
		Theme:    "dark_glass",
		Revision: 1,
		Blocks: `[
			{
				"id": "stack_overlap_parent",
				"type": "stack",
				"props": {
					"mode": "overlap",
					"children": [
						{
							"id": "bg_image",
							"type": "image",
							"props": {
								"image_url": "https://example.com/banner.jpg",
								"aspect_ratio": "16:9"
							}
						},
						{
							"id": "floating_badge",
							"type": "action_button",
							"props": {
								"text": "热门",
								"width": 100,
								"justify_self": "flex-end",
								"align_self": "flex-end"
							}
						}
					]
				}
			}
		]`,
	}

	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{})
	if err != nil {
		t.Fatalf("生成 Overlap 对齐 IR 失败: %v", err)
	}

	if len(ir.Nodes) != 1 {
		t.Fatalf("预期 1 个父容器，实际: %d", len(ir.Nodes))
	}

	parent := ir.Nodes[0]
	if len(parent.Children) != 2 {
		t.Fatalf("预期 2 个子节点，实际: %d", len(parent.Children))
	}

	bgImage := parent.Children[0]
	floatingBtn := parent.Children[1]

	// 1. 底图应占满可用宽度 (父容器宽度减去左右 padding)
	availableWidth := parent.BoundingBox.Width - parent.Padding*2
	if bgImage.BoundingBox.Width != availableWidth {
		t.Fatalf("底图宽度应占满容器内容区，预期 %d，实际: %d", availableWidth, bgImage.BoundingBox.Width)
	}

	// 2. 浮动按钮显式 width 为 100
	if floatingBtn.BoundingBox.Width != 100 {
		t.Fatalf("浮动按钮宽度应为 100，实际: %d", floatingBtn.BoundingBox.Width)
	}

	// 3. 浮动按钮 justify_self 为 flex-end，X 坐标应靠右对齐内容区
	expectedBtnX := parent.BoundingBox.X + parent.Padding + availableWidth - 100
	if floatingBtn.BoundingBox.X != expectedBtnX {
		t.Fatalf("浮动按钮靠右对齐 X 坐标不匹配，预期 %d，实际: %d", expectedBtnX, floatingBtn.BoundingBox.X)
	}

	// 4. 浮动按钮 align_self 为 flex-end，Y 坐标应靠底
	if floatingBtn.BoundingBox.Y <= bgImage.BoundingBox.Y {
		t.Fatalf("浮动按钮靠底对齐 Y 坐标 (%d) 应大于底图起始 Y 坐标 (%d)", floatingBtn.BoundingBox.Y, bgImage.BoundingBox.Y)
	}
}

// TestLayoutIR_StyleUtilitiesKeepMetricsInSync 验证工具令牌与前端 rpx 工具类使用同一组逻辑像素。
func TestLayoutIR_StyleUtilitiesKeepMetricsInSync(t *testing.T) {
	page := &models.DynamicPage{
		PageID: "utility_metrics",
		Title:  "工具令牌",
		Blocks: `[{"id":"box","type":"container","style":{"utilities":["layout/flat","space/y-6","padding/6","radius/lg","gap/4"]},"props":{"children":[{"id":"a","type":"text","props":{"text":"A"}},{"id":"b","type":"text","props":{"text":"B"}}]}}]`,
	}
	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", nil)
	if err != nil {
		t.Fatalf("生成工具令牌 IR 失败: %v", err)
	}
	if len(ir.Nodes) != 1 {
		t.Fatalf("预期一个容器节点，实际 %d", len(ir.Nodes))
	}
	node := ir.Nodes[0]
	if node.MarginY != 12 || node.Padding != 12 || node.BorderRadius != 12 || node.GlassBlur {
		t.Fatalf("工具令牌未正确映射: margin=%d padding=%d radius=%d glass=%v", node.MarginY, node.Padding, node.BorderRadius, node.GlassBlur)
	}
	if len(node.Children) != 2 || node.Children[1].BoundingBox.Y-node.Children[0].BoundingBox.Y < 8 {
		t.Fatalf("容器 gap/4 未生效: %+v", node.Children)
	}
}
