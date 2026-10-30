// Package services acceptance_database_test.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"testing"
	"time"
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
	t.Run("订单确认与会员广告端点", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.User{}, &models.Product{}, &models.PaymentOrder{}, &models.DomainRecord{}, &models.PlatformOperation{}, &models.MembershipLevel{}, &models.UserMembership{}); err != nil {
			t.Fatal(err)
		}
		const appID = "wx7a779add6a689881"
		// 该租户可能已由前面的 MCP 子测试创建，这里复用并补充广告位配置。
		var app models.MiniApp
		if err := testDB.Where("app_id = ?", appID).First(&app).Error; err != nil {
			app = models.MiniApp{AppID: appID, AppName: "领域验收", AdUnitIDs: "adunit-a,adunit-b"}
			if err := testDB.Create(&app).Error; err != nil {
				t.Fatal(err)
			}
		} else if err := testDB.Model(&app).Update("ad_unit_ids", "adunit-a,adunit-b").Error; err != nil {
			t.Fatal(err)
		}

		user := models.User{AppID: appID, WechatOpenID: "domain-acceptance-openid", NickName: "领域验收用户"}
		if err := testDB.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		plan := models.MembershipLevel{AppID: appID, Level: 9, Name: "领域验收会员", SKU: "domain-acceptance-sku", PriceFen: 100, DurationDays: 30, Status: "active"}
		if err := testDB.Create(&plan).Error; err != nil {
			t.Fatal(err)
		}

		// 会员开通端点只读返回套餐与当前状态，不需要幂等键。
		opened, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "membership.open", nil, "", false)
		if err != nil {
			t.Fatal(err)
		}
		openedJSON, _ := json.Marshal(opened)
		if !strings.Contains(string(openedJSON), "domain-acceptance-sku") || !strings.Contains(string(openedJSON), "inactive") {
			t.Fatalf("会员端点未返回可售套餐或状态: %s", openedJSON)
		}

		// 广告位解析必须允许匿名调用，并按 slot_id 命中配置。
		adResult, err := NewActionEndpointService().ExecuteActionEndpoint(appID, "guest_acceptance", "ads.load", map[string]interface{}{"slot_id": "adunit-b"}, "")
		if err != nil {
			t.Fatal(err)
		}
		resolved, ok := adResult.(AdSlotResolution)
		if !ok || !resolved.Enabled || resolved.AdUnitID != "adunit-b" {
			t.Fatalf("广告位解析结果不符合预期: %#v", adResult)
		}

		// 地址簿与发票资料按用户持久化，读取无需幂等键。
		if _, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "address.save", map[string]interface{}{
			"name": "验收用户", "phone": "13800000000", "region": "广东省深圳市", "detail": "南山区", "is_default": true,
		}, "address-save-1", false); err != nil {
			t.Fatalf("地址保存失败: %v", err)
		}
		addresses, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "address.list", nil, "", false)
		if err != nil {
			t.Fatal(err)
		}
		addressJSON, _ := json.Marshal(addresses)
		if !strings.Contains(string(addressJSON), "南山区") {
			t.Fatalf("地址列表未返回保存结果: %s", addressJSON)
		}
		// 公开动作端点必须登记，否则客户端 request_data 无法访问地址能力。
		if _, err := NewActionEndpointService().ExecuteActionEndpoint(appID, user.WechatOpenID, "address.list", map[string]interface{}{}, ""); err != nil {
			t.Fatalf("address.list 未在受控动作端点登记: %v", err)
		}
		if _, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "invoice.save", map[string]interface{}{
			"type": "company", "title": "验收公司", "tax_no": "TAX-ACCEPTANCE", "email": "acceptance@example.com",
		}, "invoice-save-1", false); err != nil {
			t.Fatalf("发票信息保存失败: %v", err)
		}
		invoice, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "invoice.get", nil, "", false)
		if err != nil {
			t.Fatal(err)
		}
		invoiceJSON, _ := json.Marshal(invoice)
		if !strings.Contains(string(invoiceJSON), "TAX-ACCEPTANCE") {
			t.Fatalf("发票资料未返回保存结果: %s", invoiceJSON)
		}

		// 订单确认的付款查重必须使用双方言 SQL；先绑定 trade-1，另一个订单复用必须被拒绝。
		bound := models.DomainRecord{ID: "order-bound", AppID: appID, Kind: "order", UserID: user.ID, Status: "created", Revision: 1, Data: `{"amount_fen":100,"out_trade_no":"trade-1"}`}
		target := models.DomainRecord{ID: "order-target", AppID: appID, Kind: "order", UserID: user.ID, Status: "created", Revision: 1, Data: `{"amount_fen":100}`}
		if err := testDB.Create(&bound).Error; err != nil {
			t.Fatal(err)
		}
		if err := testDB.Create(&target).Error; err != nil {
			t.Fatal(err)
		}
		payment := models.PaymentOrder{AppID: appID, UserID: user.ID, ProductID: plan.ID, OutTradeNo: "trade-1", Attach: "attach-1", OpenID: user.WechatOpenID, Description: "验收", AmountFen: 100, Status: models.PaymentOrderPaid}
		if err := testDB.Create(&payment).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "order.confirm", map[string]interface{}{"id": "order-target", "out_trade_no": "trade-1"}, "confirm-reused", false); err == nil || !strings.Contains(err.Error(), "PAYMENT_REUSED") {
			t.Fatalf("复用付款应被拒绝，实际: %v", err)
		}

		// 第二个支付单可以正常确认，证明确认链路在 MySQL 与 PostgreSQL 下均可执行。
		payment2 := models.PaymentOrder{AppID: appID, UserID: user.ID, ProductID: plan.ID, OutTradeNo: "trade-2", Attach: "attach-2", OpenID: user.WechatOpenID, Description: "验收2", AmountFen: 100, Status: models.PaymentOrderPaid}
		if err := testDB.Create(&payment2).Error; err != nil {
			t.Fatal(err)
		}
		confirmed, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "order.confirm", map[string]interface{}{"id": "order-target", "out_trade_no": "trade-2"}, "confirm-ok", false)
		if err != nil {
			t.Fatal(err)
		}
		confirmedJSON, _ := json.Marshal(confirmed)
		if !strings.Contains(string(confirmedJSON), "confirmed") {
			t.Fatalf("订单未进入 confirmed: %s", confirmedJSON)
		}

		// 评价必须绑定本人已完成订单：提交、防重复、非法评分与公开列表在两种数据库下都可用。
		product := models.Product{AppID: appID, SKU: "review-acceptance", Name: "评价商品", PriceFen: 100, Status: models.ProductStatusActive}
		if err := testDB.Create(&product).Error; err != nil {
			t.Fatal(err)
		}
		reviewOrder := models.DomainRecord{
			ID: "order-review", AppID: appID, Kind: "order", UserID: user.ID, Status: "confirmed", Revision: 1,
			Data: fmt.Sprintf(`{"product_id":%d,"sku":"%s","amount_fen":100}`, product.ID, product.SKU),
		}
		if err := testDB.Create(&reviewOrder).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "review.submit", map[string]interface{}{
			"order_id": "order-review", "rating": 5, "content": "服务很好，验收通过",
		}, "review-submit-1", false); err != nil {
			t.Fatalf("评价提交失败: %v", err)
		}
		if _, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "review.submit", map[string]interface{}{
			"order_id": "order-review", "rating": 4, "content": "重复评价内容",
		}, "review-submit-2", false); err == nil || !strings.Contains(err.Error(), "CONFLICT") {
			t.Fatalf("同一订单重复评价应被拒绝，实际: %v", err)
		}
		if _, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "review.submit", map[string]interface{}{
			"order_id": "order-review", "rating": 6, "content": "非法评分",
		}, "review-submit-3", false); err == nil || !strings.Contains(err.Error(), "INVALID_ARGUMENT") {
			t.Fatalf("非法评分应被拒绝，实际: %v", err)
		}
		reviewList, err := NewActionEndpointService().ExecuteActionEndpoint(appID, "guest_review", "review.list", map[string]interface{}{"product_id": product.ID}, "")
		if err != nil {
			t.Fatalf("评价列表公开端点执行失败: %v", err)
		}
		reviewJSON, _ := json.Marshal(reviewList)
		if !strings.Contains(string(reviewJSON), `"total":1`) || !strings.Contains(string(reviewJSON), "领域验收用户") || !strings.Contains(string(reviewJSON), "服务很好") {
			t.Fatalf("评价列表未返回提交内容: %s", reviewJSON)
		}

		// MCP 运营工具：先读订单，再执行履约迁移，验证只读与写入门禁在两种数据库下都可用。
		mcp := NewMCPService()
		listed, err := mcp.ExecuteToolWithContext("acceptance", appID, []string{"read"}, "sdui.domain.list", map[string]interface{}{"app_id": appID, "kind": "order"})
		if err != nil {
			t.Fatalf("sdui.domain.list 执行失败: %v", err)
		}
		listedJSON, _ := json.Marshal(listed)
		if !strings.Contains(string(listedJSON), "order-target") {
			t.Fatalf("sdui.domain.list 未返回目标订单: %s", listedJSON)
		}
		fulfilled, err := mcp.ExecuteToolWithContext("acceptance", appID, []string{"release"}, "sdui.domain.execute", map[string]interface{}{
			"app_id": appID, "endpoint": "order.fulfill", "confirmed": true, "idempotency_key": "domain-fulfill-1",
			"payload": map[string]interface{}{"id": "order-target", "status": "received"},
		})
		if err != nil {
			t.Fatalf("sdui.domain.execute 执行失败: %v", err)
		}
		fulfilledJSON, _ := json.Marshal(fulfilled)
		if !strings.Contains(string(fulfilledJSON), "received") {
			t.Fatalf("订单履约未进入 received: %s", fulfilledJSON)
		}
	})
	t.Run("购物车服务端报价", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.Product{}); err != nil {
			t.Fatal(err)
		}
		const appID = "wx7a779add6a689881"
		for _, product := range []models.Product{
			{AppID: appID, SKU: "cart-a", Name: "报价商品 A", PriceFen: 100, Status: models.ProductStatusActive},
			{AppID: appID, SKU: "cart-b", Name: "报价商品 B", PriceFen: 250, Status: models.ProductStatusActive},
		} {
			if err := testDB.Create(&product).Error; err != nil {
				t.Fatal(err)
			}
		}
		service := NewActionEndpointService()
		quoted, err := service.ExecuteActionEndpoint(appID, "guest_cart", "cart.quote", map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"sku": "cart-a", "quantity": 2, "price_fen": 1},
				map[string]interface{}{"sku": "cart-b", "quantity": 1},
			},
		}, "")
		if err != nil {
			t.Fatalf("购物车报价失败: %v", err)
		}
		quotedJSON, _ := json.Marshal(quoted)
		if !strings.Contains(string(quotedJSON), `"total_fen":450`) || !strings.Contains(string(quotedJSON), `"count":3`) {
			t.Fatalf("服务端报价金额错误（客户端金额应被忽略）: %s", quotedJSON)
		}
		for name, payload := range map[string]map[string]interface{}{
			"数量为零":  {"items": []interface{}{map[string]interface{}{"sku": "cart-a", "quantity": 0}}},
			"重复SKU": {"items": []interface{}{map[string]interface{}{"sku": "cart-a", "quantity": 1}, map[string]interface{}{"sku": "cart-a", "quantity": 2}}},
			"未知SKU": {"items": []interface{}{map[string]interface{}{"sku": "cart-missing", "quantity": 1}}},
		} {
			if _, err := service.ExecuteActionEndpoint(appID, "guest_cart", "cart.quote", payload, ""); err == nil {
				t.Fatalf("%s 应被拒绝", name)
			}
		}
	})
	t.Run("购物车沙箱支付与订单落地", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.User{}, &models.Product{}, &models.PaymentOrder{}, &models.DomainRecord{}, &models.MembershipLevel{}, &models.Article{}); err != nil {
			t.Fatal(err)
		}
		const appID = "wx7a779add6a689881"
		var app models.MiniApp
		if err := testDB.Where("app_id = ?", appID).First(&app).Error; err != nil {
			if err := testDB.Create(&models.MiniApp{AppID: appID, AppName: "购物车支付验收"}).Error; err != nil {
				t.Fatal(err)
			}
		}
		user := models.User{AppID: appID, WechatOpenID: "cart-pay-openid", NickName: "购物车用户"}
		if err := testDB.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		for _, product := range []models.Product{
			{AppID: appID, SKU: "pay-a", Name: "支付商品 A", PriceFen: 100, Status: models.ProductStatusActive},
			{AppID: appID, SKU: "pay-b", Name: "支付商品 B", PriceFen: 250, Status: models.ProductStatusActive},
		} {
			if err := testDB.Create(&product).Error; err != nil {
				t.Fatal(err)
			}
		}
		items := []map[string]interface{}{
			{"sku": "pay-a", "quantity": 2},
			{"sku": "pay-b", "quantity": 1},
		}
		service := NewPaymentService()
		order, err := service.CreateSandboxCartOrder(appID, user.ID, "cart-pay-openid", items, "cart-pay-1")
		if err != nil {
			t.Fatalf("创建购物车沙箱订单失败: %v", err)
		}
		if order.Kind != models.PaymentOrderKindCart || order.AmountFen != 450 || order.Items == "" {
			t.Fatalf("购物车支付订单快照错误: %#v", order)
		}
		if _, err := service.ApplySandboxNotify(appID, order.OutTradeNo, true); err != nil {
			t.Fatalf("购物车支付回调失败: %v", err)
		}
		var domainOrder models.DomainRecord
		recordID := "cart_" + order.OutTradeNo
		if err := testDB.Where("id = ? AND app_id = ?", recordID, appID).First(&domainOrder).Error; err != nil {
			t.Fatalf("支付成功后未落地领域订单: %v", err)
		}
		if domainOrder.Status != "confirmed" || domainOrder.UserID != user.ID {
			t.Fatalf("领域订单状态或归属错误: %#v", domainOrder)
		}
		if !strings.Contains(domainOrder.Data, "pay-a") || !strings.Contains(domainOrder.Data, "out_trade_no") {
			t.Fatalf("领域订单缺少商品或支付信息: %s", domainOrder.Data)
		}
		listed, err := ExecuteDomainOperation(appID, user.ID, "cart-pay-openid", "order.list", nil, "", false)
		if err != nil {
			t.Fatalf("用户订单列表读取失败: %v", err)
		}
		listedJSON, _ := json.Marshal(listed)
		if !strings.Contains(string(listedJSON), recordID) || !strings.Contains(string(listedJSON), "confirmed") {
			t.Fatalf("订单列表未返回购物车订单: %s", listedJSON)
		}
		var before int64
		testDB.Model(&models.DomainRecord{}).Where("id = ?", recordID).Count(&before)
		if _, err := service.ApplySandboxNotify(appID, order.OutTradeNo, true); err != nil {
			t.Fatalf("重复支付回调失败: %v", err)
		}
		var after int64
		testDB.Model(&models.DomainRecord{}).Where("id = ?", recordID).Count(&after)
		if before != after || before != 1 {
			t.Fatalf("重复回调重复落地订单: before=%d after=%d", before, after)
		}
		if _, err := service.ApplySandboxTransition(appID, user.ID, order.OutTradeNo, "refund"); err != nil {
			t.Fatalf("购物车订单退款失败: %v", err)
		}
		if err := testDB.Where("id = ?", recordID).First(&domainOrder).Error; err != nil || domainOrder.Status != "refunded" {
			t.Fatalf("退款后领域订单状态错误: %#v err=%v", domainOrder, err)
		}
		refundedRevision := domainOrder.Revision
		if _, err := service.ApplySandboxTransition(appID, user.ID, order.OutTradeNo, "refund"); err != nil {
			t.Fatalf("重复退款回调应保持幂等: %v", err)
		}
		if err := testDB.Where("id = ?", recordID).First(&domainOrder).Error; err != nil || domainOrder.Revision != refundedRevision {
			t.Fatalf("重复退款回调不应递增版本: revision=%d want=%d err=%v", domainOrder.Revision, refundedRevision, err)
		}

		// 会员/文章权益商品不允许混入购物车结算，避免付款后无法按订单发放权益。
		if err := testDB.AutoMigrate(&models.MembershipLevel{}); err != nil {
			t.Fatal(err)
		}
		benefitProduct := models.Product{AppID: appID, SKU: "cart-benefit-sku", Name: "购物车权益商品", PriceFen: 100, Status: models.ProductStatusActive}
		if err := testDB.Create(&benefitProduct).Error; err != nil {
			t.Fatal(err)
		}
		benefitPlan := models.MembershipLevel{AppID: appID, SKU: benefitProduct.SKU, Level: 99, DurationDays: 30, Name: "购物车权益", Status: "active"}
		if err := testDB.Create(&benefitPlan).Error; err != nil {
			t.Fatal(err)
		}
		_, err = service.CreateSandboxCartOrderWithCoupon(appID, user.ID, "cart-pay-openid", []map[string]interface{}{{"sku": benefitProduct.SKU, "quantity": 1}}, "cart-benefit-1", "")
		if err == nil || !strings.Contains(err.Error(), "单独购买") {
			t.Fatalf("权益商品混入购物车应被拒绝，实际: %v", err)
		}
	})
	t.Run("优惠券领取与支付抵扣", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.MiniApp{}, &models.User{}, &models.Product{}, &models.PaymentOrder{}, &models.DomainRecord{}, &models.PlatformOperation{}, &models.MembershipLevel{}, &models.Article{}); err != nil {
			t.Fatal(err)
		}
		const appID = "wx8b8e899d4829481a"
		var app models.MiniApp
		if err := testDB.Where("app_id = ?", appID).First(&app).Error; err != nil {
			if err := testDB.Create(&models.MiniApp{AppID: appID, AppName: "优惠券验收"}).Error; err != nil {
				t.Fatal(err)
			}
		}
		user := models.User{AppID: appID, WechatOpenID: "coupon-openid", NickName: "优惠券用户"}
		if err := testDB.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		for _, product := range []models.Product{
			{AppID: appID, SKU: "coupon-a", Name: "优惠商品 A", PriceFen: 100, Status: models.ProductStatusActive},
			{AppID: appID, SKU: "coupon-b", Name: "优惠商品 B", PriceFen: 250, Status: models.ProductStatusActive},
		} {
			if err := testDB.Create(&product).Error; err != nil {
				t.Fatal(err)
			}
		}

		defined, err := ExecuteDomainOperation(appID, 1, "admin", "coupon.define", map[string]interface{}{
			"title": "满 2 元减 1 元", "amount_fen": 100, "min_spend_fen": 200,
			"total_stock": 10, "valid_until": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		}, "coupon-define-1", true)
		if err != nil {
			t.Fatalf("定义优惠券失败: %v", err)
		}
		definedMap, _ := defined.(map[string]interface{})
		definitionID, _ := definedMap["id"].(string)
		if definitionID == "" {
			t.Fatalf("优惠券定义未返回 ID: %#v", defined)
		}

		claimed, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "coupon.claim", map[string]interface{}{"definition_id": definitionID}, "coupon-claim-1", false)
		if err != nil {
			t.Fatalf("领取优惠券失败: %v", err)
		}
		claimedMap, _ := claimed.(map[string]interface{})
		couponID, _ := claimedMap["id"].(string)
		if couponID == "" {
			t.Fatalf("领取未返回优惠券 ID: %#v", claimed)
		}
		again, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "coupon.claim", map[string]interface{}{"definition_id": definitionID}, "coupon-claim-2", false)
		if err != nil {
			t.Fatalf("重复领取应保持幂等: %v", err)
		}
		againMap, _ := again.(map[string]interface{})
		if againMap["id"] != couponID {
			t.Fatalf("重复领取生成了不同优惠券: %#v", again)
		}
		listed, err := ExecuteDomainOperation(appID, user.ID, user.WechatOpenID, "coupon.list", nil, "", false)
		if err != nil {
			t.Fatalf("优惠券列表读取失败: %v", err)
		}
		listedJSON, _ := json.Marshal(listed)
		if !strings.Contains(string(listedJSON), couponID) || !strings.Contains(string(listedJSON), "available") {
			t.Fatalf("优惠券列表未返回可用券: %s", listedJSON)
		}

		items := []map[string]interface{}{{"sku": "coupon-a", "quantity": 2}, {"sku": "coupon-b", "quantity": 1}}
		service := NewPaymentService()
		order, err := service.CreateSandboxCartOrderWithCoupon(appID, user.ID, user.WechatOpenID, items, "coupon-pay-1", couponID)
		if err != nil {
			t.Fatalf("使用优惠券创建支付订单失败: %v", err)
		}
		if order.AmountFen != 350 || order.DiscountFen != 100 || order.CouponID != couponID {
			t.Fatalf("优惠券抵扣金额错误: %#v", order)
		}
		if _, err := service.ApplySandboxNotify(appID, order.OutTradeNo, true); err != nil {
			t.Fatalf("优惠券订单支付回调失败: %v", err)
		}
		var coupon models.DomainRecord
		if err := testDB.Where("id = ?", couponID).First(&coupon).Error; err != nil || coupon.Status != "used" {
			t.Fatalf("支付成功后优惠券应为 used: %#v err=%v", coupon, err)
		}
		if _, err := service.ApplySandboxTransition(appID, user.ID, order.OutTradeNo, "refund"); err != nil {
			t.Fatalf("优惠券订单退款失败: %v", err)
		}
		if err := testDB.Where("id = ?", couponID).First(&coupon).Error; err != nil || coupon.Status != "available" {
			t.Fatalf("退款后优惠券应恢复 available: %#v err=%v", coupon, err)
		}

		// 未达到门槛时必须拒绝，且不创建支付订单。
		var before int64
		testDB.Model(&models.PaymentOrder{}).Where("app_id = ? AND coupon_id = ?", appID, couponID).Count(&before)
		_, err = service.CreateSandboxCartOrderWithCoupon(appID, user.ID, user.WechatOpenID, []map[string]interface{}{{"sku": "coupon-a", "quantity": 1}}, "coupon-pay-2", couponID)
		if err == nil {
			t.Fatal("未达到门槛的优惠券应被拒绝")
		}

		// 模拟支付流程中断：锁定超过 TTL 后允许重新使用；管理端可批量回收。
		staleOrder, err := service.CreateSandboxCartOrderWithCoupon(appID, user.ID, user.WechatOpenID, items, "coupon-pay-3", couponID)
		if err != nil {
			t.Fatalf("再次锁定优惠券失败: %v", err)
		}
		if staleOrder.AmountFen != 350 {
			t.Fatalf("锁定订单金额错误: %#v", staleOrder)
		}
		expireCouponReservation := func() {
			var record models.DomainRecord
			if err := testDB.Where("id = ?", couponID).First(&record).Error; err != nil {
				t.Fatal(err)
			}
			data := recordData(&record)
			data["reserved_at"] = time.Now().Add(-4 * time.Hour).UTC().Format(time.RFC3339)
			encoded, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := testDB.Model(&models.DomainRecord{}).Where("id = ?", record.ID).Update("data", string(encoded)).Error; err != nil {
				t.Fatal(err)
			}
		}
		expireCouponReservation()
		reclaimed, err := service.CreateSandboxCartOrderWithCoupon(appID, user.ID, user.WechatOpenID, items, "coupon-pay-4", couponID)
		if err != nil {
			t.Fatalf("超过锁定 TTL 后应允许重新使用: %v", err)
		}
		if reclaimed.AmountFen != 350 {
			t.Fatalf("回收后订单金额错误: %#v", reclaimed)
		}
		expireCouponReservation()
		cleaned, err := ExecuteDomainOperation(appID, 1, "admin", "coupon.cleanup", nil, "coupon-cleanup-1", true)
		if err != nil {
			t.Fatalf("回收过期锁定失败: %v", err)
		}
		cleanedJSON, _ := json.Marshal(cleaned)
		if !strings.Contains(string(cleanedJSON), `"released":1`) {
			t.Fatalf("回收数量错误: %s", cleanedJSON)
		}
		if err := testDB.Where("id = ?", couponID).First(&coupon).Error; err != nil || coupon.Status != "available" {
			t.Fatalf("回收后优惠券应为 available: %#v err=%v", coupon, err)
		}

		// 进程内定时任务使用同一回收逻辑，独立于管理员端点。
		if _, err := service.CreateSandboxCartOrderWithCoupon(appID, user.ID, user.WechatOpenID, items, "coupon-pay-5", couponID); err != nil {
			t.Fatalf("再次锁定优惠券失败: %v", err)
		}
		expireCouponReservation()
		released, err := CleanupStaleCouponReservations()
		if err != nil || released < 1 {
			t.Fatalf("定时回收失败: released=%d err=%v", released, err)
		}
		if err := testDB.Where("id = ?", couponID).First(&coupon).Error; err != nil || coupon.Status != "available" {
			t.Fatalf("定时回收后优惠券应为 available: %#v err=%v", coupon, err)
		}
	})
	t.Run("运行时能力门禁", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.MiniApp{}); err != nil {
			t.Fatal(err)
		}
		const appID = "wx-capability-gate"
		testDB.Where("app_id = ?", appID).Delete(&models.MiniApp{})
		matrix, err := BuildCapabilityMatrix(map[string]string{"payment": "disabled", "orders": "enabled", "webview": "enabled"})
		if err != nil {
			t.Fatal(err)
		}
		if err := testDB.Create(&models.MiniApp{AppID: appID, AppName: "能力门禁", CapabilityMatrix: matrix}).Error; err != nil {
			t.Fatal(err)
		}
		if allowed, err := EnsureAppCapability(appID, []string{"payment"}); err != nil || allowed {
			t.Fatalf("disabled 支付能力不应放行: allowed=%v err=%v", allowed, err)
		}
		if allowed, err := EnsureAppCapability(appID, []string{"orders"}); err != nil || !allowed {
			t.Fatalf("enabled 订单能力应放行: allowed=%v err=%v", allowed, err)
		}
		if allowed, err := EnsureAppCapability(appID, []string{"payment", "orders"}); err != nil || !allowed {
			t.Fatalf("任一能力启用即应放行: allowed=%v err=%v", allowed, err)
		}
		// 未配置能力矩阵的旧租户按兼容模式放行。
		const legacyAppID = "wx-capability-legacy"
		testDB.Where("app_id = ?", legacyAppID).Delete(&models.MiniApp{})
		if err := testDB.Create(&models.MiniApp{AppID: legacyAppID, AppName: "兼容租户"}).Error; err != nil {
			t.Fatal(err)
		}
		if allowed, err := EnsureAppCapability(legacyAppID, []string{"payment"}); err != nil || !allowed {
			t.Fatalf("空能力矩阵应按兼容模式放行: allowed=%v err=%v", allowed, err)
		}
	})
	t.Run("页面兜底与维护页", func(t *testing.T) {
		if err := testDB.AutoMigrate(&models.MiniApp{}, &models.DynamicPage{}); err != nil {
			t.Fatal(err)
		}
		const appID = "wx-fallback-test"
		testDB.Where("app_id = ?", appID).Delete(&models.MiniApp{})
		testDB.Where("app_id = ?", appID).Delete(&models.DynamicPage{})
		app := models.MiniApp{AppID: appID, AppName: "兜底验收", CurrentPage: "home", FallbackPageID: "safe", ReleaseMode: "normal"}
		if err := testDB.Create(&app).Error; err != nil {
			t.Fatal(err)
		}
		pages := []models.DynamicPage{
			{AppID: appID, PageID: "home", Title: "首页", BusinessType: "custom", Status: "published", Hidden: false, Blocks: "[]"},
			{AppID: appID, PageID: "safe", Title: "兜底页", BusinessType: "custom", Status: "published", Hidden: false, Blocks: "[]"},
			{AppID: appID, PageID: "hidden", Title: "隐藏页", BusinessType: "custom", Status: "published", Hidden: true, Blocks: "[]"},
		}
		for index := range pages {
			if err := testDB.Create(&pages[index]).Error; err != nil {
				t.Fatal(err)
			}
		}
		service := NewSDUIService()
		envelope, err := service.GetPublishedDynamicPageEnvelope(appID, "hidden", nil, false)
		if err != nil || envelope.Page.PageID != "safe" {
			t.Fatalf("隐藏页应降级到租户兜底页: page=%v err=%v", envelope, err)
		}
		envelope, err = service.GetPublishedDynamicPageEnvelope(appID, "missing", nil, false)
		if err != nil || envelope.Page.PageID != "safe" {
			t.Fatalf("缺失页应降级到租户兜底页: page=%v err=%v", envelope, err)
		}
		if err := testDB.Model(&app).Update("release_mode", "fallback").Error; err != nil {
			t.Fatal(err)
		}
		envelope, err = service.GetPublishedDynamicPageEnvelope(appID, "home", nil, false)
		if err != nil || envelope.Page.PageID != "safe" {
			t.Fatalf("应急模式应强制兜底页: page=%v err=%v", envelope, err)
		}
		if err := testDB.Model(&models.DynamicPage{}).Where("app_id = ? AND page_id IN ?", appID, []string{"home", "safe"}).Update("hidden", true).Error; err != nil {
			t.Fatal(err)
		}
		envelope, err = service.GetPublishedDynamicPageEnvelope(appID, "home", nil, false)
		if err != nil || envelope.Page.PageID != "maintenance" {
			t.Fatalf("无可用页面时应返回维护页: page=%v err=%v", envelope, err)
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
