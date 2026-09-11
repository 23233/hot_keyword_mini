// Package services mcp_contracts.go
package services

// mcpBlockContracts 返回 AI 编排页面时可直接消费的完整积木契约。
func mcpBlockContracts() map[string]interface{} {
	contracts := map[string]interface{}{}
	add := func(names []string, purpose string, props []string, example map[string]interface{}) {
		for _, name := range names {
			contracts[name] = map[string]interface{}{"purpose": purpose, "props": props, "example": example}
		}
	}

	add([]string{"container", "stack", "list"}, "嵌套排列子积木；支持普通流式布局或 overlap/zstack 层叠", []string{"children|blocks|items: BlockItem[]", "direction: row|column", "mode: overlap|zstack", "gap", "align", "justify", "wrap"}, map[string]interface{}{"children": []interface{}{map[string]interface{}{"id": "title", "type": "text", "props": map[string]interface{}{"text": "标题"}}}, "direction": "column", "gap": "16rpx"})
	add([]string{"grid"}, "按 1-4 列排列嵌套子积木", []string{"children|blocks|items: BlockItem[]", "columns: 1..4", "gap", "align", "justify"}, map[string]interface{}{"columns": 2, "children": []interface{}{map[string]interface{}{"id": "cell", "type": "text", "props": map[string]interface{}{"text": "单元格"}}}})
	add([]string{"tabs"}, "分段标签切换并渲染每个标签内的子积木", []string{"tabs: [{key,title,blocks|children|child,action?}]", "active_key|active_tab", "default_active_key|default_index", "on_tab_change_action"}, map[string]interface{}{"tabs": []interface{}{map[string]interface{}{"key": "latest", "title": "最新", "blocks": []interface{}{map[string]interface{}{"id": "latest_text", "type": "text", "props": map[string]interface{}{"text": "最新内容"}}}}}})
	add([]string{"carousel"}, "轮播嵌套积木或图片项", []string{"items|children|blocks: BlockItem[]", "autoplay", "interval", "duration", "indicator", "height"}, map[string]interface{}{"height": "360rpx", "items": []interface{}{map[string]interface{}{"id": "slide", "type": "image", "props": map[string]interface{}{"image_url": "https://cdn.example.com/a.jpg"}}}})
	add([]string{"spacer"}, "提供固定宽度或高度的布局间距", []string{"height", "width"}, map[string]interface{}{"height": "24rpx"})

	add([]string{"text", "rich_text"}, "显示普通文本或微信 RichText 内容", []string{"text|content|nodes", "font_size", "font_weight", "line_height", "align|text_align", "color", "max_lines"}, map[string]interface{}{"text": "正文内容", "font_size": 16})
	add([]string{"image"}, "显示图片并可选点击预览", []string{"image_url|src|url", "fallback_url|placeholder", "aspect_ratio", "width", "height", "mode", "preview"}, map[string]interface{}{"image_url": "https://cdn.example.com/cover.jpg", "aspect_ratio": "16:9", "mode": "aspectFill"})
	add([]string{"video"}, "显示微信原生视频播放器", []string{"video_url|src", "cover_url|poster", "autoplay", "controls", "loop", "aspect_ratio", "on_ended_action"}, map[string]interface{}{"video_url": "https://cdn.example.com/video.mp4", "cover_url": "https://cdn.example.com/cover.jpg", "controls": true})
	add([]string{"notice"}, "显示短提示或公告", []string{"text|content", "icon"}, map[string]interface{}{"text": "重要公告"})
	add([]string{"timeline"}, "显示按时间排列的节点，节点可单独绑定 action", []string{"title", "nodes|items: [{time,title,content?,tag?,is_active?,action?}]"}, map[string]interface{}{"title": "进展", "nodes": []interface{}{map[string]interface{}{"time": "09:00", "title": "已发布", "is_active": true}}})
	add([]string{"empty"}, "显示空、过期、离线、库存不足或能力不足状态", []string{"state: empty|expired|offline|out_of_stock|capability_missing", "icon", "title", "desc|message", "btn_text|action_text"}, map[string]interface{}{"state": "empty", "title": "暂无内容"})
	add([]string{"skeleton"}, "显示加载骨架占位", []string{"rows: 0..20", "hero"}, map[string]interface{}{"rows": 3, "hero": true})

	add([]string{"media_hero"}, "显示焦点封面、标题和视频入口", []string{"title", "subtitle", "badge", "cover_url", "video_url", "rating", "on_ended_action"}, map[string]interface{}{"title": "焦点内容", "cover_url": "https://cdn.example.com/cover.jpg"})
	add([]string{"resource_card"}, "显示网盘或资源领取入口", []string{"title", "desc", "pan_name", "pan_url|url", "extract_code", "channels|pan_list", "btn_text", "fetch_code"}, map[string]interface{}{"title": "资源下载", "pan_name": "下载地址", "pan_url": "https://example.com", "extract_code": "1234"})
	add([]string{"action_button"}, "显示主操作按钮并执行 block.action", []string{"text", "badge", "variant"}, map[string]interface{}{"text": "继续"})
	add([]string{"game_card"}, "显示游戏封面、版本和兑换入口", []string{"title", "subtitle", "cover_url", "version", "redeem_code", "card_action", "redeem_action", "copy_action"}, map[string]interface{}{"title": "游戏名称", "version": "1.0"})
	add([]string{"form"}, "收集一个输入值并将值注入动作上下文", []string{"title", "input_label", "placeholder", "btn_text", "value|initial_value|default_value", "allow_empty", "empty_toast"}, map[string]interface{}{"title": "信息查询", "input_label": "查询号", "placeholder": "请输入", "btn_text": "查询"})
	add([]string{"episode_list"}, "显示剧集编号并将 episode_num 注入动作", []string{"title", "total_episodes|total: 0..1000", "current_episode"}, map[string]interface{}{"title": "选集", "total_episodes": 20, "current_episode": 1})
	add([]string{"item_grid"}, "显示带图片、标题和独立动作的数据卡片网格", []string{"title", "columns: 1..4", "items: [{id?,title,subtitle?,image_url?,badge?,action?}]"}, map[string]interface{}{"columns": 2, "items": []interface{}{map[string]interface{}{"id": "one", "title": "项目"}}})
	add([]string{"score_panel"}, "显示分数、等级和指标", []string{"title", "subtitle", "score|total_score", "rank", "level|badge", "avatar_url|icon_url", "unit", "items: [{label|name,value|val}]"}, map[string]interface{}{"title": "评分", "score": 95, "unit": "分"})
	add([]string{"coupon_card", "redeem_code_card"}, "显示优惠券或兑换码并支持领取/复制", []string{"title", "desc|description", "discount|amount", "redeem_code|code", "valid_until|expire_time|expires", "min_spend", "status", "btn_text", "btn_action"}, map[string]interface{}{"title": "专属礼包", "redeem_code": "ABCD", "btn_text": "复制"})
	add([]string{"countdown"}, "显示目标时间倒计时", []string{"title", "target_time|end_time", "expired_text"}, map[string]interface{}{"title": "距离结束", "target_time": "2026-12-31T23:59:59+08:00"})
	add([]string{"server_status"}, "显示服务区服状态与延迟", []string{"title|server_name", "status", "region", "latency", "notice"}, map[string]interface{}{"server_name": "一区", "status": "online", "latency": 32})
	add([]string{"product_card"}, "显示商品信息；购买金额只由后端 SKU 商品配置决定", []string{"title|name", "image_url|cover_url", "price", "original_price", "sales", "sku|product_sku", "btn_text", "btn_action"}, map[string]interface{}{"title": "会员套餐", "price": "9.90", "sku": "member_1"})
	add([]string{"download_card"}, "显示应用或文件下载信息", []string{"title|name", "desc|description", "icon_url|logo", "version", "size", "platform", "download_url|url", "btn_text", "btn_action"}, map[string]interface{}{"title": "客户端", "version": "1.0", "size": "20 MB"})

	add([]string{"collection_nav", "category_nav"}, "通用集合导航；通过 fields 映射任意后端数据", []string{"title", "items|items_path", "fields: {key,label}", "active_value"}, map[string]interface{}{"items_path": "$entity.content.categories", "fields": map[string]interface{}{"key": "slug", "label": "name"}})
	add([]string{"content_feed", "article_feed"}, "通用内容流；支持筛选、排序、图文/无图和字段映射", []string{"title", "items|items_path", "fields: {key,title,summary,image,meta,badge,eyebrow}", "filter: {field,value,all_value?}", "sort: {field,direction}", "layout: rows|feature", "limit"}, map[string]interface{}{"items_path": "$entity.content.items", "fields": map[string]interface{}{"key": "id", "title": "title", "summary": "summary", "image": "cover_url"}, "layout": "rows"})
	add([]string{"content_detail", "article_detail"}, "通用详情；从同源接口取数并按字段映射显示正文、试读和解锁态", []string{"resource_url", "fields: {title,meta,image,content,preview,readable,locked_reason,locked_title,unlock_label,purchase_value}", "preview_label", "loading_text", "error_text", "locked_action", "purchase_action"}, map[string]interface{}{"resource_url": "/api/v1/articles/{{$query.id}}", "fields": map[string]interface{}{"title": "title", "content": "markdown", "preview": "preview_markdown", "readable": "can_read", "purchase_value": "pay_sku"}})
	add([]string{"offer_list", "membership_plan_list"}, "通用套餐列表；数据、价格文本和 SKU 都由服务端返回", []string{"title", "items|items_path", "fields: {key,title,summary,caption,price}", "purchase_action", "purchase_text"}, map[string]interface{}{"items_path": "$entity.content.offers", "fields": map[string]interface{}{"key": "key", "title": "title", "price": "price"}, "purchase_action": map[string]interface{}{"type": "request_payment", "payload": map[string]interface{}{"sku": "$item.purchase_value"}}})
	add([]string{"discussion_thread", "comment_thread"}, "通用两级评论；一级自动加载，二级点击后懒加载", []string{"title", "list_url", "replies_url", "submit_url", "fields: {key,author,content,reply_count,reply_to}", "content_field", "parent_field", "require_auth", "placeholder"}, map[string]interface{}{"list_url": "/api/v1/articles/{{$query.id}}/comments", "replies_url": "/api/v1/comments/{{$item.id}}/replies", "submit_url": "/api/v1/articles/{{$query.id}}/comments", "fields": map[string]interface{}{"key": "id", "author": "nickname", "content": "content", "reply_count": "reply_count"}})
	add([]string{"bottom_nav"}, "固定底部安全区导航，入口和显隐由页面协议配置", []string{"items: [{key,label,icon_url?,page_id?,query?,action?,hidden?}]", "active_key"}, map[string]interface{}{"active_key": "home", "items": []interface{}{map[string]interface{}{"key": "home", "label": "首页", "page_id": "home"}}})
	add([]string{"floating_action"}, "固定安全区浮动操作入口", []string{"label", "icon_url?", "position: left|right"}, map[string]interface{}{"label": "客服", "position": "right"})
	add([]string{"media_picker"}, "选择图片或视频并将结果交给受控上传动作", []string{"title", "items", "max_count", "hint"}, map[string]interface{}{"title": "上传图片", "max_count": 9})
	add([]string{"upload_progress"}, "显示上传进度与状态", []string{"title", "progress: 0..100", "status_text"}, map[string]interface{}{"title": "正在上传", "progress": 40})
	add([]string{"media_gallery"}, "展示可预览的通用媒体网格", []string{"title", "items: [{id?,url|image_url}]"}, map[string]interface{}{"title": "图片", "items": []interface{}{map[string]interface{}{"url": "https://cdn.example.com/a.jpg"}}})
	add([]string{"location_picker"}, "显示腾讯地图位置并支持选点或导航", []string{"title", "address", "latitude", "longitude", "scale", "btn_text"}, map[string]interface{}{"title": "服务地点", "latitude": 39.9, "longitude": 116.4})
	add([]string{"service_entry"}, "默认打开微信内置客服，也可切换小程序内客服", []string{"title", "description", "mode: wechat|internal", "session_from", "btn_text"}, map[string]interface{}{"title": "联系客服", "mode": "wechat"})
	add([]string{"webview_entry", "webview_state"}, "展示已登记 WebView 入口或加载状态", []string{"title", "description", "btn_text", "disabled"}, map[string]interface{}{"title": "打开网页", "btn_text": "打开"})
	add([]string{"payment_result"}, "展示支付结果状态摘要", []string{"title", "status", "content|description", "order_id"}, map[string]interface{}{"title": "支付结果", "status": "pending"})
	add([]string{"price_breakdown"}, "展示订单价格明细摘要", []string{"title", "content|description", "items", "total"}, map[string]interface{}{"title": "价格明细", "total": "¥0.00"})
	add([]string{"address_card"}, "展示通用服务地址摘要", []string{"title|name", "address", "latitude", "longitude", "content|description", "btn_text", "btn_action"}, map[string]interface{}{"title": "服务地址", "address": "待选择"})
	add([]string{"qr_code"}, "展示已登记二维码信息和保存入口", []string{"title", "image_url|url", "content|description", "btn_text", "btn_action"}, map[string]interface{}{"title": "二维码"})
	add([]string{"feature_gate"}, "按租户能力矩阵显示或隐藏功能分支", []string{"title", "capability", "enabled", "content|description", "fallback"}, map[string]interface{}{"title": "功能开关", "capability": "membership"})
	add([]string{"compliance_panel"}, "展示隐私、类目和审核状态提示", []string{"title", "content|description", "policy_url", "status"}, map[string]interface{}{"title": "合规提示", "status": "local"})
	add([]string{"chat_thread"}, "组合消息列表、未读摘要和消息输入的通用客服线程", []string{"title", "peer_name", "status", "unread_count", "messages|items", "thread_id", "send_action", "refresh_action"}, map[string]interface{}{"title": "在线客服", "messages": []interface{}{map[string]interface{}{"content": "您好"}}})
	add([]string{"message_list"}, "展示通用消息列表", []string{"messages|items", "empty_text"}, map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "消息"}}})
	add([]string{"message_composer"}, "发送通用文本消息", []string{"placeholder", "send_text", "send_action"}, map[string]interface{}{"placeholder": "输入消息"})
	add([]string{"unread_badge"}, "展示消息未读数", []string{"count|unread_count", "hide_zero"}, map[string]interface{}{"count": 2})
	add([]string{"order_card"}, "展示订单状态摘要和受控操作", []string{"title", "order_id|id", "order_no", "amount", "status", "status_text", "updated_at", "action_text"}, map[string]interface{}{"title": "订单", "status": "pending"})
	add([]string{"order_summary"}, "展示订单金额摘要", []string{"title", "total|amount", "action_text"}, map[string]interface{}{"title": "订单摘要", "total": "¥0.00"})
	add([]string{"order_timeline"}, "展示订单状态时间线", []string{"title", "nodes|timeline"}, map[string]interface{}{"title": "订单进度", "nodes": []interface{}{map[string]interface{}{"title": "已创建"}}})
	add([]string{"logistics_track"}, "展示物流轨迹和刷新入口", []string{"title", "order_id", "company", "tracking_no", "tracks|items"}, map[string]interface{}{"title": "物流轨迹", "tracks": []interface{}{}})
	add([]string{"after_sale_form"}, "提交通用售后申请", []string{"title", "order_id", "reason", "placeholder", "submit_text"}, map[string]interface{}{"title": "申请售后", "order_id": "order-1"})
	add([]string{"evidence_list"}, "展示售后证据并提供受控上传入口", []string{"title", "order_id", "evidence|items", "allow_upload"}, map[string]interface{}{"title": "凭证材料", "allow_upload": true})
	add([]string{"service_card"}, "展示通用服务项目和接单入口", []string{"title|name", "description", "price", "status", "task_id|id", "action_text"}, map[string]interface{}{"title": "服务项目"})
	add([]string{"task_card"}, "展示斗师待接任务", []string{"task_title|title", "description", "task_id|id", "action_text"}, map[string]interface{}{"task_title": "待接任务"})
	add([]string{"quote_card"}, "展示服务报价和提交入口", []string{"title", "price|amount", "valid_until", "action_text"}, map[string]interface{}{"title": "服务报价", "price": "¥0.00"})
	add([]string{"schedule_picker"}, "收集服务排期文本并提交受控状态动作", []string{"title", "value", "placeholder", "confirm_text"}, map[string]interface{}{"title": "选择服务时间"})
	add([]string{"membership_card"}, "展示会员状态、有效期和权益", []string{"title|level_name", "status", "expire_at", "benefits", "action_text"}, map[string]interface{}{"title": "会员权益", "status": "inactive"})
	add([]string{"ad_slot"}, "展示受控广告位占位和策略状态", []string{"slot_id", "label", "title", "description", "disabled"}, map[string]interface{}{"slot_id": "ad-1", "title": "内容推荐"})
	add([]string{"wallet_card"}, "展示钱包余额和刷新入口", []string{"title", "balance", "action_text"}, map[string]interface{}{"title": "可用余额", "balance": "¥0.00"})
	add([]string{"withdraw_form"}, "提交受控提现申请", []string{"title", "placeholder", "submit_text"}, map[string]interface{}{"title": "申请提现"})

	add([]string{"result_table"}, "通用结果卡片；当前由通用渲染器显示核心图文", []string{"title", "rows", "content|description", "image_url", "btn_text", "btn_action"}, map[string]interface{}{"title": "查询结果", "content": "已查询到结果"})
	add([]string{"contact_card"}, "显示联系人或客服信息", []string{"title|name", "subtitle", "content|description", "image_url", "btn_text", "btn_action"}, map[string]interface{}{"title": "联系客服", "content": "工作日 9:00-18:00"})
	add([]string{"map_card"}, "显示位置摘要；无原生地图时使用确定性占位", []string{"title|name", "subtitle", "content|description", "image_url", "btn_text", "btn_action"}, map[string]interface{}{"title": "活动地点", "content": "地址说明"})
	add([]string{"game_header"}, "显示游戏详情头部", []string{"title|name", "subtitle", "cover_url|image_url", "badge|tag", "content|description", "btn_text", "btn_action"}, map[string]interface{}{"title": "游戏名称", "subtitle": "版本信息"})
	add([]string{"event_card"}, "显示活动摘要和操作入口", []string{"title", "subtitle", "content|description", "cover_url|image_url", "badge|tag", "btn_text", "btn_action"}, map[string]interface{}{"title": "限时活动", "content": "活动说明"})
	add([]string{"poll"}, "显示投票摘要；提交行为由动作或受控端点完成", []string{"title", "content|description", "options", "btn_text", "btn_action"}, map[string]interface{}{"title": "投票", "content": "请选择"})
	add([]string{"feed_list"}, "显示通用信息流摘要", []string{"title", "items", "content|description", "image_url", "btn_text", "btn_action"}, map[string]interface{}{"title": "信息流", "content": "内容摘要"})
	add([]string{"custom", "custom_block"}, "自由图文卡片；仅组合既有字段和动作，不执行自定义代码", []string{"title|name|text", "subtitle", "content|desc|description", "image_url|cover_url|src|poster", "badge|tag|category", "btn_text|button_text", "btn_action|button_action"}, map[string]interface{}{"title": "自定义卡片", "content": "说明", "btn_text": "查看"})
	return contracts
}

// mcpActionContracts 返回 AI 编排动作时可直接消费的完整载荷契约。
func mcpActionContracts() map[string]interface{} {
	return map[string]interface{}{
		"require_auth":           map[string]interface{}{"purpose": "确保微信会话可用后继续 on_success", "payload": []string{}},
		"copy_text":              map[string]interface{}{"purpose": "复制文本", "payload": []string{"text|content|path", "toast?"}},
		"toast":                  map[string]interface{}{"purpose": "显示短提示", "payload": []string{"title|message|text", "icon?", "duration?"}},
		"refresh":                map[string]interface{}{"purpose": "重新加载当前页面", "payload": []string{}},
		"navigate_page":          map[string]interface{}{"purpose": "打开 SDUI 页面", "payload": []string{"page_id", "query?", "id?", "open_type?: navigate|redirect|reLaunch"}},
		"open_channels_activity": map[string]interface{}{"purpose": "打开微信视频号动态", "payload": []string{"feed_id", "finder_user_name"}},
		"open_mini_program":      map[string]interface{}{"purpose": "打开其他小程序", "payload": []string{"target_app_id", "target_path?", "extra_data?", "env_version?"}},
		"open_webview":           map[string]interface{}{"purpose": "打开当前 AppID 已登记且启用的 HTTPS H5 入口", "payload": []string{"url_key", "title?"}},
		"preview_image":          map[string]interface{}{"purpose": "预览图片", "payload": []string{"current|url", "urls?"}},
		"request":                map[string]interface{}{"purpose": "request_data 的兼容名称", "payload": []string{"endpoint 或同源 url", "method?", "path_params?", "query?", "body?", "idempotency_key?", "timeout_ms?", "response?: {data_path?,save_as?}"}},
		"request_data":           map[string]interface{}{"purpose": "调用登记端点或同源相对接口并保存结果", "payload": []string{"endpoint 或同源 url", "method?", "path_params?", "query?", "body?", "idempotency_key?", "timeout_ms?", "target?", "response?: {data_path?,save_as?}"}},
		"request_payment":        map[string]interface{}{"purpose": "按后端商品 SKU 创建订单并调起微信支付", "payload": []string{"sku|product_sku", "idempotency_key?"}},
		"share":                  map[string]interface{}{"purpose": "打开微信分享菜单", "payload": []string{"title?", "toast?"}},
		"subscribe_message":      map[string]interface{}{"purpose": "请求微信订阅消息授权", "payload": []string{"tmpl_ids|template_ids|template_id|tmpl_id"}},
		"set_state":              map[string]interface{}{"purpose": "写入页面局部状态", "payload": []string{"key|name|target", "value|val"}},
		"toggle_state":           map[string]interface{}{"purpose": "切换布尔状态", "payload": []string{"key|name|target"}},
		"reset_state":            map[string]interface{}{"purpose": "清空页面局部状态", "payload": []string{"key|name|target?"}},
		"show_error_state":       map[string]interface{}{"purpose": "将目标积木切换为 error 分支", "payload": []string{"target"}},
		"show_empty_state":       map[string]interface{}{"purpose": "将目标积木切换为 empty 分支", "payload": []string{"target"}},
		"show_loading_state":     map[string]interface{}{"purpose": "将目标积木切换为 loading 分支", "payload": []string{"target"}},
		"reset_block_state":      map[string]interface{}{"purpose": "将目标积木恢复 normal 分支", "payload": []string{"target"}},
		"choose_media":           map[string]interface{}{"purpose": "调用微信媒体选择器", "payload": []string{"count?", "media_type?", "source_type?"}},
		"upload_file":            map[string]interface{}{"purpose": "使用后端返回的 HTTPS 预签名地址上传媒体", "payload": []string{"file_path|temp_file_path", "presigned_url", "upload_headers?", "final_url|final_cos_file_url?"}},
		"delete_media":           map[string]interface{}{"purpose": "通过受控端点删除已归属当前用户/业务的媒体", "payload": []string{"endpoint", "body?", "idempotency_key?"}},
		"request_location":       map[string]interface{}{"purpose": "请求 GCJ-02 当前定位", "payload": []string{}},
		"choose_location":        map[string]interface{}{"purpose": "打开微信位置选择器", "payload": []string{}},
		"open_map":               map[string]interface{}{"purpose": "打开腾讯地图位置", "payload": []string{"latitude", "longitude", "name|title?", "address?", "scale?"}},
		"open_wechat_service":    map[string]interface{}{"purpose": "打开微信原生客服会话；默认客服入口优先使用 Button open-type=contact", "payload": []string{"ext_info?", "corp_id?", "url?"}},
		"save_qr":                map[string]interface{}{"purpose": "下载并保存已登记客服二维码", "payload": []string{"url|image_url"}},
		"open_internal_chat":     map[string]interface{}{"purpose": "打开通用 SDUI 小程序内客服页", "payload": []string{"page_id?", "context?"}},
		"send_message":           map[string]interface{}{"purpose": "发送聊天消息到受控会话端点", "payload": []string{"thread_id?", "content"}},
		"mark_read":              map[string]interface{}{"purpose": "标记聊天会话已读", "payload": []string{"thread_id?"}},
		"poll_messages":          map[string]interface{}{"purpose": "轮询受控聊天消息", "payload": []string{"thread_id?", "cursor?"}},
		"connect_message":        map[string]interface{}{"purpose": "建立受控消息连接", "payload": []string{"thread_id?"}},
		"upload_chat_media":      map[string]interface{}{"purpose": "上传聊天媒体到当前租户", "payload": []string{"thread_id?", "media_id?"}},
		"create_order":           map[string]interface{}{"purpose": "创建业务订单", "payload": []string{"sku?", "amount?"}},
		"confirm_order":          map[string]interface{}{"purpose": "确认订单", "payload": []string{"order_id|id"}},
		"cancel_order":           map[string]interface{}{"purpose": "取消订单", "payload": []string{"order_id|id", "reason?"}},
		"confirm_receipt":        map[string]interface{}{"purpose": "确认收货或服务完成", "payload": []string{"order_id|id"}},
		"refresh_logistics":      map[string]interface{}{"purpose": "刷新物流轨迹", "payload": []string{"order_id|id"}},
		"apply_after_sale":       map[string]interface{}{"purpose": "申请售后", "payload": []string{"order_id|id", "reason"}},
		"upload_evidence":        map[string]interface{}{"purpose": "上传售后证据", "payload": []string{"order_id|id", "media_id?"}},
		"accept_task":            map[string]interface{}{"purpose": "斗师接受服务任务", "payload": []string{"task_id|id"}},
		"reject_task":            map[string]interface{}{"purpose": "斗师拒绝服务任务", "payload": []string{"task_id|id", "reason?"}},
		"submit_quote":           map[string]interface{}{"purpose": "斗师提交服务报价", "payload": []string{"task_id|id", "price|amount"}},
		"update_service_status":  map[string]interface{}{"purpose": "更新服务订单状态", "payload": []string{"task_id|order_id|id", "status|schedule?"}},
		"open_membership":        map[string]interface{}{"purpose": "打开会员能力或开通入口", "payload": []string{"plan_id|sku?"}},
		"load_ad":                map[string]interface{}{"purpose": "加载受控广告位", "payload": []string{"slot_id"}},
		"refresh_wallet":         map[string]interface{}{"purpose": "刷新钱包余额", "payload": []string{}},
		"request_withdraw":       map[string]interface{}{"purpose": "提交提现申请", "payload": []string{"amount"}},
	}
}
