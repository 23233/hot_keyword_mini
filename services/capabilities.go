// Package services capabilities.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// CapabilityDefinition 描述一个可由 SDUI 复用的通用能力包。
type CapabilityDefinition struct {
	// 能力唯一键。
	Key string `json:"key"`
	// 能力协议版本。
	Version string `json:"version"`
	// 依赖的通用 Block。
	Blocks []string `json:"blocks"`
	// 依赖的受控 Action。
	Actions []string `json:"actions"`
	// 依赖的客户端/微信原生能力。
	Native []string `json:"native"`
	// 当前实现状态: ready / partial / planned。
	ImplementationStatus string `json:"implementation_status"`
}

// CapabilityValidationReport 是后台、MCP 和发布流程共用的能力校验结果。
type CapabilityValidationReport struct {
	// 是否允许发布或启用。
	IsValid bool `json:"is_valid"`
	// 页面实际依赖的能力。
	Required []string `json:"required"`
	// 阻断原因。
	Errors []string `json:"errors"`
	// 兼容或迁移提示。
	Warnings []string `json:"warnings"`
}

// AppCapabilityStatus 合并公共能力定义与指定租户的配置状态。
type AppCapabilityStatus struct {
	CapabilityDefinition
	// 当前租户配置；未配置时返回 disabled。
	Config models.CapabilityMatrixEntry `json:"config"`
}

// capabilityDefinitions 是公共能力注册表。业务租户只配置启用状态，不复制这份定义。
var capabilityDefinitions = []CapabilityDefinition{
	{Key: "video", Version: "1.0", Blocks: []string{"video", "media_hero"}, Actions: []string{"open_channels_activity"}, Native: []string{"wx.video", "wx.channels"}, ImplementationStatus: "ready"},
	{Key: "clipboard", Version: "1.0", Blocks: []string{"redeem_code_card", "resource_card"}, Actions: []string{"copy_text"}, Native: []string{"wx.setClipboardData"}, ImplementationStatus: "ready"},
	{Key: "payment", Version: "1.0", Blocks: []string{"product_card", "order_summary", "payment_result", "price_breakdown"}, Actions: []string{"request_payment"}, Native: []string{"wx.requestPayment", "payment.sandbox"}, ImplementationStatus: "ready"},
	{Key: "media_upload", Version: "1.0", Blocks: []string{"media_picker", "upload_progress", "media_gallery"}, Actions: []string{"choose_media", "upload_file", "delete_media"}, Native: []string{"wx.chooseMedia", "cos.presigned_upload"}, ImplementationStatus: "ready"},
	{Key: "location", Version: "1.0", Blocks: []string{"map_card", "address_card", "location_picker"}, Actions: []string{"request_location", "choose_location", "open_map"}, Native: []string{"wx.getLocation", "wx.chooseLocation", "tencent.map"}, ImplementationStatus: "ready"},
	{Key: "wechat_customer_service", Version: "1.0", Blocks: []string{"contact_card", "service_entry", "qr_code"}, Actions: []string{"open_wechat_service", "save_qr"}, Native: []string{"wx.openCustomerServiceChat", "wx.open-type.contact"}, ImplementationStatus: "ready"},
	{Key: "internal_chat", Version: "1.0", Blocks: []string{"chat_thread", "message_list", "message_composer", "unread_badge"}, Actions: []string{"open_internal_chat", "send_message", "mark_read"}, Native: []string{"sdui.chat", "platform_operations"}, ImplementationStatus: "ready"},
	{Key: "realtime_chat", Version: "1.0", Blocks: []string{"chat_thread", "message_list", "message_composer", "unread_badge"}, Actions: []string{"poll_messages", "connect_message", "upload_chat_media"}, Native: []string{"polling", "platform_operations"}, ImplementationStatus: "ready"},
	{Key: "orders", Version: "1.0", Blocks: []string{"order_card", "order_summary", "order_timeline"}, Actions: []string{"create_order", "confirm_order", "cancel_order", "confirm_receipt"}, Native: []string{"platform_operations"}, ImplementationStatus: "ready"},
	{Key: "logistics", Version: "1.0", Blocks: []string{"logistics_track"}, Actions: []string{"refresh_logistics"}, Native: []string{"platform_operations"}, ImplementationStatus: "ready"},
	{Key: "after_sale", Version: "1.0", Blocks: []string{"after_sale_form", "evidence_list"}, Actions: []string{"apply_after_sale", "upload_evidence"}, Native: []string{"platform_operations"}, ImplementationStatus: "ready"},
	{Key: "service_orders", Version: "1.0", Blocks: []string{"service_card", "task_card", "quote_card", "schedule_picker"}, Actions: []string{"accept_task", "reject_task", "submit_quote", "update_service_status"}, Native: []string{"platform_operations"}, ImplementationStatus: "ready"},
	{Key: "webview", Version: "1.0", Blocks: []string{"webview_entry", "webview_state"}, Actions: []string{"open_webview"}, Native: []string{"wx.web_view"}, ImplementationStatus: "ready"},
	{Key: "membership", Version: "1.0", Blocks: []string{"membership_card"}, Actions: []string{"open_membership"}, Native: []string{"sdui.membership", "payment.sandbox"}, ImplementationStatus: "ready"},
	{Key: "ads", Version: "1.0", Blocks: []string{"ad_slot"}, Actions: []string{"load_ad"}, Native: []string{"wx.ad"}, ImplementationStatus: "ready"},
	{Key: "wallet", Version: "1.0", Blocks: []string{"wallet_card", "withdraw_form"}, Actions: []string{"refresh_wallet", "request_withdraw"}, Native: []string{"virtual_ledger", "platform_operations"}, ImplementationStatus: "ready"},
	{Key: "subscribe_message", Version: "1.0", Blocks: []string{"action_button"}, Actions: []string{"subscribe_message"}, Native: []string{"wx.requestSubscribeMessage"}, ImplementationStatus: "ready"},
}

var capabilityByKey = func() map[string]CapabilityDefinition {
	result := make(map[string]CapabilityDefinition, len(capabilityDefinitions))
	for _, definition := range capabilityDefinitions {
		result[definition.Key] = definition
	}
	return result
}()

var capabilityBlockKeys = map[string]string{
	"video": "video", "media_hero": "video",
	"order_summary": "orders", "payment_result": "payment", "price_breakdown": "payment",
	"media_picker": "media_upload", "upload_progress": "media_upload", "media_gallery": "media_upload",
	"map_card": "location", "address_card": "location", "location_picker": "location",
	"service_entry": "wechat_customer_service", "qr_code": "wechat_customer_service",
	"chat_thread": "internal_chat", "message_list": "internal_chat", "message_composer": "internal_chat", "unread_badge": "internal_chat",
	"order_card": "orders", "order_timeline": "orders", "logistics_track": "logistics", "after_sale_form": "after_sale", "evidence_list": "after_sale",
	"service_card": "service_orders", "task_card": "service_orders", "quote_card": "service_orders", "schedule_picker": "service_orders",
	"webview_entry": "webview", "webview_state": "webview", "membership_card": "membership", "ad_slot": "ads", "wallet_card": "wallet", "withdraw_form": "wallet",
}

// ListCapabilityDefinitions 返回公共能力注册表的副本，供后台和 MCP 使用。
func ListCapabilityDefinitions() []CapabilityDefinition {
	result := make([]CapabilityDefinition, len(capabilityDefinitions))
	copy(result, capabilityDefinitions)
	return result
}

// RequiredCapabilitiesForBlocks 从页面 Block 树和动作链提取能力依赖。
func RequiredCapabilitiesForBlocks(blocks []models.BlockItem) []string {
	set := map[string]bool{}
	var scanAction func(*models.BlockAction)
	scanAction = func(action *models.BlockAction) {
		if action == nil {
			return
		}
		for _, capability := range actionCapabilities(*action) {
			set[capability] = true
		}
		for index := range action.OnSuccess {
			scanAction(&action.OnSuccess[index])
		}
		for index := range action.OnError {
			scanAction(&action.OnError[index])
		}
	}
	var scan func(models.BlockItem)
	scan = func(block models.BlockItem) {
		if capability := capabilityBlockKeys[block.Type]; capability != "" {
			set[capability] = true
		}
		scanAction(block.Action)
		for _, actions := range block.Events {
			for index := range actions {
				scanAction(&actions[index])
			}
		}
		for _, state := range []*models.BlockItem{block.Loading, block.Empty, block.Error, block.Fallback} {
			if state != nil {
				scan(*state)
			}
		}
		if block.Props != nil {
			for _, key := range []string{"children", "blocks", "items"} {
				if raw, ok := block.Props[key]; ok {
					var nested []models.BlockItem
					if encoded, err := json.Marshal(raw); err == nil && json.Unmarshal(encoded, &nested) == nil {
						for _, child := range nested {
							scan(child)
						}
					}
				}
			}
		}
	}
	for _, block := range blocks {
		scan(block)
	}
	result := make([]string, 0, len(set))
	for capability := range set {
		result = append(result, capability)
	}
	sort.Strings(result)
	return result
}

func actionCapabilities(action models.BlockAction) []string {
	result := []string{}
	switch action.Type {
	case "copy_text":
		result = append(result, "clipboard")
	case "open_channels_activity":
		result = append(result, "video")
	case "request_payment":
		result = append(result, "payment")
	case "open_webview":
		result = append(result, "webview")
	case "subscribe_message":
		result = append(result, "subscribe_message")
	case "choose_media", "upload_file", "delete_media":
		result = append(result, "media_upload")
	case "request_location", "choose_location", "open_map":
		result = append(result, "location")
	case "open_wechat_service", "save_qr":
		result = append(result, "wechat_customer_service")
	case "open_internal_chat", "send_message", "mark_read":
		result = append(result, "internal_chat")
	case "poll_messages", "connect_message", "upload_chat_media":
		result = append(result, "realtime_chat")
	case "create_order", "confirm_order", "cancel_order", "confirm_receipt":
		result = append(result, "orders")
	case "refresh_logistics":
		result = append(result, "logistics")
	case "apply_after_sale", "upload_evidence":
		result = append(result, "after_sale")
	case "accept_task", "reject_task", "submit_quote", "update_service_status":
		result = append(result, "service_orders")
	case "open_membership":
		result = append(result, "membership")
	case "load_ad":
		result = append(result, "ads")
	case "refresh_wallet", "request_withdraw":
		result = append(result, "wallet")
	}
	return result
}

// CapabilityMatrixForApp 读取租户能力矩阵；空矩阵用于兼容尚未迁移的旧租户。
func CapabilityMatrixForApp(app *models.MiniApp) (map[string]models.CapabilityMatrixEntry, error) {
	if app == nil || strings.TrimSpace(app.CapabilityMatrix) == "" {
		return map[string]models.CapabilityMatrixEntry{}, nil
	}
	result := map[string]models.CapabilityMatrixEntry{}
	if err := json.Unmarshal([]byte(app.CapabilityMatrix), &result); err != nil {
		return nil, fmt.Errorf("租户能力矩阵 JSON 无效: %w", err)
	}
	return result, nil
}

// ValidatePageCapabilities 校验页面依赖的能力是否已在租户矩阵中启用。
func ValidatePageCapabilities(appID string, blocks []models.BlockItem) (CapabilityValidationReport, error) {
	if strings.TrimSpace(appID) == "" || db.Mysql == nil {
		return ValidatePageCapabilitiesForMatrix(blocks, nil, false), nil
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return CapabilityValidationReport{}, fmt.Errorf("读取租户能力矩阵失败: %w", err)
	}
	matrix, err := CapabilityMatrixForApp(&app)
	if err != nil {
		return CapabilityValidationReport{}, err
	}
	return ValidatePageCapabilitiesForMatrix(blocks, matrix, len(matrix) > 0), nil
}

// ValidatePageCapabilitiesForMatrix 是不依赖数据库的公共发布门禁核心。
func ValidatePageCapabilitiesForMatrix(blocks []models.BlockItem, matrix map[string]models.CapabilityMatrixEntry, strict bool) CapabilityValidationReport {
	report := CapabilityValidationReport{IsValid: true, Required: RequiredCapabilitiesForBlocks(blocks), Errors: []string{}, Warnings: []string{}}
	if !strict {
		report.Warnings = append(report.Warnings, "租户尚未配置能力矩阵，当前按兼容模式校验")
		return report
	}
	for _, capability := range report.Required {
		definition := capabilityByKey[capability]
		if definition.ImplementationStatus != "ready" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("页面依赖能力 %q，但实现状态为 %q，尚未通过完整验收", capability, definition.ImplementationStatus))
			continue
		}
		entry, ok := matrix[capability]
		if !ok {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("页面依赖能力 %q，但租户能力矩阵未配置", capability))
			continue
		}
		if entry.State != "enabled" {
			report.IsValid = false
			report.Errors = append(report.Errors, fmt.Sprintf("页面依赖能力 %q，但当前状态为 %q", capability, entry.State))
		}
	}
	return report
}

// ValidateCapabilityKey 确保后台/MCP 只操作公共注册表中的能力。
func ValidateCapabilityKey(key string) error {
	if _, ok := capabilityByKey[key]; !ok {
		return errors.New("未知 SDUI 能力: " + key)
	}
	return nil
}

// ValidateCapabilityMatrix 校验租户能力矩阵的能力键与状态。
func ValidateCapabilityMatrix(matrix map[string]models.CapabilityMatrixEntry) error {
	validStates := map[string]bool{"disabled": true, "configured": true, "enabled": true, "blocked": true, "degraded": true}
	for key, entry := range matrix {
		if err := ValidateCapabilityKey(key); err != nil {
			return err
		}
		if !validStates[entry.State] {
			return fmt.Errorf("能力 %q 的状态 %q 无效", key, entry.State)
		}
	}
	return nil
}

// BuildCapabilityMatrix 构造包含全部公共能力的租户矩阵 JSON。
func BuildCapabilityMatrix(states map[string]string) (string, error) {
	matrix := make(map[string]models.CapabilityMatrixEntry, len(capabilityDefinitions))
	for _, definition := range capabilityDefinitions {
		state := "disabled"
		if configured := strings.TrimSpace(states[definition.Key]); configured != "" {
			state = configured
		}
		matrix[definition.Key] = models.CapabilityMatrixEntry{
			State: state, ProtocolVersion: definition.Version, ReviewStatus: "local_development",
		}
	}
	if err := ValidateCapabilityMatrix(matrix); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(matrix)
	return string(encoded), err
}

// BuildReadyCapabilityMatrix 为本地验收租户生成全部已实现能力的启用矩阵。
func BuildReadyCapabilityMatrix() (string, error) {
	states := make(map[string]string, len(capabilityDefinitions))
	for _, definition := range capabilityDefinitions {
		if definition.ImplementationStatus == "ready" {
			states[definition.Key] = "enabled"
		}
	}
	return BuildCapabilityMatrix(states)
}

// ListAppCapabilities 返回指定租户的完整能力矩阵，未配置项显式标记为 disabled。
func ListAppCapabilities(appID string) ([]AppCapabilityStatus, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, errors.New("AppID 不能为空")
	}
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化")
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil, fmt.Errorf("读取小程序失败: %w", err)
	}
	matrix, err := CapabilityMatrixForApp(&app)
	if err != nil {
		return nil, err
	}
	result := make([]AppCapabilityStatus, 0, len(capabilityDefinitions))
	for _, definition := range capabilityDefinitions {
		entry, ok := matrix[definition.Key]
		if !ok {
			entry = models.CapabilityMatrixEntry{State: "disabled", ProtocolVersion: definition.Version, ReviewStatus: "not_submitted"}
		}
		result = append(result, AppCapabilityStatus{CapabilityDefinition: definition, Config: entry})
	}
	return result, nil
}

// ConfigureAppCapability 原子更新单项能力配置并写入审计记录。
func ConfigureAppCapability(appID, capability string, entry models.CapabilityMatrixEntry, operator string) error {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(operator) == "" {
		return errors.New("AppID 与操作人不能为空")
	}
	if err := ValidateCapabilityKey(capability); err != nil {
		return err
	}
	definition := capabilityByKey[capability]
	if entry.State == "enabled" && definition.ImplementationStatus != "ready" {
		return fmt.Errorf("能力 %q 当前实现状态为 %q，未通过完整验收前不能启用", capability, definition.ImplementationStatus)
	}
	if err := ValidateCapabilityMatrix(map[string]models.CapabilityMatrixEntry{capability: entry}); err != nil {
		return err
	}
	if db.Mysql == nil {
		return errors.New("数据库未初始化")
	}
	return db.Mysql.Transaction(func(tx *gorm.DB) error {
		var app models.MiniApp
		if err := tx.Where("app_id = ?", appID).First(&app).Error; err != nil {
			return fmt.Errorf("读取小程序失败: %w", err)
		}
		matrix, err := CapabilityMatrixForApp(&app)
		if err != nil {
			return err
		}
		previousState := "disabled"
		if previous, ok := matrix[capability]; ok && previous.State != "" {
			previousState = previous.State
		}
		matrix[capability] = entry
		encodedMatrix, err := json.Marshal(matrix)
		if err != nil {
			return err
		}
		if err := tx.Model(&app).Updates(map[string]interface{}{"capability_matrix": string(encodedMatrix), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		snapshot, _ := json.Marshal(entry)
		return tx.Create(&models.CapabilityChangeLog{
			AppID: appID, Capability: capability, PreviousState: previousState, NewState: entry.State,
			ConfigSnapshot: string(snapshot), CreatedBy: operator, CreatedAt: time.Now(),
		}).Error
	})
}
