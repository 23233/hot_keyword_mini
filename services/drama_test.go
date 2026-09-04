// Package services drama_test.go
package services

import (
	"testing"
)

// TestGetEpisodeTitle 测试集数标题生成逻辑
func TestGetEpisodeTitle(t *testing.T) {
	title1 := getEpisodeTitle(1)
	if title1 != "潜龙出渊，猴王降世" {
		t.Fatalf("期望第1集标题为'潜龙出渊，猴王降世'，但获取到: %s", title1)
	}

	title20 := getEpisodeTitle(20)
	if title20 != "大结局前篇：猴王登顶都市至尊" {
		t.Fatalf("期望第20集标题为'大结局前篇：猴王登顶都市至尊'，但获取到: %s", title20)
	}

	title99 := getEpisodeTitle(99)
	if title99 == "" {
		t.Fatalf("期望第99集能正常生成默认标题")
	}
}

// TestGetMockRecommendations 测试推荐短剧列表逻辑
func TestGetMockRecommendations(t *testing.T) {
	s := NewDramaService()
	list := s.getMockRecommendations()
	if len(list) == 0 {
		t.Fatalf("期望返回推荐短剧列表，但列表为空")
	}
	if list[0].Title == "" {
		t.Fatalf("期望推荐短剧包含有效标题")
	}
}

// TestGetMockGalleryList 测试画廊列表生成与播放模式丰富性
func TestGetMockGalleryList(t *testing.T) {
	s := NewDramaService()
	list := s.getMockGalleryList(nil)
	if len(list) < 5 {
		t.Fatalf("期望画廊短剧数量不少于5部，实际为: %d", len(list))
	}

	hasChannels := false
	hasDirect := false
	hasWeb := false
	hasNone := false

	hasEmbedded := false

	for _, d := range list {
		if d.PlayMode == "channels_video" {
			hasChannels = true
		}
		if d.PlayMode == "channels_embedded" {
			hasEmbedded = true
			if d.FinderUserName == "" {
				t.Fatalf("内嵌视频号模式必须包含 FinderUserName")
			}
		}
		if d.PlayMode == "direct_video" {
			hasDirect = true
		}
		if d.PlayMode == "web_view" {
			hasWeb = true
		}
		if d.PlayMode == "none" {
			hasNone = true
		}
	}

	if !hasChannels || !hasEmbedded || !hasDirect || !hasWeb || !hasNone {
		t.Fatalf("期望覆盖所有播放模式(channels_video, channels_embedded, direct_video, web_view, none)")
	}
}
