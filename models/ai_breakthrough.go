// Package models ai_breakthrough.go
package models

import "time"

// ArticleCategory AI 破甲资讯栏目。
type ArticleCategory struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID     string    `gorm:"column:app_id;size:64;not null;index;uniqueIndex:idx_category_tenant_slug" json:"app_id"`
	Slug      string    `gorm:"column:slug;size:64;not null;uniqueIndex:idx_category_tenant_slug" json:"slug"`
	Name      string    `gorm:"column:name;size:128;not null" json:"name"`
	Summary   string    `gorm:"column:summary;size:255" json:"summary"`
	Sort      int       `gorm:"column:sort;default:0" json:"sort"`
	Status    string    `gorm:"column:status;size:16;default:'active';index" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 返回栏目表名。
func (ArticleCategory) TableName() string { return "article_categories" }

// Article AI 破甲资讯文章。
type Article struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID      string `gorm:"column:app_id;size:64;not null;index;uniqueIndex:idx_article_tenant_slug" json:"app_id"`
	CategoryID int64  `gorm:"column:category_id;index" json:"category_id"`
	Slug       string `gorm:"column:slug;size:128;not null;uniqueIndex:idx_article_tenant_slug" json:"slug"`
	Title      string `gorm:"column:title;size:255;not null" json:"title"`
	Summary    string `gorm:"column:summary;size:500" json:"summary"`
	CoverURL   string `gorm:"column:cover_url;size:512" json:"cover_url"`
	Markdown   string `gorm:"column:markdown;type:longtext;not null" json:"markdown"`
	// 是否启用单篇付费；启用后需购买该文章商品才能阅读全文。
	IsPaid bool `gorm:"column:is_paid;default:false;index" json:"is_paid"`
	// 单篇文章商品 SKU，金额由后台商品配置读取。
	PaySKU string `gorm:"column:pay_sku;size:64" json:"pay_sku"`
	// 单篇文章价格，单位分；仅用于后台展示，支付金额以商品表为准。
	PriceFen int64 `gorm:"column:price_fen;default:0" json:"price_fen"`
	// 访客可阅读的免费试读 Markdown 片段。
	FreeMarkdown  string     `gorm:"column:free_markdown;type:text" json:"free_markdown"`
	Author        string     `gorm:"column:author;size:128" json:"author"`
	Tags          string     `gorm:"column:tags;size:500" json:"tags"`
	RequiredLevel int        `gorm:"column:required_level;default:0;index" json:"required_level"`
	AllowComments bool       `gorm:"column:allow_comments;default:true" json:"allow_comments"`
	Status        string     `gorm:"column:status;size:16;default:'draft';index" json:"status"`
	PublishedAt   *time.Time `gorm:"column:published_at;index" json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 返回文章表名。
func (Article) TableName() string { return "articles" }

// MembershipLevel 会员等级及其可售套餐配置。
type MembershipLevel struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID        string    `gorm:"column:app_id;size:64;not null;index;uniqueIndex:idx_membership_tenant_level;uniqueIndex:idx_membership_tenant_sku" json:"app_id"`
	Level        int       `gorm:"column:level;not null;uniqueIndex:idx_membership_tenant_level" json:"level"`
	Name         string    `gorm:"column:name;size:128;not null" json:"name"`
	SKU          string    `gorm:"column:sku;size:64;not null;uniqueIndex:idx_membership_tenant_sku" json:"sku"`
	PriceFen     int64     `gorm:"column:price_fen;not null" json:"price_fen"`
	DurationDays int       `gorm:"column:duration_days;not null" json:"duration_days"`
	Description  string    `gorm:"column:description;size:500" json:"description"`
	Status       string    `gorm:"column:status;size:16;default:'active';index" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 返回会员等级表名。
func (MembershipLevel) TableName() string { return "membership_levels" }

// UserMembership 用户当前有效会员权益。
type UserMembership struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID       string    `gorm:"column:app_id;size:64;not null;uniqueIndex:idx_membership_user" json:"app_id"`
	UserID      int64     `gorm:"column:user_id;not null;uniqueIndex:idx_membership_user" json:"user_id"`
	Level       int       `gorm:"column:level;not null;index" json:"level"`
	StartedAt   time.Time `gorm:"column:started_at;not null" json:"started_at"`
	ExpiresAt   time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	LastOrderID int64     `gorm:"column:last_order_id;index" json:"last_order_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 返回用户会员表名。
func (UserMembership) TableName() string { return "user_memberships" }

// ArticleComment 文章评论，最多允许两级嵌套。
type ArticleComment struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID           string    `gorm:"column:app_id;size:64;not null;index" json:"app_id"`
	ArticleID       int64     `gorm:"column:article_id;not null;index" json:"article_id"`
	UserID          int64     `gorm:"column:user_id;not null;index" json:"user_id"`
	ParentID        int64     `gorm:"column:parent_id;default:0;index" json:"parent_id"`
	RootID          int64     `gorm:"column:root_id;default:0;index" json:"root_id"`
	Level           int       `gorm:"column:level;not null;default:1" json:"level"`
	ContentType     string    `gorm:"column:content_type;size:16;not null;default:'text'" json:"content_type"`
	Content         string    `gorm:"column:content;type:text" json:"content"`
	ImageURL        string    `gorm:"column:image_url;size:512" json:"image_url,omitempty"`
	ReplyToUserID   int64     `gorm:"column:reply_to_user_id;default:0" json:"reply_to_user_id,omitempty"`
	ReplyToNickname string    `gorm:"column:reply_to_nickname;size:128" json:"reply_to_nickname,omitempty"`
	AuditStatus     string    `gorm:"column:audit_status;size:16;not null;default:'pending';index" json:"audit_status"`
	AuditReason     string    `gorm:"column:audit_reason;size:255" json:"audit_reason,omitempty"`
	ReplyCount      int       `gorm:"column:reply_count;default:0" json:"reply_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName 返回评论表名。
func (ArticleComment) TableName() string { return "article_comments" }

// ContentAuditRecord 微信内容安全审核记录。
type ContentAuditRecord struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID     string    `gorm:"column:app_id;size:64;not null;index" json:"app_id"`
	CommentID int64     `gorm:"column:comment_id;index" json:"comment_id"`
	MediaType string    `gorm:"column:media_type;size:16;not null" json:"media_type"`
	Provider  string    `gorm:"column:provider;size:32;not null;default:'wechat'" json:"provider"`
	Status    string    `gorm:"column:status;size:16;not null" json:"status"`
	TraceID   string    `gorm:"column:trace_id;size:128" json:"trace_id,omitempty"`
	Reason    string    `gorm:"column:reason;size:255" json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 返回内容审核记录表名。
func (ContentAuditRecord) TableName() string { return "content_audit_records" }

// ArticlePurchase 用户单篇文章购买记录，一次购买永久解锁该文章。
type ArticlePurchase struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID       string    `gorm:"column:app_id;size:64;not null;uniqueIndex:idx_article_purchase_user_article" json:"app_id"`
	UserID      int64     `gorm:"column:user_id;not null;uniqueIndex:idx_article_purchase_user_article" json:"user_id"`
	ArticleID   int64     `gorm:"column:article_id;not null;uniqueIndex:idx_article_purchase_user_article" json:"article_id"`
	OrderID     int64     `gorm:"column:order_id;not null;uniqueIndex" json:"order_id"`
	PurchasedAt time.Time `gorm:"column:purchased_at;not null" json:"purchased_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 返回单篇文章购买记录表名。
func (ArticlePurchase) TableName() string { return "article_purchases" }
