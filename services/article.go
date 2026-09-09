// Package services article.go
package services

import (
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"html"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ArticleViewer 当前文章访问者权限。
type ArticleViewer struct {
	LoggedIn            bool       `json:"logged_in"`
	MembershipLevel     int        `json:"membership_level"`
	MembershipExpiresAt *time.Time `json:"membership_expires_at,omitempty"`
}

// ArticleSummary 文章列表安全摘要，不包含受保护正文。
type ArticleSummary struct {
	ID            int64      `json:"id"`
	CategoryID    int64      `json:"category_id"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Summary       string     `json:"summary"`
	CoverURL      string     `json:"cover_url"`
	Author        string     `json:"author"`
	Tags          []string   `json:"tags"`
	RequiredLevel int        `json:"required_level"`
	IsPaid        bool       `json:"is_paid"`
	PaySKU        string     `json:"pay_sku,omitempty"`
	PriceFen      int64      `json:"price_fen,omitempty"`
	AllowComments bool       `json:"allow_comments"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CanRead       bool       `json:"can_read"`
	// 展示层元信息，由服务端统一计算，供通用内容积木直接映射。
	DisplayMeta string `json:"display_meta,omitempty"`
	// 展示层权限标记，由服务端统一计算。
	AccessBadge string `json:"access_badge,omitempty"`
}

// ArticleDetail 文章详情，正文仅在 CanRead 时返回。
type ArticleDetail struct {
	ArticleSummary
	Markdown        string `json:"markdown,omitempty"`
	PreviewMarkdown string `json:"preview_markdown,omitempty"`
	LockedReason    string `json:"locked_reason,omitempty"`
	// 锁定态标题，避免客户端根据业务字段拼接文案。
	LockedTitle string `json:"locked_title,omitempty"`
	// 锁定态操作文案，避免客户端判断支付或会员分支。
	UnlockLabel string `json:"unlock_label,omitempty"`
}

// ArticleService 文章和权限服务。
type ArticleService struct{}

// NewArticleService 创建文章服务。
func NewArticleService() *ArticleService { return &ArticleService{} }

func articleTags(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func articleCanRead(required, level int) bool { return required <= 0 || level >= required }

func articleCanReadForViewer(article models.Article, level int, purchased bool) bool {
	if article.IsPaid && !purchased {
		return false
	}
	return articleCanRead(article.RequiredLevel, level)
}

func toArticleSummary(article models.Article, level int) ArticleSummary {
	meta := "公开阅读"
	badge := ""
	if article.IsPaid {
		meta = fmt.Sprintf("单篇 ¥%.2f", float64(article.PriceFen)/100)
		badge = "单篇"
	} else if article.RequiredLevel > 0 {
		meta = "会员可读"
		badge = "会员"
	}
	if article.Author != "" {
		meta = article.Author + " · " + meta
	}
	return ArticleSummary{ID: article.ID, CategoryID: article.CategoryID, Slug: article.Slug, Title: article.Title, Summary: article.Summary, CoverURL: article.CoverURL, Author: article.Author, Tags: articleTags(article.Tags), RequiredLevel: article.RequiredLevel, IsPaid: article.IsPaid, PaySKU: article.PaySKU, PriceFen: article.PriceFen, AllowComments: article.AllowComments, PublishedAt: article.PublishedAt, CanRead: articleCanRead(article.RequiredLevel, level), DisplayMeta: meta, AccessBadge: badge}
}

func toArticleSummaryForViewer(article models.Article, level int, purchased bool) ArticleSummary {
	result := toArticleSummary(article, level)
	result.CanRead = articleCanReadForViewer(article, level, purchased)
	return result
}

// ListPublished 返回已发布文章列表。
func (s *ArticleService) ListPublished(appID string, categoryID int64, limit, offset, level int) ([]ArticleSummary, error) {
	return s.ListPublishedForViewer(appID, categoryID, limit, offset, level, 0)
}

// ListPublishedForViewer 返回文章摘要并按会员等级与单篇购买记录计算可读性。
func (s *ArticleService) ListPublishedForViewer(appID string, categoryID int64, limit, offset, level int, userID int64) ([]ArticleSummary, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := db.Mysql.Where("app_id = ? AND status = ?", appID, "published")
	if categoryID > 0 {
		query = query.Where("category_id = ?", categoryID)
	}
	var articles []models.Article
	if err := query.Order("published_at desc, id desc").Limit(limit).Offset(offset).Find(&articles).Error; err != nil {
		return nil, err
	}
	result := make([]ArticleSummary, 0, len(articles))
	for _, article := range articles {
		purchased := userID > 0 && s.hasPurchase(appID, userID, article.ID)
		result = append(result, toArticleSummaryForViewer(article, level, purchased))
	}
	return result, nil
}

// GetPublished 返回文章详情，并在服务端裁剪会员正文。
func (s *ArticleService) GetPublished(appID string, id int64, level int) (*ArticleDetail, error) {
	return s.GetPublishedForViewer(appID, id, level, 0)
}

// GetPublishedForViewer 返回文章详情；正文、试读片段和单篇购买权限均在服务端裁剪。
func (s *ArticleService) GetPublishedForViewer(appID string, id int64, level int, userID int64) (*ArticleDetail, error) {
	var article models.Article
	if err := db.Mysql.Where("app_id = ? AND id = ? AND status = ?", appID, id, "published").First(&article).Error; err != nil {
		return nil, err
	}
	purchased := userID > 0 && s.hasPurchase(appID, userID, article.ID)
	detail := &ArticleDetail{ArticleSummary: toArticleSummaryForViewer(article, level, purchased)}
	if detail.CanRead {
		detail.Markdown = article.Markdown
	} else {
		detail.PreviewMarkdown = article.FreeMarkdown
		if article.IsPaid {
			detail.LockedReason = fmt.Sprintf("单篇购买后解锁全文，价格 ¥%.2f", float64(article.PriceFen)/100)
			detail.LockedTitle = "购买后阅读全文"
			detail.UnlockLabel = "购买全文"
		} else {
			detail.LockedReason = fmt.Sprintf("需要 level %d 会员权限", article.RequiredLevel)
			detail.LockedTitle = "会员专享内容"
			detail.UnlockLabel = "查看会员方案"
		}
	}
	return detail, nil
}

func (s *ArticleService) hasPurchase(appID string, userID, articleID int64) bool {
	if db.Mysql == nil || userID <= 0 || articleID <= 0 {
		return false
	}
	var count int64
	db.Mysql.Model(&models.ArticlePurchase{}).Where("app_id = ? AND user_id = ? AND article_id = ?", appID, userID, articleID).Count(&count)
	return count > 0
}

// MarkdownToHTML 将受控 Markdown 转为 RichText 可消费的 HTML。
func MarkdownToHTML(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var result strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		level := 0
		for level < len(trimmed) && level < 3 && trimmed[level] == '#' {
			level++
		}
		if level > 0 && level < len(trimmed) && trimmed[level] == ' ' {
			content := html.EscapeString(strings.TrimSpace(trimmed[level:]))
			result.WriteString(fmt.Sprintf("<h%d>%s</h%d>", level, content, level))
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			result.WriteString("<p>• " + html.EscapeString(strings.TrimSpace(trimmed[2:])) + "</p>")
			continue
		}
		content := html.EscapeString(trimmed)
		content = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(content, "<strong>$1</strong>")
		result.WriteString("<p>" + content + "</p>")
	}
	return result.String()
}

// SaveCategory 保存资讯栏目。
func (s *ArticleService) SaveCategory(category *models.ArticleCategory) error {
	if category == nil || category.AppID == "" || category.Slug == "" || category.Name == "" {
		return errors.New("栏目参数不完整")
	}
	category.Status = strings.TrimSpace(category.Status)
	if category.Status == "" {
		category.Status = "active"
	}
	if category.Status != "active" && category.Status != "inactive" {
		return errors.New("栏目状态必须为 active 或 inactive")
	}
	return db.Mysql.Where("app_id = ? AND slug = ?", category.AppID, category.Slug).Assign(map[string]interface{}{"name": category.Name, "summary": category.Summary, "sort": category.Sort, "status": category.Status, "updated_at": time.Now()}).FirstOrCreate(category).Error
}

var articleAdminStatuses = map[string]bool{"draft": true, "reviewing": true, "published": true, "archived": true}

// ValidateArticleConfig 校验文章发布所需的权限、付费和状态配置。
func (s *ArticleService) ValidateArticleConfig(article *models.Article) error {
	if article == nil || strings.TrimSpace(article.AppID) == "" || strings.TrimSpace(article.Slug) == "" || strings.TrimSpace(article.Title) == "" || strings.TrimSpace(article.Markdown) == "" {
		return errors.New("文章参数不完整")
	}
	article.AppID, article.Slug, article.Title = strings.TrimSpace(article.AppID), strings.TrimSpace(article.Slug), strings.TrimSpace(article.Title)
	article.Status = strings.TrimSpace(article.Status)
	if article.Status == "" {
		article.Status = "draft"
	}
	if !articleAdminStatuses[article.Status] {
		return errors.New("文章状态必须为 draft、reviewing、published 或 archived")
	}
	if article.RequiredLevel < 0 {
		return errors.New("文章最低会员等级不能小于 0")
	}
	if article.CategoryID > 0 {
		var category models.ArticleCategory
		if err := db.Mysql.Where("app_id = ? AND id = ?", article.AppID, article.CategoryID).First(&category).Error; err != nil {
			return errors.New("文章栏目不存在或不属于当前小程序")
		}
	}
	if article.RequiredLevel > 0 {
		var plan models.MembershipLevel
		if err := db.Mysql.Where("app_id = ? AND level = ?", article.AppID, article.RequiredLevel).First(&plan).Error; err != nil {
			return fmt.Errorf("文章要求的会员等级 level %d 尚未配置", article.RequiredLevel)
		}
	}
	if article.IsPaid {
		if article.PriceFen <= 0 {
			return errors.New("单篇付费文章价格必须大于 0")
		}
		if strings.TrimSpace(article.PaySKU) == "" {
			article.PaySKU = "article_" + article.Slug
		}
		free := strings.TrimSpace(article.FreeMarkdown)
		markdown := strings.TrimSpace(article.Markdown)
		if free == "" || !strings.HasPrefix(markdown, free) {
			return errors.New("付费文章必须提供正文开头的免费试读内容")
		}
		var plan models.MembershipLevel
		if err := db.Mysql.Where("app_id = ? AND sku = ?", article.AppID, article.PaySKU).First(&plan).Error; err == nil {
			return errors.New("单篇文章 SKU 不能与会员套餐重复")
		}
	}
	return nil
}

// SaveArticle 保存文章并同步单篇付费商品，支持按 ID 更新或按 slug 创建。
func (s *ArticleService) SaveArticle(article *models.Article) error {
	if err := s.ValidateArticleConfig(article); err != nil {
		return err
	}
	now := time.Now()
	if article.PublishedAt == nil && article.Status == "published" {
		article.PublishedAt = &now
	}
	article.UpdatedAt = now
	if article.CreatedAt.IsZero() {
		article.CreatedAt = now
	}
	updates := map[string]interface{}{"category_id": article.CategoryID, "slug": article.Slug, "title": article.Title, "summary": article.Summary, "cover_url": article.CoverURL, "markdown": article.Markdown, "free_markdown": article.FreeMarkdown, "is_paid": article.IsPaid, "pay_sku": article.PaySKU, "price_fen": article.PriceFen, "author": article.Author, "tags": article.Tags, "required_level": article.RequiredLevel, "allow_comments": article.AllowComments, "status": article.Status, "published_at": article.PublishedAt, "updated_at": article.UpdatedAt}
	return db.Mysql.Transaction(func(tx *gorm.DB) error {
		var existing models.Article
		oldSKU := ""
		query := tx.Where("app_id = ?", article.AppID)
		if article.ID > 0 {
			query = query.Where("id = ?", article.ID)
		} else {
			query = query.Where("slug = ?", article.Slug)
		}
		err := query.First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if article.ID > 0 {
				return errors.New("文章不存在或不属于当前小程序")
			}
			article.CreatedAt = now
			if err := tx.Create(article).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			oldSKU = existing.PaySKU
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			article.ID, article.CreatedAt = existing.ID, existing.CreatedAt
		}
		if oldSKU != "" && oldSKU != article.PaySKU {
			if err := tx.Model(&models.Product{}).Where("app_id = ? AND sku = ?", article.AppID, oldSKU).Update("status", models.ProductStatusInactive).Error; err != nil {
				return err
			}
		}
		if article.IsPaid {
			productStatus := models.ProductStatusInactive
			if article.Status == "published" {
				productStatus = models.ProductStatusActive
			}
			product := models.Product{AppID: article.AppID, SKU: article.PaySKU, Name: article.Title, Description: article.Summary, PriceFen: article.PriceFen, Status: productStatus, CreatedAt: now, UpdatedAt: now}
			if err := tx.Where("app_id = ? AND sku = ?", article.AppID, article.PaySKU).Assign(map[string]interface{}{"name": product.Name, "description": product.Description, "price_fen": product.PriceFen, "status": productStatus, "updated_at": now}).FirstOrCreate(&product).Error; err != nil {
				return err
			}
		} else if article.PaySKU != "" {
			if err := tx.Model(&models.Product{}).Where("app_id = ? AND sku = ?", article.AppID, article.PaySKU).Update("status", models.ProductStatusInactive).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SeedDefaultContent 创建 AI 破甲首页栏目与少量示例文章。
func (s *ArticleService) SeedDefaultContent(appID string) error {
	categories := []models.ArticleCategory{
		{Slug: "featured", Name: "精选", Summary: "值得收藏的 AI 观察与拆解", Sort: 1},
		{Slug: "news", Name: "最新", Summary: "模型、产品与行业动态", Sort: 2},
		{Slug: "deep-dive", Name: "深度", Summary: "方法论与案例拆解", Sort: 3},
		{Slug: "tools", Name: "工具", Summary: "可直接上手的工具和教程", Sort: 4},
		{Slug: "members", Name: "会员", Summary: "面向会员的深度内容", Sort: 5},
	}
	for _, category := range categories {
		category.AppID = appID
		if err := s.SaveCategory(&category); err != nil {
			return err
		}
	}
	var featured models.ArticleCategory
	if err := db.Mysql.Where("app_id = ? AND slug = ?", appID, "featured").First(&featured).Error; err != nil {
		return err
	}
	var members models.ArticleCategory
	if err := db.Mysql.Where("app_id = ? AND slug = ?", appID, "members").First(&members).Error; err != nil {
		return err
	}
	now := time.Now()
	articles := []models.Article{
		{AppID: appID, CategoryID: featured.ID, Slug: "ai-breakthrough-start", Title: "AI 破甲：从热点信息到可验证结论", Summary: "一套面向普通读者的 AI 资讯导航与事实核验方法。", CoverURL: "/assets/ai-editorial-cover.png", Markdown: "# 欢迎来到 AI 破甲\n\n这里聚合模型、产品、案例和工具，优先给出可验证来源。\n\n- 每日精选\n- 深度拆解\n- 工具教程", Author: "AI 破甲编辑部", Tags: "AI,导航,方法论", RequiredLevel: 0, AllowComments: true, Status: "published", PublishedAt: &now},
		{AppID: appID, CategoryID: featured.ID, Slug: "member-research-001", Title: "会员专享：AI 产品拆解周报", Summary: "从产品定位、能力边界到落地成本的系统分析。", CoverURL: "/assets/ai-editorial-cover.png", Markdown: "# AI 产品拆解周报\n\n本篇包含会员专享的完整研究框架与数据附录。", Author: "AI 破甲研究组", Tags: "研究,产品,会员", RequiredLevel: 1, AllowComments: true, Status: "published", PublishedAt: &now},
		{AppID: appID, CategoryID: members.ID, Slug: "single-article-product-teardown", Title: "单篇解锁：AI 产品深度拆解", Summary: "免费试读方法框架，购买后查看完整数据和结论。", Markdown: "# AI 产品深度拆解\n\n免费试读：本文先解释产品定位、用户路径和核心指标。\n\n付费内容：完整竞品矩阵、成本测算与落地建议。", FreeMarkdown: "# AI 产品深度拆解\n\n免费试读：本文先解释产品定位、用户路径和核心指标。", IsPaid: true, PaySKU: "article_single_product_teardown", PriceFen: 1990, Author: "AI 破甲研究组", Tags: "研究,产品,单篇付费", RequiredLevel: 0, AllowComments: true, Status: "published", PublishedAt: &now},
	}
	for _, article := range articles {
		article.AppID = appID
		if article.IsPaid && article.PaySKU == "" {
			article.PaySKU = "article_" + article.Slug
		}
		if err := db.Mysql.Where("app_id = ? AND slug = ?", appID, article.Slug).Assign(map[string]interface{}{"category_id": article.CategoryID, "title": article.Title, "summary": article.Summary, "cover_url": article.CoverURL, "markdown": article.Markdown, "free_markdown": article.FreeMarkdown, "is_paid": article.IsPaid, "pay_sku": article.PaySKU, "price_fen": article.PriceFen, "author": article.Author, "tags": article.Tags, "required_level": article.RequiredLevel, "allow_comments": article.AllowComments, "status": article.Status, "published_at": article.PublishedAt, "updated_at": now}).FirstOrCreate(&article).Error; err != nil {
			return err
		}
		if article.IsPaid {
			product := models.Product{AppID: appID, SKU: article.PaySKU, Name: article.Title, Description: article.Summary, PriceFen: article.PriceFen, Status: models.ProductStatusActive, CreatedAt: now, UpdatedAt: now}
			if err := db.Mysql.Where("app_id = ? AND sku = ?", appID, article.PaySKU).Assign(map[string]interface{}{"name": product.Name, "description": product.Description, "price_fen": product.PriceFen, "status": product.Status, "updated_at": now}).FirstOrCreate(&product).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
