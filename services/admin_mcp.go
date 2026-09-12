// Package services admin_mcp.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

// AdminMCPOperations 是 MCP 可代理的管理后台业务操作白名单。
// 管理员用户和 MCP Token 永远不在此列表中。
var AdminMCPOperations = []string{
	"app.get", "app.save", "capability.get", "capability.save", "webview.get", "webview.save",
	"product.list", "product.save", "category.list", "category.save", "category.archive",
	"membership.list", "membership.save", "membership.archive", "article.list", "article.save", "article.archive",
	"comment.list", "comment.moderate", "operation.list", "config.get", "config.save", "drama.list", "drama.save", "ai_breakthrough.seed",
}

var adminMCPReadOperations = map[string]bool{
	"app.get": true, "capability.get": true, "webview.get": true, "product.list": true,
	"category.list": true, "membership.list": true, "article.list": true, "comment.list": true,
	"operation.list": true, "config.get": true, "drama.list": true,
}

// ExecuteAdminMCPOperation 通过统一服务层代理管理后台操作，不接受任意 SQL、URL 或脚本。
func ExecuteAdminMCPOperation(actor, appID, operation string, payload map[string]interface{}) (interface{}, error) {
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(appID) == "" {
		return nil, errors.New("操作人和 app_id 不能为空")
	}
	if operation == "admin.create" || operation == "admin.update" || operation == "admin.delete" || strings.HasPrefix(operation, "mcp_token.") {
		return nil, errors.New("MCP 禁止管理员账号和 MCP Token 管理操作")
	}
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化")
	}
	if !containsString(AdminMCPOperations, operation) {
		return nil, fmt.Errorf("MCP 未登记管理操作: %s", operation)
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if !adminMCPReadOperations[operation] && payload["confirmed"] != true {
		return nil, errors.New("管理写操作必须传入 confirmed=true 二次确认")
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else if errors.Is(err, gorm.ErrRecordNotFound) && operation != "app.save" {
		return nil, errors.New("小程序不存在")
	}
	switch operation {
	case "app.get":
		return safeMiniApp(&app), nil
	case "app.save":
		var input models.MiniApp
		if err := decodePayload(payload, &input); err != nil {
			return nil, err
		}
		input.AppID = appID
		if err := NewSDUIService().SaveApp(&input); err != nil {
			return nil, err
		}
		var saved models.MiniApp
		if err := db.Mysql.Where("app_id = ?", appID).First(&saved).Error; err != nil {
			return nil, err
		}
		return safeMiniApp(&saved), nil
	case "capability.get":
		return ListAppCapabilities(appID)
	case "capability.save":
		capability := textPayload(payload, "capability")
		var entry models.CapabilityMatrixEntry
		if err := decodePayload(payload["config"], &entry); err != nil {
			return nil, err
		}
		if err := ConfigureAppCapability(appID, capability, entry, actor); err != nil {
			return nil, err
		}
		return entry, nil
	case "webview.get":
		return ListWebViews(appID)
	case "webview.save":
		var entries []WebViewEntry
		if err := decodePayload(payload["entries"], &entries); err != nil {
			return nil, err
		}
		if err := SaveWebViews(appID, entries); err != nil {
			return nil, err
		}
		return entries, nil
	case "product.list":
		var rows []models.Product
		err := db.Mysql.Where("app_id = ?", appID).Order("id asc").Find(&rows).Error
		return rows, err
	case "product.save":
		var row models.Product
		if err := decodePayload(payload, &row); err != nil {
			return nil, err
		}
		row.AppID = appID
		if row.SKU == "" || row.Name == "" || row.PriceFen <= 0 {
			return nil, errors.New("商品 SKU、名称和金额必须有效")
		}
		row.Status = normalizeStatus(row.Status)
		if err := db.Mysql.Where("app_id = ? AND sku = ?", appID, row.SKU).Assign(map[string]interface{}{"name": row.Name, "description": row.Description, "price_fen": row.PriceFen, "status": row.Status, "updated_at": time.Now()}).FirstOrCreate(&row).Error; err != nil {
			return nil, err
		}
		return row, nil
	case "category.list":
		var rows []models.ArticleCategory
		err := db.Mysql.Where("app_id = ?", appID).Order("sort asc,id asc").Find(&rows).Error
		return rows, err
	case "category.save":
		var row models.ArticleCategory
		if err := decodePayload(payload, &row); err != nil {
			return nil, err
		}
		row.AppID = appID
		if row.Slug == "" || row.Name == "" {
			return nil, errors.New("栏目标识和名称必须有效")
		}
		if err := db.Mysql.Where("app_id = ? AND slug = ?", appID, row.Slug).Assign(map[string]interface{}{"name": row.Name, "summary": row.Summary, "sort": row.Sort, "status": normalizeStatus(row.Status), "updated_at": time.Now()}).FirstOrCreate(&row).Error; err != nil {
			return nil, err
		}
		return row, nil
	case "category.archive":
		return archiveRow(appID, payload, &models.ArticleCategory{}, "栏目")
	case "membership.list":
		var rows []models.MembershipLevel
		err := db.Mysql.Where("app_id = ?", appID).Order("level asc").Find(&rows).Error
		return rows, err
	case "membership.save":
		var row models.MembershipLevel
		if err := decodePayload(payload, &row); err != nil {
			return nil, err
		}
		row.AppID = appID
		if err := NewMembershipService().SavePlan(&row); err != nil {
			return nil, err
		}
		return row, nil
	case "membership.archive":
		return archiveRow(appID, payload, &models.MembershipLevel{}, "会员等级")
	case "article.list":
		var rows []models.Article
		err := db.Mysql.Where("app_id = ?", appID).Order("id desc").Find(&rows).Error
		return rows, err
	case "article.save":
		var row models.Article
		if err := decodePayload(payload, &row); err != nil {
			return nil, err
		}
		row.AppID = appID
		if err := NewArticleService().SaveArticle(&row); err != nil {
			return nil, err
		}
		return row, nil
	case "article.archive":
		return archiveRow(appID, payload, &models.Article{}, "文章")
	case "comment.list":
		var rows []models.ArticleComment
		err := db.Mysql.Where("app_id = ?", appID).Order("id desc").Find(&rows).Error
		return rows, err
	case "comment.moderate":
		id, ok := numericID(payload["id"])
		if !ok || id <= 0 {
			return nil, errors.New("评论 id 无效")
		}
		if err := NewCommentService().Moderate(appID, id, textPayload(payload, "status"), textPayload(payload, "reason")); err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "updated", "id": id}, nil
	case "operation.list":
		var rows []models.PlatformOperation
		err := db.Mysql.Where("app_id = ?", appID).Order("updated_at desc").Limit(100).Find(&rows).Error
		return rows, err
	case "config.get":
		return getAdminConfig()
	case "config.save":
		return saveAdminConfig(payload)
	case "ai_breakthrough.seed":
		if err := NewMembershipService().SeedDefaultPlans(appID); err != nil {
			return nil, err
		}
		if err := NewArticleService().SeedDefaultContent(appID); err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "seeded", "app_id": appID}, nil
	case "drama.list":
		var rows []models.Drama
		err := db.Mysql.Order("id desc").Find(&rows).Error
		return rows, err
	case "drama.save":
		var row models.Drama
		if err := decodePayload(payload, &row); err != nil {
			return nil, err
		}
		if row.ID <= 0 {
			return nil, errors.New("短剧 id 无效")
		}
		updates := map[string]interface{}{"title": row.Title, "subtitle": row.Subtitle, "cover_url": row.CoverUrl, "banner_url": row.BannerUrl, "total_episodes": row.TotalEpisodes, "rating": row.Rating, "hot_score": row.HotScore, "tags": row.Tags, "description": row.Description, "highlights": row.Highlights, "play_mode": row.PlayMode, "finder_user_name": row.FinderUserName, "channels_feed_id": row.ChannelsFeedID, "web_url": row.WebUrl, "updated_at": time.Now()}
		if err := db.Mysql.Model(&models.Drama{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		return row, nil
	}
	return nil, errors.New("管理操作执行失败")
}

func safeMiniApp(app *models.MiniApp) map[string]interface{} {
	raw, _ := json.Marshal(app)
	var result map[string]interface{}
	_ = json.Unmarshal(raw, &result)
	return result
}
func decodePayload(value interface{}, target interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("管理操作参数无效: %w", err)
	}
	return nil
}
func textPayload(payload map[string]interface{}, key string) string {
	v, _ := payload[key].(string)
	return strings.TrimSpace(v)
}
func normalizeStatus(value string) string {
	if value == "inactive" {
		return value
	}
	return "active"
}

// archiveRow 统一执行可恢复的后台停用操作。
func archiveRow(app string, payload map[string]interface{}, model interface{}, label string) (interface{}, error) {
	id, ok := numericID(payload["id"])
	if !ok || id <= 0 {
		return nil, fmt.Errorf("%s id 无效", label)
	}
	status := "inactive"
	if label == "文章" {
		status = "archived"
	}
	result := db.Mysql.Model(model).Where("app_id = ? AND id = ?", app, id).Update("status", status)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return map[string]interface{}{"status": status, "id": id}, nil
}

func numericID(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), v == float64(int64(v))
	case int:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		i, err := v.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

type adminConfigPayload struct {
	DisplayMode    string                 `json:"display_mode"`
	WebviewURL     string                 `json:"webview_url"`
	PageTitle      string                 `json:"page_title"`
	PageSubtitle   string                 `json:"page_subtitle"`
	Announcement   string                 `json:"announcement"`
	ShareTitle     string                 `json:"share_title"`
	ShareDesc      string                 `json:"share_desc"`
	ShareCover     string                 `json:"share_cover"`
	ActionChannels []models.ActionChannel `json:"action_channels"`
	FloatingButton *models.FloatingButton `json:"floating_button"`
}

func getAdminConfig() (interface{}, error) {
	drama, err := NewDramaService().GetDefaultDrama()
	if err != nil {
		return nil, err
	}
	var cfg models.PageConfig
	if err := db.Mysql.Where("drama_id = ?", drama.ID).First(&cfg).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"drama": drama, "config": cfg}, nil
}

func saveAdminConfig(payload map[string]interface{}) (interface{}, error) {
	var input adminConfigPayload
	if err := decodePayload(payload, &input); err != nil {
		return nil, err
	}
	drama, err := NewDramaService().GetDefaultDrama()
	if err != nil {
		return nil, err
	}
	channels, _ := json.Marshal(input.ActionChannels)
	updates := map[string]interface{}{"display_mode": input.DisplayMode, "webview_url": input.WebviewURL, "page_title": input.PageTitle, "page_subtitle": input.PageSubtitle, "announcement": input.Announcement, "share_title": input.ShareTitle, "share_desc": input.ShareDesc, "share_cover": input.ShareCover, "action_channels": string(channels), "updated_at": time.Now()}
	if input.FloatingButton != nil {
		raw, _ := json.Marshal(input.FloatingButton)
		updates["floating_button"] = string(raw)
	}
	if result := db.Mysql.Model(&models.PageConfig{}).Where("drama_id = ?", drama.ID).Updates(updates); result.Error != nil {
		return nil, result.Error
	}
	return getAdminConfig()
}
