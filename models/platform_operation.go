// Package models platform_operation.go
package models

import "time"

// PlatformOperation 保存通用 SDUI 能力动作的幂等结果和状态。
// 领域能力共用该模型，业务差异由 Kind、EntityID 和 Payload 表达。
type PlatformOperation struct {
	// 自增主键。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// 所属小程序 AppID。
	AppID string `gorm:"column:app_id;size:64;not null;index:idx_platform_operation;uniqueIndex:idx_platform_operation_idem" json:"app_id"`
	// 所属用户；匿名能力使用 0。
	UserID int64 `gorm:"column:user_id;not null;index:idx_platform_operation;uniqueIndex:idx_platform_operation_idem" json:"user_id"`
	// 微信用户标识。
	OpenID string `gorm:"column:open_id;size:128;not null;index:idx_platform_operation;uniqueIndex:idx_platform_operation_idem" json:"-"`
	// 受控端点名称。
	Kind string `gorm:"column:kind;size:64;not null;index:idx_platform_operation;uniqueIndex:idx_platform_operation_idem" json:"kind"`
	// 业务实体标识。
	EntityID string `gorm:"column:entity_id;size:128;not null;index:idx_platform_operation" json:"entity_id"`
	// 幂等键；同一租户、用户、端点和幂等键只产生一个结果。
	IdempotencyKey string `gorm:"column:idempotency_key;size:128;not null;uniqueIndex:idx_platform_operation_idem" json:"idempotency_key"`
	// 当前状态。
	Status string `gorm:"column:status;size:32;not null;index" json:"status"`
	// 请求载荷快照。
	Payload string `gorm:"column:payload;type:text" json:"payload,omitempty"`
	// 对外返回结果快照。
	Result string `gorm:"column:result;type:text" json:"result,omitempty"`
	// 创建时间。
	CreatedAt time.Time `json:"created_at"`
	// 更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 返回通用能力动作表名。
func (PlatformOperation) TableName() string { return "platform_operations" }

// WalletAccount 保存本地虚拟账本余额，生产提现仍需接入真实资金服务。
type WalletAccount struct {
	// 自增主键。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// 所属小程序 AppID。
	AppID string `gorm:"column:app_id;size:64;not null;uniqueIndex:idx_wallet_account_user" json:"app_id"`
	// 所属用户。
	UserID int64 `gorm:"column:user_id;not null;uniqueIndex:idx_wallet_account_user" json:"user_id"`
	// 可用余额，单位分。
	BalanceFen int64 `gorm:"column:balance_fen;not null;default:0" json:"balance_fen"`
	// 冻结余额，单位分。
	FrozenFen int64 `gorm:"column:frozen_fen;not null;default:0" json:"frozen_fen"`
	// 创建时间。
	CreatedAt time.Time `json:"created_at"`
	// 更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 返回钱包表名。
func (WalletAccount) TableName() string { return "wallet_accounts" }
