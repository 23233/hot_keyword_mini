// Package models membership_grant.go
package models

import "time"

// MembershipGrant 保存订单权益快照，退款只撤销对应订单的权益。
type MembershipGrant struct {
	// 自增主键。
	ID int64 `gorm:"primaryKey" json:"id"`
	// 所属租户。
	AppID string `gorm:"size:64;not null;index:idx_grant_user" json:"app_id"`
	// 权益所属用户。
	UserID int64 `gorm:"not null;index:idx_grant_user" json:"user_id"`
	// 支付订单，同一订单只发放一次。
	OrderID int64 `gorm:"not null;uniqueIndex" json:"order_id"`
	// 会员等级快照。
	Level int `json:"level"`
	// 有效天数快照。
	DurationDays int `json:"duration_days"`
	// 单篇文章权益，会员套餐为零。
	ArticleID int64 `json:"article_id"`
	// 实际发放时间，用于确定性重算。
	GrantedAt time.Time `json:"granted_at"`
	// 退款撤销时间，为空表示仍有效。
	ReversedAt *time.Time `json:"reversed_at,omitempty"`
}
