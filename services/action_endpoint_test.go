// Package services action_endpoint_test.go
package services

import (
	"testing"
)

// TestEndpointRegistry 测试受控端点注册表机制
func TestEndpointRegistry(t *testing.T) {
	service := NewActionEndpointService()

	// 1. 测试未登记端点被拒绝
	_, err := service.ExecuteActionEndpoint("wx516563cfe994bbc6", "user_1", "illegal.proxy.url", nil, "")
	if err == nil {
		t.Fatalf("未登记的非法端点应当被严格拒绝")
	}

	// 2. 测试合法内置端点 game.redeem 执行
	res, err := service.ExecuteActionEndpoint("wx516563cfe994bbc6", "user_1", "game.redeem", map[string]interface{}{
		"package_id": "pkg_game_novice_888",
	}, "idem_test_12345")
	if err != nil {
		t.Fatalf("执行 game.redeem 异常: %v", err)
	}

	dataMap, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("返回值应为 map[string]interface{}")
	}

	if dataMap["code"] == "" {
		t.Fatalf("应成功签发兑换码")
	}
}
// TestActionEndpointValidation 测试入参校验
func TestActionEndpointValidation(t *testing.T) {
	service := NewActionEndpointService()

	if _, err := service.ExecuteActionEndpoint("", "user_1", "game.redeem", nil, ""); err == nil {
		t.Fatalf("空 app_id 应报错")
	}

	if _, err := service.ExecuteActionEndpoint("wx516", "user_1", "", nil, ""); err == nil {
		t.Fatalf("空 endpoint 应报错")
	}
}

// TestQueryScoreEndpoint 测试官方考分查询受控端点
func TestQueryScoreEndpoint(t *testing.T) {
	service := NewActionEndpointService()

	// 1. 成功查询
	res, err := service.ExecuteActionEndpoint("wx516563cfe994bbc6", "", "query.score", map[string]interface{}{
		"query_value": "110101199003072345",
	}, "")
	if err != nil {
		t.Fatalf("执行 query.score 异常: %v", err)
	}

	dataMap, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("返回值应为 map[string]interface{}")
	}
	if dataMap["status"] != "success" || dataMap["score"] == nil {
		t.Fatalf("query.score 应成功返回状态与分数")
	}

	// 2. 空关键词校验
	if _, err := service.ExecuteActionEndpoint("wx516563cfe994bbc6", "", "query.score", map[string]interface{}{}, ""); err == nil {
		t.Fatalf("空 query_value 应报错")
	}
}
