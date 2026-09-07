// Package services component_lab_template.go
package services

import "hot_keyword/models"

// buildComponentLabTemplate 构建覆盖所有标准积木、嵌套、循环、状态和事件链的验收模板。
func buildComponentLabTemplate() *SDUITemplate {
	stateTarget := models.BlockItem{
		ID:   "lab_state_target",
		Type: "custom",
		Props: map[string]interface{}{
			"title":   "异步状态容器",
			"content": "触发上方事件可切换加载、空态和错误态。",
		},
		Loading: &models.BlockItem{ID: "lab_state_loading", Type: "skeleton"},
		Empty:   &models.BlockItem{ID: "lab_state_empty", Type: "empty", Props: map[string]interface{}{"title": "暂无实验数据"}},
		Error:   &models.BlockItem{ID: "lab_state_error", Type: "custom", Props: map[string]interface{}{"title": "状态请求失败", "content": "请重试"}},
	}

	blocks := []models.BlockItem{
		{
			ID:   "lab_root",
			Type: "container",
			Props: map[string]interface{}{
				"direction": "column",
				"gap":       "16rpx",
				"children": []models.BlockItem{
					{
						ID:    "lab_notice",
						Type:  "notice",
						Props: map[string]interface{}{"icon": "🧪", "text": "SDUI 全组件组合、嵌套与事件回归页面"},
					},
					{
						ID:    "lab_text",
						Type:  "text",
						Props: map[string]interface{}{"title": "后端 IR 与小程序渲染对齐", "content": "绑定: {{$page.title}}"},
					},
					{
						ID:    "lab_rich_text",
						Type:  "rich_text",
						Props: map[string]interface{}{"content": "<b>富文本</b>、图片、视频和布局块均由同一协议下发。"},
					},
					{
						ID:     "lab_image",
						Type:   "image",
						Props:  map[string]interface{}{"url": "/assets/sdui-component-lab.png", "alt": "组件实验图"},
						Action: &models.BlockAction{Type: "preview_image", Payload: map[string]interface{}{"urls": []string{"/assets/sdui-component-lab.png"}}},
					},
					{
						ID:    "lab_video",
						Type:  "video",
						Props: map[string]interface{}{"src": "https://media.w3.org/2010/05/sintel/trailer.mp4", "title": "视频积木"},
					},
					{
						ID:    "lab_state_actions",
						Type:  "action_button",
						Props: map[string]interface{}{"text": "执行状态与事件链"},
						Events: map[string][]models.BlockAction{
							"tap": {
								{Type: "set_state", Payload: map[string]interface{}{"key": "show_extended", "value": true}},
								{Type: "show_loading_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
								{Type: "reset_block_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
								{Type: "toast", Payload: map[string]interface{}{"text": "事件序列已完成"}},
							},
						},
					},
					stateTarget,
					{
						ID:    "lab_repeat",
						Type:  "custom",
						Props: map[string]interface{}{"title": "$item.title", "content": "序号 {{$item.index}} · {{$item.code}}"},
						Repeat: map[string]interface{}{"items": []interface{}{
							map[string]interface{}{"title": "循环项 A", "index": 1, "code": "A-001"},
							map[string]interface{}{"title": "循环项 B", "index": 2, "code": "B-002"},
						}},
						Action: &models.BlockAction{Type: "copy_text", Payload: map[string]interface{}{"text": "$item.code", "toast": "循环项编码已复制"}},
					},
					{
						ID:          "lab_visible_result",
						Type:        "custom_block",
						Props:       map[string]interface{}{"title": "条件渲染结果", "content": "show_extended 已开启"},
						VisibleWhen: map[string]interface{}{"path": "$state.show_extended", "eq": true},
					},
					{
						ID:   "lab_stack",
						Type: "stack",
						Props: map[string]interface{}{
							"direction": "row",
							"gap":       "12rpx",
							"children": []models.BlockItem{
								{ID: "lab_stack_left", Type: "custom", Props: map[string]interface{}{"title": "横向堆叠"}},
								{ID: "lab_stack_right", Type: "custom", Props: map[string]interface{}{"title": "共享上下文"}},
							},
						},
					},
					{
						ID:   "lab_grid",
						Type: "grid",
						Props: map[string]interface{}{
							"columns": 2,
							"gap":     "12rpx",
							"children": []models.BlockItem{
								{ID: "lab_media_hero", Type: "media_hero", Props: map[string]interface{}{"title": "媒体焦点", "subtitle": "行业模板共享组件"}},
								{ID: "lab_resource", Type: "resource_card", Props: map[string]interface{}{"title": "资源卡片", "fetch_code": "LAB-2026"}, Action: &models.BlockAction{Type: "copy_text", Payload: map[string]interface{}{"text": "LAB-2026"}}},
								{ID: "lab_game", Type: "game_card", Props: map[string]interface{}{"title": "游戏礼包", "redeem_code": "GIFT-LAB"}},
								{ID: "lab_form", Type: "form", Props: map[string]interface{}{"title": "查询表单", "placeholder": "输入测试值"}, Action: &models.BlockAction{Type: "request_data", Payload: map[string]interface{}{"endpoint": "query.score"}}},
								{ID: "lab_episode", Type: "episode_list", Props: map[string]interface{}{"title": "选集", "episodes": []interface{}{map[string]interface{}{"title": "第 1 集"}, map[string]interface{}{"title": "第 2 集"}}}},
								{ID: "lab_item_grid", Type: "item_grid", Props: map[string]interface{}{"title": "项目网格", "items": []interface{}{map[string]interface{}{"title": "项目一"}, map[string]interface{}{"title": "项目二"}}}},
								{ID: "lab_timeline", Type: "timeline", Props: map[string]interface{}{"nodes": []interface{}{map[string]interface{}{"title": "创建", "time": "09:00"}, map[string]interface{}{"title": "验证", "time": "10:00"}}}},
								{ID: "lab_score", Type: "score_panel", Props: map[string]interface{}{"title": "综合评分", "score": 98, "level": "A+"}},
								{ID: "lab_coupon", Type: "coupon_card", Props: map[string]interface{}{"title": "验证券", "code": "LAB-COUPON"}},
								{ID: "lab_countdown", Type: "countdown", Props: map[string]interface{}{"title": "活动倒计时", "target_at": "2099-01-01T00:00:00Z"}},
								{ID: "lab_result_table", Type: "result_table", Props: map[string]interface{}{"title": "结果表", "columns": []interface{}{map[string]interface{}{"title": "项目", "key": "name"}, map[string]interface{}{"title": "值", "key": "value"}}, "rows": []interface{}{map[string]interface{}{"name": "渲染", "value": "通过"}}}},
								{ID: "lab_contact", Type: "contact_card", Props: map[string]interface{}{"name": "测试支持", "phone": "400-000-0000"}},
								{ID: "lab_map", Type: "map_card", Props: map[string]interface{}{"title": "服务位置", "address": "深圳"}},
								{ID: "lab_game_header", Type: "game_header", Props: map[string]interface{}{"title": "游戏活动中心", "subtitle": "全量组件验证"}},
								{ID: "lab_redeem", Type: "redeem_code_card", Props: map[string]interface{}{"title": "兑换码", "code": "REDEEM-LAB"}},
								{ID: "lab_server", Type: "server_status", Props: map[string]interface{}{"server_name": "华南测试节点", "status": "smooth", "latency": 28}},
								{ID: "lab_product", Type: "product_card", Props: map[string]interface{}{"title": "验证商品", "sku": "sdui-lab-sku", "price": "1.00"}, Action: &models.BlockAction{Type: "request_payment", Payload: map[string]interface{}{"sku": "sdui-lab-sku"}}},
								{ID: "lab_download", Type: "download_card", Props: map[string]interface{}{"title": "测试下载", "download_url": "https://example.com/lab"}},
								{ID: "lab_event", Type: "event_card", Props: map[string]interface{}{"title": "订阅活动"}, Action: &models.BlockAction{Type: "subscribe_message", Payload: map[string]interface{}{"template_id": "lab_notice_template"}}},
								{ID: "lab_poll", Type: "poll", Props: map[string]interface{}{"title": "你看到页面了吗？", "options": []interface{}{"看到了", "继续验证"}}},
								{ID: "lab_feed", Type: "feed_list", Props: map[string]interface{}{"title": "动态流", "items": []interface{}{map[string]interface{}{"title": "状态正常"}}}},
							},
						},
					},
					{
						ID:   "lab_tabs",
						Type: "tabs",
						Props: map[string]interface{}{
							"default_active_key": "layout",
							"tabs": []interface{}{
								map[string]interface{}{"key": "layout", "title": "布局", "blocks": []interface{}{
									map[string]interface{}{"id": "lab_tab_list", "type": "list", "props": map[string]interface{}{"children": []interface{}{map[string]interface{}{"id": "lab_tab_list_text", "type": "text", "props": map[string]interface{}{"content": "列表内嵌文本"}}}}},
									map[string]interface{}{"id": "lab_tab_carousel", "type": "carousel", "props": map[string]interface{}{"items": []interface{}{map[string]interface{}{"id": "lab_slide_a", "type": "custom", "props": map[string]interface{}{"title": "轮播 A"}}, map[string]interface{}{"id": "lab_slide_b", "type": "custom", "props": map[string]interface{}{"title": "轮播 B"}}}}},
								}},
								map[string]interface{}{"key": "state", "title": "状态", "blocks": []interface{}{
									map[string]interface{}{"id": "lab_tab_empty", "type": "empty", "props": map[string]interface{}{"title": "空态"}},
									map[string]interface{}{"id": "lab_tab_skeleton", "type": "skeleton"},
								}},
							},
						},
						Events: map[string][]models.BlockAction{
							"change": {{Type: "toggle_state", Payload: map[string]interface{}{"key": "tab_changed"}}},
						},
					},
					{ID: "lab_spacer", Type: "spacer", Props: map[string]interface{}{"height": "32rpx"}},
					{
						ID:    "lab_native_actions",
						Type:  "action_button",
						Props: map[string]interface{}{"text": "动作协议覆盖"},
						Events: map[string][]models.BlockAction{
							"tap": {
								{Type: "request", Payload: map[string]interface{}{"url": "/api/v1/action/execute", "method": "POST"}},
				{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "component_lab"}},
								{Type: "open_webview", Payload: map[string]interface{}{"url": "https://example.com"}},
								{Type: "open_mini_program", Payload: map[string]interface{}{"target_app_id": "wx0000000000000000"}},
								{Type: "open_channels_activity", Payload: map[string]interface{}{"feed_id": "export/component-lab", "finder_user_name": "gh_component_lab"}},
								{Type: "require_auth"},
								{Type: "share"},
								{Type: "refresh"},
								{Type: "reset_state"},
								{Type: "show_empty_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
								{Type: "show_error_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
							},
						},
					},
				},
			},
			Style: &models.BlockStyle{Padding: "20rpx", BorderRadius: "24rpx", GlassBlur: true, AccentColor: "#0A84FF"},
		},
	}

	return &SDUITemplate{
		TemplateID:         "tpl_sdui_component_lab",
		TemplateVersion:    "1.0.0",
		Name:               "SDUI 全组件验证模板",
		BusinessType:       "custom",
		Intent:             "watch",
		Description:        "覆盖全部标准积木、嵌套、循环、条件、状态和事件链，仅用于开发与验收。",
		DefaultTheme:       "dark_glass",
		DefaultAccentColor: "#0A84FF",
		DefaultBlocks:      blocks,
	}
}
