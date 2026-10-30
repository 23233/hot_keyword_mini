// Package services admin_login_limiter_test.go
package services

import (
	"testing"
	"time"
)

// TestAdminLoginLimiter 验证失败阈值、锁定窗口、过期恢复与成功重置。
func TestAdminLoginLimiter(t *testing.T) {
	limiter := newAdminLoginLimiter()
	current := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return current }
	const key = "admin@198.51.100.1"

	for attempt := 0; attempt < adminLoginFailureLimit; attempt++ {
		if allowed, _ := limiter.allow(key); !allowed {
			t.Fatalf("第 %d 次尝试不应提前锁定", attempt+1)
		}
		limiter.recordFailure(key)
	}
	allowed, retryAfter := limiter.allow(key)
	if allowed || retryAfter <= 0 {
		t.Fatalf("达到失败阈值后应锁定，allowed=%v retry=%v", allowed, retryAfter)
	}

	// 锁定窗口结束后恢复放行。
	current = current.Add(adminLoginWindow + time.Second)
	if allowed, _ := limiter.allow(key); !allowed {
		t.Fatal("锁定窗口结束后应恢复放行")
	}

	// 成功登录会清除失败计数。
	limiter.recordFailure(key)
	limiter.reset(key)
	if allowed, _ := limiter.allow(key); !allowed {
		t.Fatal("重置后应允许登录")
	}
}

// TestAdminLoginLimiterWindowReset 验证窗口外的失败不会累计到下一窗口。
func TestAdminLoginLimiterWindowReset(t *testing.T) {
	limiter := newAdminLoginLimiter()
	current := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return current }
	const key = "editor@198.51.100.2"

	for attempt := 0; attempt < adminLoginFailureLimit-1; attempt++ {
		limiter.recordFailure(key)
	}
	current = current.Add(adminLoginWindow + time.Second)
	limiter.recordFailure(key)
	if allowed, _ := limiter.allow(key); !allowed {
		t.Fatal("跨窗口的失败次数不应累计触发锁定")
	}
}
