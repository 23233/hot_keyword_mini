// Package services mcp_test.go
package services

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMCPToolDefinitions 测试全部受控 MCP 工具的 Schema 声明
func TestMCPToolDefinitions(t *testing.T) {
	service := NewMCPService()
	tools := service.GetToolDefinitions()

	if len(tools) != 15 {
		t.Fatalf("预期注册 15 个受控工具，实际为 %d", len(tools))
	}

	expectedTools := map[string]bool{
		"sdui.app.list":            false,
		"sdui.file.prepare_upload": false,
		"sdui.page.list":           false,
		"sdui.page.get":            false,
		"sdui.template.list":       false,
		"sdui.page.create":         false,
		"sdui.page.patch":          false,
		"sdui.page.validate":       false,
		"sdui.page.preview":        false,
		"sdui.page.screenshot":     false,
		"sdui.page.publish":        false,
		"sdui.page.revisions":      false,
		"sdui.page.rollback":       false,
		"sdui.page.set_current":    false,
		"sdui.page.share_card":     false,
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

// TestMCPResources 测试 AI 可读取 API 与 SDUI 规则资源。
func TestMCPResources(t *testing.T) {
	service := NewMCPService()
	for _, uri := range []string{"sdui://api", "sdui://rules"} {
		request := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"` + uri + `"}}`
		response, err := service.HandleJSONRPC([]byte(request))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", uri, err)
		}
		var decoded JSONRPCResponse
		if err := json.Unmarshal(response, &decoded); err != nil || decoded.Error != nil || decoded.Result == nil {
			t.Fatalf("读取 %s 响应无效: %s", uri, response)
		}
	}
}

// TestMCPRulesMirrorValidator 验证规则资源与服务端实际白名单保持一致。
func TestMCPRulesMirrorValidator(t *testing.T) {
	rules := mcpRulesResource()
	blocks, ok := rules["block_types"].([]string)
	if !ok || len(blocks) != len(allowedBlockTypes) {
		t.Fatalf("规则资源中的积木枚举不完整")
	}
	actions, ok := rules["action_types"].([]string)
	if !ok || len(actions) != len(allowedActionTypes) {
		t.Fatalf("规则资源中的动作枚举不完整")
	}
	if len(rules["condition_operators"].([]string)) != 11 {
		t.Fatalf("规则资源中的条件操作符不完整")
	}
}

// TestMCPInitializeInstructions 验证初始化响应包含先读规则的强制引导。
func TestMCPInitializeInstructions(t *testing.T) {
	response, err := NewMCPService().HandleJSONRPC([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil || !strings.Contains(string(response), "resources/read") || !strings.Contains(string(response), "sdui://rules") {
		t.Fatalf("initialize 未提供规则读取引导: %s", response)
	}
}

// TestMCPCoverageResource 验证资源说明明确区分已覆盖和禁止开放的系统行为。
func TestMCPCoverageResource(t *testing.T) {
	api := mcpAPIResource(NewMCPService().GetToolDefinitions())
	coverage, ok := api["coverage"].(map[string]string)
	if !ok || coverage["draft_creation"] != "sdui.page.create" || coverage["image_upload"] != "sdui.file.prepare_upload" {
		t.Fatalf("MCP 覆盖矩阵缺少核心页面和图片行为")
	}
	unsupported, ok := api["unsupported_or_admin_only"].([]string)
	if !ok || len(unsupported) == 0 {
		t.Fatalf("MCP 未声明系统级未覆盖边界")
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
