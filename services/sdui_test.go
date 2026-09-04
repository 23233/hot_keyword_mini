// Package services sdui_test.go
package services

import (
	"encoding/json"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"testing"
	"time"
)

// TestValidation_NestedBlockContracts 验证嵌套 block、事件链和状态分支不会绕过协议校验。
func TestValidation_NestedBlockContracts(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "nested_validation",
		Title:        "嵌套校验",
		BusinessType: "custom",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks: `[{
			"id":"root","type":"container","props":{"children":[
				{"id":"child","type":"text","props":{"text":"ok"},"events":{"tap":[{"type":"bad_action"}]}}
			]},
			"loading":{"id":"child","type":"skeleton","props":{"rows":1}}
		}]`,
	}
	report := ValidateDynamicPage(page)
	if report.IsValid {
		t.Fatalf("嵌套非法动作和重复 ID 应阻断发布: %+v", report)
	}
	joined := strings.Join(report.Errors, "\n")
	if !strings.Contains(joined, "bad_action") || !strings.Contains(joined, "ID 冲突") {
		t.Fatalf("嵌套校验错误信息不完整: %s", joined)
	}
}

// TestValidation_AllStandardBlocksAndActions 验证协议白名单中的全部 block/action 均可校验并进入布局 IR。
func TestValidation_AllStandardBlocksAndActions(t *testing.T) {
	actionPayload := func(actionType string) map[string]interface{} {
		switch actionType {
		case "copy_text":
			return map[string]interface{}{"text": "matrix-test"}
		case "open_channels_activity":
			return map[string]interface{}{"feed_id": "export/test", "finder_user_name": "finder_test"}
		case "open_mini_program":
			return map[string]interface{}{"target_app_id": "wx516563cfe994bbc6"}
		case "subscribe_message":
			return map[string]interface{}{"template_id": "tmpl_test"}
		case "request_data":
			return map[string]interface{}{"endpoint": "game.redeem"}
		default:
			return map[string]interface{}{}
		}
	}
	blocks := make([]models.BlockItem, 0, len(allowedBlockTypes))
	for blockType := range allowedBlockTypes {
		block := models.BlockItem{
			ID:   "matrix_" + blockType,
			Type: blockType,
			Props: map[string]interface{}{
				"title": "matrix " + blockType,
			},
		}
		blocks = append(blocks, block)
	}
	for actionType := range allowedActionTypes {
		blocks = append(blocks, models.BlockItem{
			ID:   "matrix_action_" + actionType,
			Type: "action_button",
			Props: map[string]interface{}{
				"text": actionType,
			},
			Action: &models.BlockAction{Type: actionType, Payload: actionPayload(actionType)},
		})
	}
	raw, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("构造协议矩阵失败: %v", err)
	}
	page := &models.DynamicPage{
		AppID: "wx_test", PageID: "matrix", Title: "协议矩阵", BusinessType: "custom", Intent: "watch", Theme: "dark_glass",
		Blocks: string(raw),
	}
	report := ValidateDynamicPage(page)
	if !report.IsValid {
		t.Fatalf("全部标准 block/action 应通过校验: %+v", report.Errors)
	}
	ir, err := BuildPageLayoutIR(page, DefaultDeviceParams(), "normal")
	if err != nil || len(ir.Nodes) != len(blocks) {
		t.Fatalf("全部标准 block 应进入 Layout IR: err=%v nodes=%d expected=%d", err, len(ir.Nodes), len(blocks))
	}
}

// TestSDUIBlockSerialization 测试 SDUI 积木组件的 JSON 协议编解码
func TestSDUIBlockSerialization(t *testing.T) {
	action := models.BlockAction{
		Type:        "open_channels_activity",
		RequireAuth: false,
		Payload: map[string]interface{}{
			"feed_id":          "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
			"finder_user_name": "gh_drama_official",
		},
	}

	block := models.BlockItem{
		ID:   "block_hero_test",
		Type: "media_hero",
		Props: map[string]interface{}{
			"title":    "猴王下山",
			"subtitle": "全网爆火都市短剧",
		},
		Style: &models.BlockStyle{
			BorderRadius: "28rpx",
			GlassBlur:    true,
			AccentColor:  "#FF9F0A",
		},
		Action: &action,
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("序列化积木组件失败: %v", err)
	}

	var decoded models.BlockItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化积木组件失败: %v", err)
	}

	if decoded.ID != "block_hero_test" || decoded.Type != "media_hero" {
		t.Fatalf("积木基础字段反序列化不一致")
	}
	if decoded.Action == nil || decoded.Action.Type != "open_channels_activity" {
		t.Fatalf("动作协议反序列化异常")
	}
	if decoded.Style == nil || !decoded.Style.GlassBlur {
		t.Fatalf("样式属性反序列化异常")
	}
}

// TestEnvelopeStructure 测试响应信封规范
func TestEnvelopeStructure(t *testing.T) {
	envelope := models.PageResponseEnvelope{
		ProtocolVersion: "1.1",
		SchemaVersion:   3,
		RequestID:       "req_test_123",
		Page: models.DynamicPageDTO{
			PageID:       "home",
			Revision:     1,
			Status:       "published",
			Title:        "猴王下山",
			BusinessType: "drama",
			Intent:       "watch",
			Theme:        "dark_glass",
			Blocks:       []models.BlockItem{},
		},
		Data: map[string]interface{}{
			"keyword": "猴王下山",
		},
		CapabilitiesRequired: []string{"video", "clipboard"},
		Cache: models.EnvelopeCache{
			ETag:   "test_etag",
			MaxAge: 30,
		},
		Fallback: models.EnvelopeFallback{
			PageID: "home",
			Mode:   "static_safe",
		},
	}

	bytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("序列化信封失败: %v", err)
	}

	var decoded models.PageResponseEnvelope
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("反序列化信封失败: %v", err)
	}

	if decoded.ProtocolVersion != "1.1" || decoded.SchemaVersion != 3 {
		t.Fatalf("信封版本号不匹配")
	}
	if decoded.Fallback.PageID != "home" {
		t.Fatalf("降级页面配置错误")
	}
}

// TestAdminSDUIManagement 测试管理端小程序与页面操作逻辑
func TestAdminSDUIManagement(t *testing.T) {
	s := NewSDUIService()

	// 基础参数校验测试
	if err := s.SaveApp(nil); err == nil {
		t.Fatalf("传入空 App 应报错")
	}
	if err := s.SavePage(nil); err == nil {
		t.Fatalf("传入空 Page 应报错")
	}
	if err := s.SetCurrentPage("", ""); err == nil {
		t.Fatalf("空参数设为主页应报错")
	}
}

// TestValidation_ShareAndSubscribeActions 测试 share 与 subscribe_message 动作白名单合规校验
func TestValidation_ShareAndSubscribeActions(t *testing.T) {
	page := &models.DynamicPage{
		AppID:        "wx_test_app",
		PageID:       "test_actions",
		Title:        "测试页面",
		BusinessType: "custom",
		Blocks: `[
			{
				"id": "btn_share",
				"type": "action_button",
				"action": {
					"type": "share",
					"payload": { "title": "推荐一部好剧" }
				}
			},
			{
				"id": "btn_sub",
				"type": "action_button",
				"action": {
					"type": "subscribe_message",
					"payload": { "template_id": "tmpl_123" }
				}
			}
		]`,
	}

	report := ValidateDynamicPage(page)
	if !report.IsValid {
		t.Fatalf("share 与 subscribe_message 应为合法原子动作，校验失败: %v", report.Errors)
	}
}

// TestValidation_RequestDataSecurityURL 测试 request_data 对非法外部 URL 的安全拦截
func TestValidation_RequestDataSecurityURL(t *testing.T) {
	// 1. 包含非法外部第三方绝对 URL 时应被安全拦截
	maliciousPage := &models.DynamicPage{
		AppID:        "wx_test_app",
		PageID:       "test_sec",
		BusinessType: "custom",
		Blocks: `[
			{
				"id": "btn_steal",
				"type": "action_button",
				"action": {
					"type": "request_data",
					"payload": {
						"url": "https://evil-attacker.com/steal-token"
					}
				}
			}
		]`,
	}

	report := ValidateDynamicPage(maliciousPage)
	if report.IsValid {
		t.Fatalf("request_data 指定外部绝对 URL 必须被拦截，但校验未阻断")
	}

	// 2. 使用同源相对路径时应正常通过
	safePage := &models.DynamicPage{
		AppID:        "wx_test_app",
		PageID:       "test_safe",
		Title:        "安全页面",
		BusinessType: "custom",
		Blocks: `[
			{
				"id": "btn_safe",
				"type": "action_button",
				"action": {
					"type": "request_data",
					"payload": {
						"url": "/api/v1/action/execute",
						"endpoint": "game.redeem"
					}
				}
			}
		]`,
	}

	safeReport := ValidateDynamicPage(safePage)
	if !safeReport.IsValid {
		t.Fatalf("合法的相对路径与受控 endpoint 校验应通过: %v", safeReport.Errors)
	}
}

// TestGameRedeemTemplateBinding 测试游戏兑换模板已完整配置 path: $result.code 动作链
func TestGameRedeemTemplateBinding(t *testing.T) {
	registry := GetGlobalTemplateRegistry()
	tpl, err := registry.GetTemplate("tpl_game_redeem")
	if err != nil {
		t.Fatalf("获取 tpl_game_redeem 模板失败: %v", err)
	}

	foundCopyResult := false
	for _, block := range tpl.DefaultBlocks {
		var actions []*models.BlockAction
		if block.Action != nil {
			actions = append(actions, block.Action)
		}
		if block.Events != nil && block.Events["tap"] != nil {
			for i := range block.Events["tap"] {
				actions = append(actions, &block.Events["tap"][i])
			}
		}

		for _, act := range actions {
			if act.Type == "request_data" && act.Payload != nil {
				if onSuccess, ok := act.Payload["on_success"].([]map[string]interface{}); ok {
					for _, sub := range onSuccess {
						payloadMap, _ := sub["payload"].(map[string]interface{})
						if sub["type"] == "copy_text" && (sub["path"] == "$result.code" || payloadMap["path"] == "$result.code") {
							foundCopyResult = true
						}
					}
				} else if onSuccessSlice, ok := act.Payload["on_success"].([]interface{}); ok {
					for _, sub := range onSuccessSlice {
						if subMap, ok := sub.(map[string]interface{}); ok {
							payloadMap, _ := subMap["payload"].(map[string]interface{})
							if subMap["type"] == "copy_text" && (subMap["path"] == "$result.code" || payloadMap["path"] == "$result.code") {
								foundCopyResult = true
							}
						}
					}
				}
			}
		}
	}

	if !foundCopyResult {
		t.Fatalf("游戏兑换模板必须配置 copy_text 绑定 $result.code，防止兑换码丢失")
	}
}

// TestShareCardExpiryAndUnpublishedFallback 测试下架与过期页面的安全兜底渲染
func TestShareCardExpiryAndUnpublishedFallback(t *testing.T) {
	cardService := NewShareCardService()

	// 1. 已下架草稿页面渲染应生成安全兜底图
	unpublishedPage := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "offline_page",
		Status:       "draft",
		Title:        "绝密未发布营销案",
		BusinessType: "custom",
	}
	pngBytes, err := cardService.RenderShareCardFromPage(unpublishedPage, "app_message")
	if err != nil || len(pngBytes) == 0 {
		t.Fatalf("生成图片失败: %v", err)
	}

	// 2. 已过期的线上页面渲染
	expiredTime := time.Now().Add(-1 * time.Hour)
	expiredPage := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "expired_page",
		Status:       "published",
		Title:        "限时免费抽奖",
		ExpiresAt:    &expiredTime,
		BusinessType: "custom",
	}
	expPng, err := cardService.RenderShareCardFromPage(expiredPage, "timeline")
	if err != nil || len(expPng) == 0 {
		t.Fatalf("生成过期兜底图失败: %v", err)
	}
}

// TestDynamicDraftEnvelopeAssembly 测试 AssembleEnvelope 与草稿信封组装
func TestDynamicDraftEnvelopeAssembly(t *testing.T) {
	s := NewSDUIService()
	draftPage := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "draft_preview_1",
		Revision:     2,
		Status:       "draft",
		Title:        "待审核的新版首页",
		BusinessType: "drama",
		Blocks: `[
			{
				"id": "hero_1",
				"type": "media_hero",
				"props": { "title": "新剧预览" }
			}
		]`,
	}

	envelope, err := s.AssembleEnvelope(draftPage, map[string]string{"source": "mcp"}, "draft_preview")
	if err != nil {
		t.Fatalf("组装草稿信封失败: %v", err)
	}

	if envelope.Page.Title != "待审核的新版首页" {
		t.Fatalf("草稿信封标题未正确填充")
	}
	if envelope.Page.Status != "draft" {
		t.Fatalf("草稿信封状态应保留 draft")
	}
	if len(envelope.Page.Blocks) != 1 || envelope.Page.Blocks[0].ID != "hero_1" {
		t.Fatalf("草稿积木树未正确组装")
	}
}

// TestAdminUser_UsernameValidation 测试管理员用户名强正则校验
func TestAdminUser_UsernameValidation(t *testing.T) {
	auth := NewAdminAuthService()

	// 非法用户名：含 XSS 攻击向量
	_, err := auth.CreateAdminUser("<script>alert(1)</script>", "validpass123", "攻击者", "editor")
	if err == nil {
		t.Fatalf("应拒绝包含脚本标签的非法用户名")
	}

	// 非法用户名：含单双引号与特殊字符
	_, err = auth.CreateAdminUser("admin' OR '1'='1", "validpass123", "注入者", "editor")
	if err == nil {
		t.Fatalf("应拒绝包含 SQL/JS 逃逸字符的用户名")
	}

	// 非法用户名：过短
	_, err = auth.CreateAdminUser("ab", "validpass123", "测试", "editor")
	if err == nil {
		t.Fatalf("应拒绝小于3个字符的用户名")
	}

	// 合法用户名
	u, err := auth.CreateAdminUser("valid_admin-01", "validpass123", "合格管理员", "editor")
	if err != nil {
		t.Fatalf("合法用户名应创建成功: %v", err)
	}
	if u.Username != "valid_admin-01" {
		t.Fatalf("用户名返回不一致")
	}
}

// TestScreenshotSignature_Security 测试截图签名安全与防篡改
func TestScreenshotSignature_Security(t *testing.T) {
	appID := "wx_test"
	pageID := "draft_p1"
	hash := "a1b2c3d4"
	expiresAt := time.Now().Add(10 * time.Minute).Unix()

	// 1. 生成正常有效签名
	sig := GenerateScreenshotSignature(appID, pageID, hash, expiresAt)
	if sig == "" {
		t.Fatalf("签名生成失败")
	}

	// 2. 正常验签应通过
	if !ValidateScreenshotSignature(appID, pageID, hash, expiresAt, sig) {
		t.Fatalf("有效签名验签失败")
	}

	// 3. 篡改参数 (如修改 pageID) 应验签失败
	if ValidateScreenshotSignature(appID, "another_page", hash, expiresAt, sig) {
		t.Fatalf("篡改 pageID 的签名不应通过")
	}

	// 4. 篡改 hash 应验签失败
	if ValidateScreenshotSignature(appID, pageID, "fake_hash", expiresAt, sig) {
		t.Fatalf("篡改 hash 的签名不应通过")
	}

	// 5. 过期签名应验签失败
	pastExpiresAt := time.Now().Add(-1 * time.Minute).Unix()
	pastSig := GenerateScreenshotSignature(appID, pageID, hash, pastExpiresAt)
	if ValidateScreenshotSignature(appID, pageID, hash, pastExpiresAt, pastSig) {
		t.Fatalf("过期签名不应通过验签")
	}
}

// TestValidatePageAgainstSchema_Deep 测试深度 Schema 校验拦截
func TestValidatePageAgainstSchema_Deep(t *testing.T) {
	// 1. 缺失 title
	missingTitle := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "p1",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks:       `[]`,
	}
	if report := ValidatePageAgainstSchema(missingTitle); report.IsValid {
		t.Fatalf("缺失必填字段 title 应被 Schema 校验拦截")
	}

	// 2. 非法 BusinessType
	invalidBusType := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "p1",
		Title:        "测试页面",
		BusinessType: "illegal_business_type",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks:       `[]`,
	}
	if report := ValidatePageAgainstSchema(invalidBusType); report.IsValid {
		t.Fatalf("非法 BusinessType 枚举应被拦截")
	}

	// 3. 非法 Action Type
	invalidAction := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "p1",
		Title:        "测试页面",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks: `[
			{
				"id": "btn1",
				"type": "action_button",
				"action": {
					"type": "eval_remote_script"
				}
			}
		]`,
	}
	if report := ValidatePageAgainstSchema(invalidAction); report.IsValid {
		t.Fatalf("非法 Action 类型 eval_remote_script 应被 Schema 拦截")
	}

	// 4. action_button 的 copy_text 缺少 text 或 binding
	missingActionPayload := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "p1",
		Title:        "测试页面",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks: `[
			{
				"id": "btn2",
				"type": "action_button",
				"action": {
					"type": "copy_text",
					"payload": {}
				}
			}
		]`,
	}
	if report := ValidatePageAgainstSchema(missingActionPayload); report.IsValid {
		t.Fatalf("copy_text 缺少 text 参数应被拦截")
	}

	// 5. 非法 visible_when 表达式格式
	invalidVisibleWhen := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "p1",
		Title:        "测试页面",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks: `[
			{
				"id": "btn3",
				"type": "action_button",
				"visible_when": {
					"path": "plain_variable_without_scope"
				}
			}
		]`,
	}
	if report := ValidatePageAgainstSchema(invalidVisibleWhen); report.IsValid {
		t.Fatalf("非法 visible_when path 应被拦截")
	}

	// 6. 正确合规页面应校验通过
	validPage := &models.DynamicPage{
		AppID:        "wx_test",
		PageID:       "p1",
		Title:        "合规页面",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks: `[
			{
				"id": "btn4",
				"type": "action_button",
				"action": {
					"type": "copy_text",
					"payload": { "text": "123456" }
				},
				"visible_when": {
					"path": "$state.show_code",
					"eq": true
				}
			}
		]`,
	}
	if report := ValidatePageAgainstSchema(validPage); !report.IsValid {
		t.Fatalf("合规页面校验应成功: %v", report.Errors)
	}
}

// TestSaveDraft_CASConflict 测试并发乐观锁 CAS 机制
func TestSaveDraft_CASConflict(t *testing.T) {
	if db.Mysql == nil {
		t.Skip("数据库未初始化，跳过持久化 CAS 并发测试")
	}

	s := NewSDUIService()
	appID := "cas_test_app"
	pageID := "cas_test_page"

	// 清理历史残留
	db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).Delete(&models.DynamicPageDraft{})

	draft1 := &models.DynamicPageDraft{
		AppID:        appID,
		PageID:       pageID,
		Title:        "CAS测试1",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks:       `[]`,
	}

	// 第一次保存，版本应初始化为 1
	if err := s.SaveDraftWithAudit(draft1, "admin", 0); err != nil {
		t.Fatalf("首次保存草稿失败: %v", err)
	}

	// 用户 A 基于 v1 修改草稿
	draftA := &models.DynamicPageDraft{
		AppID:        appID,
		PageID:       pageID,
		Title:        "用户A修改",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks:       `[]`,
	}
	if err := s.SaveDraftWithAudit(draftA, "adminA", 1); err != nil {
		t.Fatalf("用户 A 基于 v1 保存应成功: %v", err)
	}

	// 用户 B 仍基于 v1 保存，此时库内版本已是 v2，预期应触发 CAS 并发冲突
	draftB := &models.DynamicPageDraft{
		AppID:        appID,
		PageID:       pageID,
		Title:        "用户B并发冲突修改",
		BusinessType: "drama",
		Intent:       "watch",
		Theme:        "dark_glass",
		Blocks:       `[]`,
	}
	err := s.SaveDraftWithAudit(draftB, "adminB", 1)
	if err == nil {
		t.Fatalf("预期发生乐观锁并发冲突，但未报错")
	}
	if !strings.Contains(err.Error(), "乐观锁 CAS 拦截") {
		t.Fatalf("错误信息未包含乐观锁提示: %v", err)
	}

	// 清理测试数据
	db.Mysql.Where("app_id = ? AND page_id = ?", appID, pageID).Delete(&models.DynamicPageDraft{})
}
