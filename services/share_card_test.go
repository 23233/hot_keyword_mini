// Package services share_card_test.go
package services

import (
	"bytes"
	"crypto/sha256"
	"hot_keyword/models"
	"image/png"
	"testing"
	"time"
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

// TestRenderPageLayoutIRScreenshotMatchesIR 验证统一截图入口返回的字节与 Layout IR 直接渲染完全一致。
func TestRenderPageLayoutIRScreenshotMatchesIR(t *testing.T) {
	page := &models.DynamicPage{
		AppID:       "wx_test",
		PageID:      "screenshot_match",
		Title:       "截图一致性",
		Theme:       "dark_glass",
		AccentColor: "#0A84FF",
		Blocks:      `[{"id":"title","type":"text","props":{"content":"一致"}}]`,
	}

	service := NewShareCardService()
	actual, ir, err := service.RenderPageLayoutIRScreenshot(page, "iPhone 12/13 Pro", "normal", "dark_glass")
	if err != nil {
		t.Fatalf("统一截图入口失败: %v", err)
	}
	direct, err := service.RenderLayoutIRScreenshot(ir)
	if err != nil {
		t.Fatalf("直接渲染 Layout IR 失败: %v", err)
	}
	actualHash := sha256.Sum256(actual)
	directHash := sha256.Sum256(direct)
	if actualHash != directHash {
		t.Fatalf("统一截图入口与 IR 直接渲染哈希不一致: %x != %x", actualHash, directHash)
	}
	if page.Theme != "dark_glass" {
		t.Fatalf("主题覆盖不应污染原始页面: %s", page.Theme)
	}
}

// TestScreenshotSignatureWithOptions 验证渲染参数纳入签名，篡改设备或状态会被拒绝。
func TestScreenshotSignatureWithOptions(t *testing.T) {
	expires := time.Now().Add(time.Hour).Unix()
	sign := GenerateScreenshotSignatureWithOptions("wx_test", "page", "hash", expires, "iPhone 12/13 Pro", "dark_glass", "normal")
	if !ValidateScreenshotSignatureWithOptions("wx_test", "page", "hash", expires, sign, "iPhone 12/13 Pro", "dark_glass", "normal") {
		t.Fatal("有效的带参数截图签名未通过校验")
	}
	if ValidateScreenshotSignatureWithOptions("wx_test", "page", "hash", expires, sign, "iPhone SE", "dark_glass", "normal") {
		t.Fatal("篡改设备参数后截图签名不应通过")
	}
}
