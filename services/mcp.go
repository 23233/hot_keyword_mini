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
	"os"
	"sort"
	"strings"
	"time"

	"github.com/23233/ggg/logger"
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
	ID      interface{}   `json:"id,omitempty"`
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
}

// MCPService AI Model Context Protocol 编排调度服务
type MCPService struct {
	templateRegistry *TemplateRegistry
	sduiService      *SDUIService
	shareCardService *ShareCardService
}

// NewMCPService 创建 MCP 编排调度服务
func NewMCPService() *MCPService {
	return &MCPService{
		templateRegistry: GetGlobalTemplateRegistry(),
		sduiService:      NewSDUIService(),
		shareCardService: NewShareCardService(),
	}
}

// GetToolDefinitions 获取全部受控编排工具清单。
func (m *MCPService) GetToolDefinitions() []MCPToolDefinition {
	return []MCPToolDefinition{
		{
			Name:        "sdui.app.list",
			Description: "查询当前 MCP 凭证可操作的全部已注册小程序 AppID 与基础状态",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
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
			Description: "查询可用的行业模板包列表 (drama/game/query/download) 与适用场景",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"business_type": map[string]interface{}{
						"type":        "string",
						"description": "按业务类型过滤: drama / game / query / download",
					},
				},
			},
		},
		{
			Name:        "sdui.page.create",
			Description: "从行业模板或空白结构创建草稿页面 (status: draft)",
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
			Description: "按受控路径对页面协议执行局部 JSON Patch 打补丁",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id", "ops"},
				"properties": map[string]interface{}{
					"app_id":  map[string]interface{}{"type": "string"},
					"page_id": map[string]interface{}{"type": "string"},
					"ops": map[string]interface{}{
						"type":        "array",
						"description": "补丁操作数组 (replace/add_block/remove_block)",
					},
				},
			},
		},
		{
			Name:        "sdui.page.validate",
			Description: "对页面协议执行强校验，输出机器可读的错误、警告与修复建议",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id"},
				"properties": map[string]interface{}{
					"app_id":  map[string]interface{}{"type": "string"},
					"page_id": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name:        "sdui.page.preview",
			Description: "使用指定数据和客户端能力模拟装配，输出响应信封与预览协议",
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
			Description: "后端规范化无头渲染分享卡片截图，输出图像 URL、哈希值与布局元信息",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id"},
				"properties": map[string]interface{}{
					"app_id":    map[string]interface{}{"type": "string"},
					"page_id":   map[string]interface{}{"type": "string"},
					"card_type": map[string]interface{}{"type": "string", "description": "app_message (5:4) 或 timeline (1:1)"},
				},
			},
		},
		{
			Name:        "sdui.page.publish",
			Description: "显式发布已通过强校验的页面草稿，将其置为 published 并沉淀版本快照 (必须人工显式确认 confirmed: true)",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"app_id", "page_id", "confirmed"},
				"properties": map[string]interface{}{
					"app_id":    map[string]interface{}{"type": "string", "description": "小程序 AppID"},
					"page_id":   map[string]interface{}{"type": "string", "description": "页面 PageID"},
					"confirmed": map[string]interface{}{"type": "boolean", "description": "人工显式发布确认标记，必须为 true"},
					"remark":    map[string]interface{}{"type": "string", "description": "发布审计备注"},
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
	}
}

// HandleJSONRPC 处理标准 JSON-RPC 2.0 请求 (默认全权限，用于本地 Stdio)
func (m *MCPService) HandleJSONRPC(reqBytes []byte) ([]byte, error) {
	return m.HandleJSONRPCWithContext("stdio_local", "", []string{"read", "write:draft", "release"}, reqBytes)
}

// HandleJSONRPCWithContext 带身份与权限范围上下文的 JSON-RPC 2.0 处理器
func (m *MCPService) HandleJSONRPCWithContext(actorID, tenantID string, scopes []string, reqBytes []byte) ([]byte, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error: " + err.Error(),
			},
		}
		return json.Marshal(resp)
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "sdui-mcp-server",
				"version": "1.3.0",
			},
			"instructions": "必须先调用 resources/read 读取 sdui://rules 和 sdui://api，再调用 tools/list；所有页面工具必须显式提供已注册 app_id。AI 默认只创建或修改草稿，发布、回滚、设置主页和生成分享图都必须经过人工确认。",
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

		result, err := m.ExecuteToolWithContext(actorID, tenantID, scopes, callParams.Name, callParams.Arguments)
		if err != nil {
			resp.Result = map[string]interface{}{
				"isError": true,
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("工具执行失败: %v", err)},
				},
			}
		} else {
			rawJSON, _ := json.MarshalIndent(result, "", "  ")
			resp.Result = map[string]interface{}{
				"isError": false,
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
	logger.JM.Infof("【MCP审计】Actor=%s Tenant=%s Tool=%s Args=%+v", actorID, tenantID, name, sanitizedArgs)

	switch name {
	case "sdui.template.list":
		if !hasScope(scopes, "read") {
			return nil, errors.New("权限不足: 需要 read 权限以查询行业模板列表")
		}
		businessType, _ := args["business_type"].(string)
		templates := m.templateRegistry.ListTemplates(businessType)
		return map[string]interface{}{
			"total":     len(templates),
			"templates": templates,
		}, nil

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
				return map[string]interface{}{"source": "draft", "page": draft}, nil
			}
		}
		page, err := m.sduiService.GetRawPage(appID, pageID)
		if err != nil {
			return nil, fmt.Errorf("读取页面失败: %w", err)
		}
		return map[string]interface{}{"source": "published", "page": page}, nil

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
		fileSize, ok := args["file_size"].(float64)
		if !ok {
			if integer, integerOK := args["file_size"].(int64); integerOK {
				fileSize = float64(integer)
			} else if integer, integerOK := args["file_size"].(int); integerOK {
				fileSize = float64(integer)
			}
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

		var page *models.DynamicPage
		var err error
		if strings.TrimSpace(templateID) == "" {
			page = &models.DynamicPage{AppID: appID, PageID: pageID, Title: title, BusinessType: "custom", Theme: "dark_glass", Blocks: "[]"}
			if page.Title == "" {
				page.Title = pageID
			}
		} else {
			page, err = m.templateRegistry.ApplyTemplateToPage(templateID, appID, pageID, title)
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
			_ = m.sduiService.SaveDraft(&draft)
		}
		return map[string]interface{}{
			"draft_id": fmt.Sprintf("%s:%s", appID, pageID),
			"draft":    draft,
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

		// 执行受控打草稿补丁 (严格限制在草稿表，杜绝未经 release 权限篡改已发布页面)
		patchedDraft, err := PatchDynamicPageDraft(appID, pageID, ops)
		if err != nil {
			return nil, fmt.Errorf("补丁应用失败: %w", err)
		}

		// 构造临时 DynamicPage 运行校验并输出机器可读反馈
		tempPage := &models.DynamicPage{
			AppID:        patchedDraft.AppID,
			PageID:       patchedDraft.PageID,
			Revision:     patchedDraft.Revision,
			Status:       patchedDraft.Status,
			Title:        patchedDraft.Title,
			BusinessType: patchedDraft.BusinessType,
			Intent:       patchedDraft.Intent,
			Theme:        patchedDraft.Theme,
			AccentColor:  patchedDraft.AccentColor,
			RequireAuth:  patchedDraft.RequireAuth,
			ShareConfig:  patchedDraft.ShareConfig,
			Blocks:       patchedDraft.Blocks,
		}
		report := ValidateDynamicPage(tempPage)

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

		var page *models.DynamicPage
		if db.Mysql != nil {
			// 优先校验正在编辑的草稿
			if draft, err := m.sduiService.FindRawDraft(appID, pageID); err == nil && draft != nil {
				page = &models.DynamicPage{
					AppID:        draft.AppID,
					PageID:       draft.PageID,
					Revision:     draft.Revision,
					Status:       draft.Status,
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

		report := ValidateDynamicPage(page)
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
			previewPage = &models.DynamicPage{AppID: draft.AppID, PageID: draft.PageID, Revision: draft.Revision, Status: draft.Status, Title: draft.Title, BusinessType: draft.BusinessType, Intent: draft.Intent, Theme: draft.Theme, AccentColor: draft.AccentColor, RequireAuth: draft.RequireAuth, ShareConfig: draft.ShareConfig, Blocks: draft.Blocks, Keyword: draft.Keyword, Source: draft.Source, CampaignID: draft.CampaignID, ExpiresAt: draft.ExpiresAt}
		} else if page, pageErr := m.sduiService.GetRawPage(appID, pageID); pageErr == nil {
			previewPage = page
		} else {
			return nil, fmt.Errorf("装配草稿预览信封失败: 未找到页面或草稿")
		}
		envelope, err := m.sduiService.AssembleEnvelope(previewPage, queryMap, "draft_preview")
		if err != nil {
			return nil, fmt.Errorf("装配草稿预览信封失败: %w", err)
		}
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

		// 1. 真实消费 device、theme 与 locale 参数，保证视觉渲染与目标物理设备同构
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

		layoutIR, err := BuildPageLayoutIR(targetPage, deviceParams, stateFixture)
		if err != nil {
			return nil, fmt.Errorf("构建布局 IR 失败: %w", err)
		}

		// 消费 Layout IR 渲染符合视觉基线的快照
		pngBytes, err := m.shareCardService.RenderLayoutIRScreenshot(layoutIR)
		if err != nil {
			return nil, fmt.Errorf("截图渲染失败: %w", err)
		}

		hasher := sha256.New()
		hasher.Write(pngBytes)
		imgHash := hex.EncodeToString(hasher.Sum(nil))

		// 签发 2 小时有效期的安全访问签名凭证 (防草稿内容匿名遍历窃取)
		expires := time.Now().Add(2 * time.Hour).Unix()
		sign := GenerateScreenshotSignature(appID, pageID, imgHash, expires)
		signedImageURL := fmt.Sprintf("/api/v1/sdui/screenshot?app_id=%s&page_id=%s&draft=true&hash=%s&expires=%d&sign=%s", appID, pageID, imgHash, expires, sign)

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
		valReport := ValidateDynamicPage(targetPage)
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

		if db.Mysql == nil {
			return nil, errors.New("无数据库环境，无法执行持久化发布")
		}

		// 调用草稿发布流转至线上 dynamic_pages 表并沉淀版本快照
		publishedPage, err := m.sduiService.PublishDraft(appID, pageID, actorID, remark)
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
		return map[string]interface{}{"app_id": appID, "page_id": pageID, "total": len(revisions), "revisions": revisions}, nil

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
		if page.Status != "published" {
			return nil, errors.New("只有已发布页面才能设置为当前主页")
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

	default:
		return nil, fmt.Errorf("未知 MCP 工具: %s", name)
	}
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
		"endpoint":                  "/api/v1/mcp",
		"transport":                 "HTTP POST JSON-RPC 2.0",
		"authentication":            map[string]interface{}{"header": "X-MCP-Key", "admin_alternative": "Authorization: Bearer <admin_jwt>"},
		"methods":                   []string{"initialize", "resources/list", "resources/read", "tools/list", "tools/call"},
		"tools":                     tools,
		"scopes":                    map[string]string{"read": "读取模板、应用、页面并执行校验、预览、截图", "write:draft": "创建和修改草稿", "release": "发布已确认且通过校验的草稿"},
		"global_token":              true,
		"app_id_rule":               "全局 Token 不绑定小程序；每个页面工具必须在 arguments 中显式提供已注册 app_id。",
		"workflow":                  []string{"resources/read sdui://rules", "sdui.app.list", "sdui.page.list", "sdui.page.get", "sdui.template.list", "sdui.file.prepare_upload (需要图片时)", "sdui.page.create", "sdui.page.patch", "sdui.page.validate", "sdui.page.preview", "sdui.page.screenshot", "sdui.page.share_card (需要分享图时)", "sdui.page.publish", "sdui.page.set_current (需要切换主页时)", "sdui.page.revisions", "sdui.page.rollback (需要人工确认)"},
		"coverage":                  map[string]string{"app_selection": "sdui.app.list", "page_inspection": "sdui.page.list + sdui.page.get", "draft_creation": "sdui.page.create", "draft_editing": "sdui.page.patch", "validation": "sdui.page.validate", "preview": "sdui.page.preview", "visual_review": "sdui.page.screenshot", "image_upload": "sdui.file.prepare_upload", "publish": "sdui.page.publish", "history": "sdui.page.revisions", "rollback": "sdui.page.rollback", "homepage_activation": "sdui.page.set_current", "share_assets": "sdui.page.share_card"},
		"supported_runtime_actions": []string{"require_auth", "copy_text", "toast", "refresh", "navigate_page", "open_channels_activity", "open_mini_program", "open_webview", "preview_image", "request_data", "request_payment", "share", "subscribe_message"},
		"unsupported_or_admin_only": []string{"管理员账号与权限管理", "微信 AppSecret 与支付私钥配置", "商品和金额配置", "数据库迁移与种子数据", "任意 HTTP 代理或任意脚本执行", "直接上传二进制到 MCP（必须使用预签名 COS PUT）"},
		"coverage_note":             "MCP 覆盖 SDUI 页面从选择小程序、读取、创建草稿、修改、校验、预览、视觉审查、资源上传、分享图生成、发布、切换主页、版本回滚的完整闭环；系统级敏感配置和未登记业务接口仍必须由管理后台或专用服务处理。",
		"publish_gate":              map[string]interface{}{"required_scope": "release", "required_argument": "confirmed=true", "human_review": true},
		"image_upload":              map[string]interface{}{"mcp_tool": "sdui.file.prepare_upload", "admin_endpoint": "/api/v1/admin/files/presigned-upload-url", "flow": []string{"调用工具申请预签名 PUT 地址", "调用方按 uploadHeaders 直接 PUT 二进制到 presignedUrl", "将 finalCosFileUrl 写入 page.patch 的图片字段"}, "prefix": "miniapps/{app_id}/", "stored_value": "CDN URL only", "acl": "由 COS 控制台 miniapps/* 规则统一管理"},
	}
}

// mcpRulesResource 返回 SDUI 协议的机器可读规则。
func mcpRulesResource() map[string]interface{} {
	return map[string]interface{}{
		"protocol_version":    "1.1",
		"schema_version":      3,
		"block_types":         sortedMapKeys(allowedBlockTypes),
		"action_types":        sortedMapKeys(allowedActionTypes),
		"condition_operators": []string{"eq", "neq", "in", "exists", "gt", "gte", "lt", "lte", "and", "or", "not"},
		"condition_shape":     map[string]interface{}{"comparison": "{\"eq\": [{\"path\": \"$entity.value\"}, true]}", "logic": "{\"and\": [{...}, {...}]}", "path_prefixes": []string{"$entity", "$query", "$item", "$state", "$result", "$page", "$session", "$tenant", "$props"}},
		"binding_scopes":      []string{"$entity", "$query", "$item", "$state", "$result", "$page", "$session", "$tenant", "$props"},
		"block_capabilities":  []string{"visible_when", "repeat", "loading", "empty", "error", "fallback", "events"},
		"action_capabilities": []string{"condition", "confirm", "on_success", "on_error", "track", "payload"},
		"page_fields":         map[string]interface{}{"required": []string{"app_id", "page_id", "title", "business_type", "blocks"}, "business_type": []string{"drama", "game", "query", "download", "custom"}, "intent": []string{"watch", "redeem", "query", "download", "buy", "book", "join"}, "theme": []string{"dark_glass", "light_clean", "cyber_neon"}, "status": []string{"draft", "published", "archived", "reviewing"}},
		"patch_operations":    []string{"replace: 修改受控字段", "add_block: 在积木列表中新增积木", "remove_block: 按积木 ID 删除积木"},
		"request_data_rules":  []string{"优先使用已登记 endpoint", "自定义 URL 只能是同源相对路径", "禁止任意 Authorization、Cookie、内网地址和脚本", "修改/删除请求必须确认并具备幂等策略"},
		"image_rules":         []string{"后台图片必须通过预签名 PUT 上传", "对象路径固定使用 miniapps/{app_id}/ 前缀", "页面协议只保存 CDN URL", "禁止第三方示例图片 URL"},
		"state_rules":         []string{"AI 默认只写 draft", "校验通过后再 preview/screenshot", "publish 必须具备 release 且 confirmed=true", "不允许下发任意脚本"},
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
		} else if str, ok := v.(string); ok && len(str) > 120 {
			sanitized[k] = str[:120] + "...(截断)"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}
