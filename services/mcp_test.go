// Package services mcp_test.go
package services

import (
	"encoding/json"
	"hot_keyword/models"
	"strings"
	"testing"
)

// TestMCPToolDefinitions 测试全部受控 MCP 工具的 Schema 声明
func TestMCPToolDefinitions(t *testing.T) {
	service := NewMCPService()
	tools := service.GetToolDefinitions()

	if len(tools) != 28 {
		t.Fatalf("预期注册 28 个受控工具，实际为 %d", len(tools))
	}

	expectedTools := map[string]bool{
		"sdui.app.list":             false,
		"sdui.capability.list":      false,
		"sdui.capability.validate":  false,
		"sdui.capability.configure": false,
		"sdui.acceptance.run":       false,
		"sdui.webview.list":         false,
		"sdui.webview.validate":     false,
		"sdui.file.prepare_upload":  false,
		"sdui.page.list":            false,
		"sdui.page.get":             false,
		"sdui.template.list":        false,
		"sdui.template.get":         false,
		"sdui.template.save":        false,
		"sdui.template.delete":      false,
		"sdui.page.create":          false,
		"sdui.page.patch":           false,
		"sdui.page.validate":        false,
		"sdui.page.preview":         false,
		"sdui.page.screenshot":      false,
		"sdui.page.publish":         false,
		"sdui.page.revisions":       false,
		"sdui.page.rollback":        false,
		"sdui.page.set_current":     false,
		"sdui.page.share_card":      false,
		"sdui.operation.execute":    false,
		"sdui.payment.sandbox":      false,
		"sdui.production.readiness": false,
		"sdui.admin.execute":        false,
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
	for _, tool := range tools {
		if strings.TrimSpace(tool.Description) == "" || tool.InputSchema["type"] != "object" {
			t.Fatalf("工具 %s 缺少描述或 object inputSchema", tool.Name)
		}
		if tool.RequiredScope == "" {
			t.Fatalf("工具 %s 缺少 requiredScope 权限契约", tool.Name)
		}
		if tool.Annotations == nil {
			t.Fatalf("工具 %s 缺少 MCP annotations", tool.Name)
		}
		if tool.Annotations["requiredScope"] != tool.RequiredScope {
			t.Fatalf("工具 %s 的 annotations.requiredScope 与工具契约不一致", tool.Name)
		}
	}
}

// TestMCPAdminExecuteBoundary 验证内部管理工具要求二次确认且永久拒绝管理员/令牌管理。
func TestMCPAdminExecuteBoundary(t *testing.T) {
	service := NewMCPService()
	if _, err := service.ExecuteToolWithContext("internal", "wx-test", []string{"release"}, "sdui.admin.execute", map[string]interface{}{"app_id": "wx-test", "operation": "product.save", "payload": map[string]interface{}{}, "confirmed": false}); err == nil || !strings.Contains(err.Error(), "confirmed") {
		t.Fatal("管理写操作缺少二次确认时未拒绝")
	}
	if _, err := ExecuteAdminMCPOperation("internal", "wx-test", "mcp_token.delete", nil); err == nil || !strings.Contains(err.Error(), "Token") {
		t.Fatal("MCP Token 管理未永久拒绝")
	}
	if _, err := ExecuteAdminMCPOperation("internal", "wx-test", "admin.create", nil); err == nil || !strings.Contains(err.Error(), "管理员") {
		t.Fatal("管理员用户管理未永久拒绝")
	}
}

// TestMCPSensitiveAuditRedaction 验证管理配置嵌套载荷不会写入明文审计日志对象。
func TestMCPSensitiveAuditRedaction(t *testing.T) {
	redacted := sanitizeMCPArgs(map[string]interface{}{"payload": map[string]interface{}{"app_secret": "secret", "payment_private_key": "pem"}})
	payload := redacted["payload"].(map[string]interface{})
	if payload["app_secret"] != "******" || payload["payment_private_key"] != "******" {
		t.Fatal("嵌套敏感字段未脱敏")
	}
}

// TestMCPWebViewToolContracts 验证 WebView 工具只读、按 AppID 隔离并要求 url_key。
func TestMCPWebViewToolContracts(t *testing.T) {
	tools := NewMCPService().GetToolDefinitions()
	for _, name := range []string{"sdui.webview.list", "sdui.webview.validate"} {
		var found *MCPToolDefinition
		for index := range tools {
			if tools[index].Name == name {
				found = &tools[index]
				break
			}
		}
		if found == nil {
			t.Fatalf("缺少 WebView 工具: %s", name)
		}
		if found.RequiredScope != "read" || found.Annotations["readOnlyHint"] != true || found.Annotations["destructiveHint"] != false {
			t.Fatalf("WebView 工具只读契约错误: %+v", found)
		}
	}
	if err := validateMCPArguments("sdui.webview.list", map[string]interface{}{}, tools); err == nil {
		t.Fatal("webview.list 缺少 app_id 时应被拒绝")
	}
	if err := validateMCPArguments("sdui.webview.validate", map[string]interface{}{"app_id": "wx-test"}, tools); err == nil {
		t.Fatal("webview.validate 缺少 url_key 时应被拒绝")
	}
	if err := validateMCPArguments("sdui.webview.validate", map[string]interface{}{"app_id": "wx-test", "url_key": "game-home"}, tools); err != nil {
		t.Fatalf("合法 WebView 参数不应被拒绝: %v", err)
	}
}

// TestMCPAcceptanceToolContract 验证验收工具仅生成只读清单，不承担发布或能力启用职责。
func TestMCPAcceptanceToolContract(t *testing.T) {
	service := NewMCPService()
	var found *MCPToolDefinition
	for index := range service.GetToolDefinitions() {
		tool := service.GetToolDefinitions()[index]
		if tool.Name == "sdui.acceptance.run" {
			found = &tool
			break
		}
	}
	if found == nil {
		t.Fatal("缺少 sdui.acceptance.run 工具")
	}
	if found.RequiredScope != "read" || found.Annotations["readOnlyHint"] != true || found.Annotations["destructiveHint"] != false {
		t.Fatalf("验收工具权限或只读标记错误: %+v", found)
	}
	if err := validateMCPArguments(found.Name, map[string]interface{}{}, service.GetToolDefinitions()); err == nil {
		t.Fatal("验收工具缺少 app_id 时应被参数校验拒绝")
	}
	if err := validateMCPArguments(found.Name, map[string]interface{}{"app_id": "wx-test"}, service.GetToolDefinitions()); err != nil {
		t.Fatalf("验收工具合法参数不应被拒绝: %v", err)
	}
}

// TestMCPToolDispatchCoverage 确认 tools/list 暴露的每个工具都存在执行分支。
func TestMCPToolDispatchCoverage(t *testing.T) {
	service := NewMCPService()
	for _, tool := range service.GetToolDefinitions() {
		_, err := service.ExecuteToolWithContext("coverage", "", []string{"read", "write:draft", "release"}, tool.Name, map[string]interface{}{})
		if err != nil && strings.Contains(err.Error(), "未知 MCP 工具") {
			t.Fatalf("工具 %s 已声明但没有执行分支: %v", tool.Name, err)
		}
	}
}

// TestMCPToolScopeContract 验证工具声明的最小权限与真实执行门禁一致。
func TestMCPToolScopeContract(t *testing.T) {
	service := NewMCPService()
	for _, tool := range service.GetToolDefinitions() {
		_, err := service.ExecuteToolWithContext("scope_test", "", nil, tool.Name, map[string]interface{}{})
		if err == nil || !strings.Contains(err.Error(), tool.RequiredScope) {
			t.Fatalf("工具 %s 未按 requiredScope=%s 拒绝调用: %v", tool.Name, tool.RequiredScope, err)
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

// TestMCPProtocolMethods 验证 MCP 基础 ping、通知和非法请求行为符合 JSON-RPC 约定。
func TestMCPProtocolMethods(t *testing.T) {
	service := NewMCPService()
	if response, err := service.HandleJSONRPC([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil || !strings.Contains(string(response), `"result"`) {
		t.Fatalf("ping 响应无效: %s err=%v", response, err)
	}
	if response, err := service.HandleJSONRPC([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil || response != nil {
		t.Fatalf("通知不应返回响应: %s err=%v", response, err)
	}
	if response, err := service.HandleJSONRPC([]byte(`{"jsonrpc":"1.0","id":1,"method":"ping"}`)); err != nil || !strings.Contains(string(response), `-32600`) {
		t.Fatalf("非法 JSON-RPC 版本未被拒绝: %s err=%v", response, err)
	}
	for _, invalid := range []string{`[]`, `null`} {
		response, err := service.HandleJSONRPC([]byte(invalid))
		if err != nil || !strings.Contains(string(response), `"code":-32600`) || !strings.Contains(string(response), `"id":null`) {
			t.Fatalf("非对象 JSON-RPC 请求未返回 Invalid Request/id=null: %s", response)
		}
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
	if err != nil || !strings.Contains(string(response), "resources/read") || !strings.Contains(string(response), "sdui://rules") || !strings.Contains(string(response), "expected_revision") {
		t.Fatalf("initialize 未提供规则读取引导: %s", response)
	}
}

// TestMCPCoverageResource 验证资源说明明确区分已覆盖和禁止开放的系统行为。
func TestMCPCoverageResource(t *testing.T) {
	api := mcpAPIResource(NewMCPService().GetToolDefinitions())
	transport, ok := api["transports"].(map[string]interface{})
	if !ok || transport["http"] == nil || transport["stdio"] == nil {
		t.Fatalf("MCP API 资源未同时声明 HTTP 与 Stdio 传输契约")
	}
	jsonrpc, ok := api["jsonrpc"].(map[string]interface{})
	if !ok || jsonrpc["version"] != "2.0" || jsonrpc["success_response"] == nil || jsonrpc["error_response"] == nil {
		t.Fatalf("MCP API 资源缺少标准 JSON-RPC 响应契约")
	}
	toolResponse, ok := api["tool_response"].(map[string]interface{})
	if !ok || toolResponse["success"] == nil || toolResponse["error"] == nil {
		t.Fatalf("MCP API 资源缺少 tools/call 响应信封契约")
	}
	coverage, ok := api["coverage"].(map[string]string)
	if !ok || coverage["draft_creation"] != "sdui.page.create" || coverage["image_upload"] != "sdui.file.prepare_upload" || coverage["webview_registry"] != "sdui.webview.list + sdui.webview.validate" {
		t.Fatalf("MCP 覆盖矩阵缺少核心页面和图片行为")
	}
	unsupported, ok := api["unsupported_or_admin_only"].([]string)
	if !ok || len(unsupported) == 0 {
		t.Fatalf("MCP 未声明系统级未覆盖边界")
	}
	if _, ok := api["ai_breakthrough_http"].(map[string]interface{}); !ok {
		t.Fatal("MCP 未声明 AI 破甲业务 HTTP 数据流")
	}
	rules, ok := mcpRulesResource()["block_contracts"].(map[string]interface{})
	if !ok || len(rules) != len(allowedBlockTypes) {
		t.Fatalf("MCP 规则未覆盖全部积木契约: got=%d want=%d", len(rules), len(allowedBlockTypes))
	}
	actions, ok := mcpRulesResource()["action_contracts"].(map[string]interface{})
	if !ok || len(actions) != len(allowedActionTypes) {
		t.Fatalf("MCP 规则未覆盖全部动作契约: got=%d want=%d", len(actions), len(allowedActionTypes))
	}
	for blockType, raw := range rules {
		contract, ok := raw.(map[string]interface{})
		if !ok || strings.TrimSpace(contract["purpose"].(string)) == "" || len(contract["props"].([]string)) == 0 || contract["example"] == nil {
			t.Fatalf("积木 %s 的契约缺少 purpose/props/example", blockType)
		}
	}
	for actionType, raw := range actions {
		contract, ok := raw.(map[string]interface{})
		if !ok || strings.TrimSpace(contract["purpose"].(string)) == "" || contract["payload"] == nil {
			t.Fatalf("动作 %s 的契约缺少 purpose/payload", actionType)
		}
	}
}

// TestMCPStructuredError 验证工具失败返回机器可读恢复建议。
func TestMCPStructuredError(t *testing.T) {
	response, err := NewMCPService().HandleJSONRPC([]byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"sdui.page.patch","arguments":{"app_id":"wx_test","page_id":"home","ops":[]}}}`))
	if err != nil || !strings.Contains(string(response), "structuredContent") || !strings.Contains(string(response), "INVALID_ARGUMENT") {
		t.Fatalf("工具错误未返回结构化修复信息: %s", response)
	}
}

// TestMCPArgumentSchema 验证工具调用会执行 tools/list 暴露的必填、类型和枚举约束。
func TestMCPArgumentSchema(t *testing.T) {
	tools := NewMCPService().GetToolDefinitions()
	if err := validateMCPArguments("sdui.page.patch", map[string]interface{}{"app_id": "wx", "page_id": "home", "ops": []interface{}{}}, tools); err == nil {
		t.Fatal("缺少 expected_revision 应被拒绝")
	}
	if err := validateMCPArguments("sdui.file.prepare_upload", map[string]interface{}{"app_id": "wx", "file_name": "a.png", "file_size": "1024", "content_type": "image/png"}, tools); err == nil {
		t.Fatal("file_size 字符串应被拒绝")
	}
	if err := validateMCPArguments("sdui.file.prepare_upload", map[string]interface{}{"app_id": "wx", "file_name": "a.png", "file_size": float64(10485761), "content_type": "image/png"}, tools); err == nil {
		t.Fatal("超过 maximum 的 file_size 应被拒绝")
	}
	if err := validateMCPArguments("sdui.page.patch", map[string]interface{}{"app_id": "wx", "page_id": "home", "expected_revision": float64(1), "ops": []interface{}{}}, tools); err == nil {
		t.Fatal("少于 minItems 的 ops 应被拒绝")
	}
	if err := validateMCPArguments("sdui.page.patch", map[string]interface{}{"app_id": "wx", "page_id": "home", "expected_revision": float64(1), "ops": []interface{}{map[string]interface{}{"op": "replace", "value": "标题"}}}, tools); err == nil {
		t.Fatal("replace 缺少 path 应被拒绝")
	}
	if err := validateMCPArguments("sdui.page.patch", map[string]interface{}{"app_id": "wx", "page_id": "home", "expected_revision": float64(1), "ops": []interface{}{map[string]interface{}{"op": "unknown", "value": "标题"}}}, tools); err == nil {
		t.Fatal("未知 patch op 应被拒绝")
	}
	if err := validateMCPArguments("sdui.template.list", map[string]interface{}{"business_type": "unknown"}, tools); err == nil {
		t.Fatal("business_type 未知枚举应被拒绝")
	}
}

// TestMCPProtocolShape 验证 page 协议对象的数组字段可供 AI 直接修改。
func TestMCPProtocolShape(t *testing.T) {
	page := &models.DynamicPage{AppID: "wx_test", PageID: "home", Blocks: `[{"id":"a","type":"text","props":{"text":"A"}}]`, ShareConfig: `{"friend":{"enabled":true}}`}
	value := mcpPageProtocol(page)
	if _, ok := value["blocks"].([]interface{}); !ok {
		t.Fatalf("blocks 应返回数组而不是 JSON 字符串: %#v", value["blocks"])
	}
	if _, ok := value["share_config"].(map[string]interface{}); !ok {
		t.Fatalf("share_config 应返回对象而不是 JSON 字符串: %#v", value["share_config"])
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

// TestMCPScreenshotContract 验证截图结果可由远程 MCP 调用方直接访问并绑定 revision。
func TestMCPScreenshotContract(t *testing.T) {
	tool := NewMCPService().GetToolDefinitions()
	for _, definition := range tool {
		if definition.Name == "sdui.page.screenshot" {
			properties := definition.InputSchema["properties"].(map[string]interface{})
			if _, ok := properties["host"]; !ok {
				t.Fatal("截图工具缺少 host 参数")
			}
			return
		}
	}
	t.Fatal("缺少截图工具")
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
