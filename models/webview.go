// Package models webview.go
package models

import "time"

// WebViewTicket 一次性登录 WebView 票据。
type WebViewTicket struct {
	// 自增主键
	ID int64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// 票据哈希，数据库不保存可直接使用的明文票据
	TokenHash string `gorm:"column:token_hash;size:64;not null;uniqueIndex" json:"-"`
	// 所属小程序 AppID
	AppID string `gorm:"column:app_id;size:64;not null;index" json:"app_id"`
	// 关联用户 ID
	UserID int64 `gorm:"column:user_id;not null;index" json:"user_id"`
	// 目标 WebView 地址
	TargetURL string `gorm:"column:target_url;type:text;not null" json:"target_url"`
	// 票据过期时间
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	// 首次消费时间，非空表示票据已使用
	UsedAt *time.Time `gorm:"column:used_at;index" json:"used_at,omitempty"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 返回 WebView 票据表名。
func (w *WebViewTicket) TableName() string { return "webview_tickets" }
