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
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
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
				"type": "object",
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
				"type": "object",
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
				"type": "object",
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
				"type": "object",
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
				"type": "object",
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
			Description: "显式发布已通过强校验的页面草稿，将其置为 published 并沉淀版本快照",
			InputSchema: map[string]interface{}{
				"type": "object",
				"required": []string{"app_id", "page_id"},
				"properties": map[string]interface{}{
					"app_id":  map[string]interface{}{"type": "string"},
					"page_id": map[string]interface{}{"type": "string"},
					"remark":  map[string]interface{}{"type": "string", "description": "发布审计备注"},
				},
			},
		},
	}
}

// HandleJSONRPC 处理标准 JSON-RPC 2.0 请求并返回响应二进制
func (m *MCPService) HandleJSONRPC(reqBytes []byte) ([]byte, error) {
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

		result, err := m.ExecuteTool(callParams.Name, callParams.Arguments)
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

// ExecuteTool 执行指定的 MCP 受控工具
func (m *MCPService) ExecuteTool(name string, args map[string]interface{}) (interface{}, error) {
	if args == nil {
		args = make(map[string]interface{})
	}

	switch name {
	case "sdui.template.list":
		businessType, _ := args["business_type"].(string)
		templates := m.templateRegistry.ListTemplates(businessType)
		return map[string]interface{}{
			"total":     len(templates),
			"templates": templates,
		}, nil

	case "sdui.page.create":
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		templateID, _ := args["template_id"].(string)
		title, _ := args["title"].(string)
		if appID == "" || pageID == "" || templateID == "" {
			return nil, errors.New("app_id, page_id 和 template_id 为必填项")
		}

		page, err := m.templateRegistry.ApplyTemplateToPage(templateID, appID, pageID, title)
		if err != nil {
			return nil, err
		}
		page.Status = "draft" // AI 初始创建默认置为草稿状态，杜绝未经审批覆盖生产
		if db.Mysql != nil {
			_ = m.sduiService.SavePage(page)
		}
		return map[string]interface{}{
			"draft_id": fmt.Sprintf("%s:%s", appID, pageID),
			"page":     page,
			"revision": page.Revision,
			"status":   page.Status,
		}, nil

	case "sdui.page.patch":
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
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

		patchedPage, err := PatchDynamicPage(appID, pageID, ops)
		if err != nil {
			return nil, err
		}
		// 打补丁后执行一次强校验并附带诊断报告
		report := ValidateDynamicPage(patchedPage)
		return map[string]interface{}{
			"page":     patchedPage,
			"revision": patchedPage.Revision,
			"report":   report,
		}, nil

	case "sdui.page.validate":
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		if appID == "" || pageID == "" {
			return nil, errors.New("app_id 与 page_id 不能为空")
		}

		var page *models.DynamicPage
		if db.Mysql != nil {
			p, err := m.sduiService.GetRawPage(appID, pageID)
			if err != nil {
				return nil, fmt.Errorf("未找到目标页面: %w", err)
			}
			page = p
		} else {
			page = &models.DynamicPage{
				AppID:        appID,
				PageID:       pageID,
				Title:        "测试页面",
				BusinessType: "drama",
				Blocks:       "[]",
			}
		}

		report := ValidateDynamicPage(page)
		return report, nil

	case "sdui.page.preview":
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		var queryMap map[string]string
		if q, ok := args["query"].(map[string]interface{}); ok {
			queryMap = make(map[string]string)
			for k, v := range q {
				queryMap[k] = fmt.Sprint(v)
			}
		}

		envelope, err := m.sduiService.GetDynamicPageEnvelope(appID, pageID, queryMap)
		if err != nil {
			return nil, fmt.Errorf("装配预览信封失败: %w", err)
		}
		return envelope, nil

	case "sdui.page.screenshot":
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		cardType, _ := args["card_type"].(string)
		if cardType == "" {
			cardType = "app_message"
		}

		pngBytes, err := m.shareCardService.RenderShareCard(appID, pageID, cardType)
		if err != nil {
			return nil, fmt.Errorf("截图渲染失败: %w", err)
		}

		hasher := sha256.New()
		hasher.Write(pngBytes)
		imgHash := hex.EncodeToString(hasher.Sum(nil))

		return map[string]interface{}{
			"card_type":       cardType,
			"image_hash":      imgHash,
			"byte_size":       len(pngBytes),
			"image_url":       fmt.Sprintf("/api/v1/share/card?app_id=%s&page_id=%s&type=%s", appID, pageID, cardType),
			"render_engine":   "pure_go_layer_compositor_v1",
			"visual_baseline": "apple_hig_dark_glass",
		}, nil

	case "sdui.page.publish":
		appID, _ := args["app_id"].(string)
		pageID, _ := args["page_id"].(string)
		remark, _ := args["remark"].(string)
		if remark == "" {
			remark = "AI MCP 显式确认发布"
		}

		var page *models.DynamicPage
		if db.Mysql != nil {
			p, err := m.sduiService.GetRawPage(appID, pageID)
			if err != nil {
				return nil, fmt.Errorf("未找到目标页面: %w", err)
			}
			page = p
		} else {
			return nil, errors.New("无数据库环境，无法执行持久化发布")
		}

		// 发布前门禁检查: 必须通过强校验且无 fatal error
		report := ValidateDynamicPage(page)
		if !report.IsValid {
			return nil, fmt.Errorf("页面协议校验未通过，发布被阻断: %s", strings.Join(report.Errors, "; "))
		}

		page.Status = "published"
		if err := m.sduiService.SavePage(page); err != nil {
			return nil, fmt.Errorf("发布持久化失败: %w", err)
		}

		return map[string]interface{}{
			"status":      "published",
			"revision":    page.Revision,
			"remark":      remark,
			"published_at": time.Now().Format(time.RFC3339),
		}, nil

	default:
		return nil, fmt.Errorf("未知 MCP 工具: %s", name)
	}
}
