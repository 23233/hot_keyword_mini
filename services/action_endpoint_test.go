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
		{endpoint: "chat.send", status: "sent"},
		{endpoint: "order.create", status: "created"},
		{endpoint: "after_sale.apply", status: "requested"},
		{endpoint: "wallet.withdraw", status: "pending"},
	} {
		payload := map[string]interface{}{"id": "entity-1"}
		if item.endpoint == "wallet.withdraw" {
			payload["amount"] = float64(100)
		}
		result, err := service.ExecuteActionEndpoint("wx-platform-test", "user-1", item.endpoint, payload, "idem-"+item.endpoint)
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

// TestPlatformDomainStateMachines 验证订单、售后、斗师和提现领域状态机。
func TestPlatformDomainStateMachines(t *testing.T) {
	tests := []struct {
		name, endpoint, current, want string
		payload                       map[string]interface{}
		wantErr                       bool
	}{
		{"创建订单", "order.create", "", "created", nil, false},
		{"确认订单", "order.confirm", "created", "confirmed", nil, false},
		{"确认收货", "order.confirm_receipt", "confirmed", "completed", nil, false},
		{"取消后禁止收货", "order.confirm_receipt", "cancelled", "", nil, true},
		{"无订单禁止取消", "order.cancel", "", "", nil, true},
		{"申请售后", "after_sale.apply", "completed", "requested", nil, false},
		{"售后上传证据", "after_sale.upload_evidence", "requested", "evidence_uploaded", nil, false},
		{"无售后单禁止上传", "after_sale.upload_evidence", "", "", nil, true},
		{"斗师接单", "service.accept_task", "pending", "accepted", nil, false},
		{"斗师报价", "service.submit_quote", "accepted", "quoted", map[string]interface{}{"amount": float64(100)}, false},
		{"未接单禁止报价", "service.submit_quote", "pending", "", map[string]interface{}{"amount": float64(100)}, true},
		{"开始服务", "service.update_status", "paid", "in_service", map[string]interface{}{"status": "in_service"}, false},
		{"非法跨级完成", "service.update_status", "quoted", "", map[string]interface{}{"status": "completed"}, true},
		{"有效提现", "wallet.withdraw", "", "pending", map[string]interface{}{"amount": float64(100)}, false},
		{"拒绝零元提现", "wallet.withdraw", "", "", map[string]interface{}{"amount": float64(0)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextPlatformStatus(tt.endpoint, tt.current, tt.payload)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("状态机返回 status=%q err=%v", got, err)
			}
		})
	}
}
