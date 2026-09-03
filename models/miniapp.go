// Package models miniapp.go
package models

import (
	"time"
)

// MiniApp 表示多小程序应用主体配置 (支持一脑多控与多租户隔离)
type MiniApp struct {
	// 小程序 AppID (主键，如 wx516563cfe994bbc6)
	AppID string `gorm:"column:app_id;primaryKey;size:64;comment:小程序AppID" json:"app_id"`
	// 小程序密钥 Secret (换取 openid 与 session_key)
	AppSecret string `gorm:"column:app_secret;size:64;not null;comment:小程序密钥Secret" json:"-"`
	// 小程序名称 (当前蹭的上升指数词/主体名称)
	AppName string `gorm:"column:app_name;size:128;not null;comment:小程序名称" json:"app_name"`
	// 当前线上激活展示的主页ID (默认 home)
	CurrentPage string `gorm:"column:current_page;size:64;default:'home';comment:当前线上激活的主页ID" json:"current_page"`
	// 发布模式: normal(正常模式) / gray(灰度体验) / fallback(应急兜底)
	ReleaseMode string `gorm:"column:release_mode;size:16;default:'normal';comment:发布模式" json:"release_mode"`
	// 故障或过期时的兜底页面ID (默认 home)
	FallbackPageID string `gorm:"column:fallback_page_id;size:64;default:'home';comment:兜底页面ID" json:"fallback_page_id"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

// TableName 自定义 MiniApp 模型的表名
func (m *MiniApp) TableName() string {
	return "mini_apps"
}
