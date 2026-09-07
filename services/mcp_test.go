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

	if len(tools) != 18 {
		t.Fatalf("预期注册 18 个受控工具，实际为 %d", len(tools))
	}

	expectedTools := map[string]bool{
		"sdui.app.list":            false,
		"sdui.file.prepare_upload": false,
		"sdui.page.list":           false,
		"sdui.page.get":            false,
		"sdui.template.list":       false,
		"sdui.template.get":        false,
		"sdui.template.save":       false,
		"sdui.template.delete":     false,
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

// TestMCPToolCall_TemplateGet 验证 MCP 与 HTTP 共用的模板服务可读取内置完整协议。
func TestMCPToolCall_TemplateGet(t *testing.T) {
	service := NewMCPService()
	result, err := service.ExecuteTool("sdui.template.get", map[string]interface{}{"template_id": "tpl_sdui_component_lab"})
	if err != nil {
		t.Fatalf("读取组件实验室模板失败: %v", err)
	}
	template, ok := result.(*SDUITemplate)
	if !ok || !template.Builtin || len(template.DefaultBlocks) == 0 {
		t.Fatalf("MCP 返回模板协议异常: %+v", result)
	}
}

// TestMCPTemplateWriteScope 验证用户模板写操作必须持有 write:draft 权限。
func TestMCPTemplateWriteScope(t *testing.T) {
	service := NewMCPService()
	_, err := service.ExecuteToolWithContext("ai_agent", "wx_template_test", []string{"read"}, "sdui.template.save", map[string]interface{}{
		"app_id":   "wx_template_test",
		"template": map[string]interface{}{"template_id": "tpl_denied", "name": "应被拒绝"},
	})
	if err == nil || !strings.Contains(err.Error(), "write:draft") {
		t.Fatalf("缺少 write:draft 权限时应被拒绝，实际: %v", err)
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

// TestMCPToolCall_PageValidateDirect 测试 MCP 校验工具直接传入页面协议对象无需依赖落库
func TestMCPToolCall_PageValidateDirect(t *testing.T) {
	service := NewMCPService()

	directPage := map[string]interface{}{
		"app_id":        "wx_test",
		"page_id":       "test_mcp_page",
		"title":         "测试页面",
		"business_type": "drama",
		"blocks": `[
			{
				"id": "hero_1",
				"type": "media_hero",
				"props": { "title": "短剧标题" }
			}
		]`,
	}

	result, err := service.ExecuteToolWithContext("ai_agent_1", "wx_test", []string{"read"}, "sdui.page.validate", map[string]interface{}{
		"page": directPage,
	})
	if err != nil {
		t.Fatalf("直接校验页面协议失败: %v", err)
	}

	report, ok := result.(ValidationReport)
	if !ok {
		t.Fatalf("返回结果非 ValidationReport 类型: %T", result)
	}
	if !report.IsValid {
		t.Fatalf("预期协议校验通过，实际结果为: %v", report)
	}

	// 2. 测试当 page 省略 app_id 时自动继承上下文 tenantID 注入
	directPageNoAppID := map[string]interface{}{
		"page_id":       "test_mcp_page_no_app",
		"title":         "测试页面",
		"business_type": "game",
		"blocks": `[
			{
				"id": "game_1",
				"type": "game_card",
				"props": { "title": "游戏标题" }
			}
		]`,
	}
	res2, err := service.ExecuteToolWithContext("ai_agent_1", "wx_tenant_auto", []string{"read"}, "sdui.page.validate", map[string]interface{}{
		"page": directPageNoAppID,
	})
	if err != nil {
		t.Fatalf("省略 app_id 继承租户校验失败: %v", err)
	}
	rep2, ok := res2.(ValidationReport)
	if !ok || !rep2.IsValid {
		t.Fatalf("预期自动继承租户后校验通过，实际为: %v", res2)
	}
}
