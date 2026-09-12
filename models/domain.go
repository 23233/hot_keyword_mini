// Package models domain.go
package models

import "time"

// DomainRecord 保存有归属、状态和版本的业务实体，具体数据由领域服务校验。
type DomainRecord struct {
	// 全局唯一业务标识。
	ID string `gorm:"size:64;primaryKey" json:"id"`
	// 所属租户。
	AppID string `gorm:"size:64;not null;index:idx_domain_owner" json:"app_id"`
	// 实体类型，只允许服务端登记类型。
	Kind string `gorm:"size:32;not null;index:idx_domain_owner" json:"kind"`
	// 实体所有者。
	UserID int64 `gorm:"not null;index:idx_domain_owner" json:"user_id"`
	// 受指派参与者，例如客服或斗师。
	PeerID int64 `gorm:"index" json:"peer_id"`
	// 服务端状态。
	Status string `gorm:"size:32;not null" json:"status"`
	// 数据快照，不直接暴露敏感材料。
	Data string `gorm:"type:longtext" json:"-"`
	// 乐观锁版本。
	Revision int64 `gorm:"not null" json:"revision"`
	// 创建时间。
	CreatedAt time.Time `json:"created_at"`
	// 更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessage 保存会话中的有序消息；序号来自数据库主键。
type ChatMessage struct {
	// 单调递增消息游标。
	ID int64 `gorm:"primaryKey" json:"id"`
	// 所属租户。
	AppID string `gorm:"size:64;not null;index:idx_chat_thread" json:"app_id"`
	// 所属会话。
	ThreadID string `gorm:"size:64;not null;index:idx_chat_thread" json:"thread_id"`
	// 发送者。
	SenderID int64 `json:"sender_id"`
	// 消息文本。
	Content string `gorm:"type:text" json:"content"`
	// 受控媒体标识。
	MediaID string `gorm:"size:64" json:"media_id,omitempty"`
	// 消息状态：sent、recalled、blocked。
	Status string `gorm:"size:16" json:"status"`
	// 发送时间。
	CreatedAt time.Time `json:"created_at"`
}

// WalletEntry 是只追加的余额流水；状态变化由正负分录表达。
type WalletEntry struct {
	// 流水主键。
	ID int64 `gorm:"primaryKey" json:"id"`
	// 所属租户。
	AppID string `gorm:"size:64;not null;index:idx_ledger_user" json:"app_id"`
	// 所属用户。
	UserID int64 `gorm:"not null;index:idx_ledger_user" json:"user_id"`
	// 可用余额变动，单位分。
	AvailableDelta int64 `json:"available_delta"`
	// 冻结余额变动，单位分。
	FrozenDelta int64 `json:"frozen_delta"`
	// 关联业务单。
	ReferenceID string `gorm:"size:64" json:"reference_id"`
	// 操作理由。
	Reason string `gorm:"size:255" json:"reason"`
	// 操作人。
	Actor string `gorm:"size:128" json:"actor"`
	// 记账时间。
	CreatedAt time.Time `json:"created_at"`
}
