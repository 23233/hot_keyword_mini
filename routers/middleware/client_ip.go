// Package middleware client_ip.go
package middleware

import (
	"net"
	"strings"

	"github.com/kataras/iris/v12"
)

// ClientIP 返回用于匿名限流与风控的客户端地址。
// 只有当直连对端是内网或回环地址（即本机反向代理）时才信任代理头，
// 避免公网直连客户端伪造 X-Real-IP / X-Forwarded-For 绕过频控。
func ClientIP(ctx iris.Context) string {
	return SelectClientIP(ctx.RemoteAddr(), ctx.GetHeader("X-Real-IP"), ctx.GetHeader("X-Forwarded-For"))
}

// SelectClientIP 从直连地址与代理头中选择稳定的客户端 IP。
func SelectClientIP(peer, realIP, forwardedFor string) string {
	peer = normalizeHost(peer)
	if !isPrivatePeer(peer) {
		return peer
	}
	if candidate := strings.TrimSpace(realIP); net.ParseIP(candidate) != nil {
		return candidate
	}
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		// 取最右侧一跳：该值由离服务最近的受信代理写入，客户端无法覆盖。
		candidate := strings.TrimSpace(parts[len(parts)-1])
		if net.ParseIP(candidate) != nil {
			return candidate
		}
	}
	return peer
}

// normalizeHost 去掉可能存在的端口，仅保留主机地址。
func normalizeHost(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return value
}

// isPrivatePeer 判断直连对端是否属于内网、回环或链路本地地址。
func isPrivatePeer(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
