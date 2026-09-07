// Package services webview_test.go
package services

import (
	"strings"
	"testing"
)

// TestWebViewTicketValidation 测试 WebView 票据创建时的参数安全校验与非法 URL 拦截
func TestWebViewTicketValidation(t *testing.T) {
	service := NewWebViewTicketService()

	// 1. 测试空 AppID 或非法 UserID
	if _, err := service.CreateTicket("", 1, "https://example.com"); err == nil {
		t.Fatalf("空 AppID 应当被拦截报错")
	}
	if _, err := service.CreateTicket("wx_test", 0, "https://example.com"); err == nil {
		t.Fatalf("UserID <= 0 应当被拦截报错")
	}
	if _, err := service.CreateTicket("wx_test", -1, "https://example.com"); err == nil {
		t.Fatalf("负数 UserID 应当被拦截报错")
	}

	// 2. 测试非法 URL 协议与格式拦截 (仅允许 HTTP / HTTPS)
	invalidURLs := []string{
		"",
		"javascript:alert(1)",
		"ftp://files.example.com",
		"file:///etc/passwd",
		"data:text/html;base64,...",
		"not-a-url",
		"https://",
	}

	for _, badURL := range invalidURLs {
		if _, err := service.CreateTicket("wx_test", 1001, badURL); err == nil {
			t.Errorf("非法地址 '%s' 应被拒绝，但未报错", badURL)
		}
	}
}

// TestWebViewTicketConsume_EmptyToken 测试空票据消费防御
func TestWebViewTicketConsume_EmptyToken(t *testing.T) {
	service := NewWebViewTicketService()

	if _, err := service.ConsumeTicket(""); err == nil {
		t.Fatalf("空 Token 消费应当报错")
	}

	if _, err := service.ConsumeTicket("   "); err == nil {
		t.Fatalf("纯空格 Token 消费应当报错")
	}
}

// TestWebViewTicketURLScheme 测试合法 HTTPS 地址能够正常格式化
func TestWebViewTicketURLScheme(t *testing.T) {
	// 当数据库未连接时，预期报错包含数据库提示，但通过前置 URL 校验
	service := NewWebViewTicketService()
	_, err := service.CreateTicket("wx_test", 1001, "https://m.example.com/h5/activity?from=share")
	if err == nil {
		t.Fatalf("未初始化数据库时应返回明确数据库错误")
	}
	if !strings.Contains(err.Error(), "数据库") {
		t.Fatalf("预期返回数据库错误，实际得到: %v", err)
	}
}
