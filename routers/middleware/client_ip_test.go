// Package middleware client_ip_test.go
package middleware

import "testing"

// TestSelectClientIP 验证代理头只在直连对端为内网时生效，并且取最右侧可信跳。
func TestSelectClientIP(t *testing.T) {
	tests := []struct {
		name         string
		peer         string
		realIP       string
		forwardedFor string
		want         string
	}{
		{name: "公网直连忽略伪造头", peer: "203.0.113.9", realIP: "1.2.3.4", forwardedFor: "5.6.7.8", want: "203.0.113.9"},
		{name: "内网代理优先 X-Real-IP", peer: "172.18.0.2", realIP: "198.51.100.7", forwardedFor: "1.1.1.1", want: "198.51.100.7"},
		{name: "内网代理取 XFF 最右跳", peer: "127.0.0.1", forwardedFor: "1.1.1.1, 198.51.100.8", want: "198.51.100.8"},
		{name: "带端口的内网对端会被规范化", peer: "10.0.0.5:54321", realIP: "198.51.100.9", want: "198.51.100.9"},
		{name: "非法代理头回落到对端", peer: "10.0.0.6", realIP: "not-an-ip", forwardedFor: "also-bad", want: "10.0.0.6"},
		{name: "IPv6 回环地址识别", peer: "::1", realIP: "198.51.100.10", want: "198.51.100.10"},
		{name: "空对端且无有效头", peer: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SelectClientIP(test.peer, test.realIP, test.forwardedFor); got != test.want {
				t.Fatalf("SelectClientIP(%q, %q, %q) = %q, want %q", test.peer, test.realIP, test.forwardedFor, got, test.want)
			}
		})
	}
}
