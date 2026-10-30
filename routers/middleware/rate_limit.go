// Package middleware rate_limit.go
package middleware

import (
	"fmt"
	"hot_keyword/jwtToken"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kataras/iris/v12"
)

const (
	// defaultHTTPRateLimit 每个来源地址每分钟的默认请求上限。
	defaultHTTPRateLimit = 600
	// rateLimitMaxEntries 单个限流器允许保留的最大来源地址数量，防止极端轮换耗尽内存。
	rateLimitMaxEntries = 100000
)

// rateLimitEntry 是固定窗口内的请求计数。
type rateLimitEntry struct {
	// 当前窗口起始时间。
	windowStart time.Time
	// 当前窗口内的请求次数。
	count int
}

// RateLimiter 是进程内固定窗口限流器。
// 多副本部署时每个实例独立计数，作为网关限流之外的应用层兜底。
type RateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	entries     map[string]*rateLimitEntry
	now         func() time.Time
	lastCleanup time.Time
}

// NewRateLimiter 创建指定上限与窗口的限流器。
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:       limit,
		window:      window,
		entries:     make(map[string]*rateLimitEntry),
		now:         time.Now,
		lastCleanup: time.Now(),
	}
}

// Allow 判断指定 key 是否仍可请求，并返回窗口剩余时间。
func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.cleanup(now)
	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.windowStart) >= l.window {
		// 极端情况下允许失败开放，避免限流器自身成为全站故障点。
		if !exists && len(l.entries) >= rateLimitMaxEntries {
			return true, 0
		}
		l.entries[key] = &rateLimitEntry{windowStart: now, count: 1}
		return true, 0
	}
	if entry.count >= l.limit {
		return false, l.window - now.Sub(entry.windowStart)
	}
	entry.count++
	return true, 0
}

// cleanup 清理已过期的窗口记录，避免长期运行内存增长。
func (l *RateLimiter) cleanup(now time.Time) {
	if now.Sub(l.lastCleanup) < l.window {
		return
	}
	l.lastCleanup = now
	for key, entry := range l.entries {
		if now.Sub(entry.windowStart) >= l.window {
			delete(l.entries, key)
		}
	}
}

// Middleware 返回按客户端地址限流的 Iris 中间件。
func (l *RateLimiter) Middleware() iris.Handler {
	return l.middlewareWithKey(ClientIP)
}

// ViewerMiddleware 返回按租户内登录用户限流的中间件，匿名请求回退客户端地址。
func (l *RateLimiter) ViewerMiddleware() iris.Handler {
	return l.middlewareWithKey(func(ctx iris.Context) string {
		return viewerRateLimitKey(ctx.GetHeader("Authorization"), GetTenantAppID(ctx), ClientIP(ctx))
	})
}

// middlewareWithKey 使用指定键提取器构建限流中间件，统一 429 响应格式。
func (l *RateLimiter) middlewareWithKey(keyFn func(iris.Context) string) iris.Handler {
	return func(ctx iris.Context) {
		allowed, retryAfter := l.Allow(keyFn(ctx))
		if !allowed {
			seconds := int(retryAfter.Seconds()) + 1
			ctx.Header("Retry-After", strconv.Itoa(seconds))
			ctx.StatusCode(iris.StatusTooManyRequests)
			_ = ctx.JSON(iris.Map{
				"code":                429,
				"msg":                 "请求过于频繁，请稍后再试",
				"retry_after_seconds": seconds,
			})
			return
		}
		ctx.Next()
	}
}

// viewerRateLimitKey 构造租户级限流键：有效登录态按用户计数，否则按客户端地址。
func viewerRateLimitKey(authHeader, tenant, ip string) string {
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		tenant = "-"
	}
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if claims, err := jwtToken.ParseTokenString(tokenStr); err == nil && claims != nil {
			switch value := claims[jwtToken.JwtUserId].(type) {
			case float64:
				if value > 0 {
					return fmt.Sprintf("tenant:%s|user:%d", tenant, int64(value))
				}
			case int64:
				if value > 0 {
					return fmt.Sprintf("tenant:%s|user:%d", tenant, value)
				}
			case string:
				if userID, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64); parseErr == nil && userID > 0 {
					return fmt.Sprintf("tenant:%s|user:%d", tenant, userID)
				}
			}
		}
	}
	return fmt.Sprintf("tenant:%s|ip:%s", tenant, ip)
}

// RateLimit 创建按客户端地址限流的路由级中间件。
func RateLimit(limit int, window time.Duration) iris.Handler {
	return NewRateLimiter(limit, window).Middleware()
}

// ViewerRateLimit 创建按租户内用户限流的路由级中间件，匿名回退客户端地址。
func ViewerRateLimit(limit int, window time.Duration) iris.Handler {
	return NewRateLimiter(limit, window).ViewerMiddleware()
}

// GlobalRateLimit 创建全站 API 限流中间件，并跳过健康检查与静态资源。
func GlobalRateLimit() iris.Handler {
	limiter := NewRateLimiter(httpRateLimitPerMinute(), time.Minute)
	inner := limiter.Middleware()
	skipPrefixes := []string{"/healthz", "/assets/", "/static/"}
	return func(ctx iris.Context) {
		path := ctx.Request().URL.Path
		for _, prefix := range skipPrefixes {
			if path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix) {
				ctx.Next()
				return
			}
		}
		inner(ctx)
	}
}

// httpRateLimitPerMinute 读取环境变量配置的全站每分钟请求上限。
func httpRateLimitPerMinute() int {
	raw := strings.TrimSpace(os.Getenv("HTTP_RATE_LIMIT_PER_MINUTE"))
	if raw == "" {
		return defaultHTTPRateLimit
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultHTTPRateLimit
	}
	return value
}
