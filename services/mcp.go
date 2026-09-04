// Package services mcp.go
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"time"

	"github.com/23233/ggg/logger"
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

// GetToolDefinitions 获取架构文档第 3.12 节规定的全部 7 个受控编排工具清单
func (m *MCPService) GetToolDefinitions() []MCPToolDefinition {
	return []MCPToolDefinition{
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
				"required": []string{"app_id", "page_id", "template_id"},
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
						"description": "所选模板ID, 如 tpl_game_redeem / tpl_drama_standard",
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
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{"listChanged": false},
			},
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

		if appID == "" || pageID == "" || templateID == "" {
			return nil, errors.New("app_id, page_id 和 template_id 为必填项")
		}

		page, err := m.templateRegistry.ApplyTemplateToPage(templateID, appID, pageID, title)
		if err != nil {
			return nil, err
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

		if tenantID != "" {
			if appID == "" {
				appID = tenantID
			} else if appID != tenantID {
				return nil, fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可跨租户打补丁 %s 页面", tenantID, appID)
			}
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

		// 强校验多租户隔离，杜绝跨租户读取协议校验
		if tenantID != "" {
			if appID == "" {
				appID = tenantID
			} else if appID != tenantID {
				return nil, fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可跨租户访问 %s 页面", tenantID, appID)
			}
		}

		var page *models.DynamicPage
		if db.Mysql != nil {
			// 优先校验正在编辑的草稿
			if draft, err := m.sduiService.GetRawDraft(appID, pageID); err == nil && draft != nil {
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

		// 强校验多租户隔离
		if tenantID != "" {
			if appID == "" {
				appID = tenantID
			} else if appID != tenantID {
				return nil, fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可跨租户预览 %s 页面", tenantID, appID)
			}
		}

		queryMap := make(map[string]string)
		if q, ok := args["query"].(map[string]interface{}); ok {
			for k, v := range q {
				queryMap[k] = fmt.Sprintf("%v", v)
			}
		}

		envelope, err := m.sduiService.GetDynamicDraftEnvelope(appID, pageID, queryMap)
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

		// 强校验多租户隔离
		if tenantID != "" {
			if appID == "" {
				appID = tenantID
			} else if appID != tenantID {
				return nil, fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可跨租户截图 %s 页面", tenantID, appID)
			}
		}

		var targetPage *models.DynamicPage
		if db.Mysql != nil {
			// 优先消费当前草稿
			if draft, err := m.sduiService.GetRawDraft(appID, pageID); err == nil && draft != nil {
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
			targetPage = &models.DynamicPage{
				AppID:        appID,
				PageID:       pageID,
				Title:        "精选热播",
				BusinessType: "drama",
			}
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

		if tenantID != "" {
			if appID == "" {
				appID = tenantID
			} else if appID != tenantID {
				return nil, fmt.Errorf("多租户越权拦截: 操作者绑定租户 %s，不可跨租户发布 %s 页面", tenantID, appID)
			}
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

	default:
		return nil, fmt.Errorf("未知 MCP 工具: %s", name)
	}
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
