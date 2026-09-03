// Package services template_test.go
package services

import (
	"encoding/json"
	"hot_keyword/models"
	"testing"
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
