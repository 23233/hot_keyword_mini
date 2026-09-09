// Package services layout_ir.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/models"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 导出别名，保持与 models 层完全同构并对齐历史调用
type BoundingBox = models.BoundingBox
type DeviceParams = models.DeviceParams
type BlockLayoutNode = models.BlockLayoutNode
type PageLayoutIR = models.PageLayoutIR

// DefaultDeviceParams 返回微信开发者工具默认的 iPhone 12/13 Pro 设备参数
func DefaultDeviceParams() DeviceParams {
	return models.DefaultDeviceParams()
}

// ResolveDeviceParams 根据设备名称标识解析目标物理设备与点阵参数
func ResolveDeviceParams(deviceName string) DeviceParams {
	lower := strings.ToLower(strings.TrimSpace(deviceName))
	switch lower {
	case "iphone_12", "iphone 12", "iphone_12_pro", "iphone 12 pro",
		"iphone_13", "iphone 13", "iphone_13_pro", "iphone 13 pro",
		"iphone 12/13 pro", "iphone 12/13 (pro)", "default":
		return DefaultDeviceParams()
	case "iphone_16_pro", "iphone 16 pro":
		// 保留旧设备标识的显式兼容，不再作为默认设备。
		return DeviceParams{Name: "iPhone 16 Pro", Width: 393, Height: 852, DPR: 3.0}
	case "iphone_se", "iphone se", "se":
		return DeviceParams{Name: "iPhone SE", Width: 375, Height: 667, DPR: 2.0}
	case "iphone_14_plus", "iphone_15_plus", "iphone 15 plus", "plus", "max":
		return DeviceParams{Name: "iPhone 15 Plus", Width: 430, Height: 932, DPR: 3.0}
	case "ipad", "ipad_mini", "ipad mini", "tablet":
		return DeviceParams{Name: "iPad mini", Width: 744, Height: 1133, DPR: 2.0}
	default:
		return DefaultDeviceParams()
	}
}

// BuildPageLayoutIR 根据 DynamicPage 与设备参数计算统一的布局中间表示 IR (兼容基础入参)
func BuildPageLayoutIR(page *models.DynamicPage, device DeviceParams, stateFixture string) (*PageLayoutIR, error) {
	context := make(map[string]interface{})
	return BuildPageLayoutIRWithContext(page, device, stateFixture, context)
}

// BuildPageLayoutIRWithContext 结合上下文计算真实完成数据绑定、条件求值与列表展开的同构布局中间表示 IR
func BuildPageLayoutIRWithContext(page *models.DynamicPage, device DeviceParams, stateFixture string, context map[string]interface{}) (*PageLayoutIR, error) {
	if page == nil {
		return nil, fmt.Errorf("动态页面对象不能为空")
	}
	if device.Width <= 0 {
		device = DefaultDeviceParams()
	}
	if stateFixture == "" {
		stateFixture = "normal"
	}

	theme := page.Theme
	if theme == "" {
		theme = "dark_glass"
	}

	ir := &PageLayoutIR{
		ProtocolVersion: "1.1",
		SchemaVersion:   3,
		Revision:        page.Revision,
		Device:          device,
		Theme:           theme,
		Locale:          "zh-CN",
		StateFixture:    stateFixture,
		Nodes:           make([]BlockLayoutNode, 0),
		NativeStubs:     make([]string, 0),
		Warnings:        make([]string, 0),
	}

	// 基础上下文初始化，注入页面元数据
	if context == nil {
		context = make(map[string]interface{})
	} else {
		// IR 构建只消费调用方上下文，页面元数据与别名写入内部副本，避免跨请求状态污染。
		context = shallowCopyMap(context)
	}
	normalizeScopedContextAliases(context)
	pageContext := map[string]interface{}{
		"page_id":       page.PageID,
		"title":         page.Title,
		"business_type": page.BusinessType,
		"intent":        page.Intent,
		"keyword":       page.Keyword,
	}
	context["$page"] = pageContext
	context["page"] = pageContext

	var rawBlocks []models.BlockItem
	if page.Blocks != "" {
		_ = json.Unmarshal([]byte(page.Blocks), &rawBlocks)
	}

	// 视口安全内容宽度 (左右边距默认 16px)
	contentWidth := device.Width - 32
	currentY := deviceTopInset(device) // 顶部导航栏避让安全预留高度，与小程序标题栏同构
	stubMap := make(map[string]bool)

	// 遍历并展开积木组件
	for _, block := range rawBlocks {
		// 1. 处理 repeat 列表循环展开
		expandedBlocks := ExpandBlockRepeat(block, context)

		for _, itemBlock := range expandedBlocks {
			// 构建当前节点私有上下文 (继承并注入当前 item)
			nodeCtx := contextForBlock(context, itemBlock)

			// 2. 状态多态分支处理 (loading / empty / error)
			targetBlock := resolveBlockStateVariant(itemBlock, stateFixture)

			// 3. 执行受控数据绑定深度求值 (解析 $entity.*, $query.*, $item.*, $state.* 等)
			resolvedProps := ResolveBlockPropsBindings(targetBlock.Props, nodeCtx, targetBlock.Type)
			targetBlock.Props = resolvedProps

			// 4. 真实受控条件可见性计算 (执行 eq/neq/in/exists/gt/gte/lt/lte/and/or/not)
			visible := true
			if targetBlock.VisibleWhen != nil {
				visible = EvaluateCondition(targetBlock.VisibleWhen, nodeCtx)
			}
			// 特殊状态 fixture 门禁微调
			if stateFixture == "empty" && targetBlock.Type != "notice" && targetBlock.Type != "empty" {
				visible = false
			}

			// 5. 解析样式规范
			padding := 14
			borderRadius := 14
			marginY := 8
			glassBlur := true
			accentColor := resolveStyleAccent(targetBlock.Style, page.AccentColor)

			marginY, padding, borderRadius, glassBlur = resolveStyleMetrics(targetBlock.Style, marginY, padding, borderRadius, glassBlur)

			// 6. 动态自适应排版高度与原生能力替身计算
			blockHeight, nativeStub := CalculateAdaptiveBlockHeight(&targetBlock, targetBlock.Props, contentWidth)

			// 提取真实文本摘要
			textSummary := extractTextSummary(&targetBlock, targetBlock.Props)

			actionType := ""
			if targetBlock.Action != nil {
				actionType = targetBlock.Action.Type
			}

			node := BlockLayoutNode{
				ID:     targetBlock.ID,
				Type:   targetBlock.Type,
				Props:  targetBlock.Props,
				Action: targetBlock.Action,
				Events: targetBlock.Events,
				BoundingBox: BoundingBox{
					X:      16,
					Y:      currentY + marginY,
					Width:  contentWidth,
					Height: blockHeight,
				},
				Visible:      visible,
				VisibleWhen:  targetBlock.VisibleWhen,
				Repeat:       targetBlock.Repeat,
				MarginY:      marginY,
				Padding:      padding,
				BorderRadius: borderRadius,
				GlassBlur:    glassBlur,
				AccentColor:  accentColor,
				TextSummary:  textSummary,
				ActionType:   actionType,
				NativeStub:   nativeStub,
				Loading:      targetBlock.Loading,
				Empty:        targetBlock.Empty,
				Error:        targetBlock.Error,
				Fallback:     targetBlock.Fallback,
			}

			// 布局块递归生成子节点，确保容器内图片、文本和按钮保留同一棵同构渲染树，并按容器布局特性 (Grid/Overlap/Row/Column) 准确计算尺寸
			if children := extractNestedBlocks(targetBlock.Props); len(children) > 0 {
				var childHeight int
				headerOffset := 0
				if targetBlock.Type == "tabs" {
					headerOffset = 46 // 选项卡顶部分段胶囊标签栏高度避让
				}
				node.Children, childHeight = buildNestedLayoutNodesForParent(&targetBlock, children, nodeCtx, device, accentColor, stateFixture, node.BoundingBox.X+padding, node.BoundingBox.Y+padding+headerOffset, contentWidth-padding*2, 1)
				if childHeight+padding*2+headerOffset > node.BoundingBox.Height {
					node.BoundingBox.Height = childHeight + padding*2 + headerOffset
				}
			}

			if nativeStub != "" && !stubMap[nativeStub] {
				stubMap[nativeStub] = true
				ir.NativeStubs = append(ir.NativeStubs, nativeStub)
			}

			ir.Nodes = append(ir.Nodes, node)

			if visible {
				currentY += node.BoundingBox.Height + marginY*2
			}
		}
	}

	ir.TotalHeight = currentY + 40
	return ir, nil
}

// normalizeScopedContextAliases 为协议受控作用域补齐带 $ 与不带 $ 的同一引用，确保预览与小程序解析一致。
func normalizeScopedContextAliases(context map[string]interface{}) {
	for _, scope := range []string{"entity", "query", "item", "state", "result", "session", "tenant", "props"} {
		plainKey := scope
		dollarKey := "$" + scope
		if value, ok := context[plainKey]; ok {
			context[dollarKey] = value
			continue
		}
		if value, ok := context[dollarKey]; ok {
			context[plainKey] = value
		}
	}
}

// deviceTopInset 返回标题栏占用的顶部安全高度，单位为逻辑像素。
// 数值与 AppleNavbar 的系统状态栏 + 胶囊导航栏公式保持一致。
func deviceTopInset(device DeviceParams) int {
	lower := strings.ToLower(strings.TrimSpace(device.Name))
	switch {
	case strings.Contains(lower, "iphone se"):
		return 64 // 状态栏约 20 + 导航栏 44
	case strings.Contains(lower, "ipad"):
		return 74 // iPad 状态栏约 24 + 导航栏 50
	default:
		return 91 // iPhone 12/13 Pro: 状态栏约 47 + 导航栏 44
	}
}

// resolveBlockStateVariant 根据当前状态 fixture 或块状态切换目标子块 (loading / empty / error)
func resolveBlockStateVariant(block models.BlockItem, stateFixture string) models.BlockItem {
	switch stateFixture {
	case "loading":
		if block.Loading != nil {
			cloned := *block.Loading
			if cloned.ID == "" {
				cloned.ID = block.ID + "_loading"
			}
			return cloned
		}
	case "empty":
		if block.Empty != nil {
			cloned := *block.Empty
			if cloned.ID == "" {
				cloned.ID = block.ID + "_empty"
			}
			return cloned
		}
	case "error", "offline", "out_of_stock", "expired":
		if block.Error != nil {
			cloned := *block.Error
			if cloned.ID == "" {
				cloned.ID = block.ID + "_error"
			}
			return cloned
		}
	}
	return block
}

// ExpandBlockRepeat 展开积木 repeat 循环配置
func ExpandBlockRepeat(block models.BlockItem, context map[string]interface{}) []models.BlockItem {
	if block.Repeat == nil {
		return []models.BlockItem{block}
	}

	var repeatList []interface{}
	hasExplicitSource := false
	if itemsRaw, ok := block.Repeat["items"]; ok {
		hasExplicitSource = true
		if itemsSlice, ok := itemsRaw.([]interface{}); ok {
			repeatList = itemsSlice
		}
	} else if pathVal, ok := block.Repeat["path"].(string); ok {
		hasExplicitSource = true
		resolved := ResolveBindingValue(pathVal, context)
		if slice, ok := resolved.([]interface{}); ok {
			repeatList = slice
		}
	}

	// 若显式指定了 repeat 数据源但列表为空，杜绝渲染含未解析表达式的幽灵积木
	if hasExplicitSource && len(repeatList) == 0 {
		if block.Empty != nil {
			return []models.BlockItem{*block.Empty}
		}
		return []models.BlockItem{}
	}

	if len(repeatList) == 0 {
		return []models.BlockItem{block}
	}

	results := make([]models.BlockItem, 0, len(repeatList))
	for idx, itemData := range repeatList {
		cloned := block
		cloned.ID = fmt.Sprintf("%s_%d", block.ID, idx)
		cloned.Repeat = nil

		// 克隆 props 并执行 item 数据求值
		clonedProps := make(map[string]interface{})
		for k, v := range block.Props {
			clonedProps[k] = v
		}
		clonedProps["_repeat_item"] = itemData
		clonedProps["_repeat_index"] = idx

		itemCtx := shallowCopyMap(context)
		itemCtx["$item"] = itemData
		itemCtx["item"] = itemData

		resolvedProps := ResolveBlockPropsBindings(clonedProps, itemCtx, block.Type)
		cloned.Props = resolvedProps
		results = append(results, cloned)
	}

	return results
}

// contextForBlock 为展开后的循环块恢复独立 $item 上下文，确保条件、绑定和动作与小程序一致。
func contextForBlock(context map[string]interface{}, block models.BlockItem) map[string]interface{} {
	result := shallowCopyMap(context)
	if block.Props == nil {
		return result
	}
	if item, ok := block.Props["_repeat_item"]; ok {
		result["item"] = item
		result["$item"] = item
	}
	return result
}

// isKnownScopedPath 判断路径是否属于受控作用域（如 $entity.title 或 entity.title）
func isKnownScopedPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	if strings.HasPrefix(trimmed, "$") {
		trimmed = trimmed[1:]
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "entity", "query", "item", "state", "result", "page", "session", "tenant", "props":
		return true
	default:
		return false
	}
}

// inlineInterpolationRegex 匹配字符串内嵌的数据绑定插值表达式 (如 "恭喜 {{$entity.name}} 领取成功")
var inlineInterpolationRegex = regexp.MustCompile(`\{\{\s*(\$?[a-zA-Z0-9_.]+)\s*\}\}`)

// ResolveBindingValue 解析单个受控路径绑定值 (如 $entity.title, entity.title, $query.id, $item.name)
func ResolveBindingValue(val interface{}, context map[string]interface{}) interface{} {
	if val == nil {
		return nil
	}

	// 1. 处理对象形式: { "path": "$entity.title" } 或 { "path": "entity.title" }
	if m, ok := val.(map[string]interface{}); ok {
		if pathStr, ok := m["path"].(string); ok && pathStr != "" {
			return resolvePathString(pathStr, context)
		}
	}

	// 2. 处理直接字符串形式: "$entity.title", "entity.title", "{{entity.title}}" 或内嵌插值文本 "前缀 {{path}} 后缀"
	if s, ok := val.(string); ok {
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") && !strings.Contains(trimmed[2:len(trimmed)-2], "{{") {
			innerPath := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			return resolvePathString(innerPath, context)
		}
		if isKnownScopedPath(trimmed) {
			return resolvePathString(trimmed, context)
		}
		if strings.Contains(s, "{{") && strings.Contains(s, "}}") {
			interpolated := inlineInterpolationRegex.ReplaceAllStringFunc(s, func(match string) string {
				sub := inlineInterpolationRegex.FindStringSubmatch(match)
				if len(sub) > 1 {
					res := resolvePathString(sub[1], context)
					if res != nil {
						return fmt.Sprintf("%v", res)
					}
				}
				return match
			})
			return interpolated
		}
	}

	return val
}

// resolvePathString 根据点号分割路径提取上下文深度属性 (支持显式 $ 作用域与省略 $ 语法双向归一化)
func resolvePathString(path string, context map[string]interface{}) interface{} {
	if context == nil || path == "" {
		return path
	}

	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return path
	}

	rootKey := parts[0]
	var current interface{}

	// 双向兼容：支持 context 中无论以 $xxx 还是以无 $ 存储均能精准命中文本
	if val, ok := context[rootKey]; ok {
		current = val
	} else if strings.HasPrefix(rootKey, "$") {
		if val, ok := context[strings.TrimPrefix(rootKey, "$")]; ok {
			current = val
		}
	} else {
		if val, ok := context["$"+rootKey]; ok {
			current = val
		}
	}

	if current == nil {
		return nil
	}

	for i := 1; i < len(parts); i++ {
		if current == nil {
			return nil
		}
		seg := parts[i]
		if m, ok := current.(map[string]interface{}); ok {
			current = m[seg]
		} else {
			return nil
		}
	}

	return current
}

// ResolveObjectBindings 递归求值结构体内所有数据绑定表达式
func ResolveObjectBindings(target interface{}, context map[string]interface{}) interface{} {
	if target == nil {
		return nil
	}

	// 若自身是绑定描述
	if m, ok := target.(map[string]interface{}); ok {
		if pathStr, ok := m["path"].(string); ok && len(m) == 1 && (strings.HasPrefix(pathStr, "$") || isKnownScopedPath(pathStr)) {
			return ResolveBindingValue(pathStr, context)
		}

		result := make(map[string]interface{}, len(m))
		for k, v := range m {
			result[k] = ResolveObjectBindings(v, context)
		}
		return result
	}

	if s, ok := target.(string); ok {
		return ResolveBindingValue(s, context)
	}

	if slice, ok := target.([]interface{}); ok {
		result := make([]interface{}, len(slice))
		for i, item := range slice {
			result[i] = ResolveObjectBindings(item, context)
		}
		return result
	}

	return target
}

// ResolveBlockPropsBindings 解析 block 自身属性，同时保留嵌套子 block 的绑定直到子节点渲染阶段。
func ResolveBlockPropsBindings(props map[string]interface{}, context map[string]interface{}, blockType string) map[string]interface{} {
	if props == nil {
		return map[string]interface{}{}
	}
	tabField := "items"
	if _, ok := props["tabs"].([]interface{}); ok {
		tabField = "tabs"
	}
	resolved := make(map[string]interface{}, len(props))
	for key, value := range props {
		resolved[key] = resolvePropsPreservingBlocks(value, context, blockType == "tabs" && key == tabField)
	}
	return resolved
}

func resolvePropsPreservingBlocks(value interface{}, context map[string]interface{}, preserveIdentity bool) interface{} {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		// 子 block 的 props/条件需要在其自身上下文中求值，不能在父容器提前消费。
		if _, hasType := m["type"]; hasType {
			return m
		}
		if path, isBinding := m["path"].(string); isBinding && len(m) == 1 && (strings.HasPrefix(path, "$") || isKnownScopedPath(path)) {
			return ResolveObjectBindings(m, context)
		}
		result := make(map[string]interface{}, len(m))
		for k, v := range m {
			// Tab 描述对象的 key/id 是结构标识；其他业务属性仍允许 $item.id 等受控绑定。
			if preserveIdentity && (k == "key" || k == "id") {
				result[k] = v
				continue
			}
			result[k] = resolvePropsPreservingBlocks(v, context, false)
		}
		return result
	}
	if list, ok := value.([]interface{}); ok {
		result := make([]interface{}, len(list))
		for i, item := range list {
			result[i] = resolvePropsPreservingBlocks(item, context, preserveIdentity)
		}
		return result
	}
	return ResolveObjectBindings(value, context)
}

func extractNestedBlocks(props map[string]interface{}) []models.BlockItem {
	if props == nil {
		return nil
	}
	for _, key := range []string{"children", "items", "blocks"} {
		if raw, ok := props[key].([]interface{}); ok {
			children := make([]models.BlockItem, 0, len(raw))
			for _, item := range raw {
				data, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				encoded, err := json.Marshal(data)
				if err != nil {
					continue
				}
				var child models.BlockItem
				if json.Unmarshal(encoded, &child) == nil && child.Type != "" {
					children = append(children, child)
				}
			}
			if len(children) > 0 {
				return children
			}
		}
	}
	// tabs 按协议定义的初始 activeKey 规则展开当前选中栏，保持 IR 与 page.blocks 结构对照一致。
	if rawTabs, ok := props["tabs"].([]interface{}); ok && len(rawTabs) > 0 {
		if tab := selectActiveTab(rawTabs, props); tab != nil {
			if children := extractNestedBlocks(tab); len(children) > 0 {
				return children
			}
			if child, ok := tab["child"].(map[string]interface{}); ok {
				encoded, _ := json.Marshal(child)
				var item models.BlockItem
				if json.Unmarshal(encoded, &item) == nil && item.Type != "" {
					return []models.BlockItem{item}
				}
			}
		}
	}
	return nil
}

// selectActiveTab 按 TabsBlock 的 active_key、active_tab、default_active_key、default_index 顺序选择当前栏。
func selectActiveTab(tabs []interface{}, props map[string]interface{}) map[string]interface{} {
	selectedIndex := 0
	selectedKey := ""
	for _, key := range []string{"active_key", "active_tab", "default_active_key"} {
		if value, exists := props[key]; exists && value != nil {
			selectedKey = fmt.Sprintf("%v", value)
			break
		}
	}
	if selectedKey == "" {
		if defaultIndex, exists := props["default_index"]; exists {
			selectedIndex = int(toFloat64(defaultIndex))
		}
	} else {
		for index, rawTab := range tabs {
			tab, ok := rawTab.(map[string]interface{})
			if !ok {
				continue
			}
			key := fmt.Sprintf("%v", index)
			if value, exists := tab["key"]; exists && value != nil {
				key = fmt.Sprintf("%v", value)
			} else if value, exists := tab["id"]; exists && value != nil {
				key = fmt.Sprintf("%v", value)
			}
			if key == selectedKey {
				selectedIndex = index
				break
			}
		}
	}
	if selectedIndex < 0 || selectedIndex >= len(tabs) {
		selectedIndex = 0
	}
	tab, _ := tabs[selectedIndex].(map[string]interface{})
	return tab
}

// buildNestedLayoutNodesForParent 根据父容器的布局特性 (Grid/Overlap/Row/Column) 精准计算子节点点阵坐标与高度
func buildNestedLayoutNodesForParent(parent *models.BlockItem, blocks []models.BlockItem, context map[string]interface{}, device DeviceParams, accentColor, stateFixture string, x, y, width, depth int) ([]BlockLayoutNode, int) {
	if depth > 32 || width <= 0 {
		return nil, 0
	}

	parentType := ""
	var parentProps map[string]interface{}
	if parent != nil {
		parentType = parent.Type
		parentProps = parent.Props
	}
	if parentProps == nil {
		parentProps = make(map[string]interface{})
	}

	// 1. 识别是否为多列网格 (grid / item_grid)
	if parentType == "grid" || parentType == "item_grid" {
		cols := 2
		if c := int(toFloat64(parentProps["columns"])); c >= 1 && c <= 4 {
			cols = c
		}
		gap := resolveStyleGap(parent, 8)
		if gapStr, ok := parentProps["gap"].(string); ok {
			gap = parsePixelValue(gapStr, 8)
		}
		totalGap := (cols - 1) * gap
		cellWidth := (width - totalGap) / cols
		if cellWidth <= 0 {
			cellWidth = width
		}

		flatItems := make([]models.BlockItem, 0)
		for _, b := range blocks {
			flatItems = append(flatItems, ExpandBlockRepeat(b, context)...)
		}

		result := make([]BlockLayoutNode, 0, len(flatItems))
		currentY := y
		rowMaxH := 0

		for idx, item := range flatItems {
			nodeCtx := contextForBlock(context, item)
			target := resolveBlockStateVariant(item, stateFixture)
			props := ResolveBlockPropsBindings(target.Props, nodeCtx, target.Type)
			visible := target.VisibleWhen == nil || EvaluateCondition(target.VisibleWhen, nodeCtx)

			colIdx := idx % cols
			if colIdx == 0 && idx > 0 {
				currentY += rowMaxH + gap
				rowMaxH = 0
			}

			cellX := x + colIdx*(cellWidth+gap)
			cellY := currentY

			h, stub := CalculateAdaptiveBlockHeight(&target, props, cellWidth)
			marginY, padding, radius, glass := 4, 8, 12, true
			if target.Style != nil {
				marginY, padding, radius, glass = resolveStyleMetrics(target.Style, marginY, padding, radius, glass)
			}

			child := BlockLayoutNode{
				ID: target.ID, Type: target.Type, Props: props, Visible: visible,
				VisibleWhen: target.VisibleWhen, Repeat: target.Repeat,
				BoundingBox: BoundingBox{X: cellX, Y: cellY + marginY, Width: cellWidth, Height: h},
				MarginY:     marginY, Padding: padding, BorderRadius: radius, GlassBlur: glass,
				AccentColor: resolveStyleAccent(target.Style, accentColor), TextSummary: extractTextSummary(&target, props),
				Action: target.Action, Events: target.Events, NativeStub: stub,
				Loading: target.Loading, Empty: target.Empty, Error: target.Error, Fallback: target.Fallback,
			}
			if target.Action != nil {
				child.ActionType = target.Action.Type
			}

			if nested := extractNestedBlocks(props); len(nested) > 0 {
				var childH int
				child.Children, childH = buildNestedLayoutNodesForParent(&target, nested, nodeCtx, device, child.AccentColor, stateFixture, cellX+padding, child.BoundingBox.Y+padding, cellWidth-padding*2, depth+1)
				if childH+padding*2 > child.BoundingBox.Height {
					child.BoundingBox.Height = childH + padding*2
				}
			}

			result = append(result, child)
			if child.BoundingBox.Height+marginY*2 > rowMaxH {
				rowMaxH = child.BoundingBox.Height + marginY*2
			}
		}

		totalH := (currentY - y) + rowMaxH
		return result, totalH
	}

	// 2. 识别是否为层叠重叠堆叠 (Stack Overlap / ZStack)
	mode, _ := parentProps["mode"].(string)
	if mode == "" {
		mode, _ = parentProps["layout"].(string)
	}
	if (parentType == "stack" || parentType == "container") && (mode == "overlap" || mode == "zstack") {
		flatItems := make([]models.BlockItem, 0)
		for _, b := range blocks {
			flatItems = append(flatItems, ExpandBlockRepeat(b, context)...)
		}

		type tempChildNode struct {
			target  models.BlockItem
			props   map[string]interface{}
			visible bool
			nodeCtx map[string]interface{}
			height  int
			width   int
			stub    string
			marginY int
			padding int
			radius  int
			glass   bool
			nested  []models.BlockItem
		}

		temps := make([]tempChildNode, 0, len(flatItems))
		maxH := 0

		// 第一趟扫描：计算每个子项的自适应高度与宽度，求出整个 Overlap 容器的最大高度 maxH
		for _, item := range flatItems {
			nodeCtx := contextForBlock(context, item)
			target := resolveBlockStateVariant(item, stateFixture)
			props := ResolveBlockPropsBindings(target.Props, nodeCtx, target.Type)
			visible := target.VisibleWhen == nil || EvaluateCondition(target.VisibleWhen, nodeCtx)

			childW := width
			if explicitW := int(toFloat64(props["width"])); explicitW > 0 && explicitW <= width {
				childW = explicitW
			}

			h, stub := CalculateAdaptiveBlockHeight(&target, props, childW)
			marginY, padding, radius, glass := 0, 0, 14, false
			if target.Style != nil {
				marginY, padding, radius, glass = resolveStyleMetrics(target.Style, marginY, padding, radius, glass)
			}

			nested := extractNestedBlocks(props)

			t := tempChildNode{
				target:  target,
				props:   props,
				visible: visible,
				nodeCtx: nodeCtx,
				height:  h,
				width:   childW,
				stub:    stub,
				marginY: marginY,
				padding: padding,
				radius:  radius,
				glass:   glass,
				nested:  nested,
			}
			temps = append(temps, t)

			if h+marginY*2 > maxH {
				maxH = h + marginY*2
			}
		}

		// 第二趟扫描：结合容器总高度与对齐属性 (justify_self, align_self) 精准定位每个子节点的坐标
		result := make([]BlockLayoutNode, 0, len(temps))
		for _, t := range temps {
			justifySelf := ""
			if js, ok := t.props["justify_self"].(string); ok {
				justifySelf = strings.ToLower(strings.TrimSpace(js))
			}
			alignSelf := ""
			if as, ok := t.props["align_self"].(string); ok {
				alignSelf = strings.ToLower(strings.TrimSpace(as))
			}

			// 水平坐标计算
			childX := x
			switch justifySelf {
			case "end", "flex-end", "right":
				if t.width < width {
					childX = x + width - t.width
				}
			case "center", "middle":
				if t.width < width {
					childX = x + (width-t.width)/2
				}
			default:
				childX = x
			}

			// 垂直坐标计算
			childY := y + t.marginY
			switch alignSelf {
			case "end", "flex-end", "bottom":
				if t.height < maxH {
					childY = y + maxH - t.height - t.marginY
				}
			case "center", "middle":
				if t.height < maxH {
					childY = y + (maxH-t.height)/2
				}
			default:
				childY = y + t.marginY
			}

			child := BlockLayoutNode{
				ID: t.target.ID, Type: t.target.Type, Props: t.props, Visible: t.visible,
				VisibleWhen: t.target.VisibleWhen, Repeat: t.target.Repeat,
				BoundingBox: BoundingBox{X: childX, Y: childY, Width: t.width, Height: t.height},
				MarginY:     t.marginY, Padding: t.padding, BorderRadius: t.radius, GlassBlur: t.glass,
				AccentColor: resolveStyleAccent(t.target.Style, accentColor), TextSummary: extractTextSummary(&t.target, t.props),
				Action: t.target.Action, Events: t.target.Events, NativeStub: t.stub,
				Loading: t.target.Loading, Empty: t.target.Empty, Error: t.target.Error, Fallback: t.target.Fallback,
			}
			if t.target.Action != nil {
				child.ActionType = t.target.Action.Type
			}

			if len(t.nested) > 0 {
				var childH int
				child.Children, childH = buildNestedLayoutNodesForParent(&t.target, t.nested, t.nodeCtx, device, child.AccentColor, stateFixture, childX+t.padding, child.BoundingBox.Y+t.padding, t.width-t.padding*2, depth+1)
				if childH+t.padding*2 > child.BoundingBox.Height {
					child.BoundingBox.Height = childH + t.padding*2
				}
			}

			result = append(result, child)
			if child.BoundingBox.Height+t.marginY*2 > maxH {
				maxH = child.BoundingBox.Height + t.marginY*2
			}
		}

		return result, maxH
	}

	// 3. 识别是否为水平弹性排列 (direction: row)
	direction, _ := parentProps["direction"].(string)
	if direction == "" {
		direction, _ = parentProps["flex_direction"].(string)
	}
	if (parentType == "stack" || parentType == "container") && (direction == "row" || direction == "row-reverse") {
		flatItems := make([]models.BlockItem, 0)
		for _, b := range blocks {
			flatItems = append(flatItems, ExpandBlockRepeat(b, context)...)
		}

		itemCount := len(flatItems)
		if itemCount == 0 {
			return nil, 0
		}

		gap := resolveStyleGap(parent, 8)
		if gapStr, ok := parentProps["gap"].(string); ok {
			gap = parsePixelValue(gapStr, 8)
		}

		totalGap := (itemCount - 1) * gap
		colWidth := (width - totalGap) / itemCount
		if colWidth <= 0 {
			colWidth = width
		}

		result := make([]BlockLayoutNode, 0, itemCount)
		currentX := x
		maxRowH := 0

		for _, item := range flatItems {
			nodeCtx := contextForBlock(context, item)
			target := resolveBlockStateVariant(item, stateFixture)
			props := ResolveBlockPropsBindings(target.Props, nodeCtx, target.Type)
			visible := target.VisibleWhen == nil || EvaluateCondition(target.VisibleWhen, nodeCtx)

			itemW := colWidth
			if explicitW := int(toFloat64(props["width"])); explicitW > 0 {
				itemW = explicitW
			}

			h, stub := CalculateAdaptiveBlockHeight(&target, props, itemW)
			marginY, padding, radius, glass := 4, 8, 12, false
			if target.Style != nil {
				marginY, padding, radius, glass = resolveStyleMetrics(target.Style, marginY, padding, radius, glass)
			}

			child := BlockLayoutNode{
				ID: target.ID, Type: target.Type, Props: props, Visible: visible,
				VisibleWhen: target.VisibleWhen, Repeat: target.Repeat,
				BoundingBox: BoundingBox{X: currentX, Y: y + marginY, Width: itemW, Height: h},
				MarginY:     marginY, Padding: padding, BorderRadius: radius, GlassBlur: glass,
				AccentColor: resolveStyleAccent(target.Style, accentColor), TextSummary: extractTextSummary(&target, props),
				Action: target.Action, Events: target.Events, NativeStub: stub,
				Loading: target.Loading, Empty: target.Empty, Error: target.Error, Fallback: target.Fallback,
			}
			if target.Action != nil {
				child.ActionType = target.Action.Type
			}

			if nested := extractNestedBlocks(props); len(nested) > 0 {
				var childH int
				child.Children, childH = buildNestedLayoutNodesForParent(&target, nested, nodeCtx, device, child.AccentColor, stateFixture, currentX+padding, child.BoundingBox.Y+padding, itemW-padding*2, depth+1)
				if childH+padding*2 > child.BoundingBox.Height {
					child.BoundingBox.Height = childH + padding*2
				}
			}

			result = append(result, child)
			currentX += itemW + gap
			if child.BoundingBox.Height+marginY*2 > maxRowH {
				maxRowH = child.BoundingBox.Height + marginY*2
			}
		}

		return result, maxRowH
	}

	// 4. 默认常规纵向弹性流布局 (Column)
	startY := y
	headerOffset := 0
	if parentType == "tabs" {
		headerOffset = 46 // 选项卡顶部分段胶囊标签栏高度避让
		startY = y + headerOffset
	}
	nodes, h := buildNestedLayoutNodes(blocks, context, device, accentColor, stateFixture, x, startY, width, depth)
	return nodes, h + headerOffset
}

// buildNestedLayoutNodes 递归构建常规纵向布局子树。子树使用与顶层相同的绑定、条件、状态和尺寸规则。
func buildNestedLayoutNodes(blocks []models.BlockItem, context map[string]interface{}, device DeviceParams, accentColor, stateFixture string, x, y, width, depth int) ([]BlockLayoutNode, int) {
	if depth > 32 || width <= 0 {
		return nil, 0
	}
	result := make([]BlockLayoutNode, 0, len(blocks))
	currentY := y
	for _, block := range blocks {
		for _, expanded := range ExpandBlockRepeat(block, context) {
			nodeCtx := contextForBlock(context, expanded)
			target := resolveBlockStateVariant(expanded, stateFixture)
			props := ResolveBlockPropsBindings(target.Props, nodeCtx, target.Type)
			visible := target.VisibleWhen == nil || EvaluateCondition(target.VisibleWhen, nodeCtx)
			height, nativeStub := CalculateAdaptiveBlockHeight(&target, props, width)
			marginY, padding, radius, glass := 8, 14, 14, true
			if target.Style != nil {
				marginY, padding, radius, glass = resolveStyleMetrics(target.Style, marginY, padding, radius, glass)
			}
			child := BlockLayoutNode{
				ID: target.ID, Type: target.Type, Props: props, Visible: visible,
				VisibleWhen: target.VisibleWhen, Repeat: target.Repeat,
				BoundingBox: BoundingBox{X: x, Y: currentY + marginY, Width: width, Height: height},
				MarginY:     marginY, Padding: padding, BorderRadius: radius, GlassBlur: glass,
				AccentColor: resolveStyleAccent(target.Style, accentColor), TextSummary: extractTextSummary(&target, props),
				Action: target.Action, Events: target.Events, NativeStub: nativeStub,
				Loading: target.Loading, Empty: target.Empty, Error: target.Error, Fallback: target.Fallback,
			}
			if target.Action != nil {
				child.ActionType = target.Action.Type
			}
			if nested := extractNestedBlocks(props); len(nested) > 0 {
				var childHeight int
				headerOffset := 0
				if target.Type == "tabs" {
					headerOffset = 46
				}
				child.Children, childHeight = buildNestedLayoutNodesForParent(&target, nested, nodeCtx, device, child.AccentColor, stateFixture, x+padding, child.BoundingBox.Y+padding+headerOffset, width-padding*2, depth+1)
				if childHeight+padding*2+headerOffset > child.BoundingBox.Height {
					child.BoundingBox.Height = childHeight + padding*2 + headerOffset
				}
			}
			result = append(result, child)
			if visible {
				currentY += child.BoundingBox.Height + marginY*2
			}
		}
	}
	return result, currentY - y
}

// EvaluateCondition 受控条件求值引擎 (支持 and, or, not, eq, neq, in, exists, gt, gte, lt, lte)
func EvaluateCondition(cond interface{}, context map[string]interface{}) bool {
	if cond == nil {
		return true
	}

	if b, ok := cond.(bool); ok {
		return b
	}

	m, ok := cond.(map[string]interface{})
	if !ok {
		return true
	}

	if hide, ok := m["hide"].(bool); ok && hide {
		return false
	}

	// 逻辑与: and
	if andRaw, ok := m["and"]; ok {
		if slice, ok := andRaw.([]interface{}); ok {
			for _, sub := range slice {
				if !EvaluateCondition(sub, context) {
					return false
				}
			}
			return true
		}
	}

	// 逻辑或: or
	if orRaw, ok := m["or"]; ok {
		if slice, ok := orRaw.([]interface{}); ok {
			for _, sub := range slice {
				if EvaluateCondition(sub, context) {
					return true
				}
			}
			return false
		}
	}

	// 逻辑非: not
	if notRaw, ok := m["not"]; ok {
		return !EvaluateCondition(notRaw, context)
	}

	// 属性简写: {"path":"$state.enabled", "eq":true}。
	// Schema 已支持此写法，需与小程序端保持同一运行语义。
	if path, ok := m["path"].(string); ok && path != "" {
		left := ResolveBindingValue(map[string]interface{}{"path": path}, context)
		if right, exists := m["eq"]; exists {
			return compareLooseEqual(left, ResolveBindingValue(right, context))
		}
		if right, exists := m["neq"]; exists {
			return !compareLooseEqual(left, ResolveBindingValue(right, context))
		}
		if expected, exists := m["exists"]; exists {
			existsValue := left != nil && fmt.Sprintf("%v", left) != ""
			if expectedBool, isBool := expected.(bool); isBool {
				return existsValue == expectedBool
			}
			return existsValue
		}
		if right, exists := m["in"]; exists {
			if values, isSlice := ResolveBindingValue(right, context).([]interface{}); isSlice {
				for _, value := range values {
					if compareLooseEqual(left, value) {
						return true
					}
				}
			}
			return false
		}
		for _, operator := range []string{"gt", "gte", "lt", "lte"} {
			if right, exists := m[operator]; exists {
				leftNumber, leftOK := toFloatStrict(left)
				rightNumber, rightOK := toFloatStrict(ResolveBindingValue(right, context))
				if !leftOK || !rightOK {
					return false
				}
				switch operator {
				case "gt":
					return leftNumber > rightNumber
				case "gte":
					return leftNumber >= rightNumber
				case "lt":
					return leftNumber < rightNumber
				default:
					return leftNumber <= rightNumber
				}
			}
		}
	}

	// 等值比较: eq
	if eqRaw, ok := m["eq"]; ok {
		if slice, ok := eqRaw.([]interface{}); ok && len(slice) >= 2 {
			l := ResolveBindingValue(slice[0], context)
			r := ResolveBindingValue(slice[1], context)
			return compareLooseEqual(l, r)
		}
	}

	// 不等比较: neq
	if neqRaw, ok := m["neq"]; ok {
		if slice, ok := neqRaw.([]interface{}); ok && len(slice) >= 2 {
			l := ResolveBindingValue(slice[0], context)
			r := ResolveBindingValue(slice[1], context)
			return !compareLooseEqual(l, r)
		}
	}

	// 存在性检查: exists
	if existsRaw, ok := m["exists"]; ok {
		val := ResolveBindingValue(existsRaw, context)
		return val != nil && fmt.Sprintf("%v", val) != ""
	}

	// 集合包含: in (双向容错与 compareLooseEqual 类型安全等值比较)
	if inRaw, ok := m["in"]; ok {
		if slice, ok := inRaw.([]interface{}); ok && len(slice) >= 2 {
			val0 := ResolveBindingValue(slice[0], context)
			val1 := ResolveBindingValue(slice[1], context)
			if arr, ok := val1.([]interface{}); ok {
				for _, el := range arr {
					if compareLooseEqual(el, val0) {
						return true
					}
				}
			}
			if arr, ok := val0.([]interface{}); ok {
				for _, el := range arr {
					if compareLooseEqual(el, val1) {
						return true
					}
				}
			}
			return false
		}
	}

	// 数值比较: gt, gte, lt, lte
	if gtRaw, ok := m["gt"]; ok {
		if slice, ok := gtRaw.([]interface{}); ok && len(slice) >= 2 {
			l, leftOK := toFloatStrict(ResolveBindingValue(slice[0], context))
			r, rightOK := toFloatStrict(ResolveBindingValue(slice[1], context))
			return leftOK && rightOK && l > r
		}
	}
	if gteRaw, ok := m["gte"]; ok {
		if slice, ok := gteRaw.([]interface{}); ok && len(slice) >= 2 {
			l, leftOK := toFloatStrict(ResolveBindingValue(slice[0], context))
			r, rightOK := toFloatStrict(ResolveBindingValue(slice[1], context))
			return leftOK && rightOK && l >= r
		}
	}
	if ltRaw, ok := m["lt"]; ok {
		if slice, ok := ltRaw.([]interface{}); ok && len(slice) >= 2 {
			l, leftOK := toFloatStrict(ResolveBindingValue(slice[0], context))
			r, rightOK := toFloatStrict(ResolveBindingValue(slice[1], context))
			return leftOK && rightOK && l < r
		}
	}
	if lteRaw, ok := m["lte"]; ok {
		if slice, ok := lteRaw.([]interface{}); ok && len(slice) >= 2 {
			l, leftOK := toFloatStrict(ResolveBindingValue(slice[0], context))
			r, rightOK := toFloatStrict(ResolveBindingValue(slice[1], context))
			return leftOK && rightOK && l <= r
		}
	}

	return true
}

// CalculateAdaptiveBlockHeight 根据组件类型、文本长短、图片宽高比与网格列数自适应计算像素高度
func CalculateAdaptiveBlockHeight(block *models.BlockItem, props map[string]interface{}, contentWidth int) (int, string) {
	nativeStub := ""
	if props == nil {
		props = make(map[string]interface{})
	}

	switch block.Type {
	// 1. 通用图片积木
	case "image":
		// 支持通过 props.aspect_ratio 或 width/height 自适应
		if ar, ok := props["aspect_ratio"].(string); ok {
			switch ar {
			case "16:9":
				return int(float64(contentWidth) * 9.0 / 16.0), ""
			case "4:3":
				return int(float64(contentWidth) * 3.0 / 4.0), ""
			case "1:1":
				return contentWidth, ""
			case "3:2":
				return int(float64(contentWidth) * 2.0 / 3.0), ""
			}
		}
		if hVal := toFloat64(props["height"]); hVal > 0 {
			if wVal := toFloat64(props["width"]); wVal > 0 {
				ratio := hVal / wVal
				return int(float64(contentWidth) * ratio), ""
			}
			return int(hVal), ""
		}
		// 默认图片 16:9 优雅比例
		return int(float64(contentWidth) * 9.0 / 16.0), ""

	// 2. 通用文本积木
	case "text", "rich_text":
		text, _ := props["text"].(string)
		if text == "" {
			text, _ = props["content"].(string)
		}
		charCount := utf8.RuneCountInString(text)
		fontSize := 15
		if fs := toFloat64(props["font_size"]); fs > 0 {
			fontSize = int(fs)
		}
		charsPerLine := contentWidth / (fontSize + 1)
		if charsPerLine <= 0 {
			charsPerLine = 20
		}
		lines := int(math.Ceil(float64(charCount) / float64(charsPerLine)))
		if lines <= 0 {
			lines = 1
		}
		if maxLines := int(toFloat64(props["max_lines"])); maxLines > 0 && lines > maxLines {
			lines = maxLines
		}
		lineHeight := int(float64(fontSize) * 1.5)
		return 24 + lines*lineHeight, ""

	// 3. 媒体大焦点卡片
	case "media_hero":
		nativeStub = "channels_video_native_player_stub"
		// 16:9 视频/海报区域 + 底部标题与全网评分条
		mediaHeight := int(float64(contentWidth) * 9.0 / 16.0)
		return mediaHeight + 64, nativeStub

	// 4. 通用视频
	case "video":
		nativeStub = "native_video_player_stub"
		return int(float64(contentWidth) * 9.0 / 16.0), nativeStub

	// 5. 苹果大胶囊操作按钮
	case "action_button":
		return 52, ""

	// 6. 跑马灯公告
	case "notice":
		return 40, ""

	// 7. 网盘多渠道资源卡片
	case "resource_card":
		h := 116
		if rawCh, ok := props["channels"].([]interface{}); ok && len(rawCh) > 1 {
			h += 26
		} else if rawCh, ok := props["pan_list"].([]interface{}); ok && len(rawCh) > 1 {
			h += 26
		}
		return h, ""

	// 8. 游戏礼包卡片
	case "game_card":
		return 132, ""

	// 9. 输入查询表单
	case "form":
		fieldCount := 1
		if fields, ok := props["fields"].([]interface{}); ok && len(fields) > 0 {
			fieldCount = len(fields)
		}
		return 50 + fieldCount*56 + 54, ""

	// 10. 网格布局与列表
	case "item_grid", "grid":
		cols := 2
		if c := int(toFloat64(props["columns"])); c > 0 {
			cols = c
		}
		itemCount := 4
		if items, ok := props["items"].([]interface{}); ok && len(items) > 0 {
			itemCount = len(items)
		}
		rows := int(math.Ceil(float64(itemCount) / float64(cols)))
		if rows <= 0 {
			rows = 1
		}
		cardH := 140
		return rows*cardH + (rows-1)*10 + 20, ""

	// 11. 选集列表
	case "episode_list":
		epCount := 10
		if episodes, ok := props["episodes"].([]interface{}); ok && len(episodes) > 0 {
			epCount = len(episodes)
		}
		rows := int(math.Ceil(float64(epCount) / 5.0))
		if rows <= 0 {
			rows = 1
		}
		return 40 + rows*44, ""

	// 12. 时间线
	case "timeline":
		count := 3
		if items, ok := props["items"].([]interface{}); ok && len(items) > 0 {
			count = len(items)
		}
		return 40 + count*56, ""

	// 13. 间距器
	case "spacer":
		h := 20
		if sh := int(toFloat64(props["height"])); sh > 0 {
			h = sh
		}
		return h, ""

	// 14. 空状态
	case "empty":
		return 160, ""

	// 15. 骨架屏
	case "skeleton":
		rows := 3
		if r := int(toFloat64(props["rows"])); r > 0 {
			rows = r
		}
		return 20 + rows*36, ""

	// 16. 轮播走马灯
	case "carousel":
		return 180, ""

	// 17. 选项卡
	case "tabs":
		return 220, ""

	// 18. 容器/堆叠
	case "container", "stack":
		return 160, ""

	// 19. 通用自由编排卡片积木
	case "custom", "custom_block":
		h := 80
		if img, ok := props["image_url"].(string); ok && img != "" {
			h += 130
		} else if cover, ok := props["cover_url"].(string); ok && cover != "" {
			h += 130
		}
		if text, ok := props["content"].(string); ok && text != "" {
			h += 30
		}
		if btn, ok := props["btn_text"].(string); ok && btn != "" {
			h += 40
		}
		return h, ""

	// 20. 战力/积分/成绩面板积木
	case "score_panel":
		h := 130
		if items, ok := props["items"].([]interface{}); ok && len(items) > 0 {
			rows := int(math.Ceil(float64(len(items)) / 2.0))
			h += rows * 36
		}
		return h, ""

	// 21. 优惠卡券与兑换码积木
	case "coupon_card":
		return 96, ""
	case "redeem_code_card":
		return 108, ""

	// 22. 实时倒计时积木
	case "countdown":
		return 106, ""

	// 23. 服务节点监控积木
	case "server_status":
		h := 84
		if notice, ok := props["notice"].(string); ok && notice != "" {
			h += 20
		}
		return h, ""

	// 24. 苹果风商品卡片积木
	case "product_card":
		return 118, ""

	// 25. 应用与资源下载卡片积木
	case "download_card":
		h := 68
		if desc, ok := props["desc"].(string); ok && desc != "" {
			h += 26
		} else if desc, ok := props["description"].(string); ok && desc != "" {
			h += 26
		}
		return h, ""

	// 26. 结果表格积木
	case "result_table":
		rowCount := 2
		if rows, ok := props["rows"].([]interface{}); ok && len(rows) > 0 {
			rowCount = len(rows)
		}
		return 40 + rowCount*34, ""

	// 27. 联系人与客服卡片
	case "contact_card":
		return 88, ""

	// 28. 地图与地理位置卡片
	case "map_card":
		return 160, "map_view_stub"

	// 29. 游戏头图与活动日程卡片
	case "game_header":
		return 180, ""
	case "event_card":
		return 116, ""

	// 30. 投票与问卷
	case "poll":
		optCount := 2
		if opts, ok := props["options"].([]interface{}); ok && len(opts) > 0 {
			optCount = len(opts)
		}
		return 44 + optCount*42, ""

	// 31. 信息流列表
	case "feed_list":
		itemCount := 3
		if items, ok := props["items"].([]interface{}); ok && len(items) > 0 {
			itemCount = len(items)
		}
		return itemCount * 88, ""

	// 32. 资讯栏目导航与文章流，按同一组 props 估算客户端实际内容高度。
	case "category_nav", "collection_nav":
		return 42, ""
	case "article_feed", "content_feed":
		itemCount := sliceLength(props["items_path"])
		if itemCount == 0 {
			itemCount = sliceLength(props["items"])
		}
		limit := int(toFloat64(props["limit"]))
		if limit <= 0 || limit > itemCount {
			limit = itemCount
		}
		if limit == 0 {
			limit = 1
		}
		layout, _ := props["layout"].(string)
		if layout == "featured" || layout == "feature" {
			// 图片焦点 16:9，加上栏目、标题、摘要和阅读入口。
			return int(float64(contentWidth)*9.0/16.0) + 150, ""
		}
		return limit * 86, ""

	case "membership_plan_list", "offer_list":
		count := sliceLength(props["items_path"])
		if count == 0 {
			count = sliceLength(props["items"])
		}
		if count == 0 {
			count = 1
		}
		return count * 92, ""
	case "article_detail", "content_detail":
		return 520, ""
	case "comment_thread", "discussion_thread":
		return 300, ""

	default:
		return 90, ""
	}
}

// sliceLength 返回绑定数据中的切片或数组长度。
func sliceLength(value interface{}) int {
	if value == nil {
		return 0
	}
	resolved := reflect.ValueOf(value)
	if resolved.Kind() != reflect.Slice && resolved.Kind() != reflect.Array {
		return 0
	}
	return resolved.Len()
}

// extractTextSummary 从解析后的真实属性中提炼具有业务代表性的文字摘要
func extractTextSummary(block *models.BlockItem, props map[string]interface{}) string {
	if props == nil {
		return block.Type
	}

	for _, k := range []string{"title", "text", "content", "subtitle", "desc", "message"} {
		if val, ok := props[k].(string); ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return block.Type
}

// 辅助工具: 解析像素值 (如 "24rpx" -> 12px, "16px" -> 16px)
func parsePixelValue(str string, fallback int) int {
	str = strings.TrimSpace(str)
	if str == "" {
		return fallback
	}
	if strings.HasSuffix(str, "rpx") {
		numStr := strings.TrimSuffix(str, "rpx")
		if n, err := strconv.Atoi(numStr); err == nil {
			return n / 2 // rpx 按 750rpx 设计稿换算为逻辑像素
		}
	}
	if strings.HasSuffix(str, "px") {
		numStr := strings.TrimSuffix(str, "px")
		if n, err := strconv.Atoi(numStr); err == nil {
			return n
		}
	}
	if n, err := strconv.Atoi(str); err == nil {
		return n
	}
	return fallback
}

// resolveStyleMetrics 将协议工具令牌映射为 IR 使用的统一逻辑像素，保持预览与小程序端同构。
func resolveStyleMetrics(style *models.BlockStyle, marginY, padding, radius int, glass bool) (int, int, int, bool) {
	if style == nil {
		return marginY, padding, radius, glass
	}
	if style.GlassBlur != nil {
		glass = *style.GlassBlur
	}
	for _, token := range style.Utilities {
		switch token {
		case "layout/flat":
			glass = false
		case "layout/card":
			glass = false
		case "space/y-0":
			marginY = 0
		case "space/y-2":
			marginY = 4
		case "space/y-4":
			marginY = 8
		case "space/y-6":
			marginY = 12
		case "space/y-8":
			marginY = 16
		case "space/y-10":
			marginY = 20
		case "space/y-12":
			marginY = 24
		case "padding/none", "padding/0":
			padding = 0
		case "padding/2":
			padding = 4
		case "padding/4", "padding/x-4", "padding/y-4":
			padding = 8
		case "padding/5", "padding/x-5":
			padding = 10
		case "padding/6", "padding/y-6":
			padding = 12
		case "padding/8":
			padding = 16
		case "padding/x-6":
			padding = 12
		case "radius/none":
			radius = 0
		case "radius/sm":
			radius = 4
		case "radius/md":
			radius = 8
		case "radius/lg":
			radius = 12
		case "radius/xl":
			radius = 14
		case "radius/full":
			radius = 499
		}
	}
	return marginY, padding, radius, glass
}

// resolveStyleAccent 使用与前端工具类相同的强调色值。
func resolveStyleAccent(style *models.BlockStyle, fallback string) string {
	if style == nil {
		return fallback
	}
	colors := map[string]string{"accent/blue": "#1769e0", "accent/ink": "#17191d", "accent/amber": "#c06a00", "accent/green": "#30d158"}
	for _, token := range style.Utilities {
		if color, ok := colors[token]; ok {
			fallback = color
		}
	}
	return fallback
}

// resolveStyleGap 读取容器的 gap 属性或对应工具令牌。
func resolveStyleGap(parent *models.BlockItem, fallback int) int {
	if parent == nil || parent.Style == nil {
		return fallback
	}
	for _, token := range parent.Style.Utilities {
		switch token {
		case "gap/2":
			return 4
		case "gap/4":
			return 8
		case "gap/6":
			return 12
		case "gap/8":
			return 16
		}
	}
	return fallback
}

// 辅助工具: 通用安全转换为 float64
func toFloat64(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}

// 辅助工具: 浅克隆 map
func shallowCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return make(map[string]interface{})
	}
	res := make(map[string]interface{}, len(m))
	for k, v := range m {
		res[k] = v
	}
	return res
}

// compareLooseEqual 比较两个受控值的等值性，与前端 isLooseEqual 绝对同构对齐
func compareLooseEqual(l, r interface{}) bool {
	if l == nil && r == nil {
		return true
	}
	if l == nil || r == nil {
		return false
	}
	// 布尔比较
	if lb, ok := toBool(l); ok {
		if rb, ok := toBool(r); ok {
			return lb == rb
		}
	}
	// 数值比较
	if ln, ok := toFloatStrict(l); ok {
		if rn, ok := toFloatStrict(r); ok {
			return ln == rn
		}
	}
	return fmt.Sprintf("%v", l) == fmt.Sprintf("%v", r)
}

func toBool(val interface{}) (bool, bool) {
	if b, ok := val.(bool); ok {
		return b, true
	}
	if s, ok := val.(string); ok {
		lower := strings.ToLower(strings.TrimSpace(s))
		if lower == "true" {
			return true, true
		}
		if lower == "false" {
			return false, true
		}
	}
	return false, false
}

func toFloatStrict(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
