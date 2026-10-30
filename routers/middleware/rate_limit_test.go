// Package middleware rate_limit_test.go
package middleware

import (
	"hot_keyword/jwtToken"
	"testing"
	"time"
)

// TestRateLimiterWindow 验证固定窗口内的放行、拦截与窗口恢复。
func TestRateLimiterWindow(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)
	current := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return current }
	limiter.lastCleanup = current
	const key = "198.51.100.20"

	for attempt := 0; attempt < 3; attempt++ {
		if allowed, _ := limiter.Allow(key); !allowed {
			t.Fatalf("第 %d 次请求不应被拦截", attempt+1)
		}
	}
	allowed, retryAfter := limiter.Allow(key)
	if allowed || retryAfter <= 0 {
		t.Fatalf("超过窗口上限后应拦截，allowed=%v retry=%v", allowed, retryAfter)
	}

	current = current.Add(time.Minute + time.Second)
	if allowed, _ := limiter.Allow(key); !allowed {
		t.Fatal("窗口结束后应恢复放行")
	}
}

// TestRateLimiterIsolationAndCleanup 验证不同来源独立计数并清理过期记录。
func TestRateLimiterIsolationAndCleanup(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	current := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return current }
	limiter.lastCleanup = current

	if allowed, _ := limiter.Allow("198.51.100.21"); !allowed {
		t.Fatal("首个来源应放行")
	}
	if allowed, _ := limiter.Allow("198.51.100.22"); !allowed {
		t.Fatal("不同来源应独立计数")
	}
	if allowed, _ := limiter.Allow("198.51.100.21"); allowed {
		t.Fatal("同一来源超过上限应拦截")
	}

	current = current.Add(2 * time.Minute)
	limiter.Allow("198.51.100.23")
	if len(limiter.entries) != 1 {
		t.Fatalf("过期记录应被清理，当前剩余 %d 条", len(limiter.entries))
	}
}

// TestViewerRateLimitKey 验证登录用户按租户+用户计数、匿名回退地址。
func TestViewerRateLimitKey(t *testing.T) {
	token, _ := jwtToken.GenAccessToken("openid-test", "wx_app", "session-test", 42)

	if got := viewerRateLimitKey("", "wx_app", "203.0.113.7"); got != "tenant:wx_app|ip:203.0.113.7" {
		t.Fatalf("匿名限流键错误: %s", got)
	}
	if got := viewerRateLimitKey("Bearer "+token, "wx_app", "203.0.113.7"); got != "tenant:wx_app|user:42" {
		t.Fatalf("登录用户限流键错误: %s", got)
	}
	if got := viewerRateLimitKey("Bearer invalid-token", "wx_app", "203.0.113.7"); got != "tenant:wx_app|ip:203.0.113.7" {
		t.Fatalf("无效凭证应回退地址: %s", got)
	}
	if got := viewerRateLimitKey("", "", "203.0.113.7"); got != "tenant:-|ip:203.0.113.7" {
		t.Fatalf("缺少租户时限流键错误: %s", got)
	}
}

// TestViewerRateLimitSeparatesUsersBehindSameIP 验证同一出口 IP 下不同登录用户独立计数。
func TestViewerRateLimitSeparatesUsersBehindSameIP(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	current := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return current }
	limiter.lastCleanup = current

	tokenA, _ := jwtToken.GenAccessToken("openid-a", "wx_app", "session-a", 101)
	tokenB, _ := jwtToken.GenAccessToken("openid-b", "wx_app", "session-b", 102)
	keyA := viewerRateLimitKey("Bearer "+tokenA, "wx_app", "203.0.113.8")
	keyB := viewerRateLimitKey("Bearer "+tokenB, "wx_app", "203.0.113.8")

	if allowed, _ := limiter.Allow(keyA); !allowed {
		t.Fatal("用户 A 首次请求应放行")
	}
	if allowed, _ := limiter.Allow(keyB); !allowed {
		t.Fatal("同一 IP 下用户 B 不应被用户 A 的计数拦截")
	}
	if allowed, _ := limiter.Allow(keyA); allowed {
		t.Fatal("用户 A 超过上限后应被拦截")
	}
}
