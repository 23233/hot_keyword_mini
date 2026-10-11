// Package services acceptance_database_test.go
package services

import (
	"encoding/json"
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/models"
	"testing"
)

// TestLocalDatabaseAcceptance 使用独立临时数据库执行被普通单元测试跳过的持久化验收。
// 通过 ACCEPTANCE_DB=mysql|pg 选择目标数据库，连接参数读取 ../config.yaml 并支持标准 DB_* 环境变量覆盖。
func TestLocalDatabaseAcceptance(t *testing.T) {
	dialect, err := db.AcceptanceDialect()
	if err != nil {
		t.Fatal(err)
	}
	if dialect == "" {
		t.Skip("设置 ACCEPTANCE_DB=mysql 或 ACCEPTANCE_DB=pg 执行独立数据库验收")
	}
	cfg, err := db.AcceptanceTestConfig("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	testDB, cleanup, err := db.OpenAcceptanceTestDB(cfg, dialect)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	sqlDB, _ := testDB.DB()
	defer sqlDB.Close()
	previous := db.DB
	db.DB = testDB
	defer func() { db.DB = previous }()
	if err := testDB.AutoMigrate(&models.MiniApp{}, &models.DynamicPage{}, &models.DynamicPageDraft{}, &models.DynamicPageRevision{}, &models.DynamicPageTemplate{}, &models.UserSession{}, &models.CapabilityChangeLog{}); err != nil {
		t.Fatal(err)
	}
	t.Run("模板持久化", TestCustomTemplateCRUD)
	t.Run("草稿版本冲突", TestSaveDraft_CASConflict)
	t.Run("会话重放", TestReplayAttackDetection)
	t.Run("WebView租户隔离", TestWebViewRegistryTenantIsolation)
	t.Run("支付回调与退款权益", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.Product{}, &models.PaymentOrder{}, &models.Article{}, &models.ArticlePurchase{}, &models.MembershipLevel{}, &models.UserMembership{}, &models.MembershipGrant{}); err != nil {
			t.Fatal(err)
		}
		for _, appID := range []string{"wx7a779add6a689881", "wx8b8e899d4829481a"} {
			t.Run(appID, func(t *testing.T) {
				product := models.Product{AppID: appID, SKU: "acceptance-member", Name: "验收会员", PriceFen: 100, Status: models.ProductStatusActive}
				if err := testDB.Create(&product).Error; err != nil {
					t.Fatal(err)
				}
				plan := models.MembershipLevel{AppID: appID, SKU: product.SKU, Level: 1, DurationDays: 30, Name: "验收", Status: "active"}
				if err := testDB.Create(&plan).Error; err != nil {
					t.Fatal(err)
				}
				s := NewPaymentService()
				order, err := s.CreateSandboxOrder(appID, 101, "acceptance-openid", product.SKU, "same-request")
				if err != nil {
					t.Fatal(err)
				}
				again, err := s.CreateSandboxOrder(appID, 101, "acceptance-openid", product.SKU, "same-request")
				if err != nil || again.ID != order.ID || order.AmountFen != 100 {
					t.Fatal("订单幂等或价格快照错误")
				}
				if _, err := s.ApplySandboxNotify("wrong-tenant", order.OutTradeNo, true); err == nil {
					t.Fatal("跨租户支付回调未被拒绝")
				}
				if _, err := s.ApplySandboxNotify(appID, order.OutTradeNo, true); err != nil {
					t.Fatal(err)
				}
				var first models.UserMembership
				if err := testDB.Where("app_id = ? AND user_id = ?", appID, 101).First(&first).Error; err != nil {
					t.Fatal(err)
				}
				if _, err := s.ApplySandboxNotify(appID, order.OutTradeNo, true); err != nil {
					t.Fatal(err)
				}
				var repeated models.UserMembership
				if err := testDB.First(&repeated, first.ID).Error; err != nil {
					t.Fatal(err)
				}
				if !first.ExpiresAt.Equal(repeated.ExpiresAt) {
					t.Fatal("重复支付回调重复发放权益")
				}
				if _, err := s.ApplySandboxTransition(appID, 101, order.OutTradeNo, "refund"); err != nil {
					t.Fatal(err)
				}
				var refunded models.UserMembership
				if err := testDB.First(&refunded, first.ID).Error; err != nil {
					t.Fatal(err)
				}
				if refunded.Level != 0 {
					t.Fatal("退款后会员权益未回收")
				}
			})
		}
	})
	t.Run("更新密钥保留首页", func(t *testing.T) {
		s := NewSDUIService()
		const appID = "wx-acceptance-config"
		if err := s.SaveApp(&models.MiniApp{AppID: appID, AppName: "验收", CurrentPage: "game_home", FallbackPageID: "safe", ReleaseMode: "published"}); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveApp(&models.MiniApp{AppID: appID, AppName: "验收", AppSecret: "local-test-secret"}); err != nil {
			t.Fatal(err)
		}
		var app models.MiniApp
		if err := testDB.Where("app_id = ?", appID).First(&app).Error; err != nil {
			t.Fatal(err)
		}
		if app.CurrentPage != "game_home" || app.FallbackPageID != "safe" || app.ReleaseMode != "published" {
			t.Fatal("更新密钥破坏页面配置")
		}
	})
	t.Run("双租户MCP草稿发布闭环", func(t *testing.T) {
		for _, appID := range []string{"wx7a779add6a689881", "wx8b8e899d4829481a"} {
			t.Run(appID, func(t *testing.T) {
				if err := NewSDUIService().SaveApp(&models.MiniApp{AppID: appID, AppName: "MCP验收"}); err != nil {
					t.Fatal(err)
				}
				m := NewMCPService()
				call := func(tool string, extra map[string]interface{}) interface{} {
					t.Helper()
					args := map[string]interface{}{"app_id": appID, "page_id": "acceptance_home"}
					for k, v := range extra {
						args[k] = v
					}
					result, err := m.ExecuteToolWithContext("acceptance", appID, []string{"read", "write:draft", "release"}, tool, args)
					if err != nil {
						t.Fatalf("%s: %v", tool, err)
					}
					return result
				}
				call("sdui.page.create", map[string]interface{}{"title": "验收页面"})
				call("sdui.page.get", nil)
				call("sdui.page.patch", map[string]interface{}{"expected_revision": float64(1), "ops": []interface{}{map[string]interface{}{"op": "replace", "path": "/title", "value": "修改后标题"}}})
				call("sdui.page.validate", nil)
				call("sdui.capability.validate", nil)
				call("sdui.page.preview", nil)
				call("sdui.page.publish", map[string]interface{}{"expected_revision": float64(2), "confirmed": true})
				call("sdui.page.set_current", map[string]interface{}{"confirmed": true})
				page, err := NewSDUIService().GetRawPage(appID, "acceptance_home")
				if err != nil || page.Title != "修改后标题" {
					t.Fatal("发布内容与草稿不一致")
				}
				call("sdui.page.revisions", nil)
				call("sdui.page.rollback", map[string]interface{}{"target_revision": float64(page.Revision), "confirmed": true})
				actual := call("sdui.production.readiness", nil)
				expected, err := CheckProductionReadiness(appID, config.Cfg)
				if err != nil {
					t.Fatal(err)
				}
				a, _ := json.Marshal(actual)
				b, _ := json.Marshal(expected)
				if string(a) != string(b) {
					t.Fatal("MCP和后台共用服务的预检结果不一致")
				}
				if _, err := m.ExecuteToolWithContext("acceptance", "other-tenant", []string{"read"}, "sdui.page.get", map[string]interface{}{"app_id": appID, "page_id": "acceptance_home"}); err == nil {
					t.Fatal("MCP未拒绝跨租户读取")
				}
			})
		}
	})
	t.Run("双租户栏目文章标识", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.ArticleCategory{}, &models.Article{}); err != nil {
			t.Fatal(err)
		}
		for _, appID := range []string{"wx7a779add6a689881", "wx8b8e899d4829481a"} {
			category := models.ArticleCategory{AppID: appID, Slug: "same-slug", Name: "测试栏目"}
			if err := testDB.Create(&category).Error; err != nil {
				t.Fatal(err)
			}
			article := models.Article{AppID: appID, Slug: "same-slug", Title: "测试文章", Markdown: "测试正文"}
			if err := testDB.Create(&article).Error; err != nil {
				t.Fatal(err)
			}
			duplicate := models.Article{AppID: appID, Slug: "same-slug", Title: "重复文章"}
			if err := testDB.Create(&duplicate).Error; !isDuplicateKeyError(err) {
				t.Fatal("同租户重复标识必须拒绝")
			}
		}
	})
}
