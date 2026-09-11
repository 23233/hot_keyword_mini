// Package services webview_registry_test.go
package services

import (
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"testing"
	"time"
)

// TestValidateWebViewEntry 验证 WebView 入口只允许无查询参数的 HTTPS 地址。
func TestValidateWebViewEntry(t *testing.T) {
	valid := WebViewEntry{URLKey: "game-home", URL: "https://wx.a0free.com/h5/game", Purpose: "游戏 H5", Version: "2026.09", Enabled: true}
	if err := ValidateWebViewEntry(valid); err != nil {
		t.Fatalf("合法 WebView 入口不应被拒绝: %v", err)
	}
	for _, entry := range []WebViewEntry{
		{URLKey: "game-home", URL: "http://wx.a0free.com/h5/game", Purpose: "游戏 H5", Version: "2026.09"},
		{URLKey: "game-home", URL: "https://wx.a0free.com/h5/game?from=share", Purpose: "游戏 H5", Version: "2026.09"},
		{URLKey: "Game-Home", URL: "https://wx.a0free.com/h5/game", Purpose: "游戏 H5", Version: "2026.09"},
		{URLKey: "game-home", URL: "https://wx.a0free.com/h5/game", Purpose: "", Version: "2026.09"},
	} {
		if err := ValidateWebViewEntry(entry); err == nil {
			t.Fatalf("非法 WebView 入口应被拒绝: %+v", entry)
		}
	}
}

// TestWebViewRegistryPureRules 验证重复键、下线入口、不存在入口和租户切片隔离。
func TestWebViewRegistryPureRules(t *testing.T) {
	entries := []WebViewEntry{{URLKey: "game-home", URL: "https://wx.a0free.com", Purpose: "游戏主页", Version: "2026.09", Enabled: true}}
	if _, err := findWebViewEntry(entries, "missing"); err == nil || !strings.Contains(err.Error(), "尚未登记") {
		t.Fatalf("不存在入口应返回明确错误: %v", err)
	}
	disabled := []WebViewEntry{{URLKey: "game-home", URL: "https://wx.a0free.com", Purpose: "游戏主页", Version: "2026.09", Enabled: false}}
	if _, err := findWebViewEntry(disabled, "game-home"); err == nil || !strings.Contains(err.Error(), "下线") {
		t.Fatalf("下线入口应被拒绝: %v", err)
	}
	if err := validateWebViewEntries([]WebViewEntry{
		entries[0],
		{URLKey: " game-home ", URL: "https://wx.a0free.com/other", Purpose: "重复入口", Version: "2026.09", Enabled: true},
	}); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("规范化后的重复 url_key 应被拒绝: %v", err)
	}
	otherTenant := []WebViewEntry{{URLKey: "game-home", URL: "https://other.example.com", Purpose: "另一租户主页", Version: "2026.09", Enabled: true}}
	entry, err := findWebViewEntry(otherTenant, "game-home")
	if err != nil || entry.URL != "https://other.example.com" {
		t.Fatalf("不同租户的登记切片不应串用: entry=%+v err=%v", entry, err)
	}
}

// TestWebViewRegistryDatabaseGuards 验证数据库未初始化时的边界错误，以及重复校验优先于数据库访问。
func TestWebViewRegistryDatabaseGuards(t *testing.T) {
	if _, err := ListWebViews(""); err == nil || !strings.Contains(err.Error(), "AppID") {
		t.Fatalf("空 AppID 应被拒绝: %v", err)
	}
	if err := SaveWebViews("wx-test", []WebViewEntry{
		{URLKey: "same-key", URL: "https://wx.a0free.com", Purpose: "入口", Version: "2026.09"},
		{URLKey: " same-key ", URL: "https://wx.a0free.com/other", Purpose: "重复入口", Version: "2026.09"},
	}); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重复 url_key 应在数据库访问前被拒绝: %v", err)
	}
	if db.Mysql == nil {
		if err := SaveWebViews("wx-test", nil); err == nil || !strings.Contains(err.Error(), "数据库未初始化") {
			t.Fatalf("数据库未初始化应返回明确错误: %v", err)
		}
	}
}

// TestWebViewRegistryTenantIsolation 在本地存在数据库时验证两个 AppID 的登记表互不读取。
func TestWebViewRegistryTenantIsolation(t *testing.T) {
	if db.Mysql == nil || !db.Mysql.Migrator().HasTable(&models.MiniApp{}) {
		t.Skip("未配置可用数据库，跳过 WebView 多租户集成测试")
	}
	suffix := time.Now().UnixNano()
	appA := fmt.Sprintf("wx-webview-a-%d", suffix)
	appB := fmt.Sprintf("wx-webview-b-%d", suffix)
	for _, appID := range []string{appA, appB} {
		if err := db.Mysql.Create(&models.MiniApp{AppID: appID, AppName: appID, CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
			t.Fatalf("创建测试租户失败: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Mysql.Where("app_id IN ?", []string{appA, appB}).Delete(&models.MiniApp{})
	})
	if err := SaveWebViews(appA, []WebViewEntry{{URLKey: "home", URL: "https://a.example.com", Purpose: "A", Version: "1", Enabled: true}}); err != nil {
		t.Fatalf("保存租户 A WebView 失败: %v", err)
	}
	if err := SaveWebViews(appB, []WebViewEntry{{URLKey: "home", URL: "https://b.example.com", Purpose: "B", Version: "1", Enabled: true}}); err != nil {
		t.Fatalf("保存租户 B WebView 失败: %v", err)
	}
	entry, err := ValidateWebViewKey(appA, "home")
	if err != nil || entry.URL != "https://a.example.com" {
		t.Fatalf("租户 A 读取到错误入口: %+v err=%v", entry, err)
	}
	if _, err := ValidateWebViewKey(appA, "only-b"); err == nil {
		t.Fatal("租户 A 不应读取租户 B 的入口")
	}
	if _, err := ValidateWebViewKey(appB, "only-a"); err == nil {
		t.Fatal("租户 B 不应读取租户 A 的入口")
	}
}
