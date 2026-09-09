// Package services comment_test.go
package services

import "testing"

// TestIsTrustedCOSURL 限制评论图片只能来自当前租户的 HTTPS COS CDN。
func TestIsTrustedCOSURL(t *testing.T) {
	tests := []struct {
		name, base, image string
		want              bool
	}{
		{"合法 CDN", "https://cdn.example.com", "https://cdn.example.com/miniapps/a/comment.png", true},
		{"禁止 HTTP", "https://cdn.example.com", "http://cdn.example.com/a.png", false},
		{"禁止域名前缀欺骗", "https://cdn.example.com", "https://cdn.example.com.evil/a.png", false},
		{"禁止外部域名", "https://cdn.example.com", "https://evil.example/a.png", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTrustedCOSURL(tt.base, tt.image); got != tt.want {
				t.Fatalf("isTrustedCOSURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
