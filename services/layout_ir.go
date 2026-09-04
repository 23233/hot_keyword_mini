// Package services layout_ir.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/models"
	"math"
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
	}
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
			nodeCtx := shallowCopyMap(context)

			// 2. 状态多态分支处理 (loading / empty / error)
			targetBlock := resolveBlockStateVariant(itemBlock, stateFixture)

			// 3. 执行受控数据绑定深度求值 (解析 $entity.*, $query.*, $item.*, $state.* 等)
			resolvedProps := ResolveBlockPropsBindings(targetBlock.Props, nodeCtx)
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
			accentColor := page.AccentColor

			if targetBlock.Style != nil {
				glassBlur = targetBlock.Style.GlassBlur
				if targetBlock.Style.AccentColor != "" {
					accentColor = targetBlock.Style.AccentColor
				}
				if targetBlock.Style.MarginY != "" {
					marginY = parsePixelValue(targetBlock.Style.MarginY, 8)
				}
				if targetBlock.Style.Padding != "" {
					padding = parsePixelValue(targetBlock.Style.Padding, 14)
				}
				if targetBlock.Style.BorderRadius != "" {
					borderRadius = parsePixelValue(targetBlock.Style.BorderRadius, 14)
				}
			}

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

			// 布局块递归生成子节点，确保容器内图片、文本和按钮保留同一棵渲染树
			if children := extractNestedBlocks(targetBlock.Props); len(children) > 0 {
				var childHeight int
				node.Children, childHeight = buildNestedLayoutNodes(children, nodeCtx, device, accentColor, stateFixture, node.BoundingBox.X+padding, node.BoundingBox.Y+padding, contentWidth-padding*2, 1)
				if childHeight+padding*2 > node.BoundingBox.Height {
					node.BoundingBox.Height = childHeight + padding*2
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
	case "error", "offline", "out_of_stock":
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
	if itemsRaw, ok := block.Repeat["items"]; ok {
		if itemsSlice, ok := itemsRaw.([]interface{}); ok {
			repeatList = itemsSlice
		}
	} else if pathVal, ok := block.Repeat["path"].(string); ok {
		resolved := ResolveBindingValue(pathVal, context)
		if slice, ok := resolved.([]interface{}); ok {
			repeatList = slice
		}
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

		itemCtx := shallowCopyMap(context)
		itemCtx["$item"] = itemData
		itemCtx["item"] = itemData

		resolvedProps := ResolveBlockPropsBindings(clonedProps, itemCtx)
		cloned.Props = resolvedProps
		results = append(results, cloned)
	}

	return results
}

// ResolveBindingValue 解析单个受控路径绑定值 (如 $entity.title, $query.id, $item.name)
func ResolveBindingValue(val interface{}, context map[string]interface{}) interface{} {
	if val == nil {
		return nil
	}

	// 1. 处理对象形式: { "path": "$entity.title" }
	if m, ok := val.(map[string]interface{}); ok {
		if pathStr, ok := m["path"].(string); ok && pathStr != "" {
			return resolvePathString(pathStr, context)
		}
	}

	// 2. 处理直接字符串形式: "$entity.title" 或 "{{entity.title}}"
	if s, ok := val.(string); ok {
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "$") {
			return resolvePathString(trimmed, context)
		}
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
			innerPath := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			if !strings.HasPrefix(innerPath, "$") {
				innerPath = "$" + innerPath
			}
			return resolvePathString(innerPath, context)
		}
	}

	return val
}

// resolvePathString 根据点号分割路径提取上下文深度属性
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

	// 支持直接 $xxx 查找，也支持无 $ 别名兼容
	if val, ok := context[rootKey]; ok {
		current = val
	} else if val, ok := context[strings.TrimPrefix(rootKey, "$")]; ok {
		current = val
	} else {
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
		if pathStr, ok := m["path"].(string); ok && len(m) == 1 && strings.HasPrefix(pathStr, "$") {
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
func ResolveBlockPropsBindings(props map[string]interface{}, context map[string]interface{}) map[string]interface{} {
	if props == nil {
		return map[string]interface{}{}
	}
	resolved, ok := resolvePropsPreservingBlocks(props, context).(map[string]interface{})
	if !ok {
		return props
	}
	return resolved
}

func resolvePropsPreservingBlocks(value interface{}, context map[string]interface{}) interface{} {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		// 子 block 的 props/条件需要在其自身上下文中求值，不能在父容器提前消费。
		if _, hasType := m["type"]; hasType {
			return m
		}
		if path, isBinding := m["path"].(string); isBinding && len(m) == 1 && strings.HasPrefix(path, "$") {
			return ResolveObjectBindings(m, context)
		}
		result := make(map[string]interface{}, len(m))
		for k, v := range m {
			result[k] = resolvePropsPreservingBlocks(v, context)
		}
		return result
	}
	if list, ok := value.([]interface{}); ok {
		result := make([]interface{}, len(list))
		for i, item := range list {
			result[i] = resolvePropsPreservingBlocks(item, context)
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
	// tabs 只展开默认首个 tab，与小程序初始 activeKey 行为保持一致。
	if rawTabs, ok := props["tabs"].([]interface{}); ok && len(rawTabs) > 0 {
		if tab, ok := rawTabs[0].(map[string]interface{}); ok {
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

// buildNestedLayoutNodes 递归构建布局子树。子树使用与顶层相同的绑定、条件、状态和尺寸规则。
func buildNestedLayoutNodes(blocks []models.BlockItem, context map[string]interface{}, device DeviceParams, accentColor, stateFixture string, x, y, width, depth int) ([]BlockLayoutNode, int) {
	if depth > 32 || width <= 0 {
		return nil, 0
	}
	result := make([]BlockLayoutNode, 0, len(blocks))
	currentY := y
	for _, block := range blocks {
		for _, expanded := range ExpandBlockRepeat(block, context) {
			nodeCtx := shallowCopyMap(context)
			target := resolveBlockStateVariant(expanded, stateFixture)
			props := ResolveBlockPropsBindings(target.Props, nodeCtx)
			visible := target.VisibleWhen == nil || EvaluateCondition(target.VisibleWhen, nodeCtx)
			height, nativeStub := CalculateAdaptiveBlockHeight(&target, props, width)
			marginY, padding, radius, glass := 8, 14, 14, true
			if target.Style != nil {
				glass = target.Style.GlassBlur
				marginY = parsePixelValue(target.Style.MarginY, marginY)
				padding = parsePixelValue(target.Style.Padding, padding)
				radius = parsePixelValue(target.Style.BorderRadius, radius)
			}
			child := BlockLayoutNode{
				ID: target.ID, Type: target.Type, Props: props, Visible: visible,
				BoundingBox: BoundingBox{X: x, Y: currentY + marginY, Width: width, Height: height},
				MarginY:     marginY, Padding: padding, BorderRadius: radius, GlassBlur: glass,
				AccentColor: accentColor, TextSummary: extractTextSummary(&target, props),
				Action: target.Action, Events: target.Events, NativeStub: nativeStub,
				Loading: target.Loading, Empty: target.Empty, Error: target.Error, Fallback: target.Fallback,
			}
			if target.Action != nil {
				child.ActionType = target.Action.Type
			}
			if nested := extractNestedBlocks(props); len(nested) > 0 {
				var childHeight int
				child.Children, childHeight = buildNestedLayoutNodes(nested, nodeCtx, device, accentColor, stateFixture, x+padding, child.BoundingBox.Y+padding, width-padding*2, depth+1)
				if childHeight+padding*2 > child.BoundingBox.Height {
					child.BoundingBox.Height = childHeight + padding*2
				}
			}
			result = append(result, child)
			if visible {
				currentY += height + marginY*2
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

	// 等值比较: eq
	if eqRaw, ok := m["eq"]; ok {
		if slice, ok := eqRaw.([]interface{}); ok && len(slice) >= 2 {
			l := ResolveBindingValue(slice[0], context)
			r := ResolveBindingValue(slice[1], context)
			return fmt.Sprintf("%v", l) == fmt.Sprintf("%v", r)
		}
	}

	// 不等比较: neq
	if neqRaw, ok := m["neq"]; ok {
		if slice, ok := neqRaw.([]interface{}); ok && len(slice) >= 2 {
			l := ResolveBindingValue(slice[0], context)
			r := ResolveBindingValue(slice[1], context)
			return fmt.Sprintf("%v", l) != fmt.Sprintf("%v", r)
		}
	}

	// 存在性检查: exists
	if existsRaw, ok := m["exists"]; ok {
		val := ResolveBindingValue(existsRaw, context)
		return val != nil && fmt.Sprintf("%v", val) != ""
	}

	// 集合包含: in
	if inRaw, ok := m["in"]; ok {
		if slice, ok := inRaw.([]interface{}); ok && len(slice) >= 2 {
			item := ResolveBindingValue(slice[0], context)
			list := ResolveBindingValue(slice[1], context)
			itemStr := fmt.Sprintf("%v", item)
			if arr, ok := list.([]interface{}); ok {
				for _, el := range arr {
					if fmt.Sprintf("%v", el) == itemStr {
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
			l := toFloat64(ResolveBindingValue(slice[0], context))
			r := toFloat64(ResolveBindingValue(slice[1], context))
			return l > r
		}
	}
	if gteRaw, ok := m["gte"]; ok {
		if slice, ok := gteRaw.([]interface{}); ok && len(slice) >= 2 {
			l := toFloat64(ResolveBindingValue(slice[0], context))
			r := toFloat64(ResolveBindingValue(slice[1], context))
			return l >= r
		}
	}
	if ltRaw, ok := m["lt"]; ok {
		if slice, ok := ltRaw.([]interface{}); ok && len(slice) >= 2 {
			l := toFloat64(ResolveBindingValue(slice[0], context))
			r := toFloat64(ResolveBindingValue(slice[1], context))
			return l < r
		}
	}
	if lteRaw, ok := m["lte"]; ok {
		if slice, ok := lteRaw.([]interface{}); ok && len(slice) >= 2 {
			l := toFloat64(ResolveBindingValue(slice[0], context))
			r := toFloat64(ResolveBindingValue(slice[1], context))
			return l <= r
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
		return 116, ""

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

	default:
		return 90, ""
	}
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
