// Package models capability.go
package models

import "time"

// CapabilityMatrixEntry 表示单个小程序租户的能力开关与验收状态。
type CapabilityMatrixEntry struct {
	// 能力状态: disabled / configured / enabled / blocked / degraded。
	State string `json:"state"`
	// 能力协议版本。
	ProtocolVersion string `json:"protocol_version,omitempty"`
	// 最低支持的客户端版本。
	MinimumClientVersion string `json:"minimum_client_version,omitempty"`
	// 微信审核状态。
	ReviewStatus string `json:"review_status,omitempty"`
	// 允许使用的 WebView 域名键。
	DomainKeys []string `json:"domain_keys,omitempty"`
	// 状态说明或阻断原因。
	Reason string `json:"reason,omitempty"`
}

// CapabilityChangeLog 记录租户能力状态变更，供后台发布和 MCP 审计。
type CapabilityChangeLog struct {
	// 自增主键。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// 所属小程序 AppID。
	AppID string `gorm:"column:app_id;size:64;not null;index:idx_capability_log" json:"app_id"`
	// 能力唯一键。
	Capability string `gorm:"column:capability;size:64;not null;index:idx_capability_log" json:"capability"`
	// 变更前状态。
	PreviousState string `gorm:"column:previous_state;size:32" json:"previous_state"`
	// 变更后状态。
	NewState string `gorm:"column:new_state;size:32;not null" json:"new_state"`
	// 本次能力配置快照 JSON。
	ConfigSnapshot string `gorm:"column:config_snapshot;type:text;not null" json:"config_snapshot"`
	// 操作人。
	CreatedBy string `gorm:"column:created_by;size:128;not null" json:"created_by"`
	// 创建时间。
	CreatedAt time.Time `gorm:"column:created_at;index:idx_capability_log" json:"created_at"`
}

// TableName 返回能力变更审计表名。
func (c *CapabilityChangeLog) TableName() string { return "capability_change_logs" }
