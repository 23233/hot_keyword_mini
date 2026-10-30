// Package services admin_login_limiter.go
package services

import (
	"sync"
	"time"
)

const (
	// adminLoginFailureLimit 同一账号与来源地址在窗口内允许的最大失败次数。
	adminLoginFailureLimit = 5
	// adminLoginWindow 失败计数窗口，同时也是触发后的锁定时长。
	adminLoginWindow = 15 * time.Minute
)

// adminLoginAttempt 保存单个账号与来源地址的失败计数。
type adminLoginAttempt struct {
	// 当前窗口内的失败次数。
	failures int
	// 当前窗口的起始时间。
	firstAt time.Time
	// 锁定截止时间；零值表示未锁定。
	lockedUntil time.Time
}

// adminLoginLimiter 是进程内的管理员登录失败限流器。
// 多副本部署时每个实例独立计数，仍可显著降低单实例暴力破解风险。
type adminLoginLimiter struct {
	mu      sync.Mutex
	entries map[string]*adminLoginAttempt
	now     func() time.Time
}

var defaultAdminLoginLimiter = newAdminLoginLimiter()

// newAdminLoginLimiter 创建带默认时钟的限流器。
func newAdminLoginLimiter() *adminLoginLimiter {
	return &adminLoginLimiter{entries: make(map[string]*adminLoginAttempt), now: time.Now}
}

// CheckAdminLoginAllowed 判断当前账号与来源地址是否允许继续尝试登录。
func CheckAdminLoginAllowed(key string) (bool, time.Duration) {
	return defaultAdminLoginLimiter.allow(key)
}

// RecordAdminLoginFailure 记录一次登录失败，达到阈值后锁定窗口时长。
func RecordAdminLoginFailure(key string) {
	defaultAdminLoginLimiter.recordFailure(key)
}

// ResetAdminLoginFailures 在登录成功后清除失败计数。
func ResetAdminLoginFailures(key string) {
	defaultAdminLoginLimiter.reset(key)
}

// allow 返回是否放行以及剩余锁定时间。
func (l *adminLoginLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.cleanup(now)
	entry, exists := l.entries[key]
	if !exists {
		return true, 0
	}
	if now.Before(entry.lockedUntil) {
		return false, entry.lockedUntil.Sub(now)
	}
	return true, 0
}

// recordFailure 累加失败次数；窗口过期后重新计数。
func (l *adminLoginLimiter) recordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.firstAt) > adminLoginWindow {
		entry = &adminLoginAttempt{firstAt: now}
		l.entries[key] = entry
	}
	entry.failures++
	if entry.failures >= adminLoginFailureLimit {
		entry.lockedUntil = now.Add(adminLoginWindow)
		entry.failures = 0
		entry.firstAt = now
	}
}

// reset 清除指定 key 的失败状态。
func (l *adminLoginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// cleanup 清理已过期且未锁定的记录，避免长期运行内存增长。
func (l *adminLoginLimiter) cleanup(now time.Time) {
	for key, entry := range l.entries {
		if now.Before(entry.lockedUntil) {
			continue
		}
		if now.Sub(entry.firstAt) > adminLoginWindow {
			delete(l.entries, key)
		}
	}
}
