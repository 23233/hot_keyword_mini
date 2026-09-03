// Package services share_card_test.go
package services

import (
	"bytes"
	"image/png"
	"testing"
)

// TestShareCardPngGeneration 测试分享卡片生成与 PNG 编码规范
func TestShareCardPngGeneration(t *testing.T) {
	service := NewShareCardService()

	// 1. 测试 5:4 聊天卡片 (1000x800)
	bytes54, err := service.RenderShareCard("wx516563cfe994bbc6", "home", "app_message")
	if err != nil {
		t.Fatalf("生成 5:4 分享卡片失败: %v", err)
	}

	// 校验 PNG 魔数头
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(bytes54, pngHeader) {
		t.Fatalf("生成的数据不符合 PNG 标准二进制格式")
	}

	config54, err := png.DecodeConfig(bytes.NewReader(bytes54))
	if err != nil {
		t.Fatalf("解码 5:4 PNG 配置失败: %v", err)
	}
	if config54.Width != 1000 || config54.Height != 800 {
		t.Fatalf("5:4 卡片尺寸不符合规范: 期望 1000x800, 实际 %dx%d", config54.Width, config54.Height)
	}

	// 2. 测试 1:1 朋友圈卡片 (800x800)
	bytes11, err := service.RenderShareCard("wx516563cfe994bbc6", "home", "timeline")
	if err != nil {
		t.Fatalf("生成 1:1 朋友圈卡片失败: %v", err)
	}

	config11, err := png.DecodeConfig(bytes.NewReader(bytes11))
	if err != nil {
		t.Fatalf("解码 1:1 PNG 配置失败: %v", err)
	}
	if config11.Width != 800 || config11.Height != 800 {
		t.Fatalf("1:1 卡片尺寸不符合规范: 期望 800x800, 实际 %dx%d", config11.Width, config11.Height)
	}
}
