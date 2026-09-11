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
								{ID: "lab_category_nav", Type: "category_nav", Props: map[string]interface{}{"title": "资讯栏目", "items": []interface{}{map[string]interface{}{"slug": "news", "name": "最新资讯"}}}},
								{ID: "lab_article_feed", Type: "article_feed", Props: map[string]interface{}{"title": "文章列表", "items": []interface{}{map[string]interface{}{"id": 1, "title": "测试文章", "summary": "组件实验"}}}},
								{ID: "lab_article_detail", Type: "article_detail", Props: map[string]interface{}{"title": "文章详情"}},
								{ID: "lab_membership_plans", Type: "membership_plan_list", Props: map[string]interface{}{"title": "会员套餐", "items": []interface{}{map[string]interface{}{"level": 1, "name": "普通会员", "price_fen": 990, "duration_days": 30, "sku": "lab-member-1"}}}},
								{ID: "lab_comment_thread", Type: "comment_thread", Props: map[string]interface{}{"title": "评论线程"}},
								{ID: "lab_collection_nav", Type: "collection_nav", Props: map[string]interface{}{"title": "通用集合导航", "items": []interface{}{map[string]interface{}{"key": "all", "label": "全部"}}, "fields": map[string]interface{}{"key": "key", "label": "label"}}},
								{ID: "lab_content_feed", Type: "content_feed", Props: map[string]interface{}{"title": "通用内容流", "items": []interface{}{map[string]interface{}{"key": "content-1", "title": "通用内容", "summary": "字段映射与动作均来自协议", "meta": "验证中"}}, "fields": map[string]interface{}{"key": "key", "title": "title", "summary": "summary", "meta": "meta"}}},
								{ID: "lab_content_detail", Type: "content_detail", Props: map[string]interface{}{"title": "通用详情", "resource_url": ""}},
								{ID: "lab_offer_list", Type: "offer_list", Props: map[string]interface{}{"title": "通用套餐", "items": []interface{}{map[string]interface{}{"key": "offer-1", "title": "验证方案", "price": "¥9.90"}}, "fields": map[string]interface{}{"key": "key", "title": "title", "price": "price"}}},
								{ID: "lab_discussion_thread", Type: "discussion_thread", Props: map[string]interface{}{"title": "通用讨论", "list_url": ""}},
								{ID: "lab_media_picker", Type: "media_picker", Props: map[string]interface{}{"title": "媒体选择", "max_count": 3, "hint": "支持相册与相机"}},
								{ID: "lab_upload_progress", Type: "upload_progress", Props: map[string]interface{}{"title": "上传进度", "progress": 64, "status_text": "正在上传"}},
								{ID: "lab_media_gallery", Type: "media_gallery", Props: map[string]interface{}{"title": "媒体画廊", "items": []interface{}{map[string]interface{}{"id": "cover", "url": "/assets/sdui-component-lab.png"}}}},
								{ID: "lab_location_picker", Type: "location_picker", Props: map[string]interface{}{"title": "选择服务位置", "address": "深圳市", "latitude": 22.5431, "longitude": 114.0579}},
								{ID: "lab_service_entry", Type: "service_entry", Props: map[string]interface{}{"title": "微信客服", "description": "验证微信内置客服入口", "mode": "wechat"}},
								{ID: "lab_webview_entry", Type: "webview_entry", Props: map[string]interface{}{"title": "网页业务", "btn_text": "打开"}, Action: &models.BlockAction{Type: "open_webview", Payload: map[string]interface{}{"url_key": "component-lab"}}},
								{ID: "lab_webview_state", Type: "webview_state", Props: map[string]interface{}{"title": "网页业务暂未开放", "disabled": true}},
								{ID: "lab_chat_thread", Type: "chat_thread", Props: map[string]interface{}{"title": "在线客服", "status": "沙箱连接", "unread_count": 2, "messages": []interface{}{map[string]interface{}{"id": "m1", "sender_name": "客服", "content": "您好，需要什么帮助？"}, map[string]interface{}{"id": "m2", "mine": true, "content": "验证聊天组件"}}}},
								{ID: "lab_message_list", Type: "message_list", Props: map[string]interface{}{"title": "消息列表", "messages": []interface{}{map[string]interface{}{"id": "m3", "content": "列表消息"}}}},
								{ID: "lab_message_composer", Type: "message_composer", Props: map[string]interface{}{"placeholder": "输入测试消息"}},
								{ID: "lab_unread_badge", Type: "unread_badge", Props: map[string]interface{}{"count": 3}},
								{ID: "lab_order_card", Type: "order_card", Props: map[string]interface{}{"title": "测试订单", "order_no": "LAB-001", "status": "pending", "status_text": "待处理", "action_text": "确认订单"}},
								{ID: "lab_order_summary", Type: "order_summary", Props: map[string]interface{}{"title": "订单摘要", "total": "¥9.90", "action_text": "创建订单"}},
								{ID: "lab_order_timeline", Type: "order_timeline", Props: map[string]interface{}{"title": "订单进度", "nodes": []interface{}{map[string]interface{}{"title": "已创建", "time": "10:00", "active": true}, map[string]interface{}{"title": "待处理", "time": "10:05"}}}},
								{ID: "lab_logistics_track", Type: "logistics_track", Props: map[string]interface{}{"title": "物流轨迹", "company": "测试物流", "tracking_no": "LAB-EXPRESS", "tracks": []interface{}{map[string]interface{}{"status": "已揽收", "time": "10:20", "location": "深圳"}}}},
								{ID: "lab_after_sale_form", Type: "after_sale_form", Props: map[string]interface{}{"title": "售后申请", "order_id": "LAB-001"}},
								{ID: "lab_evidence_list", Type: "evidence_list", Props: map[string]interface{}{"title": "售后凭证", "allow_upload": true, "evidence": []interface{}{map[string]interface{}{"name": "订单截图", "status": "已提交"}}}},
								{ID: "lab_service_card", Type: "service_card", Props: map[string]interface{}{"title": "斗师服务", "description": "通用服务卡片", "price": "¥99", "action_text": "接受服务"}},
								{ID: "lab_task_card", Type: "task_card", Props: map[string]interface{}{"task_title": "待接任务", "description": "服务任务沙箱", "action_text": "接受任务"}},
								{ID: "lab_quote_card", Type: "quote_card", Props: map[string]interface{}{"title": "服务报价", "price": "¥199", "valid_until": "今日 18:00", "action_text": "提交报价"}},
								{ID: "lab_schedule_picker", Type: "schedule_picker", Props: map[string]interface{}{"title": "服务排期", "placeholder": "输入时间"}},
								{ID: "lab_membership_card", Type: "membership_card", Props: map[string]interface{}{"title": "高级会员", "status": "active", "expire_at": "2027-01-01", "benefits": []interface{}{"专属内容", "优先客服"}, "action_text": "查看会员"}},
								{ID: "lab_ad_slot", Type: "ad_slot", Props: map[string]interface{}{"slot_id": "lab-ad", "title": "测试推荐", "description": "受控广告位沙箱"}},
								{ID: "lab_wallet_card", Type: "wallet_card", Props: map[string]interface{}{"title": "可用余额", "balance": "¥128.00", "action_text": "刷新余额"}},
								{ID: "lab_withdraw_form", Type: "withdraw_form", Props: map[string]interface{}{"title": "申请提现", "placeholder": "输入金额", "submit_text": "提交申请"}},
								{ID: "lab_payment_result", Type: "payment_result", Props: map[string]interface{}{"title": "支付结果", "content": "沙箱支付状态"}},
								{ID: "lab_price_breakdown", Type: "price_breakdown", Props: map[string]interface{}{"title": "价格明细", "content": "商品价格与服务费"}},
								{ID: "lab_address_card", Type: "address_card", Props: map[string]interface{}{"title": "服务地址", "content": "深圳市南山区"}},
								{ID: "lab_qr_code", Type: "qr_code", Props: map[string]interface{}{"title": "客服二维码", "content": "已登记二维码"}},
								{ID: "lab_feature_gate", Type: "feature_gate", Props: map[string]interface{}{"title": "功能开关", "content": "由租户能力矩阵控制显示"}},
								{ID: "lab_compliance_panel", Type: "compliance_panel", Props: map[string]interface{}{"title": "合规提示", "content": "当前为本地验收环境"}},
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
					{ID: "lab_bottom_nav", Type: "bottom_nav", Props: map[string]interface{}{"active_key": "lab", "items": []interface{}{map[string]interface{}{"key": "lab", "label": "实验室", "page_id": "component_lab"}, map[string]interface{}{"key": "home", "label": "首页", "page_id": "home"}}}},
					{ID: "lab_floating_action", Type: "floating_action", Props: map[string]interface{}{"label": "客服", "position": "right"}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "customer_service"}}},
					{
						ID: "lab_native_actions", Type: "container",
						Props: map[string]interface{}{"children": componentLabActionBlocks()},
					},
				},
			},
			Style: &models.BlockStyle{Utilities: []string{"padding/5", "radius/lg", "accent/blue"}, GlassBlur: models.Bool(true)},
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

// componentLabActionBlocks 为每个协议动作提供独立入口，单项失败不会阻断其余验收。
func componentLabActionBlocks() []models.BlockItem {
	actions := []models.BlockAction{
		{Type: "copy_text", Payload: map[string]interface{}{"text": "SDUI-COPY"}},
		{Type: "toast", Payload: map[string]interface{}{"text": "SDUI-TOAST"}},
		{Type: "set_state", Payload: map[string]interface{}{"key": "show_extended", "value": true}},
		{Type: "toggle_state", Payload: map[string]interface{}{"key": "show_extended"}},
		{Type: "reset_state"},
		{Type: "show_loading_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
		{Type: "show_empty_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
		{Type: "show_error_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
		{Type: "reset_block_state", Payload: map[string]interface{}{"target": "lab_state_target"}},
		{Type: "refresh", Payload: map[string]interface{}{"target": "lab_state_target"}},
		{Type: "request", Payload: map[string]interface{}{"endpoint": "query.score", "body": map[string]interface{}{"query_value": "SDUI-QUERY"}, "response": map[string]interface{}{"save_as": "lab_request"}}},
		{Type: "request_data", Payload: map[string]interface{}{"endpoint": "query.score", "body": map[string]interface{}{"query_value": "SDUI-QUERY"}, "response": map[string]interface{}{"save_as": "lab_request"}}},
		{Type: "require_auth"},
		{Type: "share"},
		{Type: "preview_image", Payload: map[string]interface{}{"urls": []string{"/assets/sdui-component-lab.png"}}},
		{Type: "open_mini_program", Payload: map[string]interface{}{"target_app_id": "wx0000000000000000"}},
		{Type: "open_channels_activity", Payload: map[string]interface{}{"feed_id": "export/component-lab", "finder_user_name": "gh_component_lab"}},
		{Type: "subscribe_message", Payload: map[string]interface{}{"template_id": "lab_notice_template"}},
		{Type: "request_payment", Payload: map[string]interface{}{"sku": "sdui-lab-sku"}},
		{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "home"}},
		{Type: "open_webview", Payload: map[string]interface{}{"url_key": "component-lab"}},
		{Type: "choose_media", Payload: map[string]interface{}{"count": 1, "media_type": []interface{}{"image"}}},
		{Type: "upload_file", Payload: map[string]interface{}{"file_path": "$result.temp_files.0.tempFilePath", "presigned_url": "https://upload.example.com/presigned", "upload_headers": map[string]interface{}{"Content-Type": "image/png"}}},
		{Type: "delete_media", Payload: map[string]interface{}{"endpoint": "media.delete", "body": map[string]interface{}{"media_id": "lab-media"}}},
		{Type: "request_location"},
		{Type: "choose_location"},
		{Type: "open_map", Payload: map[string]interface{}{"latitude": 22.5431, "longitude": 114.0579, "name": "深圳", "scale": 16}},
		{Type: "open_wechat_service", Payload: map[string]interface{}{"corp_id": "ww_component_lab", "url": "https://work.weixin.qq.com/kfid/lab"}},
		{Type: "save_qr", Payload: map[string]interface{}{"url": "https://example.com/customer-service.png"}},
		{Type: "open_internal_chat", Payload: map[string]interface{}{"page_id": "customer_service", "context": map[string]interface{}{"source": "component_lab"}}},
		{Type: "send_message", Payload: map[string]interface{}{"thread_id": "lab-thread", "content": "测试消息"}},
		{Type: "mark_read", Payload: map[string]interface{}{"thread_id": "lab-thread"}},
		{Type: "poll_messages", Payload: map[string]interface{}{"thread_id": "lab-thread"}},
		{Type: "connect_message", Payload: map[string]interface{}{"thread_id": "lab-thread"}},
		{Type: "upload_chat_media", Payload: map[string]interface{}{"thread_id": "lab-thread", "media_id": "lab-media"}},
		{Type: "create_order", Payload: map[string]interface{}{"sku": "sdui-lab-sku"}},
		{Type: "confirm_order", Payload: map[string]interface{}{"order_id": "lab-order"}},
		{Type: "cancel_order", Payload: map[string]interface{}{"order_id": "lab-order", "reason": "组件实验"}},
		{Type: "confirm_receipt", Payload: map[string]interface{}{"order_id": "lab-order"}},
		{Type: "refresh_logistics", Payload: map[string]interface{}{"order_id": "lab-order"}},
		{Type: "apply_after_sale", Payload: map[string]interface{}{"order_id": "lab-order", "reason": "组件实验"}},
		{Type: "upload_evidence", Payload: map[string]interface{}{"order_id": "lab-order", "media_id": "lab-media"}},
		{Type: "accept_task", Payload: map[string]interface{}{"task_id": "lab-task"}},
		{Type: "reject_task", Payload: map[string]interface{}{"task_id": "lab-task", "reason": "组件实验"}},
		{Type: "submit_quote", Payload: map[string]interface{}{"task_id": "lab-task", "price": 199}},
		{Type: "update_service_status", Payload: map[string]interface{}{"task_id": "lab-task", "status": "in_service"}},
		{Type: "open_membership", Payload: map[string]interface{}{"plan_id": "lab-plan"}},
		{Type: "load_ad", Payload: map[string]interface{}{"slot_id": "lab-ad"}},
		{Type: "refresh_wallet", Payload: map[string]interface{}{}},
		{Type: "request_withdraw", Payload: map[string]interface{}{"amount": "10.00"}},
	}
	blocks := []models.BlockItem{{ID: "lab_action_result", Type: "text", Props: map[string]interface{}{"content": "动作结果 {{$state.lab_action}} / 请求 {{$state.lab_request.status}}"}}}
	for _, action := range actions {
		action.OnSuccess = []models.BlockAction{{Type: "set_state", Payload: map[string]interface{}{"key": "lab_action", "value": "通过:" + action.Type}}}
		action.OnError = []models.BlockAction{{Type: "set_state", Payload: map[string]interface{}{"key": "lab_action", "value": "失败:" + action.Type}}}
		blocks = append(blocks, models.BlockItem{ID: "lab_action_" + action.Type, Type: "action_button", Props: map[string]interface{}{"text": "验收 " + action.Type}, Action: &action})
	}
	return blocks
}
