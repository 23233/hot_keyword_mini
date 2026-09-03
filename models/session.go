// Package models session.go
package models

import (
	"time"
)

// UserSession 用户登录会话模型 (实现双Token生命周期管理、Refresh Token轮换与重放检测)
type UserSession struct {
	// 会话ID自增主键
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:自增主键" json:"id"`
	// 会话全局唯一标识 UUID/随机串
	SessionID string `gorm:"column:session_id;size:64;not null;uniqueIndex:idx_session_id;comment:会话全局唯一ID" json:"session_id"`
	// 所属小程序AppID
	AppID string `gorm:"column:app_id;size:64;not null;index:idx_session_app_user;comment:小程序AppID" json:"app_id"`
	// 关联的用户ID
	UserID int64 `gorm:"column:user_id;not null;index:idx_session_app_user;comment:用户ID" json:"user_id"`
	// Refresh Token 安全哈希值 (SHA256，不保存明文，防数据库泄露)
	RefreshTokenHash string `gorm:"column:refresh_token_hash;size:128;not null;index:idx_refresh_hash;comment:RefreshToken哈希" json:"-"`
	// 会话是否已被撤销 (如用户登出、管理员强制下线或检测到重放攻击)
	Revoked bool `gorm:"column:revoked;default:false;comment:是否已撤销" json:"revoked"`
	// 撤销原因 (如 logout, token_rotated, replay_detected, admin_kick)
	RevokedReason string `gorm:"column:revoked_reason;size:128;comment:撤销原因" json:"revoked_reason,omitempty"`
	// Refresh Token 过期绝对时间
	ExpiresAt time.Time `gorm:"column:expires_at;not null;comment:过期时间" json:"expires_at"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

// TableName 自定义 UserSession 模型的表名
func (s *UserSession) TableName() string {
	return "user_sessions"
}
