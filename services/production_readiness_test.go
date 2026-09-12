// Package services production_readiness_test.go
package services

import "testing"

// TestValidHTTPSRoot 验证生产资源地址只接受无参数 HTTPS 根地址。
func TestValidHTTPSRoot(t *testing.T) {
	for value, want := range map[string]bool{
		"https://cdn.example.com":     true,
		"http://cdn.example.com":      false,
		"https://cdn.example.com?a=1": false,
		"":                            false,
	} {
		if got := validHTTPSRoot(value); got != want {
			t.Fatalf("validHTTPSRoot(%q)=%v", value, got)
		}
	}
}
