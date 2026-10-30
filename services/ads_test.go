// Package services ads_test.go
package services

import (
	"hot_keyword/models"
	"testing"
)

// TestResolveAdSlot 验证广告位在未配置、已配置和指定 slot_id 时的降级行为。
func TestResolveAdSlot(t *testing.T) {
	tests := []struct {
		name        string
		app         *models.MiniApp
		slotID      string
		wantEnabled bool
		wantUnit    string
		wantStatus  string
	}{
		{name: "空租户降级", app: nil, slotID: "ad-1", wantEnabled: false, wantUnit: "", wantStatus: "unconfigured"},
		{name: "未配置广告位", app: &models.MiniApp{AppID: "wx-test"}, wantEnabled: false, wantStatus: "unconfigured"},
		{name: "空白配置视为未配置", app: &models.MiniApp{AppID: "wx-test", AdUnitIDs: " , "}, wantEnabled: false, wantStatus: "unconfigured"},
		{name: "默认取首个广告位", app: &models.MiniApp{AppID: "wx-test", AdUnitIDs: "adunit-a, adunit-b"}, slotID: "other", wantEnabled: true, wantUnit: "adunit-a", wantStatus: "ready"},
		{name: "命中指定广告位", app: &models.MiniApp{AppID: "wx-test", AdUnitIDs: "adunit-a, adunit-b"}, slotID: "adunit-b", wantEnabled: true, wantUnit: "adunit-b", wantStatus: "ready"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ResolveAdSlot(test.app, test.slotID)
			if result.Enabled != test.wantEnabled || result.AdUnitID != test.wantUnit || result.Status != test.wantStatus {
				t.Fatalf("ResolveAdSlot() = %+v, want enabled=%v unit=%q status=%q", result, test.wantEnabled, test.wantUnit, test.wantStatus)
			}
			if result.Provider != "wechat_ad" {
				t.Fatalf("广告提供方应为 wechat_ad，实际为 %q", result.Provider)
			}
		})
	}
}
