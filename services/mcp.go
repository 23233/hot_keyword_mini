// Package services mcp.go
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/23233/ggg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONRPCRequest 标准 JSON-RPC 2.0 请求
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse 标准 JSON-RPC 2.0 响应
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError 标准 JSON-RPC 2.0 错误
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPToolDefinition MCP 工具元信息与 Schema 定义
type MCPToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	// RequiredScope 是执行该工具所需的最小 MCP 权限范围。
	RequiredScope string                 `json:"requiredScope"`
	Annotations   map[string]interface{} `json:"annotations,omitempty"`
}

// MCPService AI Model Context Protocol 编排调度服务
type MCPService struct {
	templateService  *TemplateService
	sduiService      *SDUIService
	shareCardService *ShareCardService
}

// NewMCPService 创建 MCP 编排调度服务
func NewMCPService() *MCPService {
	return &MCPService{
		templateService:  NewTemplateService(),
		sduiService:      NewSDUIService(),
		shareCardService: NewShareCardService(),
	}
}

// GetToolDefinitions 获取全部受控编排工具清单。
func (m *MCPService) GetToolDefinitions() []MCPToolDefinition {
	tools := []MCPToolDefinition{
		{
			Name:        "sdui.app.list",
			Description: "查询当前 MCP 凭证可操作的全部已注册小程序 AppID 与基础状态",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "sdui.capability.list",
			Description: "读取公共能力注册表及指定 AppID 的能力状态，区分 ready、partial 与 planned",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.capability.validate",
			Description: "使用后台同一发布门禁校验页面依赖的租户能力",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "page_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "page_id": map[string]interface{}{"type": "string"}, "draft": map[string]interface{}{"type": "boolean"}}},
		},
		{
			Name:        "sdui.capability.configure",
			Description: "原子配置指定 AppID 的单项能力并写入审计，必须人工确认",
			InputSchema: map[string]interface{}{
				"type": "object", "required": []string{"app_id", "capability", "config", "confirmed"},
				"properties": map[string]interface{}{
					"app_id": map[string]interface{}{"type": "string"}, "capability": map[string]interface{}{"type": "string"}, "confirmed": map[string]interface{}{"type": "boolean"},
					"config": map[string]interface{}{"type": "object", "required": []string{"state"}, "properties": map[string]interface{}{
						"state":            map[string]interface{}{"type": "string", "enum": []string{"disabled", "configured", "enabled", "blocked", "degraded"}},
						"protocol_version": map[string]interface{}{"type": "string"}, "minimum_client_version": map[string]interface{}{"type": "string"},
						"review_status": map[string]interface{}{"type": "string"}, "domain_keys": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}, "reason": map[string]interface{}{"type": "string"},
					}},
				},
			},
		},
		{
			Name:        "sdui.acceptance.run",
			Description: "生成指定 AppID 页面和能力的只读验收清单，汇总协议、能力、Layout IR、Block/Action 和视觉/微信证据要求；不发布、不启用能力",
			InputSchema: map[string]interface{}{
				"type": "object", "required": []string{"app_id"},
				"properties": map[string]interface{}{
					"app_id":     map[string]interface{}{"type": "string", "description": "已注册小程序 AppID"},
					"page_id":    map[string]interface{}{"type": "string", "description": "可选；只检查指定页面"},
					"capability": map[string]interface{}{"type": "string", "description": "可选；只返回依赖指定能力的页面"},
				},
			},
		},
		{
			Name:        "sdui.webview.list",
			Description: "读取指定 AppID 已登记的 WebView url_key、用途、版本和启用状态",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.webview.validate",
			Description: "校验指定 AppID 的 WebView url_key 是否已登记并启用，返回受控 HTTPS 地址",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "url_key"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "url_key": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.page.list",
			Description: "读取指定小程序的全部页面及发布状态",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string", "description": "已注册小程序 AppID"}}},
		},
		{
			Name:        "sdui.page.get",
			Description: "读取页面原始协议或当前草稿协议",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "page_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "page_id": map[string]interface{}{"type": "string"}, "draft": map[string]interface{}{"type": "boolean", "description": "是否优先读取草稿，默认 true"}}},
		},
		{
			Name:        "sdui.file.prepare_upload",
			Description: "为图片申请短时 COS 预签名 PUT 地址；调用方上传完成后将 finalCosFileUrl 写入页面协议",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "file_name", "file_size", "content_type"},
				"properties": map[string]interface{}{
					"app_id":       map[string]interface{}{"type": "string", "description": "已注册小程序 AppID"},
					"file_name":    map[string]interface{}{"type": "string", "description": "原始图片文件名"},
					"file_size":    map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 10485760, "description": "文件大小，单位字节，最大 10MB"},
					"content_type": map[string]interface{}{"type": "string", "enum": []string{"image/jpeg", "image/png", "image/webp", "image/gif"}},
					"owner_type":   map[string]interface{}{"type": "string", "enum": []string{"sdui", "drama", "share", "resources"}, "description": "资源分类，默认 resources"},
				},
			},
		},
		{
			Name:        "sdui.template.list",
			Description: "查询内置行业模板与指定小程序可复用的用户模板；AI 破甲使用 ai_breakthrough / ai_article 过滤",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"app_id": map[string]interface{}{
						"type":        "string",
						"description": "可选；传入后同时返回该小程序的用户模板",
					},
					"business_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"drama", "game", "query", "download", "custom", "ai_breakthrough", "ai_article"},
						"description": "按业务类型过滤",
					},
				},
			},
		},
		{
			Name:        "sdui.template.get",
			Description: "读取一个内置模板或指定小程序内的用户模板完整协议",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"template_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "template_id": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.template.save",
			Description: "创建或更新用户 SDUI 模板；模板协议使用 default_blocks/default_share，与 HTTP 管理接口完全一致",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "template"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "template": map[string]interface{}{"type": "object", "description": "模板对象，必须含 template_id、name，更新时搭配 expected_revision"}, "expected_revision": map[string]interface{}{"type": "integer", "minimum": 0}}},
		},
		{
			Name:        "sdui.template.delete",
			Description: "按修订版本删除指定小程序内的用户模板；内置模板不可删除",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "template_id", "expected_revision"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "template_id": map[string]interface{}{"type": "string"}, "expected_revision": map[string]interface{}{"type": "integer", "minimum": 1}}},
		},
		{
			Name:        "sdui.page.create",
			Description: "从行业模板或空白结构创建全新草稿页面；page_id 已存在时拒绝覆盖，请改用 page.get + page.patch",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id"},
				"properties": map[string]interface{}{
					"app_id": map[string]interface{}{
						"type":        "string",
						"description": "归属小程序 AppID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "页面唯一标识, 如 game_detail / nov_redeem",
					},
					"template_id": map[string]interface{}{
						"type":        "string",
						"description": "所选模板ID，可留空以创建空白草稿",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "页面主标题 (可选，默认为模板预设名称)",
					},
				},
			},
		},
		{
			Name:        "sdui.page.patch",
			Description: "基于 expected_revision 对草稿执行受控补丁；replace 支持页面字段和 /blocks/{block_id}，并支持 add_block/remove_block",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id", "expected_revision", "ops"},
				"properties": map[string]interface{}{
					"app_id":            map[string]interface{}{"type": "string"},
					"page_id":           map[string]interface{}{"type": "string"},
					"expected_revision": map[string]interface{}{"type": "integer", "minimum": 1, "description": "page.get 返回的当前草稿 revision；版本不一致时拒绝覆盖"},
					"ops": map[string]interface{}{
						"type":        "array",
						"minItems":    1,
						"description": "操作格式：replace 用 path=/title 等页面字段或 /blocks/{id}；add_block 的 value 为完整 BlockItem；remove_block 的 value 为积木 ID",
						"items":       map[string]interface{}{"type": "object", "required": []string{"op", "value"}, "properties": map[string]interface{}{"op": map[string]interface{}{"type": "string", "enum": []string{"replace", "add_block", "remove_block"}}, "path": map[string]interface{}{"type": "string"}, "value": map[string]interface{}{}}},
					},
				},
			},
		},
		{
			Name:        "sdui.page.validate",
			Description: "对页面协议执行强校验，输出机器可读错误、警告和修复建议；直接传 page 时 blocks 推荐使用 BlockItem 数组，也兼容 JSON 字符串",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"app_id":   map[string]interface{}{"type": "string", "description": "归属小程序 AppID"},
					"page_id":  map[string]interface{}{"type": "string", "description": "页面唯一标识 (与 page 二选一)"},
					"page":     map[string]interface{}{"type": "object", "description": "待校验的完整页面协议对象 (与 page_id 二选一)"},
					"protocol": map[string]interface{}{"type": "string", "description": "待校验的完整页面协议 JSON 字符串"},
				},
			},
		},
		{
			Name:        "sdui.page.preview",
			Description: "使用指定查询参数装配草稿信封；客户端能力协商由正式页面请求的 X-Client-Capabilities 处理",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id"},
				"properties": map[string]interface{}{
					"app_id":  map[string]interface{}{"type": "string"},
					"page_id": map[string]interface{}{"type": "string"},
					"query":   map[string]interface{}{"type": "object", "description": "模拟查询参数"},
				},
			},
		},
		{
			Name:        "sdui.page.screenshot",
			Description: "对草稿或已发布页面执行规范化布局截图，输出签名图像 URL、哈希、Layout IR、结构树、原生能力替身和问题列表",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id"},
				"properties": map[string]interface{}{
					"app_id":    map[string]interface{}{"type": "string"},
					"page_id":   map[string]interface{}{"type": "string"},
					"device":    map[string]interface{}{"type": "string", "description": "设备预设，默认 iphone_12_13_pro"},
					"host":      map[string]interface{}{"type": "string", "description": "服务 HTTPS 根地址，可选；为空时使用 PUBLIC_BASE_URL"},
					"card_type": map[string]interface{}{"type": "string", "enum": []string{"app_message", "timeline"}, "description": "兼容旧客户端的分享图比例标识；截图结构仍以 device 为准"},
					"theme":     map[string]interface{}{"type": "string", "enum": []string{"dark_glass", "light_clean", "cyber_neon"}},
					"locale":    map[string]interface{}{"type": "string", "description": "语言标识，默认 zh-CN"},
					"state":     map[string]interface{}{"type": "string", "enum": []string{"normal", "loading", "empty", "error", "offline", "expired", "unauthenticated"}, "description": "视觉状态 Fixture，默认 normal"},
				},
			},
		},
		{
			Name:        "sdui.page.publish",
			Description: "显式发布已通过强校验的页面草稿，将其置为 published 并沉淀版本快照 (必须人工显式确认 confirmed: true)",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id", "expected_revision", "confirmed"},
				"properties": map[string]interface{}{
					"app_id":            map[string]interface{}{"type": "string", "description": "小程序 AppID"},
					"page_id":           map[string]interface{}{"type": "string", "description": "页面 PageID"},
					"confirmed":         map[string]interface{}{"type": "boolean", "description": "人工显式发布确认标记，必须为 true"},
					"expected_revision": map[string]interface{}{"type": "integer", "minimum": 1, "description": "人工实际审查通过的草稿 revision，发布时必须仍完全一致"},
					"remark":            map[string]interface{}{"type": "string", "description": "发布审计备注"},
				},
			},
		},
		{
			Name:        "sdui.page.revisions",
			Description: "读取页面历史发布版本，供 AI 审查和选择回滚目标",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "page_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "page_id": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.page.rollback",
			Description: "将页面原子回滚到历史版本并立即发布（需要人工确认）",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "page_id", "target_revision", "confirmed"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "page_id": map[string]interface{}{"type": "string"}, "target_revision": map[string]interface{}{"type": "integer", "minimum": 1}, "confirmed": map[string]interface{}{"type": "boolean", "description": "人工确认必须为 true"}}},
		},
		{
			Name:        "sdui.page.set_current",
			Description: "将已发布页面设置为指定小程序当前激活主页（需要人工确认）",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "page_id", "confirmed"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "page_id": map[string]interface{}{"type": "string"}, "confirmed": map[string]interface{}{"type": "boolean", "description": "人工确认必须为 true"}}},
		},
		{
			Name:        "sdui.page.share_card",
			Description: "为已发布页面生成并持久化微信好友/朋友圈分享图（需要人工确认）",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "page_id", "confirmed"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "page_id": map[string]interface{}{"type": "string"}, "host": map[string]interface{}{"type": "string", "description": "服务 HTTPS 根地址，可选"}, "confirmed": map[string]interface{}{"type": "boolean", "description": "人工确认必须为 true"}}},
		},
		{
			Name:        "sdui.operation.execute",
			Description: "执行游戏兑换和查询等通用动作；管理领域写操作请使用 sdui.admin.execute",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "endpoint"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "endpoint": map[string]interface{}{"type": "string"}, "id": map[string]interface{}{"type": "string"}, "payload": map[string]interface{}{"type": "object"}, "idempotency_key": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.payment.sandbox",
			Description: "创建或推进本地支付沙箱订单，覆盖成功、取消、失败、退款和会员权益发放",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "operation"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "operation": map[string]interface{}{"type": "string", "enum": []string{"create", "success", "cancel", "fail", "refund"}}, "user_id": map[string]interface{}{"type": "integer", "minimum": 1}, "openid": map[string]interface{}{"type": "string"}, "sku": map[string]interface{}{"type": "string"}, "out_trade_no": map[string]interface{}{"type": "string"}, "idempotency_key": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.production.readiness",
			Description: "只读检查指定小程序上线所需生产配置，不返回任何密钥原文",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}}},
		},
		{
			Name:        "sdui.admin.execute",
			Description: "内部高权限管理操作：配置小程序、能力、WebView、商品、会员、栏目、文章、评论、短剧和运营记录；管理员账号与 MCP Token 永不开放",
			InputSchema: map[string]interface{}{"type": "object", "required": []string{"app_id", "operation", "confirmed"}, "properties": map[string]interface{}{"app_id": map[string]interface{}{"type": "string"}, "operation": map[string]interface{}{"type": "string", "enum": AdminMCPOperations}, "payload": map[string]interface{}{"type": "object"}, "confirmed": map[string]interface{}{"type": "boolean", "description": "写操作必须为 true；只读操作可为 false"}}},
		},
	}
	for index := range tools {
		name := tools[index].Name
		readOnly := name == "sdui.acceptance.run" || name == "sdui.production.readiness" || strings.HasSuffix(name, ".list") || strings.HasSuffix(name, ".get") || strings.HasSuffix(name, ".validate") || strings.HasSuffix(name, ".preview") || strings.HasSuffix(name, ".screenshot") || strings.HasSuffix(name, ".revisions")
		tools[index].RequiredScope = mcpToolRequiredScope(name)
		tools[index].Annotations = map[string]interface{}{"readOnlyHint": readOnly, "destructiveHint": strings.HasSuffix(name, ".delete") || strings.HasSuffix(name, ".rollback"), "openWorldHint": false, "requiredScope": tools[index].RequiredScope, "required_scope": tools[index].RequiredScope}
	}
	return tools
}

// mcpToolRequiredScope 返回工具执行所需的最小权限范围，供 AI 在调用前规划授权。
func mcpToolRequiredScope(name string) string {
	switch name {
	case "sdui.app.list", "sdui.capability.list", "sdui.capability.validate", "sdui.acceptance.run", "sdui.production.readiness", "sdui.webview.list", "sdui.webview.validate", "sdui.page.list", "sdui.page.get", "sdui.template.list", "sdui.template.get", "sdui.page.validate", "sdui.page.preview", "sdui.page.screenshot", "sdui.page.revisions":
		return "read"
	case "sdui.admin.execute":
		return "release"
	case "sdui.file.prepare_upload", "sdui.template.save", "sdui.template.delete", "sdui.page.create", "sdui.page.patch":
		return "write:draft"
	case "sdui.capability.configure", "sdui.page.publish", "sdui.page.rollback", "sdui.page.set_current", "sdui.page.share_card":
		return "release"
	case "sdui.operation.execute", "sdui.payment.sandbox":
		return "write:draft"
	default:
		return ""
	}
}

// HandleJSONRPC 处理标准 JSON-RPC 2.0 请求 (默认全权限，用于本地 Stdio)
func (m *MCPService) HandleJSONRPC(reqBytes []byte) ([]byte, error) {
	return m.HandleJSONRPCWithContext("stdio_local", "", []string{"read", "write:draft", "release"}, reqBytes)
}

// HandleJSONRPCWithContext 带身份与权限范围上下文的 JSON-RPC 2.0 处理器
func (m *MCPService) HandleJSONRPCWithContext(actorID, tenantID string, scopes []string, reqBytes []byte) ([]byte, error) {
	var rawMessage json.RawMessage
	if err := json.Unmarshal(reqBytes, &rawMessage); err != nil {
		return json.Marshal(JSONRPCResponse{JSONRPC: "2.0", Error: &JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()}})
	}
	var rawObject map[string]json.RawMessage
	if err := json.Unmarshal(rawMessage, &rawObject); err != nil || rawObject == nil {
		return json.Marshal(JSONRPCResponse{JSONRPC: "2.0", Error: &JSONRPCError{Code: -32600, Message: "Invalid Request: JSON-RPC 消息必须为对象"}})
	}
	var req JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		return json.Marshal(JSONRPCResponse{JSONRPC: "2.0", Error: &JSONRPCError{Code: -32600, Message: "Invalid Request: " + err.Error()}})
	}
	if req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" {
		return json.Marshal(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCError{Code: -32600, Message: "Invalid Request: jsonrpc 必须为 2.0 且 method 不能为空"}})
	}
	if req.Method == "notifications/initialized" || req.Method == "notifications/cancelled" {
		return nil, nil
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "ping":
		resp.Result = map[string]interface{}{}

	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "sdui-mcp-server",
				"version": "1.3.0",
			},
			"instructions": "先读取 resources/read 资源 sdui://rules 和 sdui://api，再调用 tools/list。先 app.list/page.list/page.get，优先复用模板；所有写入都基于最新 expected_revision。AI 默认只写草稿；发布、回滚、设置主页和生成分享图必须由人审查后 confirmed=true。任何工具失败都先按 structuredContent.recovery 修复，不得绕过权限或校验。",
			"capabilities": map[string]interface{}{
				"tools":     map[string]bool{"listChanged": false},
				"resources": map[string]bool{"subscribe": false, "listChanged": false},
			},
		}

	case "resources/list":
		resp.Result = map[string]interface{}{"resources": []map[string]interface{}{
			{"uri": "sdui://api", "name": "MCP 接口与自动编排流程", "description": "MCP JSON-RPC 接口、权限和推荐调用顺序", "mimeType": "application/json"},
			{"uri": "sdui://rules", "name": "SDUI 完整协议规则", "description": "积木、动作、绑定、状态和图片资源规则", "mimeType": "application/json"},
		}}

	case "resources/read":
		var resourceParams struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &resourceParams); err != nil || strings.TrimSpace(resourceParams.URI) == "" {
			resp.Error = &JSONRPCError{Code: -32602, Message: "resources/read 需要 uri 参数"}
			break
		}
		var resource interface{}
		switch resourceParams.URI {
		case "sdui://api":
			resource = mcpAPIResource(m.GetToolDefinitions())
		case "sdui://rules":
			resource = mcpRulesResource()
		default:
			resp.Error = &JSONRPCError{Code: -32004, Message: "未知 MCP 资源: " + resourceParams.URI}
			break
		}
		if resp.Error == nil {
			content, _ := json.Marshal(resource)
			resp.Result = map[string]interface{}{"contents": []map[string]interface{}{{"uri": resourceParams.URI, "mimeType": "application/json", "text": string(content)}}}
		}

	case "tools/list":
		resp.Result = map[string]interface{}{
			"tools": m.GetToolDefinitions(),
		}

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			resp.Error = &JSONRPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
			break
		}
		if strings.TrimSpace(callParams.Name) == "" {
			resp.Error = &JSONRPCError{Code: -32602, Message: "tools/call 需要 name 参数"}
			break
		}
		if err := validateMCPArguments(callParams.Name, callParams.Arguments, m.GetToolDefinitions()); err != nil {
			requestID := uuid.NewString()
			resp.Result = map[string]interface{}{"isError": true, "requestId": requestID, "structuredContent": mcpToolErrorDetail(callParams.Name, err), "content": []map[string]string{{"type": "text", "text": fmt.Sprintf("工具执行失败: %v", err)}}}
			break
		}

		requestID := uuid.NewString()
		result, err := m.ExecuteToolWithContext(actorID, tenantID, scopes, callParams.Name, callParams.Arguments)
		if err != nil {
			detail := mcpToolErrorDetail(callParams.Name, err)
			mcpAuditf("【MCP审计】Request=%s Actor=%s Tenant=%s Tool=%s Result=error Code=%s", requestID, actorID, tenantID, callParams.Name, detail["code"])
			resp.Result = map[string]interface{}{
				"isError":           true,
				"requestId":         requestID,
				"structuredContent": detail,
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("工具执行失败: %v", err)},
				},
			}
		} else {
			rawJSON, _ := json.MarshalIndent(result, "", "  ")
			mcpAuditf("【MCP审计】Request=%s Actor=%s Tenant=%s Tool=%s Result=success", requestID, actorID, tenantID, callParams.Name)
			resp.Result = map[string]interface{}{
				"isError":           false,
				"requestId":         requestID,
				"structuredContent": result,
				"content": []map[string]string{
					{"type": "text", "text": string(rawJSON)},
				},
				"data": result,
			}
		}

	default:
		resp.Error = &JSONRPCError{Code: -32601, Message: "Method not found: " + req.Method}
	}

	return json.Marshal(resp)
}

func mcpToolErrorDetail(tool string, err error) map[string]interface{} {
	message := err.Error()
	code := "TOOL_EXECUTION_FAILED"
	recovery := "检查工具参数后重试；不要绕过服务端校验"
	switch {
	case strings.Contains(message, "未知 MCP 工具"):
		code, recovery = "UNKNOWN_TOOL", "先调用 tools/list 获取当前可用工具名称"
	case strings.Contains(message, "权限不足") || strings.Contains(message, "越权"):
		code, recovery = "PERMISSION_DENIED", "使用具备所需 scope 的凭证，并确认 app_id 位于授权租户范围"
	case strings.Contains(message, "未找到") || strings.Contains(message, "不存在"):
		code, recovery = "NOT_FOUND", "先调用对应 list/get 工具确认资源标识"
	case strings.Contains(message, "必填") || strings.Contains(message, "参数") || strings.Contains(message, "格式"):
		code, recovery = "INVALID_ARGUMENT", "读取 tools/list 的 inputSchema 与 sdui://rules 后修正参数"
	case strings.Contains(message, "并发") || strings.Contains(message, "revision"):
		code, recovery = "REVISION_CONFLICT", "重新调用 page.get 读取最新草稿，基于新的 revision 重新生成补丁并再次校验"
	}
	return map[string]interface{}{"code": code, "tool": tool, "message": message, "recovery": recovery}
}

// containsString 判断字符串切片是否包含目标值。
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// acceptanceCoverage 提取页面树中的 Block 与 Action 类型，供验收摘要和覆盖率证据使用。
func acceptanceCoverage(blocks []models.BlockItem) ([]string, []string) {
	blockSet := make(map[string]bool)
	actionSet := make(map[string]bool)
	var scanAction func(*models.BlockAction)
	scanAction = func(action *models.BlockAction) {
		if action == nil {
			return
		}
		if action.Type != "" {
			actionSet[action.Type] = true
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
		if block.Type != "" {
			blockSet[block.Type] = true
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
		for _, child := range collectNestedBlocks(block.Props) {
			scan(child)
		}
	}
	for _, block := range blocks {
		scan(block)
	}
	return sortedMapKeys(blockSet), sortedMapKeys(actionSet)
}

// errorText 将可选错误转换为稳定的验收输出字段。
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// layoutHeight 读取可选 Layout IR 的总高度。
func layoutHeight(ir *models.PageLayoutIR) int {
	if ir == nil {
		return 0
	}
	return ir.TotalHeight
}

// validateMCPArguments 在执行前落实 tools/list 暴露的最小参数契约。
func validateMCPArguments(name string, args map[string]interface{}, tools []MCPToolDefinition) error {
	var definition *MCPToolDefinition
	for index := range tools {
		if tools[index].Name == name {
			definition = &tools[index]
			break
		}
	}
	if definition == nil {
		return fmt.Errorf("未知 MCP 工具: %s", name)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	if err := validateMCPSchemaValue("arguments", args, definition.InputSchema); err != nil {
		return err
	}
	if name == "sdui.page.patch" {
		rawOps, ok := args["ops"].([]interface{})
		if !ok {
			return errors.New("参数 ops 必须为数组")
		}
		for index, rawOp := range rawOps {
			op, ok := rawOp.(map[string]interface{})
			if !ok {
				return fmt.Errorf("参数 ops[%d] 必须为对象", index)
			}
			switch op["op"] {
			case "replace":
				if path, _ := op["path"].(string); strings.TrimSpace(path) == "" {
					return fmt.Errorf("参数 ops[%d].path 在 replace 操作中不能为空", index)
				}
			case "add_block":
				if _, ok := op["value"].(map[string]interface{}); !ok {
					return fmt.Errorf("参数 ops[%d].value 在 add_block 操作中必须为 BlockItem 对象", index)
				}
			case "remove_block":
				if value, _ := op["value"].(string); strings.TrimSpace(value) == "" {
					return fmt.Errorf("参数 ops[%d].value 在 remove_block 操作中必须为积木 ID 字符串", index)
				}
			}
		}
	}
	if name == "sdui.page.validate" {
		if _, hasPage := args["page"]; !hasPage {
			if protocol, hasProtocol := args["protocol"]; !hasProtocol || strings.TrimSpace(fmt.Sprintf("%v", protocol)) == "" {
				if _, hasApp := args["app_id"]; !hasApp {
					return errors.New("page/protocol 直接校验或 app_id + page_id 至少提供一组")
				}
			}
		}
	}
	return nil
}

// validateMCPSchemaValue 执行 MCP 工具声明中当前使用的 JSON Schema 约束。
func validateMCPSchemaValue(path string, value interface{}, schema map[string]interface{}) error {
	if expected, _ := schema["type"].(string); expected != "" && !mcpValueMatchesType(value, expected) {
		return fmt.Errorf("参数 %s 类型必须为 %s", path, expected)
	}
	if enum, ok := schema["enum"].([]string); ok {
		actual, _ := value.(string)
		for _, candidate := range enum {
			if actual == candidate {
				return nil
			}
		}
		return fmt.Errorf("参数 %s 不在允许枚举范围内", path)
	}

	if actual, ok := mcpIntArg(value); ok {
		if minimum, exists := schema["minimum"]; exists {
			if minValue, valid := mcpIntArg(minimum); valid && actual < minValue {
				return fmt.Errorf("参数 %s 不得小于 %d", path, minValue)
			}
		}
		if maximum, exists := schema["maximum"]; exists {
			if maxValue, valid := mcpIntArg(maximum); valid && actual > maxValue {
				return fmt.Errorf("参数 %s 不得大于 %d", path, maxValue)
			}
		}
	}

	switch typed := value.(type) {
	case map[string]interface{}:
		if required, ok := schema["required"].([]string); ok {
			for _, key := range required {
				requiredValue, exists := typed[key]
				if !exists || requiredValue == nil {
					return fmt.Errorf("参数 %s.%s 为必填项", path, key)
				}
				if text, ok := requiredValue.(string); ok && strings.TrimSpace(text) == "" {
					return fmt.Errorf("参数 %s.%s 不能为空", path, key)
				}
			}
		}
		properties, _ := schema["properties"].(map[string]interface{})
		for key, childValue := range typed {
			childSchema, ok := properties[key].(map[string]interface{})
			if ok {
				if err := validateMCPSchemaValue(path+"."+key, childValue, childSchema); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		if minimum, ok := mcpIntArg(schema["minItems"]); ok && len(typed) < minimum {
			return fmt.Errorf("参数 %s 至少需要 %d 项", path, minimum)
		}
		if itemSchema, ok := schema["items"].(map[string]interface{}); ok {
			for index, item := range typed {
				if err := validateMCPSchemaValue(fmt.Sprintf("%s[%d]", path, index), item, itemSchema); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func mcpValueMatchesType(value interface{}, expected string) bool {
	if value == nil {
		return false
	}
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		_, ok := mcpIntArg(value)
		return ok
	case "array":
		return reflect.ValueOf(value).Kind() == reflect.Array || reflect.ValueOf(value).Kind() == reflect.Slice
	case "object":
		return reflect.ValueOf(value).Kind() == reflect.Map || reflect.ValueOf(value).Kind() == reflect.Struct
	default:
		return true
	}
}

// hasScope 判断当前持有的权限集是否包含指定权限
func hasScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target || s == "*" {
			return true
		}
	}
	return false
}

// ExecuteTool 执行指定的 MCP 受控工具 (默认本地 Stdio 全权限调用)
func (m *MCPService) ExecuteTool(name string, args map[string]interface{}) (interface{}, error) {
	return m.ExecuteToolWithContext("stdio_local", "", []string{"read", "write:draft", "release"}, name, args)
}

// ExecuteToolWithContext 在租户与权限隔离上下文下安全执行 MCP 工具
func (m *MCPService) ExecuteToolWithContext(actorID, tenantID string, scopes []string, name string, args map[string]interface{}) (interface{}, error) {
	if args == nil {
		args = make(map[string]interface{})
	}

	// 记录调用审计跟踪 (对敏感字段进行安全脱敏)
	sanitizedArgs := sanitizeMCPArgs(args)
	mcpAuditf("【MCP审计】Actor=%s Tenant=%s Tool=%s Args=%+v", actorID, tenantID, name, sanitizedArgs)

	switch name {
	case "sdui.template.list":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以查询行业模板列表")
		}
		businessType, _ := args["business_type"].(string)
		appID, err := resolveMCPOptionalAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if appID != "" {
			if err := ensureMCPAppExists(appID); err != nil {
				return nil, err
			}
		}
		templates, err := m.templateService.ListTemplates(appID, businessType)
		if err != nil {
			return nil, fmt.Errorf("查询模板列表失败: %w", err)
		}
		return map[string]interface{}{
			"total":     len(templates),
			"templates": templates,
		}, nil

	case "sdui.template.get":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以读取模板")
		}
		templateID, _ := args["template_id"].(string)
		appID, err := resolveMCPOptionalAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if appID != "" {
			if err := ensureMCPAppExists(appID); err != nil {
				return nil, err
			}
		}
		return m.templateService.GetTemplate(appID, templateID)

	case "sdui.template.save":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以保存用户模板")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		rawTemplate, ok := args["template"]
		if !ok {
			return nil, errors.New("template 为必填对象")
		}
		templateBytes, err := json.Marshal(rawTemplate)
		if err != nil {
			return nil, fmt.Errorf("模板参数序列化失败: %w", err)
		}
		var template SDUITemplate
		if err := ValidateSDUIStyleJSON(templateBytes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(templateBytes, &template); err != nil {
			return nil, fmt.Errorf("模板参数格式无效: %w", err)
		}
		if template.AppID != "" && template.AppID != appID {
			return nil, fmt.Errorf("多租户越权拦截: 模板 app_id %s 与目标小程序 %s 不一致", template.AppID, appID)
		}
		template.AppID = appID
		expectedRevision := 0
		if rawRevision, exists := args["expected_revision"]; exists {
			var ok bool
			expectedRevision, ok = mcpIntArg(rawRevision)
			if !ok || expectedRevision < 0 {
				return nil, errors.New("expected_revision 必须为 0 或正整数")
			}
		}
		return m.templateService.SaveCustomTemplate(&template, actorID, expectedRevision)

	case "sdui.template.delete":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以删除用户模板")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		templateID, _ := args["template_id"].(string)
		expectedRevision, ok := mcpIntArg(args["expected_revision"])
		if !ok {
			return nil, errors.New("expected_revision 必须为正整数")
		}
		if err := m.templateService.DeleteCustomTemplate(appID, templateID, expectedRevision); err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "deleted", "app_id": appID, "template_id": templateID}, nil

	case "sdui.app.list":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以查询小程序列表")
		}
		if db.Mysql == nil {
			return nil, errors.New("数据库未初始化，无法查询小程序列表")
		}
		apps, err := m.sduiService.ListApps()
		if err != nil {
			return nil, fmt.Errorf("查询小程序列表失败: %w", err)
		}
		allowed := mcpAllowedTenantSet()
		if len(allowed) > 0 {
			filtered := apps[:0]
			for _, app := range apps {
				if allowed[app.AppID] {
					filtered = append(filtered, app)
				}
			}
			apps = filtered
		}
		return map[string]interface{}{"total": len(apps), "apps": apps}, nil

	case "sdui.capability.list":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以查询能力矩阵")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		capabilities, err := ListAppCapabilities(appID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"app_id": appID, "total": len(capabilities), "capabilities": capabilities}, nil

	case "sdui.acceptance.run":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以生成验收清单")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		capabilityFilter, _ := args["capability"].(string)
		pages, err := m.sduiService.ListPages(appID)
		if err != nil {
			return nil, fmt.Errorf("读取验收页面失败: %w", err)
		}
		type acceptancePage struct {
			page   models.DynamicPage
			source string
		}
		candidates := make([]acceptancePage, 0, len(pages))
		for _, page := range pages {
			candidates = append(candidates, acceptancePage{page: page, source: "published"})
		}
		// 验收工具还要读取草稿，否则新页面必须先发布才能被检查，无法满足发布前门禁。
		var drafts []models.DynamicPageDraft
		if err := db.Mysql.Where("app_id = ?", appID).Order("updated_at desc").Find(&drafts).Error; err != nil {
			return nil, fmt.Errorf("读取验收草稿失败: %w", err)
		}
		for _, draft := range drafts {
			candidates = append(candidates, acceptancePage{page: models.DynamicPage{
				ID: draft.ID, AppID: draft.AppID, PageID: draft.PageID, Revision: draft.Revision, Status: draft.Status,
				Hidden: draft.Hidden, Title: draft.Title, BusinessType: draft.BusinessType, Intent: draft.Intent,
				Theme: draft.Theme, AccentColor: draft.AccentColor, RequireAuth: draft.RequireAuth,
				ShareConfig: draft.ShareConfig, Blocks: draft.Blocks, Keyword: draft.Keyword, Source: draft.Source,
				CampaignID: draft.CampaignID, ExpiresAt: draft.ExpiresAt, CreatedAt: draft.CreatedAt, UpdatedAt: draft.UpdatedAt,
			}, source: "draft"})
		}
		items := make([]map[string]interface{}, 0)
		coverageBlocks := map[string]bool{}
		coverageActions := map[string]bool{}
		protocolValidCount, capabilityValidCount, layoutAvailableCount := 0, 0, 0
		for _, candidate := range candidates {
			page := candidate.page
			if pageID != "" && page.PageID != pageID {
				continue
			}
			var blocks []models.BlockItem
			blockDecodeErr := json.Unmarshal([]byte(page.Blocks), &blocks)
			required := RequiredCapabilitiesForBlocks(blocks)
			if capabilityFilter != "" && !containsString(required, capabilityFilter) {
				continue
			}
			protocolReport := ValidateDynamicPage(&page)
			capabilityReport := CapabilityValidationReport{IsValid: blockDecodeErr == nil, Required: required, Errors: []string{}, Warnings: []string{}}
			var capabilityErr error
			if blockDecodeErr != nil {
				capabilityErr = blockDecodeErr
				capabilityReport.Errors = append(capabilityReport.Errors, "Blocks JSON 无效: "+blockDecodeErr.Error())
			} else {
				capabilityReport, capabilityErr = ValidatePageCapabilities(appID, blocks)
			}
			var webviewErr error
			if blockDecodeErr == nil && containsString(required, "webview") {
				webviewErr = validateWebViewKeysForRelease(appID, blocks)
				if webviewErr != nil {
					capabilityReport.IsValid = false
					capabilityReport.Errors = append(capabilityReport.Errors, webviewErr.Error())
				}
			}
			var ir *models.PageLayoutIR
			var irErr error
			if blockDecodeErr == nil {
				ir, irErr = BuildPageLayoutIR(&page, DefaultDeviceParams(), "normal")
			} else {
				irErr = blockDecodeErr
			}
			blockTypes, actionTypes := acceptanceCoverage(blocks)
			for _, blockType := range blockTypes {
				coverageBlocks[blockType] = true
			}
			for _, actionType := range actionTypes {
				coverageActions[actionType] = true
			}
			protocolOK := protocolReport.IsValid
			capabilityOK := capabilityErr == nil && webviewErr == nil && capabilityReport.IsValid
			layoutOK := irErr == nil
			if protocolOK {
				protocolValidCount++
			}
			if capabilityOK {
				capabilityValidCount++
			}
			if layoutOK {
				layoutAvailableCount++
			}
			items = append(items, map[string]interface{}{
				"page_id": page.PageID, "revision": page.Revision, "status": page.Status, "hidden": page.Hidden, "source": candidate.source,
				"protocol":          map[string]interface{}{"is_valid": protocolOK, "errors": protocolReport.Errors, "warnings": protocolReport.Warnings},
				"capabilities":      map[string]interface{}{"required": required, "is_valid": capabilityOK, "errors": capabilityReport.Errors, "warnings": capabilityReport.Warnings},
				"coverage":          map[string]interface{}{"block_types": blockTypes, "action_types": actionTypes},
				"layout_ir":         map[string]interface{}{"available": layoutOK, "error": errorText(irErr), "total_height": layoutHeight(ir)},
				"required_evidence": []string{"web_preview_screenshot", "wechat_devtools_screenshot", "console_log", "protocol_revision", "capability_matrix_snapshot"},
			})
		}
		blockCoverage := sortedMapKeys(coverageBlocks)
		actionCoverage := sortedMapKeys(coverageActions)
		return map[string]interface{}{
			"tool": "sdui.acceptance.run", "mode": "read_only", "app_id": appID, "page_id": pageID, "capability": capabilityFilter,
			"generated_at": time.Now().Format(time.RFC3339), "pages": items,
			"summary": map[string]interface{}{
				"page_count": len(items), "protocol_valid_count": protocolValidCount, "capability_valid_count": capabilityValidCount,
				"layout_available_count": layoutAvailableCount, "block_types": blockCoverage, "action_types": actionCoverage,
			},
			"next_steps": []string{"逐页执行 page.validate、capability.validate、page.preview、page.screenshot", "在微信开发者工具中按同一 AppID 执行主流程与失败流程", "仅在人工确认且具备 release 权限后发布"},
		}, nil

	case "sdui.production.readiness":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以检查生产配置")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		return CheckProductionReadiness(appID, config.Cfg)

	case "sdui.admin.execute":
		if !hasScope(scopes, "release") {
			return nil, errors.New("权限不足: 需要 release 权限以执行管理操作")
		}
		confirmed, ok := args["confirmed"].(bool)
		if !ok {
			return nil, errors.New("confirmed 必须为布尔值")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		operation, _ := args["operation"].(string)
		payload, _ := args["payload"].(map[string]interface{})
		if payload == nil {
			payload = map[string]interface{}{}
		}
		if !adminMCPReadOperations[operation] && !confirmed {
			return nil, errors.New("管理写操作必须传入 confirmed=true 二次确认")
		}
		result, err := ExecuteAdminMCPOperation(actorID, appID, operation, payload)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"app_id": appID, "operation": operation, "data": result}, nil

	case "sdui.webview.list":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以读取 WebView 登记表")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		entries, err := ListWebViews(appID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"app_id": appID, "total": len(entries), "entries": entries}, nil

	case "sdui.webview.validate":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以校验 WebView url_key")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		urlKey, _ := args["url_key"].(string)
		entry, err := ValidateWebViewKey(appID, urlKey)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"app_id": appID, "valid": true, "entry": entry}, nil

	case "sdui.capability.validate":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以校验页面能力")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		readDraft := true
		if value, ok := args["draft"].(bool); ok {
			readDraft = value
		}
		blocksJSON := ""
		if readDraft {
			if draft, draftErr := m.sduiService.FindRawDraft(appID, pageID); draftErr == nil {
				blocksJSON = draft.Blocks
			}
		}
		if blocksJSON == "" {
			page, pageErr := m.sduiService.GetRawPage(appID, pageID)
			if pageErr != nil {
				return nil, pageErr
			}
			blocksJSON = page.Blocks
		}
		var blocks []models.BlockItem
		if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
			return nil, fmt.Errorf("页面 Blocks JSON 无效: %w", err)
		}
		report, err := ValidatePageCapabilities(appID, blocks)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"app_id": appID, "page_id": pageID, "report": report}, nil

	case "sdui.capability.configure":
		if !hasScope(scopes, "release") {
			return nil, errors.New("权限不足: 需要 release 权限以配置租户能力")
		}
		if confirmed, _ := args["confirmed"].(bool); !confirmed {
			return nil, errors.New("能力配置门禁拦截: 必须人工确认并传入 confirmed=true")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		capability, _ := args["capability"].(string)
		configRaw, _ := args["config"].(map[string]interface{})
		encoded, _ := json.Marshal(configRaw)
		var entry models.CapabilityMatrixEntry
		if err := json.Unmarshal(encoded, &entry); err != nil {
			return nil, fmt.Errorf("能力配置无效: %w", err)
		}
		if err := ConfigureAppCapability(appID, capability, entry, actorID); err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "configured", "app_id": appID, "capability": capability, "config": entry}, nil

	case "sdui.page.list":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以查询页面列表")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pages, err := m.sduiService.ListPages(appID)
		if err != nil {
			return nil, fmt.Errorf("查询页面列表失败: %w", err)
		}
		return map[string]interface{}{"app_id": appID, "total": len(pages), "pages": pages}, nil

	case "sdui.page.get":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以读取页面协议")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		if strings.TrimSpace(pageID) == "" {
			return nil, errors.New("page_id 为必填参数")
		}
		readDraft := true
		if value, ok := args["draft"].(bool); ok {
			readDraft = value
		}
		if readDraft {
			if draft, draftErr := m.sduiService.FindRawDraft(appID, pageID); draftErr == nil {
				return map[string]interface{}{"source": "draft", "page": draft, "protocol": mcpPageProtocol(draft)}, nil
			}
		}
		page, err := m.sduiService.GetRawPage(appID, pageID)
		if err != nil {
			return nil, fmt.Errorf("读取页面失败: %w", err)
		}
		return map[string]interface{}{"source": "published", "page": page, "protocol": mcpPageProtocol(page)}, nil

	case "sdui.file.prepare_upload":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以上传图片")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		fileSize, ok := mcpIntArg(args["file_size"])
		if !ok || fileSize <= 0 || fileSize > 10*1024*1024 {
			return nil, errors.New("file_size 必须为 1 到 10485760 的整数")
		}
		fileName, _ := args["file_name"].(string)
		contentType, _ := args["content_type"].(string)
		ownerType, _ := args["owner_type"].(string)
		result, err := PrepareCOSUpload(context.Background(), COSUploadRequest{AppID: appID, FileName: fileName, FileSize: int64(fileSize), ContentType: contentType, OwnerType: ownerType})
		if err != nil {
			return nil, err
		}
		return result, nil

	case "sdui.page.create":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以创建页面草稿")
		}
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		templateID, _ := args["template_id"].(string)
		title, _ := args["title"].(string)

		if tenantID != "" {
			if appID == "" {
				appID = tenantID
			} else if appID != tenantID {
				return nil, fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可跨租户创建 %s 页面", tenantID, appID)
			}
		}

		if appID == "" || pageID == "" {
			return nil, errors.New("app_id 和 page_id 为必填项")
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID = strings.TrimSpace(pageID)
		if pageID == "" {
			return nil, errors.New("page_id 为必填参数")
		}
		if db.Mysql != nil {
			if _, err := m.sduiService.FindRawDraft(appID, pageID); err == nil {
				return nil, errors.New("page_id 已存在草稿；请先调用 sdui.page.get，再使用 sdui.page.patch 修改")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("检查页面草稿失败: %w", err)
			}
			if _, err := m.sduiService.GetRawPage(appID, pageID); err == nil {
				return nil, errors.New("page_id 已存在已发布页面；请先调用 sdui.page.get，再使用 sdui.page.patch 修改")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("检查已发布页面失败: %w", err)
			}
		}

		var page *models.DynamicPage
		var err error
		if strings.TrimSpace(templateID) == "" {
			page = &models.DynamicPage{AppID: appID, PageID: pageID, Title: title, BusinessType: "custom", Theme: "dark_glass", Blocks: "[]"}
			if page.Title == "" {
				page.Title = pageID
			}
		} else {
			page, err = m.templateService.ApplyTemplateToPage(templateID, appID, pageID, title)
			if err != nil {
				return nil, err
			}
		}
		// AI 创建页面严格存入草稿表，绝对不污染线上已发布页面
		draft := models.DynamicPageDraft{
			AppID:        page.AppID,
			PageID:       page.PageID,
			Revision:     1,
			Status:       "draft",
			Hidden:       page.Hidden,
			Title:        page.Title,
			BusinessType: page.BusinessType,
			Intent:       page.Intent,
			Theme:        page.Theme,
			AccentColor:  page.AccentColor,
			RequireAuth:  page.RequireAuth,
			ShareConfig:  page.ShareConfig,
			Blocks:       page.Blocks,
			Keyword:      page.Keyword,
			Source:       page.Source,
			CampaignID:   page.CampaignID,
			UpdatedBy:    actorID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if db.Mysql != nil {
			if err := m.sduiService.SaveDraftWithAudit(&draft, actorID, 0); err != nil {
				return nil, fmt.Errorf("保存页面草稿失败: %w", err)
			}
		}
		return map[string]interface{}{
			"draft_id": fmt.Sprintf("%s:%s", appID, pageID),
			"draft":    draft,
			"protocol": mcpPageProtocol(&draft),
			"revision": draft.Revision,
			"status":   draft.Status,
		}, nil

	case "sdui.page.patch":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以应用局部补丁")
		}
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)

		var err error
		appID, err = resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		if _, err := m.sduiService.FindRawDraft(appID, pageID); err != nil {
			return nil, errors.New("未找到草稿，请先调用 sdui.page.create")
		}

		rawOps, ok := args["ops"]
		if !ok || appID == "" || pageID == "" {
			return nil, errors.New("app_id, page_id 和 ops 均为必填参数")
		}
		opsBytes, err := json.Marshal(rawOps)
		if err != nil {
			return nil, fmt.Errorf("ops 格式无效: %w", err)
		}
		var ops []PatchOp
		if err := json.Unmarshal(opsBytes, &ops); err != nil {
			return nil, fmt.Errorf("解析 ops 补丁集失败: %w", err)
		}
		expectedRevision, ok := mcpIntArg(args["expected_revision"])
		if !ok || expectedRevision <= 0 {
			return nil, errors.New("expected_revision 必须为 page.get 返回的正整数草稿版本")
		}

		// 执行受控打草稿补丁 (严格限制在草稿表，杜绝未经 release 权限篡改已发布页面)
		patchedDraft, err := PatchDynamicPageDraftWithRevisionBy(appID, pageID, expectedRevision, actorID, ops)
		if err != nil {
			return nil, fmt.Errorf("补丁应用失败: %w", err)
		}

		// 构造临时 DynamicPage 运行校验并输出机器可读反馈
		tempPage := &models.DynamicPage{
			AppID:        patchedDraft.AppID,
			PageID:       patchedDraft.PageID,
			Revision:     patchedDraft.Revision,
			Status:       patchedDraft.Status,
			Hidden:       patchedDraft.Hidden,
			Title:        patchedDraft.Title,
			BusinessType: patchedDraft.BusinessType,
			Intent:       patchedDraft.Intent,
			Theme:        patchedDraft.Theme,
			AccentColor:  patchedDraft.AccentColor,
			RequireAuth:  patchedDraft.RequireAuth,
			ShareConfig:  patchedDraft.ShareConfig,
			Blocks:       patchedDraft.Blocks,
		}
		report := ValidateDynamicPageForRelease(tempPage)

		return map[string]interface{}{
			"app_id":          appID,
			"page_id":         pageID,
			"revision":        patchedDraft.Revision,
			"status":          patchedDraft.Status,
			"validation":      report,
			"applied_ops_num": len(ops),
		}, nil

	case "sdui.page.validate":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以校验协议")
		}

		// 优先支持直接传入页面协议对象；否则按 app_id + page_id 读取页面，protocol 保留 JSON 字符串兼容。
		if pageRaw, ok := args["page"]; ok && pageRaw != nil {
			directPage, err := decodeMCPPage(pageRaw)
			if err != nil {
				return nil, fmt.Errorf("page 参数格式无效: %w", err)
			}
			if err := applyMCPPageTenant(directPage, args, tenantID); err != nil {
				return nil, err
			}
			if db.Mysql != nil {
				if err := ensureMCPAppExists(directPage.AppID); err != nil {
					return nil, err
				}
			}
			return ValidateDynamicPageForRelease(directPage), nil
		}
		if protocolStr, ok := args["protocol"].(string); ok && strings.TrimSpace(protocolStr) != "" {
			directPage, err := decodeMCPPage(json.RawMessage(protocolStr))
			if err != nil {
				return nil, fmt.Errorf("protocol JSON 格式无效: %w", err)
			}
			if err := applyMCPPageTenant(directPage, args, tenantID); err != nil {
				return nil, err
			}
			if db.Mysql != nil {
				if err := ensureMCPAppExists(directPage.AppID); err != nil {
					return nil, err
				}
			}
			return ValidateDynamicPageForRelease(directPage), nil
		}

		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		if strings.TrimSpace(pageID) == "" {
			return nil, errors.New("page_id 为必填参数；或提供 page/protocol 直接校验")
		}

		var err error
		appID, err = resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}

		var page *models.DynamicPage
		if db.Mysql != nil {
			// 优先校验正在编辑的草稿
			if draft, err := m.sduiService.FindRawDraft(appID, pageID); err == nil && draft != nil {
				page = &models.DynamicPage{
					AppID:        draft.AppID,
					PageID:       draft.PageID,
					Revision:     draft.Revision,
					Status:       draft.Status,
					Hidden:       draft.Hidden,
					Title:        draft.Title,
					BusinessType: draft.BusinessType,
					Intent:       draft.Intent,
					Theme:        draft.Theme,
					AccentColor:  draft.AccentColor,
					RequireAuth:  draft.RequireAuth,
					ShareConfig:  draft.ShareConfig,
					Blocks:       draft.Blocks,
				}
			} else if p, err := m.sduiService.GetRawPage(appID, pageID); err == nil {
				page = p
			} else {
				return nil, fmt.Errorf("未找到目标页面或草稿: %w", err)
			}
		} else {
			page = &models.DynamicPage{
				AppID:  appID,
				PageID: pageID,
				Blocks: "[]",
			}
		}

		report := ValidateDynamicPageForRelease(page)
		return report, nil

	case "sdui.page.preview":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以预览信封")
		}
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)

		var err error
		appID, err = resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}

		queryMap := make(map[string]string)
		if q, ok := args["query"].(map[string]interface{}); ok {
			for k, v := range q {
				queryMap[k] = fmt.Sprintf("%v", v)
			}
		}

		var previewPage *models.DynamicPage
		if draft, draftErr := m.sduiService.FindRawDraft(appID, pageID); draftErr == nil {
			previewPage = &models.DynamicPage{AppID: draft.AppID, PageID: draft.PageID, Revision: draft.Revision, Status: draft.Status, Hidden: draft.Hidden, Title: draft.Title, BusinessType: draft.BusinessType, Intent: draft.Intent, Theme: draft.Theme, AccentColor: draft.AccentColor, RequireAuth: draft.RequireAuth, ShareConfig: draft.ShareConfig, Blocks: draft.Blocks, Keyword: draft.Keyword, Source: draft.Source, CampaignID: draft.CampaignID, ExpiresAt: draft.ExpiresAt}
		} else if page, pageErr := m.sduiService.GetRawPage(appID, pageID); pageErr == nil {
			previewPage = page
		} else {
			return nil, fmt.Errorf("装配草稿预览信封失败: 未找到页面或草稿")
		}
		envelope, err := m.sduiService.AssembleEnvelope(previewPage, queryMap, "draft_preview")
		if err != nil {
			return nil, fmt.Errorf("装配草稿预览信封失败: %w", err)
		}
		// MCP 预览必须与小程序正式读取共享 AI 破甲业务数据，避免预览只有空组件壳。
		m.sduiService.attachAIBreakthroughData(envelope, appID, false, 0)
		return envelope, nil

	case "sdui.page.screenshot":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以执行截图分析")
		}
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)

		var err error
		appID, err = resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}

		var targetPage *models.DynamicPage
		if db.Mysql != nil {
			// 优先消费当前草稿
			if draft, err := m.sduiService.FindRawDraft(appID, pageID); err == nil && draft != nil {
				targetPage = &models.DynamicPage{
					AppID:        draft.AppID,
					PageID:       draft.PageID,
					Revision:     draft.Revision,
					Status:       draft.Status,
					Hidden:       draft.Hidden,
					Title:        draft.Title,
					BusinessType: draft.BusinessType,
					Intent:       draft.Intent,
					Theme:        draft.Theme,
					AccentColor:  draft.AccentColor,
					RequireAuth:  draft.RequireAuth,
					ShareConfig:  draft.ShareConfig,
					Blocks:       draft.Blocks,
				}
			} else if p, err := m.sduiService.GetRawPage(appID, pageID); err == nil {
				targetPage = p
			}
		}
		if targetPage == nil {
			return nil, errors.New("未找到目标页面或草稿，无法生成截图")
		}

		// 1. 消费受控 device、theme 与 locale 参数；当前 Layout IR 文案基线固定为 zh-CN
		deviceName, _ := args["device"].(string)
		deviceParams := ResolveDeviceParams(deviceName)

		if overrideTheme, ok := args["theme"].(string); ok && strings.TrimSpace(overrideTheme) != "" {
			targetPage.Theme = strings.TrimSpace(overrideTheme)
		}

		locale, _ := args["locale"].(string)
		if locale == "" {
			locale = "zh-CN"
		}

		// 构建同构布局中间表示 (Layout IR)，包含真实边界框与设备参数
		stateFixture, _ := args["state"].(string)
		if stateFixture == "" {
			stateFixture = "normal"
		}

		// MCP 与 HTTP 草稿截图共同消费同一渲染入口，确保签名 URL 返回的字节与哈希严格一致。
		pngBytes, layoutIR, err := m.shareCardService.RenderPageLayoutIRScreenshot(targetPage, deviceParams.Name, stateFixture, targetPage.Theme)
		if err != nil {
			return nil, fmt.Errorf("构建布局 IR 或截图渲染失败: %w", err)
		}

		hasher := sha256.New()
		hasher.Write(pngBytes)
		imgHash := hex.EncodeToString(hasher.Sum(nil))

		// 签发 2 小时有效期的安全访问签名凭证 (防草稿内容匿名遍历窃取)
		expires := time.Now().Add(2 * time.Hour).Unix()
		sign := GenerateScreenshotSignatureWithOptions(appID, pageID, imgHash, expires, deviceParams.Name, targetPage.Theme, stateFixture)
		host, _ := args["host"].(string)
		if strings.TrimSpace(host) == "" && config.Cfg != nil {
			host = config.Cfg.PublicBaseURL
		}
		host = strings.TrimRight(strings.TrimSpace(host), "/")
		if host == "" {
			return nil, errors.New("截图需要配置 PUBLIC_BASE_URL 或传入 HTTPS host")
		}
		if host != "" {
			parsed, parseErr := url.Parse(host)
			if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				return nil, errors.New("host 必须是无查询参数的 HTTPS 根地址")
			}
		}
		signedImageURL := fmt.Sprintf("%s/api/v1/sdui/screenshot?app_id=%s&page_id=%s&draft=true&device=%s&theme=%s&state=%s&hash=%s&expires=%d&sign=%s", host, appID, pageID, url.QueryEscape(deviceParams.Name), url.QueryEscape(targetPage.Theme), url.QueryEscape(stateFixture), imgHash, expires, sign)

		// 提取积木组件层级树 structure_tree
		var blocks []models.BlockItem
		if targetPage.Blocks != "" {
			_ = json.Unmarshal([]byte(targetPage.Blocks), &blocks)
		}
		structureTree := make([]map[string]interface{}, 0, len(blocks))
		for idx, b := range blocks {
			node := map[string]interface{}{
				"index": idx,
				"id":    b.ID,
				"type":  b.Type,
				"props": b.Props,
			}
			if b.VisibleWhen != nil {
				node["visible_when"] = b.VisibleWhen
			}
			if b.Action != nil {
				node["action"] = b.Action.Type
			}
			if b.Events != nil {
				node["events"] = b.Events
			}
			structureTree = append(structureTree, node)
		}

		// 运行静态校验评估视觉与协议规范 issues
		valReport := ValidateDynamicPageForRelease(targetPage)
		issues := make([]map[string]string, 0)
		for _, e := range valReport.Errors {
			issues = append(issues, map[string]string{
				"level":   "error",
				"message": e,
			})
		}
		for _, w := range valReport.Warnings {
			issues = append(issues, map[string]string{
				"level":   "warning",
				"message": w,
			})
		}

		return map[string]interface{}{
			"image_hash":      imgHash,
			"byte_size":       len(pngBytes),
			"image_url":       signedImageURL,
			"device":          deviceParams.Name,
			"theme":           targetPage.Theme,
			"locale":          locale,
			"revision":        targetPage.Revision,
			"render_engine":   "layout_ir_isomorphic_compositor_v2",
			"visual_baseline": "apple_hig_dark_glass",
			"layout_ir":       layoutIR,
			"structure_tree":  structureTree,
			"native_stubs":    layoutIR.NativeStubs,
			"issues":          issues,
		}, nil

	case "sdui.page.publish":
		// 1. 权限范围检查: 必须具备 release 权限
		if !hasScope(scopes, "release") {
			return nil, errors.New("权限不足: 当前操作者未被授予 release 权限，禁止发布页面到线上")
		}

		// 2. 人工显式确认门禁: 必须显式传 confirmed: true
		confirmed, ok := args["confirmed"].(bool)
		if !ok || !confirmed {
			return nil, errors.New("发布门禁拦截: 必须由人工审核通过并在参数中显式确认 confirmed: true 方可发布生效")
		}

		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)

		var err error
		appID, err = resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}

		remark, _ := args["remark"].(string)
		if remark == "" {
			remark = fmt.Sprintf("由 %s 显式确认发布", actorID)
		}
		expectedRevision, ok := mcpIntArg(args["expected_revision"])
		if !ok || expectedRevision <= 0 {
			return nil, errors.New("expected_revision 必须为人工审查时 page.get 返回的正整数草稿版本")
		}

		if db.Mysql == nil {
			return nil, errors.New("无数据库环境，无法执行持久化发布")
		}

		// 调用草稿发布流转至线上 dynamic_pages 表并沉淀版本快照
		publishedPage, err := m.sduiService.PublishDraftWithRevision(appID, pageID, actorID, remark, expectedRevision)
		if err != nil {
			return nil, fmt.Errorf("发布页面失败: %w", err)
		}

		return map[string]interface{}{
			"status":       "published",
			"revision":     publishedPage.Revision,
			"actor_id":     actorID,
			"remark":       remark,
			"published_at": time.Now().Format(time.RFC3339),
		}, nil

	case "sdui.page.revisions":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以查询历史版本")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		revisions, err := m.sduiService.ListPageRevisions(appID, pageID)
		if err != nil {
			return nil, fmt.Errorf("查询历史版本失败: %w", err)
		}
		protocolRevisions := make([]map[string]interface{}, 0, len(revisions))
		for _, revision := range revisions {
			protocolRevisions = append(protocolRevisions, map[string]interface{}{"id": revision.ID, "revision": revision.Revision, "title": revision.Title, "hidden": revision.Hidden, "business_type": revision.BusinessType, "intent": revision.Intent, "theme": revision.Theme, "accent_color": revision.AccentColor, "require_auth": revision.RequireAuth, "keyword": revision.Keyword, "source": revision.Source, "campaign_id": revision.CampaignID, "expires_at": revision.ExpiresAt, "remark": revision.Remark, "created_by": revision.CreatedBy, "created_at": revision.CreatedAt, "protocol": mcpPageProtocol(&revision)})
		}
		return map[string]interface{}{"app_id": appID, "page_id": pageID, "total": len(protocolRevisions), "revisions": protocolRevisions}, nil

	case "sdui.page.rollback":
		if !hasScope(scopes, "release") {
			return nil, errors.New("权限不足: 需要 release 权限以回滚页面")
		}
		if confirmed, _ := args["confirmed"].(bool); !confirmed {
			return nil, errors.New("回滚门禁拦截: 必须由人工审核并传入 confirmed=true")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		targetRevision, ok := mcpIntArg(args["target_revision"])
		if !ok || strings.TrimSpace(pageID) == "" || targetRevision <= 0 {
			return nil, errors.New("page_id 和正整数 target_revision 为必填参数")
		}
		page, err := m.sduiService.RollbackPageRevision(appID, pageID, targetRevision)
		if err != nil {
			return nil, fmt.Errorf("回滚页面失败: %w", err)
		}
		return map[string]interface{}{"status": "published", "app_id": appID, "page_id": pageID, "revision": page.Revision, "rolled_back_from": targetRevision}, nil

	case "sdui.page.set_current":
		if !hasScope(scopes, "release") {
			return nil, errors.New("权限不足: 需要 release 权限以设置当前主页")
		}
		if confirmed, _ := args["confirmed"].(bool); !confirmed {
			return nil, errors.New("设置主页门禁拦截: 必须由人工审核并传入 confirmed=true")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		page, err := m.sduiService.GetRawPage(appID, pageID)
		if err != nil {
			return nil, fmt.Errorf("目标主页不存在: %w", err)
		}
		if page.Status != "published" || page.Hidden {
			return nil, errors.New("只有已发布且未隐藏的页面才能设置为当前主页")
		}
		if err := m.sduiService.SetCurrentPage(appID, pageID); err != nil {
			return nil, fmt.Errorf("设置当前主页失败: %w", err)
		}
		return map[string]interface{}{"status": "current", "app_id": appID, "page_id": pageID}, nil

	case "sdui.page.share_card":
		if !hasScope(scopes, "release") {
			return nil, errors.New("权限不足: 需要 release 权限以生成分享图")
		}
		if confirmed, _ := args["confirmed"].(bool); !confirmed {
			return nil, errors.New("分享图生成门禁拦截: 必须由人工审核并传入 confirmed=true")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		pageID, _ := args["page_id"].(string)
		host, _ := args["host"].(string)
		if strings.TrimSpace(host) == "" && config.Cfg != nil {
			host = config.Cfg.PublicBaseURL
		}
		if err := m.shareCardService.AutoUpdatePageShareConfig(appID, pageID, host); err != nil {
			return nil, fmt.Errorf("生成分享图失败: %w", err)
		}
		return map[string]interface{}{"status": "updated", "app_id": appID, "page_id": pageID, "host": strings.TrimRight(host, "/")}, nil

	case "sdui.operation.execute":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以执行通用能力动作")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		endpoint, _ := args["endpoint"].(string)
		if strings.TrimSpace(endpoint) == "" {
			return nil, errors.New("endpoint 为必填参数")
		}
		payload, _ := args["payload"].(map[string]interface{})
		if payload == nil {
			payload = map[string]interface{}{}
		}
		if id, ok := args["id"].(string); ok && strings.TrimSpace(id) != "" {
			payload["id"] = id
		}
		if endpoint != "game.redeem" && endpoint != "query.score" && endpoint != "ads.load" {
			return nil, errors.New("MCP 通用动作仅允许只读或游戏兑换端点；领域写操作必须经过对应工具和状态机")
		}
		result, err := NewActionEndpointService().ExecuteActionEndpoint(appID, "mcp_sandbox", endpoint, payload, fmt.Sprint(args["idempotency_key"]))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"app_id": appID, "endpoint": endpoint, "result": result}, nil

	case "sdui.payment.sandbox":
		if !hasScope(scopes, "write:draft") {
			return nil, errors.New("权限不足: 需要 write:draft 权限以执行支付沙箱")
		}
		appID, err := resolveMCPAppID(args, tenantID)
		if err != nil {
			return nil, err
		}
		if err := ensureMCPAppExists(appID); err != nil {
			return nil, err
		}
		operation, _ := args["operation"].(string)
		operation = strings.ToLower(strings.TrimSpace(operation))
		userID, ok := mcpIntArg(args["user_id"])
		if !ok || userID <= 0 {
			return nil, errors.New("user_id 必须为正整数")
		}
		service := NewPaymentService()
		if operation == "create" {
			sku, _ := args["sku"].(string)
			openID, _ := args["openid"].(string)
			if strings.TrimSpace(openID) == "" {
				openID = fmt.Sprintf("mcp-user-%d", userID)
			}
			order, err := service.CreateSandboxOrder(appID, int64(userID), openID, sku, fmt.Sprint(args["idempotency_key"]))
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"sandbox": true, "operation": operation, "order": order}, nil
		}
		tradeNo, _ := args["out_trade_no"].(string)
		if strings.TrimSpace(tradeNo) == "" {
			return nil, errors.New("out_trade_no 为必填参数")
		}
		order, err := service.ApplySandboxTransition(appID, int64(userID), tradeNo, operation)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"sandbox": true, "operation": operation, "order": order}, nil

	default:
		return nil, fmt.Errorf("未知 MCP 工具: %s", name)
	}
}

// mcpAuditf 在 Stdio 模式写入 stderr，避免诊断信息破坏 stdout 的 JSON-RPC 流；HTTP 模式沿用结构化日志文件。
func mcpAuditf(format string, args ...interface{}) {
	if os.Getenv("MCP_STDIO_MODE") == "1" {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
		return
	}
	logger.JM.Infof(format, args...)
}

// mcpIntArg 将 JSON 数字安全转换为整数参数。
func mcpIntArg(value interface{}) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), number == float64(int(number))
	case int:
		return number, true
	case int64:
		return int(number), int64(int(number)) == number
	default:
		return 0, false
	}
}

// decodeMCPPage 将 AI 友好的数组/对象协议转换为内部持久化模型。
func decodeMCPPage(value interface{}) (*models.DynamicPage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]interface{}
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	for _, key := range []string{"blocks", "share_config"} {
		if field, exists := object[key]; exists {
			if _, alreadyString := field.(string); !alreadyString {
				encoded, err := json.Marshal(field)
				if err != nil {
					return nil, fmt.Errorf("%s 序列化失败: %w", key, err)
				}
				object[key] = string(encoded)
			}
		}
	}
	raw, err = json.Marshal(object)
	if err != nil {
		return nil, err
	}
	var page models.DynamicPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// applyMCPPageTenant 校验直接协议中的租户与工具上下文一致。
func applyMCPPageTenant(page *models.DynamicPage, args map[string]interface{}, tenantID string) error {
	if page == nil {
		return errors.New("page 不能为空")
	}
	argumentAppID, _ := args["app_id"].(string)
	argumentAppID = strings.TrimSpace(argumentAppID)
	page.AppID = strings.TrimSpace(page.AppID)
	if page.AppID != "" && argumentAppID != "" && page.AppID != argumentAppID {
		return errors.New("page.app_id 与 arguments.app_id 不一致")
	}
	target := page.AppID
	if target == "" {
		target = argumentAppID
	}
	if target == "" {
		target = strings.TrimSpace(tenantID)
	}
	if target == "" {
		return errors.New("直接校验页面时必须提供 app_id")
	}
	if tenantID != "" && target != tenantID {
		return fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可校验 %s", tenantID, target)
	}
	page.AppID = target
	return nil
}

// mcpPageProtocol 返回适合 AI 修改的页面协议，隐藏 JSON 字符串存储细节。
func mcpPageProtocol(page interface{}) map[string]interface{} {
	raw, _ := json.Marshal(page)
	result := map[string]interface{}{}
	_ = json.Unmarshal(raw, &result)
	for _, key := range []string{"blocks", "share_config"} {
		if value, ok := result[key].(string); ok && strings.TrimSpace(value) != "" {
			var decoded interface{}
			if json.Unmarshal([]byte(value), &decoded) == nil {
				result[key] = decoded
			}
		} else if key == "blocks" && result[key] == nil {
			result[key] = []interface{}{}
		}
	}
	return result
}

// resolveMCPAppID 解析工具参数中的目标小程序，并兼容部署级租户白名单。
func resolveMCPAppID(args map[string]interface{}, tenantID string) (string, error) {
	appID, _ := args["app_id"].(string)
	appID = strings.TrimSpace(appID)
	if tenantID != "" {
		if appID == "" {
			appID = tenantID
		} else if appID != tenantID {
			return "", fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可访问 %s", tenantID, appID)
		}
	}
	if appID == "" {
		return "", errors.New("app_id 为必填参数")
	}
	return appID, nil
}

// resolveMCPOptionalAppID 解析可选的小程序参数；仅查询内置模板时允许不传 app_id。
func resolveMCPOptionalAppID(args map[string]interface{}, tenantID string) (string, error) {
	appID, _ := args["app_id"].(string)
	appID = strings.TrimSpace(appID)
	if tenantID == "" {
		return appID, nil
	}
	if appID != "" && appID != tenantID {
		return "", fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可访问 %s", tenantID, appID)
	}
	return tenantID, nil
}

// ensureMCPAppExists 确认 MCP 请求目标属于已注册小程序，防止对任意租户标识执行操作。
func ensureMCPAppExists(appID string) error {
	if allowed := mcpAllowedTenantSet(); len(allowed) > 0 && !allowed[appID] {
		return fmt.Errorf("小程序 %s 不在 MCP_ALLOWED_TENANTS 白名单中", appID)
	}
	if db.Mysql == nil {
		return errors.New("数据库未初始化，无法校验小程序")
	}
	var app models.MiniApp
	err := db.Mysql.Where("app_id = ?", appID).First(&app).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("小程序 %s 尚未注册", appID)
	}
	if err != nil {
		return fmt.Errorf("校验小程序 %s 失败: %w", appID, err)
	}
	return nil
}

// mcpAllowedTenantSet 读取部署级 MCP 小程序白名单。
func mcpAllowedTenantSet() map[string]bool {
	value := os.Getenv("MCP_ALLOWED_TENANTS")
	if value == "" {
		value = os.Getenv("MCP_TENANT_ID")
	}
	allowed := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			allowed[item] = true
		}
	}
	return allowed
}

// mcpAPIResource 返回供 AI 读取的 MCP 接口与自动编排说明。
func mcpAPIResource(tools []MCPToolDefinition) map[string]interface{} {
	return map[string]interface{}{
		"endpoint":  "/api/v1/mcp",
		"transport": "HTTP POST JSON-RPC 2.0",
		"transports": map[string]interface{}{
			"http":  map[string]interface{}{"method": "POST", "content_type": "application/json", "framing": "single JSON-RPC 2.0 object"},
			"stdio": map[string]interface{}{"command": "go run ./cmd/mcp-server", "framing": "newline-delimited JSON-RPC 2.0", "stdout": "responses only", "stderr": "startup diagnostics and audit logs"},
		},
		"jsonrpc": map[string]interface{}{
			"version":               "2.0",
			"request":               map[string]interface{}{"jsonrpc": "2.0", "id": "request-id", "method": "tools/call", "params": map[string]interface{}{"name": "sdui.page.get", "arguments": map[string]interface{}{}}},
			"success_response":      map[string]interface{}{"jsonrpc": "2.0", "id": "request-id", "result": "method result"},
			"error_response":        map[string]interface{}{"jsonrpc": "2.0", "id": "request-id", "error": map[string]interface{}{"code": -32602, "message": "Invalid params", "data": "optional"}},
			"notification_behavior": "notifications/initialized 和 notifications/cancelled 成功处理但不返回响应",
			"error_codes":           map[string]int{"parse_error": -32700, "invalid_request": -32600, "method_not_found": -32601, "invalid_params": -32602, "internal_error": -32603},
		},
		"tool_response": map[string]interface{}{
			"success": map[string]interface{}{"isError": false, "requestId": "uuid", "structuredContent": "tool result", "content": []map[string]string{{"type": "text", "text": "JSON encoded tool result"}}, "data": "tool result"},
			"error":   map[string]interface{}{"isError": true, "requestId": "uuid", "structuredContent": map[string]string{"code": "INVALID_ARGUMENT", "tool": "sdui.page.patch", "message": "human readable detail", "recovery": "machine actionable next step"}, "content": []map[string]string{{"type": "text", "text": "工具执行失败: ..."}}},
			"note":    "tools/call 的业务失败使用 result.isError=true；只有 JSON-RPC 协议层失败才使用顶层 error。",
		},
		"authentication":            map[string]interface{}{"header": "X-MCP-Key", "admin_alternative": "Authorization: Bearer <admin_jwt>"},
		"methods":                   []string{"initialize", "notifications/initialized", "ping", "resources/list", "resources/read", "tools/list", "tools/call"},
		"tools":                     tools,
		"scopes":                    map[string]string{"read": "读取模板、应用、页面并执行校验、预览、截图", "write:draft": "创建和修改草稿", "release": "发布、回滚、当前主页切换和分享图生成；均需 confirmed=true"},
		"global_token":              true,
		"app_id_rule":               "全局 Token 不绑定小程序；除仅读取内置模板外，每个工具必须在 arguments 中显式提供已注册 app_id；tenant-bound 凭证不能跨租户。",
		"workflow":                  []string{"读取 sdui://rules 与 sdui://api", "sdui.app.list 选择 app_id", "sdui.capability.list 读取租户能力矩阵", "sdui.page.list/get 避免重复创建", "sdui.acceptance.run 生成只读验收清单", "sdui.template.list/get 选择最接近的模板", "需要新模板时调用 sdui.template.save", "sdui.page.create 创建不存在的草稿", "每轮先 page.get 取得 revision，再 page.patch(expected_revision)", "page.validate -> capability.validate -> page.preview -> page.screenshot；逐项修复 errors/issues", "人工核对实际 revision 后 page.publish(expected_revision, confirmed=true)", "需要时 page.share_card 与 page.set_current", "故障时 page.revisions 后人工确认 rollback"},
		"ai_breakthrough_workflow":  []string{"门户使用 tpl_ai_breakthrough_portal", "文章详情使用 tpl_ai_breakthrough_article", "会员页使用 tpl_ai_breakthrough_membership", "文章/VIP/评论数据由同源 HTTP 业务接口装配，MCP 只负责编排 SDUI 模板和页面", "商品金额、商户密钥、文章发布和评论后台审核不属于 SDUI MCP 权限边界"},
		"ai_breakthrough_templates": map[string]interface{}{"portal": "tpl_ai_breakthrough_portal", "article_detail": "tpl_ai_breakthrough_article", "membership": "tpl_ai_breakthrough_membership", "article_access_fields": []string{"can_read", "markdown", "preview_markdown", "locked_reason", "locked_title", "unlock_label", "pay_sku"}, "membership_levels": []map[string]interface{}{{"level": 1, "name": "普通会员", "price_yuan": 9.9, "duration_days": 30}, {"level": 2, "name": "高级会员", "price_yuan": 29.9, "duration_days": 90}, {"level": 3, "name": "年度会员", "price_yuan": 99, "duration_days": 365}}, "rule": "RequiredLevel <= 当前有效会员等级可读；IsPaid=true 时无 ArticlePurchase 只能读取 FreeMarkdown"},
		"ai_breakthrough_http": map[string]interface{}{
			"tenant_header": "X-WX-AppID",
			"public": []map[string]interface{}{
				{"method": "GET", "path": "/api/v1/article-categories", "purpose": "读取已启用资讯栏目"},
				{"method": "GET", "path": "/api/v1/articles", "purpose": "读取文章摘要；支持 category_id、limit、offset"},
				{"method": "GET", "path": "/api/v1/articles/{article_id}", "purpose": "读取文章详情；服务端裁剪会员正文和单篇付费正文"},
				{"method": "GET", "path": "/api/v1/membership/plans", "purpose": "读取启用的会员套餐和展示价格"},
				{"method": "GET", "path": "/api/v1/articles/{article_id}/comments", "purpose": "读取一级评论；只返回审核通过内容"},
				{"method": "GET", "path": "/api/v1/comments/{comment_id}/replies", "purpose": "点击后读取二级回复；只返回审核通过内容"},
			},
			"authenticated": []map[string]interface{}{
				{"method": "POST", "path": "/api/v1/auth/wechat-login", "purpose": "以微信 code 建立会话"},
				{"method": "GET", "path": "/api/v1/membership/me", "purpose": "读取当前用户会员权益", "requires": "Authorization: Bearer"},
				{"method": "POST", "path": "/api/v1/membership/orders", "purpose": "按会员 SKU 创建支付订单", "requires": "Authorization: Bearer"},
				{"method": "POST", "path": "/api/v1/articles/{article_id}/comments", "purpose": "提交评论或二级回复；服务端调用微信内容安全审核", "requires": "Authorization: Bearer"},
				{"method": "POST", "path": "/api/v1/payment/orders", "purpose": "按单篇文章或会员 SKU 创建支付订单", "requires": "Authorization: Bearer"},
			},
			"rules": []string{"文章正文权限只能由服务端返回字段 can_read、markdown、preview_markdown、locked_reason 决定", "MCP 不得把完整付费正文写入页面协议或模板", "前端动作只传 SKU，不传金额；金额由商品表读取", "评论图片必须先上传当前租户 COS CDN，再提交 image_url；服务端审核状态为 pending 时不可公开显示"},
		},
		"coverage":                  map[string]string{"app_selection": "sdui.app.list", "capability_inspection": "sdui.capability.list", "capability_validation": "sdui.capability.validate", "capability_configuration": "sdui.capability.configure", "acceptance_checklist": "sdui.acceptance.run", "webview_registry": "sdui.webview.list + sdui.webview.validate", "template_inspection": "sdui.template.list + sdui.template.get", "template_save": "sdui.template.save", "template_delete": "sdui.template.delete", "draft_creation": "sdui.page.create", "draft_editing": "sdui.page.patch", "validation": "sdui.page.validate", "preview": "sdui.page.preview", "visual_review": "sdui.page.screenshot", "image_upload": "sdui.file.prepare_upload", "publish": "sdui.page.publish", "history": "sdui.page.revisions", "rollback": "sdui.page.rollback", "homepage_activation": "sdui.page.set_current", "share_assets": "sdui.page.share_card"},
		"supported_runtime_actions": sortedMapKeys(allowedActionTypes),
		"unsupported_or_admin_only": []string{"管理员账号与权限管理", "微信 AppSecret 与支付私钥配置", "商品和金额配置", "AI 破甲文章/栏目/会员配置/评论审核管理", "数据库迁移与种子数据", "任意 HTTP 代理或任意脚本执行", "直接上传二进制到 MCP（必须使用预签名 COS PUT）"},
		"coverage_note":             "MCP 完整覆盖 SDUI 页面与模板编排，不等同于覆盖全部业务后台。MCP 与 HTTP 管理接口复用 TemplateService、SDUIService、协议校验和截图服务；业务内容及敏感配置继续由受认证管理接口处理。",
		"publish_gate":              map[string]interface{}{"required_scope": "release", "required_argument": "confirmed=true", "human_review": true},
		"image_upload":              map[string]interface{}{"mcp_tool": "sdui.file.prepare_upload", "admin_endpoint": "/api/v1/admin/files/presigned-upload-url", "flow": []string{"调用工具申请预签名 PUT 地址", "调用方按 uploadHeaders 直接 PUT 二进制到 presignedUrl", "将 finalCosFileUrl 写入 page.patch 的图片字段"}, "prefix": "miniapps/{app_id}/", "stored_value": "CDN URL only", "acl": "由 COS 控制台 miniapps/* 规则统一管理"},
	}
}

// mcpRulesResource 返回 SDUI 协议的机器可读规则。
func mcpRulesResource() map[string]interface{} {
	return map[string]interface{}{
		"protocol_version": "1.1",
		"schema_version":   3,
		"block_types":      sortedMapKeys(allowedBlockTypes),
		"action_types":     sortedMapKeys(allowedActionTypes),
		"capabilities":     ListCapabilityDefinitions(),
		"style_utilities":  sortedMapKeys(allowedStyleUtilities),
		"block_item_shape": map[string]interface{}{
			"required": []string{"id", "type"},
			"fields":   []string{"id", "type", "props", "style", "action", "events", "visible_when", "repeat", "loading", "empty", "error", "fallback"},
			"style":    map[string]interface{}{"utilities": "string[]，只能使用 style_utilities", "glass_blur": "boolean"},
			"events":   "对象，键为事件名（通常 tap/change/submit），值为 BlockAction[]；动作按数组顺序执行",
			"states":   "loading/empty/error/fallback 均为完整 BlockItem，id 必须在页面树中全局唯一",
		},
		"action_shape": map[string]interface{}{
			"required": []string{"type"},
			"fields":   []string{"type", "require_auth", "condition", "confirm", "on_success", "on_error", "track", "endpoint", "url", "payload"},
			"chain":    "on_success/on_error 为 BlockAction[]，按顺序执行；禁止脚本、任意 URL、任意请求头和凭证字段；open_webview 必须使用当前 AppID 已登记的 payload.url_key",
		},
		"block_contracts":     mcpBlockContracts(),
		"action_contracts":    mcpActionContracts(),
		"condition_operators": []string{"eq", "neq", "in", "exists", "gt", "gte", "lt", "lte", "and", "or", "not"},
		"condition_shape":     map[string]interface{}{"comparison": "{\"eq\": [{\"path\": \"$entity.value\"}, true]}", "logic": "{\"and\": [{...}, {...}]}", "path_prefixes": []string{"$entity", "$query", "$item", "$state", "$result", "$page", "$session", "$tenant", "$props"}},
		"binding_scopes":      []string{"$entity", "$query", "$item", "$state", "$result", "$page", "$session", "$tenant", "$props"},
		"block_capabilities":  []string{"visible_when", "repeat", "loading", "empty", "error", "fallback", "events"},
		"action_capabilities": []string{"condition", "confirm", "on_success", "on_error", "track", "payload"},
		"page_fields":         map[string]interface{}{"required": []string{"app_id", "page_id", "title", "business_type", "blocks"}, "business_type": []string{"drama", "game", "query", "download", "custom", "ai_breakthrough", "ai_article"}, "intent": []string{"watch", "redeem", "query", "download", "buy", "book", "join"}, "theme": []string{"dark_glass", "light_clean", "cyber_neon"}, "status": []string{"draft", "published", "archived", "reviewing"}, "hidden": "boolean；true 时公开路由、导航和首页目标均不可访问", "authoring_shape": "在 MCP page 参数中 blocks 使用 BlockItem 数组、share_config 使用对象；page.get 额外返回同形态 protocol。数据库 JSON 字符串属于内部存储细节。WebView 动作必须使用 payload.url_key，不能直接传 URL。"},
		"patch_operations":    []string{"replace: path 支持 /title、/theme、/accent_color、/business_type、/intent、/require_auth、/hidden、/share_config、/keyword、/source、/campaign_id、/expires_at、/blocks 或 /blocks/{block_id}", "add_block: value 为完整且 ID 唯一的 BlockItem", "remove_block: value 为已有积木 ID", "所有 patch 必须提交 page.get 返回的 expected_revision"},
		"request_data_rules":  []string{"优先使用已登记 endpoint", "自定义 URL 只能是同源相对路径", "禁止任意 Authorization、Cookie、内网地址和脚本", "修改/删除请求必须确认并具备幂等策略"},
		"webview_rules":       []string{"open_webview 只能使用当前 AppID 已登记且启用的 url_key", "登记地址必须是无查询参数的 HTTPS 地址", "生产环境仍需配置微信业务域名白名单"},
		"image_rules":         []string{"后台图片必须通过预签名 PUT 上传", "对象路径固定使用 miniapps/{app_id}/ 前缀", "页面协议只保存 CDN URL", "禁止第三方示例图片 URL"},
		"state_rules":         []string{"AI 默认只写 draft", "page.create 不覆盖已有 page_id", "patch 和 publish 必须绑定人工实际读取/审查的 expected_revision", "校验通过后再 preview/screenshot", "publish 必须具备 release 且 confirmed=true", "不允许下发任意脚本"},
		"quality_gate":        []string{"validate.is_valid 必须为 true", "逐项处理 screenshot.issues 中 error", "检查 normal/loading/empty/error/offline/expired/unauthenticated 状态", "检查长标题、缺图、空数组和未知块 fallback", "确认动作所需 payload、登录态、成功链和失败链", "发布前重新 page.get 并由人工确认相同 revision"},
		"examples": map[string]interface{}{
			"page":      map[string]interface{}{"app_id": "wxexample", "page_id": "home", "protocol_version": "1.1", "schema_version": 3, "blocks": []map[string]interface{}{{"id": "hero", "type": "text", "props": map[string]interface{}{"text": "标题"}}}},
			"action":    map[string]interface{}{"type": "copy_text", "payload": map[string]interface{}{"text": "复制内容"}},
			"condition": map[string]interface{}{"and": []interface{}{map[string]interface{}{"eq": []interface{}{map[string]interface{}{"path": "$state.logged_in"}, true}}}},
		},
	}
}

func sortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sanitizeMCPArgs 对记录日志的 MCP 参数进行安全脱敏，防止凭据或大规模载荷泄漏
func sanitizeMCPArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return map[string]interface{}{}
	}
	sanitized := make(map[string]interface{})
	for k, v := range args {
		lowerK := strings.ToLower(k)
		if strings.Contains(lowerK, "secret") || strings.Contains(lowerK, "password") || strings.Contains(lowerK, "token") || strings.Contains(lowerK, "key") {
			sanitized[k] = "******"
		} else if nested, ok := v.(map[string]interface{}); ok {
			sanitized[k] = sanitizeMCPArgs(nested)
		} else if str, ok := v.(string); ok && len(str) > 120 {
			sanitized[k] = str[:120] + "...(截断)"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}
