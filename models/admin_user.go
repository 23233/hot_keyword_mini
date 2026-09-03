// Package models admin_user.go
package models

import (
	"time"
)

// AdminUser 后台管理系统管理员账户模型
type AdminUser struct {
	// 自增主键ID
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	// 管理员登录账号 (全局唯一)
	Username string `gorm:"column:username;size:64;not null;uniqueIndex:idx_admin_username;comment:登录用户名" json:"username"`
	// 密码安全哈希值
	PasswordHash string `gorm:"column:password_hash;size:128;not null;comment:加盐密码哈希" json:"-"`
	// 独立安全加密盐
	Salt string `gorm:"column:salt;size:64;not null;comment:安全加密盐" json:"-"`
	// 管理员真实姓名或备注
	RealName string `gorm:"column:real_name;size:64;comment:真实姓名" json:"real_name"`
	// 角色权限: super_admin(超级管理员) / editor(运营编辑) / viewer(只读访客)
	Role string `gorm:"column:role;size:32;default:'editor';comment:角色权限" json:"role"`
	// 账户状态: active(正常启用) / disabled(已禁用冻结)
	Status string `gorm:"column:status;size:32;default:'active';comment:账户状态" json:"status"`
	// 最后登录时间
	LastLoginAt *time.Time `gorm:"column:last_login_at;comment:最后登录时间" json:"last_login_at,omitempty"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

// TableName 自定义 AdminUser 模型的表名
func (u *AdminUser) TableName() string {
	return "admin_users"
}
