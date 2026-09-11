// Package services capabilities_test.go
package services

import (
	"hot_keyword/models"
	"strings"
	"testing"
)

// TestCapabilityRegistryCoverage 验证通用能力注册表覆盖两个小程序所需能力包。
func TestCapabilityRegistryCoverage(t *testing.T) {
	required := map[string]bool{
		"payment": false, "media_upload": false, "location": false, "wechat_customer_service": false,
		"internal_chat": false, "realtime_chat": false, "orders": false, "logistics": false,
		"after_sale": false, "service_orders": false, "webview": false, "membership": false,
		"ads": false, "wallet": false,
	}
	for _, definition := range ListCapabilityDefinitions() {
		if _, ok := required[definition.Key]; ok {
			required[definition.Key] = true
		}
		if definition.Version == "" || definition.ImplementationStatus == "" {
			t.Fatalf("能力 %s 缺少版本或实现状态", definition.Key)
		}
	}
	for key, found := range required {
		if !found {
			t.Fatalf("公共能力注册表缺少 %s", key)
		}
	}
}

// TestRequiredCapabilitiesForBlocks 验证嵌套 Block 和动作链提取统一能力键。
func TestRequiredCapabilitiesForBlocks(t *testing.T) {
	blocks := []models.BlockItem{{
		ID: "root", Type: "container", Props: map[string]interface{}{"children": []models.BlockItem{
			{ID: "product", Type: "product_card", Action: &models.BlockAction{Type: "request_payment"}},
			{ID: "web", Type: "action_button", Action: &models.BlockAction{Type: "open_webview"}},
		}},
	}}
	got := strings.Join(RequiredCapabilitiesForBlocks(blocks), ",")
	for _, expected := range []string{"payment", "webview"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("能力提取结果 %q 缺少 %s", got, expected)
		}
	}
}

// TestClientAndTenantCapabilitiesAreSeparated 验证客户端原生能力与租户能力键不会混用。
func TestClientAndTenantCapabilitiesAreSeparated(t *testing.T) {
	blocks := []models.BlockItem{{ID: "pay", Type: "action_button", Action: &models.BlockAction{Type: "request_payment"}}}
	if got := strings.Join(RequiredCapabilitiesForBlocks(blocks), ","); got != "payment" {
		t.Fatalf("租户能力应使用 payment，实际为 %q", got)
	}
	if got := strings.Join(extractRequiredCapabilities(blocks), ","); got != "request_payment" {
		t.Fatalf("客户端能力应使用 request_payment，实际为 %q", got)
	}
}

// TestValidatePageCapabilitiesForMatrix 验证租户能力状态统一阻断发布。
func TestValidatePageCapabilitiesForMatrix(t *testing.T) {
	blocks := []models.BlockItem{{ID: "copy", Type: "action_button", Action: &models.BlockAction{Type: "copy_text"}}}
	disabled := map[string]models.CapabilityMatrixEntry{"clipboard": {State: "disabled"}}
	if report := ValidatePageCapabilitiesForMatrix(blocks, disabled, true); report.IsValid || len(report.Errors) == 0 {
		t.Fatalf("disabled 剪贴板能力必须阻断发布: %+v", report)
	}
	enabled := map[string]models.CapabilityMatrixEntry{"clipboard": {State: "enabled"}}
	if report := ValidatePageCapabilitiesForMatrix(blocks, enabled, true); !report.IsValid {
		t.Fatalf("enabled 剪贴板能力应通过矩阵门禁: %+v", report)
	}
	if report := ValidatePageCapabilitiesForMatrix(blocks, nil, false); !report.IsValid || len(report.Warnings) == 0 {
		t.Fatalf("旧租户兼容模式应通过并给出警告: %+v", report)
	}
}

// TestDisabledCapabilityCannotPublish 验证未启用能力不能通过发布门禁。
func TestDisabledCapabilityCannotPublish(t *testing.T) {
	blocks := []models.BlockItem{{ID: "pay", Type: "action_button", Action: &models.BlockAction{Type: "request_payment"}}}
	matrix := map[string]models.CapabilityMatrixEntry{"payment": {State: "disabled"}}
	if report := ValidatePageCapabilitiesForMatrix(blocks, matrix, true); report.IsValid || !strings.Contains(strings.Join(report.Errors, " "), "disabled") {
		t.Fatalf("未启用支付能力不能发布: %+v", report)
	}
}

// TestBuildCapabilityMatrix 验证本地租户矩阵包含全部公共能力且状态合法。
func TestBuildCapabilityMatrix(t *testing.T) {
	raw, err := BuildCapabilityMatrix(map[string]string{"clipboard": "enabled", "webview": "configured"})
	if err != nil || !strings.Contains(raw, `"clipboard"`) || !strings.Contains(raw, `"wallet"`) {
		t.Fatalf("构造完整能力矩阵失败: %s err=%v", raw, err)
	}
}
