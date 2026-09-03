// Package services sdui_test.go
package services

import (
	"encoding/json"
	"hot_keyword/models"
	"testing"
)

// TestSDUIBlockSerialization 测试 SDUI 积木组件的 JSON 协议编解码
func TestSDUIBlockSerialization(t *testing.T) {
	action := models.BlockAction{
		Type:        "open_channels_activity",
		RequireAuth: false,
		Payload: map[string]interface{}{
			"feed_id":          "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
			"finder_user_name": "gh_drama_official",
		},
	}

	block := models.BlockItem{
		ID:   "block_hero_test",
		Type: "media_hero",
		Props: map[string]interface{}{
			"title":    "猴王下山",
			"subtitle": "全网爆火都市短剧",
		},
		Style: &models.BlockStyle{
			BorderRadius: "28rpx",
			GlassBlur:    true,
			AccentColor:  "#FF9F0A",
		},
		Action: &action,
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("序列化积木组件失败: %v", err)
	}

	var decoded models.BlockItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化积木组件失败: %v", err)
	}

	if decoded.ID != "block_hero_test" || decoded.Type != "media_hero" {
		t.Fatalf("积木基础字段反序列化不一致")
	}
	if decoded.Action == nil || decoded.Action.Type != "open_channels_activity" {
		t.Fatalf("动作协议反序列化异常")
	}
	if decoded.Style == nil || !decoded.Style.GlassBlur {
		t.Fatalf("样式属性反序列化异常")
	}
}

// TestEnvelopeStructure 测试响应信封规范
func TestEnvelopeStructure(t *testing.T) {
	envelope := models.PageResponseEnvelope{
		ProtocolVersion: "1.1",
		SchemaVersion:   3,
		RequestID:       "req_test_123",
		Page: models.DynamicPageDTO{
			PageID:       "home",
			Revision:     1,
			Status:       "published",
			Title:        "猴王下山",
			BusinessType: "drama",
			Intent:       "watch",
			Theme:        "dark_glass",
			Blocks:       []models.BlockItem{},
		},
		Data: map[string]interface{}{
			"keyword": "猴王下山",
		},
		CapabilitiesRequired: []string{"video", "clipboard"},
		Cache: models.EnvelopeCache{
			ETag:   "test_etag",
			MaxAge: 30,
		},
		Fallback: models.EnvelopeFallback{
			PageID: "home",
			Mode:   "static_safe",
		},
	}

	bytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("序列化信封失败: %v", err)
	}

	var decoded models.PageResponseEnvelope
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("反序列化信封失败: %v", err)
	}

	if decoded.ProtocolVersion != "1.1" || decoded.SchemaVersion != 3 {
		t.Fatalf("信封版本号不匹配")
	}
	if decoded.Fallback.PageID != "home" {
		t.Fatalf("降级页面配置错误")
	}
}

// TestAdminSDUIManagement 测试管理端小程序与页面操作逻辑
func TestAdminSDUIManagement(t *testing.T) {
	s := NewSDUIService()

	// 基础参数校验测试
	if err := s.SaveApp(nil); err == nil {
		t.Fatalf("传入空 App 应报错")
	}
	if err := s.SavePage(nil); err == nil {
		t.Fatalf("传入空 Page 应报错")
	}
	if err := s.SetCurrentPage("", ""); err == nil {
		t.Fatalf("空参数设为主页应报错")
	}
}

// TestPageRevisionManagement 测试页面快照与回滚逻辑
func TestPageRevisionManagement(t *testing.T) {
	s := NewSDUIService()

	// 基础参数校验测试
	if _, err := s.ListPageRevisions("", ""); err == nil {
		t.Fatalf("空参数查询版本快照应报错")
	}
	if _, err := s.RollbackPageRevision("", "", 0); err == nil {
		t.Fatalf("非法参数回滚应报错")
	}
}


