// Package main main_test.go
package main

import (
	"hot_keyword/config"
	"testing"
)

// TestShouldEnsureSDUIAcceptanceData 验证验收数据不会随生产环境启动。
func TestShouldEnsureSDUIAcceptanceData(t *testing.T) {
	if shouldEnsureSDUIAcceptanceData(true, &config.Config{AppEnv: "development"}) {
		t.Fatal("生产构建不能同步验收数据")
	}
	if shouldEnsureSDUIAcceptanceData(false, &config.Config{AppEnv: "production"}) {
		t.Fatal("生产运行环境不能同步验收数据")
	}
	if !shouldEnsureSDUIAcceptanceData(false, &config.Config{AppEnv: "development"}) {
		t.Fatal("开发环境应同步验收数据")
	}
}
