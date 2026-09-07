// Package services template_test.go
package services

import (
	"encoding/json"
	"hot_keyword/db"
	"hot_keyword/models"
	"testing"
	"time"
)

// TestTemplateRegistryList 测试行业模板注册与查询
func TestTemplateRegistryList(t *testing.T) {
	r := GetGlobalTemplateRegistry()
	list := r.ListTemplates("")
	if len(list) < 4 {
		t.Fatalf("期望预设模板数量不少于4个，实际为: %d", len(list))
	}

	dramaList := r.ListTemplates("drama")
	if len(dramaList) == 0 || dramaList[0].BusinessType != "drama" {
		t.Fatalf("drama 类型模板过滤异常")
	}

	gameList := r.ListTemplates("game")
	if len(gameList) == 0 || gameList[0].BusinessType != "game" {
		t.Fatalf("game 类型模板过滤异常")
	}
}

// TestApplyTemplateToPage 测试从模板派生标准 DynamicPage
func TestApplyTemplateToPage(t *testing.T) {
	r := GetGlobalTemplateRegistry()
	appID := "wx516563cfe994bbc6"
	pageID := "game_test_page"

	page, err := r.ApplyTemplateToPage("tpl_game_redeem", appID, pageID, "绝地天王礼包专区")
	if err != nil {
		t.Fatalf("派生页面失败: %v", err)
	}

	if page.AppID != appID || page.PageID != pageID {
		t.Fatalf("派生页面的主键标识不匹配")
	}
	if page.BusinessType != "game" || page.Intent != "redeem" {
		t.Fatalf("派生页面的业务类型或意图不匹配")
	}

	var blocks []models.BlockItem
	if err := json.Unmarshal([]byte(page.Blocks), &blocks); err != nil {
		t.Fatalf("反序列化派生积木失败: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatalf("派生页面必须包含预设积木")
	}
}

// TestComponentLabTemplateCoverage 验证全组件模板可通过协议校验、生成 IR，并覆盖全部标准积木类型。
func TestComponentLabTemplateCoverage(t *testing.T) {
	service := NewTemplateService()
	page, err := service.ApplyTemplateToPage("tpl_sdui_component_lab", "wx_component_lab", "component_lab", "组件实验室")
	if err != nil {
		t.Fatalf("派生组件实验室页面失败: %v", err)
	}
	if report := ValidateDynamicPage(page); !report.IsValid {
		t.Fatalf("组件实验室页面协议校验失败: %v", report.Errors)
	}
	if report := ValidatePageAgainstSchema(page); !report.IsValid {
		t.Fatalf("组件实验室页面 Schema 校验失败: %v", report.Errors)
	}

	var blocks []models.BlockItem
	if err := json.Unmarshal([]byte(page.Blocks), &blocks); err != nil {
		t.Fatalf("反序列化组件实验室积木失败: %v", err)
	}
	actualTypes := collectTemplateBlockTypes(blocks)
	for blockType := range allowedBlockTypes {
		if !actualTypes[blockType] {
			t.Fatalf("组件实验室缺少标准积木类型: %s", blockType)
		}
	}

	context := map[string]interface{}{"$state": map[string]interface{}{"show_extended": true}}
	ir, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", context)
	if err != nil {
		t.Fatalf("生成组件实验室 IR 失败: %v", err)
	}
	if ir.TotalHeight <= 0 || len(ir.Nodes) == 0 {
		t.Fatalf("组件实验室 IR 为空: %+v", ir)
	}
	nodeIDs := collectTemplateNodeIDs(ir.Nodes)
	for _, expectedID := range []string{"lab_repeat_0", "lab_repeat_1", "lab_visible_result", "lab_tab_carousel"} {
		if !nodeIDs[expectedID] {
			t.Fatalf("组件实验室 IR 缺少关键节点: %s", expectedID)
		}
	}

	for fixture, expectedID := range map[string]string{
		"loading": "lab_state_loading",
		"empty":   "lab_state_empty",
		"error":   "lab_state_error",
	} {
		stateIR, stateErr := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), fixture, context)
		if stateErr != nil {
			t.Fatalf("生成 %s 态组件实验室 IR 失败: %v", fixture, stateErr)
		}
		if !collectTemplateNodeIDs(stateIR.Nodes)[expectedID] {
			t.Fatalf("%s 态 IR 未替换为 %s", fixture, expectedID)
		}
	}

	hiddenIR, err := BuildPageLayoutIRWithContext(page, DefaultDeviceParams(), "normal", map[string]interface{}{
		"$state": map[string]interface{}{"show_extended": false},
	})
	if err != nil {
		t.Fatalf("生成条件为假的组件实验室 IR 失败: %v", err)
	}
	if visible, found := collectTemplateNodeVisibility(hiddenIR.Nodes)["lab_visible_result"]; !found || visible {
		t.Fatalf("条件为假时 lab_visible_result 应存在于 IR 且不可见: found=%t visible=%t", found, visible)
	}

	actionTypes := collectTemplateActionTypes(blocks)
	for actionType := range allowedActionTypes {
		if !actionTypes[actionType] {
			t.Fatalf("组件实验室缺少标准动作类型: %s", actionType)
		}
	}
	var navigatePageID string
	var visitNavigate func([]models.BlockItem)
	visitNavigate = func(items []models.BlockItem) {
		for _, item := range items {
			for _, actions := range item.Events {
				for _, action := range actions {
					if action.Type == "navigate_page" {
						navigatePageID, _ = action.Payload["page_id"].(string)
					}
				}
			}
			for _, child := range collectNestedBlocks(item.Props) {
				visitNavigate([]models.BlockItem{child})
			}
		}
	}
	visitNavigate(blocks)
	if navigatePageID != "component_lab" {
		t.Fatalf("组件实验室 navigate_page 必须使用 page_id=component_lab，实际: %q", navigatePageID)
	}
}

// TestCustomTemplateCRUD 验证用户模板在可用数据库环境中的创建、读取、更新、删除闭环。
func TestCustomTemplateCRUD(t *testing.T) {
	if db.Mysql == nil {
		t.Skip("数据库未初始化，跳过用户模板持久化 CRUD 测试")
	}
	service := NewTemplateService()
	appID := "wx_template_test"
	templateID := "tpl_crud_" + time.Now().Format("20060102150405.000000000")
	template := &SDUITemplate{
		AppID:              appID,
		TemplateID:         templateID,
		Name:               "模板 CRUD 验证",
		BusinessType:       "custom",
		Intent:             "watch",
		Description:        "测试后自动清理",
		DefaultTheme:       "dark_glass",
		DefaultAccentColor: "#0A84FF",
		DefaultBlocks: []models.BlockItem{
			{ID: "template_crud_text", Type: "text", Props: map[string]interface{}{"content": "可复用模板"}},
		},
	}
	created, err := service.SaveCustomTemplate(template, "test", 0)
	if err != nil {
		t.Fatalf("创建用户模板失败: %v", err)
	}
	t.Cleanup(func() {
		_ = service.DeleteCustomTemplate(appID, templateID, created.Revision+1)
		_ = db.Mysql.Where("app_id = ? AND template_id = ?", appID, templateID).Delete(&models.DynamicPageTemplate{}).Error
	})
	if created.Builtin || created.Revision != 1 {
		t.Fatalf("创建后模板元数据异常: %+v", created)
	}

	loaded, err := service.GetTemplate(appID, templateID)
	if err != nil || loaded.Name != template.Name {
		t.Fatalf("读取用户模板失败: template=%+v err=%v", loaded, err)
	}
	loaded.Name = "模板 CRUD 已更新"
	updated, err := service.SaveCustomTemplate(loaded, "test", loaded.Revision)
	if err != nil {
		t.Fatalf("更新用户模板失败: %v", err)
	}
	if updated.Revision != 2 || updated.Name != "模板 CRUD 已更新" {
		t.Fatalf("更新结果异常: %+v", updated)
	}
	if err := service.DeleteCustomTemplate(appID, templateID, updated.Revision); err != nil {
		t.Fatalf("删除用户模板失败: %v", err)
	}
	if _, err := service.GetTemplate(appID, templateID); err == nil {
		t.Fatalf("模板删除后仍可读取")
	}
}

// collectTemplateBlockTypes 递归收集模板中的所有积木类型。
func collectTemplateBlockTypes(blocks []models.BlockItem) map[string]bool {
	result := make(map[string]bool)
	var visit func([]models.BlockItem)
	visit = func(items []models.BlockItem) {
		for _, item := range items {
			result[item.Type] = true
			for _, child := range collectNestedBlocks(item.Props) {
				visit([]models.BlockItem{child})
			}
		}
	}
	visit(blocks)
	return result
}

// collectTemplateNodeIDs 递归收集 IR 节点标识，供后端生成与前端消费基线对比。
func collectTemplateNodeIDs(nodes []BlockLayoutNode) map[string]bool {
	result := make(map[string]bool)
	var visit func([]BlockLayoutNode)
	visit = func(items []BlockLayoutNode) {
		for _, item := range items {
			result[item.ID] = true
			visit(item.Children)
		}
	}
	visit(nodes)
	return result
}

// collectTemplateNodeVisibility 递归收集 IR 节点可见性，验证条件渲染不会被后端提前丢弃。
func collectTemplateNodeVisibility(nodes []BlockLayoutNode) map[string]bool {
	result := make(map[string]bool)
	var visit func([]BlockLayoutNode)
	visit = func(items []BlockLayoutNode) {
		for _, item := range items {
			result[item.ID] = item.Visible
			visit(item.Children)
		}
	}
	visit(nodes)
	return result
}

// collectTemplateActionTypes 递归收集单动作、事件流与链式动作，确保全量协议均有复杂页面覆盖。
func collectTemplateActionTypes(blocks []models.BlockItem) map[string]bool {
	result := make(map[string]bool)
	var collectAction func(*models.BlockAction)
	collectAction = func(action *models.BlockAction) {
		if action == nil {
			return
		}
		result[action.Type] = true
		for i := range action.OnSuccess {
			collectAction(&action.OnSuccess[i])
		}
		for i := range action.OnError {
			collectAction(&action.OnError[i])
		}
	}
	var visit func([]models.BlockItem)
	visit = func(items []models.BlockItem) {
		for _, item := range items {
			collectAction(item.Action)
			for _, actions := range item.Events {
				for i := range actions {
					collectAction(&actions[i])
				}
			}
			for _, child := range collectNestedBlocks(item.Props) {
				visit([]models.BlockItem{child})
			}
			for _, variant := range []*models.BlockItem{item.Loading, item.Empty, item.Error, item.Fallback} {
				if variant != nil {
					visit([]models.BlockItem{*variant})
				}
			}
		}
	}
	visit(blocks)
	return result
}
