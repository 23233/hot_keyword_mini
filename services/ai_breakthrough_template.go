// Package services ai_breakthrough_template.go
package services

import "hot_keyword/models"

// buildAIBreakthroughTemplate 构建 AI 破甲资讯导航与会员中心模板。
func buildAIBreakthroughTemplate() *SDUITemplate {
	return &SDUITemplate{
		TemplateID:         "tpl_ai_breakthrough_portal",
		TemplateVersion:    "3.1.0",
		Name:               "AI 破甲资讯导航站",
		BusinessType:       "ai_breakthrough",
		Intent:             "watch",
		Description:        "AI 资讯导航、文章阅读、会员权益和评论入口模板。",
		DefaultTheme:       "light_clean",
		DefaultAccentColor: "#0A84FF",
		DefaultBlocks: []models.BlockItem{
			{ID: "content_nav", Type: "collection_nav", Props: map[string]interface{}{"template_version": "3.1.0", "items_path": "$entity.content.categories", "fields": map[string]interface{}{"key": "slug", "label": "name"}, "active_value": "{{$query.category}}"}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-6"}}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "home", "query": map[string]interface{}{"category": "$item.slug"}}}},
			{ID: "content_feature", Type: "content_feed", Props: map[string]interface{}{"title": "焦点", "items_path": "$entity.content.items", "filter": map[string]interface{}{"field": "featured", "value": true}, "fields": map[string]interface{}{"key": "id", "title": "title", "summary": "summary", "image": "image", "meta": "meta", "badge": "badge"}, "layout": "feature", "limit": 1}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-10", "media/rounded"}}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "article_detail", "query": map[string]interface{}{"id": "$item.id"}}}},
			{ID: "content_feed", Type: "content_feed", Props: map[string]interface{}{"title": "最新文章", "items_path": "$entity.content.items", "filter": map[string]interface{}{"field": "category_slug", "value": "{{$query.category}}", "all_value": ""}, "sort": map[string]interface{}{"field": "published_at", "direction": "desc"}, "fields": map[string]interface{}{"key": "id", "title": "title", "summary": "summary", "image": "image", "meta": "meta", "badge": "badge"}, "layout": "rows", "limit": 8}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-10", "media/rounded"}}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "article_detail", "query": map[string]interface{}{"id": "$item.id"}}}},
			{ID: "content_membership", Type: "action_button", Props: map[string]interface{}{"text": "会员中心", "variant": "quiet"}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-8"}}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "membership"}}},
		},
	}
}

// buildAIArticleTemplate 构建文章详情承载模板，正文由 article_detail 组件按接口权限获取。
func buildAIArticleTemplate() *SDUITemplate {
	return &SDUITemplate{
		TemplateID:         "tpl_ai_breakthrough_article",
		TemplateVersion:    "1.1.0",
		Name:               "AI 破甲文章详情",
		BusinessType:       "ai_article",
		Intent:             "watch",
		Description:        "文章正文、会员门槛、评论和二级回复懒加载承载模板。",
		DefaultTheme:       "light_clean",
		DefaultAccentColor: "#0A84FF",
		DefaultBlocks: []models.BlockItem{
			{ID: "content_detail", Type: "content_detail", Props: map[string]interface{}{"resource_url": "/api/v1/articles/{{$query.id}}", "fields": map[string]interface{}{"key": "id", "title": "title", "meta": "display_meta", "image": "cover_url", "content": "markdown", "preview": "preview_markdown", "readable": "can_read", "locked_reason": "locked_reason", "locked_title": "locked_title", "unlock_label": "unlock_label", "purchase_value": "pay_sku"}, "preview_label": "试读", "locked_action": map[string]interface{}{"type": "navigate_page", "payload": map[string]interface{}{"page_id": "membership"}}, "purchase_action": map[string]interface{}{"type": "request_payment", "payload": map[string]interface{}{"sku": "$item.pay_sku"}}}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-10", "media/rounded"}}},
			{ID: "content_discussion", Type: "discussion_thread", Props: map[string]interface{}{"list_url": "/api/v1/articles/{{$query.id}}/comments", "replies_url": "/api/v1/comments/{{$item.id}}/replies", "submit_url": "/api/v1/articles/{{$query.id}}/comments", "fields": map[string]interface{}{"key": "id", "author": "nickname", "content": "content", "reply_count": "reply_count", "reply_to": "reply_to_nickname"}, "title": "评论"}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-8"}}},
		},
	}
}

// buildAIMembershipTemplate 构建会员套餐与支付入口页面。
func buildAIMembershipTemplate() *SDUITemplate {
	return &SDUITemplate{
		TemplateID: "tpl_ai_breakthrough_membership", TemplateVersion: "1.1.0",
		Name: "AI 破甲会员中心", BusinessType: "ai_breakthrough", Intent: "buy",
		Description: "展示服务端会员套餐并进入微信支付流程。", DefaultTheme: "light_clean", DefaultAccentColor: "#0A84FF",
		DefaultBlocks: []models.BlockItem{
			{ID: "content_offers", Type: "offer_list", Props: map[string]interface{}{"title": "选择会员方案", "items_path": "$entity.content.offers", "fields": map[string]interface{}{"key": "key", "title": "title", "summary": "summary", "caption": "caption", "price": "price"}, "purchase_action": map[string]interface{}{"type": "request_payment", "payload": map[string]interface{}{"sku": "$item.purchase_value"}}}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-10"}}},
			{ID: "content_back", Type: "action_button", Props: map[string]interface{}{"text": "返回首页", "variant": "quiet"}, Style: &models.BlockStyle{Utilities: []string{"layout/flat", "space/y-8"}}, Action: &models.BlockAction{Type: "navigate_page", Payload: map[string]interface{}{"page_id": "home"}}},
		},
	}
}
