// Package services ai_breakthrough_provider.go
package services

import (
	"fmt"
	"time"

	"hot_keyword/db"
	"hot_keyword/models"
)

func init() {
	RegisterPageEnvelopeEnricher("ai_breakthrough", enrichAIBreakthrough)
	RegisterPageEnvelopeEnricher("ai_article", enrichAIBreakthrough)
	RegisterDefaultShareMode("ai_article", "membership")
}

// enrichAIBreakthrough 将资讯导航、会员方案和访问者权益摘要附加到统一信封。
func enrichAIBreakthrough(envelope *models.PageResponseEnvelope, appID string, authenticated bool, userID int64) error {
	if envelope == nil || db.DB == nil {
		return nil
	}
	if envelope.Data == nil {
		envelope.Data = map[string]interface{}{}
	}
	level := 0
	var membership *models.UserMembership
	if userID > 0 {
		membership, _ = NewMembershipService().GetMembership(appID, userID)
		if membership != nil && membership.ExpiresAt.After(time.Now()) {
			level = membership.Level
		}
	}
	viewer := map[string]interface{}{"logged_in": authenticated, "membership_level": level}
	if membership != nil && membership.ExpiresAt.After(time.Now()) {
		viewer["membership_expires_at"] = membership.ExpiresAt
	}
	var categories []models.ArticleCategory
	_ = db.DB.Where("app_id = ? AND status = ?", appID, "active").Order("sort asc, id asc").Find(&categories).Error
	categorySlugs := make(map[int64]string, len(categories))
	categoryNames := make(map[int64]string, len(categories))
	for _, category := range categories {
		categorySlugs[category.ID] = category.Slug
		categoryNames[category.ID] = category.Name
	}
	var articles []models.Article
	_ = db.DB.Where("app_id = ? AND status = ?", appID, "published").Order("published_at desc, id desc").Limit(12).Find(&articles).Error
	items := make([]map[string]interface{}, 0, len(articles))
	articleService := NewArticleService()
	for _, article := range articles {
		categorySlug := categorySlugs[article.CategoryID]
		categoryName := categoryNames[article.CategoryID]
		purchased := userID > 0 && articleService.hasPurchase(appID, userID, article.ID)
		summary := toArticleSummaryForViewer(article, level, purchased)
		item := map[string]interface{}{"id": article.ID, "title": article.Title, "summary": article.Summary, "cover_url": article.CoverURL, "category_id": article.CategoryID, "category_slug": categorySlug, "category_name": categoryName, "featured": categorySlug == "featured" && !article.IsPaid && article.RequiredLevel == 0, "published_at": article.PublishedAt, "required_level": article.RequiredLevel, "is_paid": article.IsPaid, "pay_sku": article.PaySKU, "price_fen": article.PriceFen, "allow_comments": article.AllowComments, "can_read": summary.CanRead, "display_meta": summary.DisplayMeta, "access_badge": summary.AccessBadge, "key": article.ID, "image": article.CoverURL, "meta": summary.DisplayMeta, "badge": summary.AccessBadge}
		items = append(items, item)
	}
	var plans []models.MembershipLevel
	_ = db.DB.Where("app_id = ? AND status = ?", appID, "active").Order("level asc").Find(&plans).Error
	offers := make([]map[string]interface{}, 0, len(plans))
	for _, plan := range plans {
		offers = append(offers, map[string]interface{}{
			"key": plan.SKU, "title": plan.Name, "summary": plan.Description,
			"caption": fmt.Sprintf("有效期 %d 天", plan.DurationDays), "price": fmt.Sprintf("¥%.2f", float64(plan.PriceFen)/100), "purchase_value": plan.SKU,
		})
	}
	if envelope.Viewer == nil {
		envelope.Viewer = map[string]interface{}{}
	}
	for key, value := range viewer {
		envelope.Viewer[key] = value
	}
	content := map[string]interface{}{"categories": categories, "items": items, "offers": offers, "articles": items, "membership_plans": plans}
	envelope.Data["content"] = content
	// 兼容旧页面协议；新模板只使用通用 content 数据键。
	envelope.Data["ai_breakthrough"] = content
	return nil
}
