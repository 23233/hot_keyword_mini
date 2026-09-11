// Package services webview_registry.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/url"
	"regexp"
	"strings"
)

// WebViewEntry 描述一个按 AppID 隔离的已登记 WebView 入口。
type WebViewEntry struct {
	// 稳定入口键，页面协议只引用该键。
	URLKey string `json:"url_key"`
	// 已审核的 HTTPS 业务地址。
	URL string `json:"url"`
	// 业务用途说明。
	Purpose string `json:"purpose"`
	// 业务版本或审核版本。
	Version string `json:"version"`
	// 是否允许公开打开。
	Enabled bool `json:"enabled"`
}

var webViewKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)

// ValidateWebViewEntry 校验 WebView 登记项的键、地址和必填说明。
func ValidateWebViewEntry(entry WebViewEntry) error {
	if !webViewKeyPattern.MatchString(strings.TrimSpace(entry.URLKey)) {
		return errors.New("url_key 必须是 2-64 位小写字母、数字、下划线或短横线")
	}
	parsed, err := url.Parse(strings.TrimSpace(entry.URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("WebView URL 必须是无查询参数的 HTTPS 地址")
	}
	if strings.TrimSpace(entry.Purpose) == "" || strings.TrimSpace(entry.Version) == "" {
		return errors.New("WebView purpose 与 version 不能为空")
	}
	return nil
}

// normalizeWebViewEntry 统一登记字段的空白格式，避免同一入口因空格产生重复键。
func normalizeWebViewEntry(entry WebViewEntry) WebViewEntry {
	entry.URLKey = strings.TrimSpace(entry.URLKey)
	entry.URL = strings.TrimSpace(entry.URL)
	entry.Purpose = strings.TrimSpace(entry.Purpose)
	entry.Version = strings.TrimSpace(entry.Version)
	return entry
}

// parseWebViewRegistry 解析并校验存储中的 WebView 登记 JSON。
func parseWebViewRegistry(raw string) ([]WebViewEntry, error) {
	if strings.TrimSpace(raw) == "" {
		return []WebViewEntry{}, nil
	}
	var entries []WebViewEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("WebView 登记 JSON 无效: %w", err)
	}
	if err := validateWebViewEntries(entries); err != nil {
		return nil, err
	}
	for index := range entries {
		entries[index] = normalizeWebViewEntry(entries[index])
	}
	return entries, nil
}

// validateWebViewEntries 校验登记表中的每个入口并拒绝重复 url_key。
func validateWebViewEntries(entries []WebViewEntry) error {
	seen := map[string]bool{}
	for _, rawEntry := range entries {
		entry := normalizeWebViewEntry(rawEntry)
		if err := ValidateWebViewEntry(entry); err != nil {
			return err
		}
		if seen[entry.URLKey] {
			return fmt.Errorf("WebView url_key 重复: %s", entry.URLKey)
		}
		seen[entry.URLKey] = true
	}
	return nil
}

// findWebViewEntry 在已校验的登记表中查找可用入口，并统一处理下线和不存在状态。
func findWebViewEntry(entries []WebViewEntry, urlKey string) (WebViewEntry, error) {
	key := strings.TrimSpace(urlKey)
	for _, rawEntry := range entries {
		entry := normalizeWebViewEntry(rawEntry)
		if entry.URLKey != key {
			continue
		}
		if !entry.Enabled {
			return WebViewEntry{}, errors.New("WebView 入口已登记但当前已下线")
		}
		return entry, nil
	}
	return WebViewEntry{}, errors.New("WebView url_key 尚未登记")
}

// ResolveWebViewPayload 将动作中的 url_key 解析为已登记的受控 HTTPS 地址。
func ResolveWebViewPayload(appID string, payload map[string]interface{}) (map[string]interface{}, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, errors.New("AppID 不能为空")
	}
	if payload == nil {
		return nil, errors.New("WebView 动作 payload 不能为空")
	}
	urlKey, _ := payload["url_key"].(string)
	entry, err := ValidateWebViewKey(appID, urlKey)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]interface{}, len(payload)+2)
	for key, value := range payload {
		resolved[key] = value
	}
	resolved["url_key"] = entry.URLKey
	resolved["url"] = entry.URL
	return resolved, nil
}

// ListWebViews 返回指定 AppID 的 WebView 登记项。
func ListWebViews(appID string) ([]WebViewEntry, error) {
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
	return parseWebViewRegistry(app.WebViewRegistry)
}

// ValidateWebViewKey 校验指定 AppID 的 WebView url_key 是否已登记并启用。
func ValidateWebViewKey(appID, urlKey string) (WebViewEntry, error) {
	entries, err := ListWebViews(appID)
	if err != nil {
		return WebViewEntry{}, err
	}
	return findWebViewEntry(entries, urlKey)
}

// SaveWebViews 保存指定 AppID 的完整 WebView 登记表，由后台管理员调用。
func SaveWebViews(appID string, entries []WebViewEntry) error {
	if strings.TrimSpace(appID) == "" {
		return errors.New("AppID 不能为空")
	}
	for index := range entries {
		entries[index] = normalizeWebViewEntry(entries[index])
	}
	if err := validateWebViewEntries(entries); err != nil {
		return err
	}
	if db.Mysql == nil {
		return errors.New("数据库未初始化")
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return fmt.Errorf("读取小程序失败: %w", err)
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return db.Mysql.Model(&models.MiniApp{}).Where("app_id = ?", appID).Updates(map[string]interface{}{"webview_registry": string(encoded)}).Error
}
