// Package services mcp_test.go
package services

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMCPToolDefinitions 测试全部 7 个受控 MCP 工具的 Schema 声明
func TestMCPToolDefinitions(t *testing.T) {
	service := NewMCPService()
	tools := service.GetToolDefinitions()

	if len(tools) != 7 {
		t.Fatalf("预期注册 7 个受控工具，实际为 %d", len(tools))
	}

	expectedTools := map[string]bool{
		"sdui.template.list":   false,
		"sdui.page.create":     false,
		"sdui.page.patch":      false,
		"sdui.page.validate":   false,
		"sdui.page.preview":    false,
		"sdui.page.screenshot": false,
		"sdui.page.publish":    false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Fatalf("缺失受控工具: %s", name)
		}
	}
}

// TestMCPInitializeAndList 测试标准 JSON-RPC 初始化与工具列表
func TestMCPInitializeAndList(t *testing.T) {
	service := NewMCPService()

	// 1. 测试 initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	initRespBytes, err := service.HandleJSONRPC([]byte(initReq))
	if err != nil {
		t.Fatalf("initialize 异常: %v", err)
	}

	var initResp JSONRPCResponse
	_ = json.Unmarshal(initRespBytes, &initResp)
	if initResp.Error != nil {
		t.Fatalf("initialize 不应报错: %v", initResp.Error)
	}

	// 2. 测试 tools/list
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	listRespBytes, err := service.HandleJSONRPC([]byte(listReq))
	if err != nil {
		t.Fatalf("tools/list 异常: %v", err)
	}

	var listResp JSONRPCResponse
	_ = json.Unmarshal(listRespBytes, &listResp)
	if listResp.Error != nil {
		t.Fatalf("tools/list 不应报错: %v", listResp.Error)
	}
}

// TestMCPToolCall_TemplateList 测试调用 sdui.template.list
func TestMCPToolCall_TemplateList(t *testing.T) {
	service := NewMCPService()

	callReq := `{
		"jsonrpc": "2.0",
		"id": 3,
		"method": "tools/call",
		"params": {
			"name": "sdui.template.list",
			"arguments": { "business_type": "game" }
		}
	}`

	callRespBytes, err := service.HandleJSONRPC([]byte(callReq))
	if err != nil {
		t.Fatalf("调用 tools/call 失败: %v", err)
	}

	var resp JSONRPCResponse
	_ = json.Unmarshal(callRespBytes, &resp)
	if resp.Error != nil {
		t.Fatalf("工具执行报错: %v", resp.Error)
	}
}

// TestMCPToolCall_PublishGate 测试发布必须人工显式确认与权限作用域控制
func TestMCPToolCall_PublishGate(t *testing.T) {
	service := NewMCPService()

	// 1. 尝试无 release 权限发布，预期被拦截
	_, err := service.ExecuteToolWithContext("ai_agent_1", "wx_test", []string{"read", "write:draft"}, "sdui.page.publish", map[string]interface{}{
		"app_id":    "wx_test",
		"page_id":   "home",
		"confirmed": true,
	})
	if err == nil || !strings.Contains(err.Error(), "release") {
		t.Fatalf("预期缺少 release 权限报错，实际为: %v", err)
	}

	// 2. 有 release 权限但缺少 confirmed: true 人工显式确认，预期被门禁拦截
	_, err = service.ExecuteToolWithContext("ai_agent_1", "wx_test", []string{"release"}, "sdui.page.publish", map[string]interface{}{
		"app_id":  "wx_test",
		"page_id": "home",
	})
	if err == nil || !strings.Contains(err.Error(), "confirmed") {
		t.Fatalf("预期缺少人工确认门禁报错，实际为: %v", err)
	}
}
