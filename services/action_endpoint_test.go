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

// TestAnonymousGameRedeem 验证游戏兑换码首发链路允许匿名领取。
func TestAnonymousGameRedeem(t *testing.T) {
	service := NewActionEndpointService()
	result, err := service.ExecuteActionEndpoint("wx-game-anonymous", "", "game.redeem", map[string]interface{}{
		"package_id": "pkg_game_novice_888",
	}, "idem-anonymous-1")
	if err != nil {
		t.Fatalf("匿名领取兑换码失败: %v", err)
	}
	data, ok := result.(map[string]interface{})
	if !ok || data["code"] == "" {
		t.Fatalf("匿名领取未返回兑换码: %#v", result)
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

// TestPlatformSandboxEndpoints 验证通用领域端点统一执行权限、租户和状态回显。
func TestPlatformSandboxEndpoints(t *testing.T) {
	service := NewActionEndpointService()
	for _, item := range []struct {
		endpoint string
		status   string
	}{
		{endpoint: "chat.send", status: "accepted"},
		{endpoint: "order.cancel", status: "cancelled"},
		{endpoint: "order.confirm_receipt", status: "completed"},
		{endpoint: "after_sale.apply", status: "requested"},
		{endpoint: "wallet.withdraw", status: "pending"},
	} {
		result, err := service.ExecuteActionEndpoint("wx-platform-test", "user-1", item.endpoint, map[string]interface{}{"id": "entity-1"}, "idem-1")
		if err != nil {
			t.Fatalf("端点 %s 执行失败: %v", item.endpoint, err)
		}
		data, ok := result.(map[string]interface{})
		if !ok || data["sandbox"] != true || data["status"] != item.status || data["app_id"] != "wx-platform-test" {
			t.Fatalf("端点 %s 返回不符合沙箱契约: %#v", item.endpoint, result)
		}
	}
	if _, err := service.ExecuteActionEndpoint("wx-platform-test", "", "chat.send", nil, ""); err == nil {
		t.Fatal("聊天端点必须拒绝未登录请求")
	}
}
