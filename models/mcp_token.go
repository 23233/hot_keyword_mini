// Package models mcp_token.go
package models

import "time"

// MCPAccessToken 表示管理后台签发的 MCP 访问令牌。
type MCPAccessToken struct {
	// 自增主键 ID。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// 令牌名称，便于识别调用方。
	Name string `gorm:"column:name;size:64;not null" json:"name"`
	// 令牌 SHA-256 哈希，数据库不保存明文。
	TokenHash string `gorm:"column:token_hash;size:64;not null;uniqueIndex:idx_mcp_token_hash" json:"-"`
	// 令牌展示前缀。
	TokenPrefix string `gorm:"column:token_prefix;size:16;not null" json:"token_prefix"`
	// 兼容旧字段；新令牌为空表示全局令牌，不绑定小程序。
	AppID string `gorm:"column:app_id;size:64;index:idx_mcp_token_app" json:"app_id,omitempty"`
	// 逗号分隔的权限范围。
	Scopes string `gorm:"column:scopes;size:128;not null" json:"scopes"`
	// 状态：active 或 disabled。
	Status string `gorm:"column:status;size:16;not null;default:'active'" json:"status"`
	// 最后使用时间。
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
	// 创建时间。
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	// 更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 返回 MCP 访问令牌表名。
func (MCPAccessToken) TableName() string {
	return "mcp_access_tokens"
}
