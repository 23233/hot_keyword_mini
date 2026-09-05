// Package services mcp_token_test.go
package services

import "testing"

// TestNormalizeMCPScopes 验证非法权限和重复权限不会进入令牌授权范围。
func TestNormalizeMCPScopes(t *testing.T) {
	got := normalizeMCPScopes([]string{"read", " release ", "read", "invalid"})
	if len(got) != 2 || got[0] != "read" || got[1] != "release" {
		t.Fatalf("MCP 权限规范化结果错误: %v", got)
	}
}
