// Package models game_redeem.go
package models

import (
	"time"
)

// GameRedeemPackage 游戏礼包套餐配置模型
type GameRedeemPackage struct {
	// 自增主键ID
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	// 所属小程序AppID (多租户物理隔离)
	AppID string `gorm:"column:app_id;size:64;not null;uniqueIndex:idx_app_pkg_id;comment:小程序AppID" json:"app_id"`
	// 礼包唯一标识 (如 pkg_game_novice_888)
	PackageID string `gorm:"column:package_id;size:64;not null;uniqueIndex:idx_app_pkg_id;comment:礼包唯一标识" json:"package_id"`
	// 关联游戏标识
	GameID string `gorm:"column:game_id;size:64;not null;comment:游戏标识" json:"game_id"`
	// 礼包标题 (如 绝地突围公测专属大礼包)
	Title string `gorm:"column:title;size:128;not null;comment:礼包标题" json:"title"`
	// 礼包简介
	Description string `gorm:"column:description;size:255;comment:礼包说明" json:"description"`
	// 初始总库存
	TotalStock int `gorm:"column:total_stock;default:10000;comment:初始总库存" json:"total_stock"`
	// 剩余可用库存
	RemainingStock int `gorm:"column:remaining_stock;default:10000;comment:剩余可用库存" json:"remaining_stock"`
	// 兑换码统一前缀 (用于发放时动态组合)
	CodePrefix string `gorm:"column:code_prefix;size:32;default:'VIP888-';comment:兑换码前缀" json:"code_prefix"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

// TableName 自定义 GameRedeemPackage 模型的表名
func (p *GameRedeemPackage) TableName() string {
	return "game_redeem_packages"
}

// GameRedeemRecord 用户领取礼包兑换码流水记录 (具备幂等防刷与唯一性约束)
type GameRedeemRecord struct {
	// 自增主键ID
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	// 所属小程序AppID
	AppID string `gorm:"column:app_id;size:64;not null;uniqueIndex:idx_app_idem;uniqueIndex:idx_app_pkg_user;comment:小程序AppID" json:"app_id"`
	// 礼包唯一标识
	PackageID string `gorm:"column:package_id;size:64;not null;uniqueIndex:idx_app_pkg_user;comment:礼包标识" json:"package_id"`
	// 领取用户微信OpenID
	OpenID string `gorm:"column:open_id;size:64;not null;uniqueIndex:idx_app_pkg_user;comment:用户微信OpenID" json:"open_id"`
	// 分配给该用户的真实兑换码
	RedeemCode string `gorm:"column:redeem_code;size:64;not null;comment:真实兑换码" json:"redeem_code"`
	// 客户端提交的幂等键 (重复请求返回同一结果)
	IdempotencyKey string `gorm:"column:idempotency_key;size:64;not null;uniqueIndex:idx_app_idem;comment:客户端幂等键" json:"idempotency_key"`
	// 领取时间
	ClaimedAt time.Time `gorm:"column:claimed_at;comment:领取时间" json:"claimed_at"`
}

// TableName 自定义 GameRedeemRecord 模型的表名
func (r *GameRedeemRecord) TableName() string {
	return "game_redeem_records"
}
