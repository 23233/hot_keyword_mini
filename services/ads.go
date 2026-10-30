// Package services ads.go
package services

import (
	"hot_keyword/models"
	"strings"
)

// AdSlotResolution 是受控广告位的解析结果，只包含客户端展示所需的公开字段。
type AdSlotResolution struct {
	// 客户端请求的广告位标识。
	SlotID string `json:"slot_id"`
	// 当前租户是否已配置可投放广告位。
	Enabled bool `json:"enabled"`
	// 广告位状态：ready / unconfigured。
	Status string `json:"status"`
	// 广告提供方；当前统一为微信流量主。
	Provider string `json:"provider"`
	// 实际下发的微信广告位 ID；未配置时为空。
	AdUnitID string `json:"ad_unit_id,omitempty"`
	// 面向客户端的降级说明。
	Message string `json:"message"`
}

// ResolveAdSlot 按租户配置解析广告位；不调用广告平台，也不返还任何密钥。
// 未配置广告位时返回可展示的降级结果，避免页面因广告能力缺失而白屏。
func ResolveAdSlot(app *models.MiniApp, slotID string) AdSlotResolution {
	slotID = strings.TrimSpace(slotID)
	result := AdSlotResolution{
		SlotID:   slotID,
		Enabled:  false,
		Status:   "unconfigured",
		Provider: "wechat_ad",
		Message:  "广告位未配置，客户端展示降级占位",
	}
	if app == nil {
		return result
	}
	units := splitAdUnitIDs(app.AdUnitIDs)
	if len(units) == 0 {
		return result
	}
	resolved := units[0]
	if slotID != "" {
		for _, unit := range units {
			if unit == slotID {
				resolved = unit
				break
			}
		}
	}
	result.Enabled = true
	result.Status = "ready"
	result.AdUnitID = resolved
	result.Message = "广告位已配置"
	return result
}

// splitAdUnitIDs 解析逗号分隔的广告位配置并过滤空值。
func splitAdUnitIDs(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if unit := strings.TrimSpace(part); unit != "" {
			result = append(result, unit)
		}
	}
	return result
}
