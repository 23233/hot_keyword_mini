// Package services webview.go
package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/url"
	"strings"
	"time"
)

const webViewTicketTTL = 2 * time.Minute

// WebViewTicketService 为登录用户创建和消费一次性 WebView 票据。
type WebViewTicketService struct{}

// NewWebViewTicketService 创建 WebView 票据服务。
func NewWebViewTicketService() *WebViewTicketService { return &WebViewTicketService{} }

// CreateTicket 创建短期一次性票据并返回附带 ticket 参数的目标地址。
func (s *WebViewTicketService) CreateTicket(appID string, userID int64, targetURL string) (string, error) {
	if appID == "" || userID <= 0 {
		return "", errors.New("小程序租户和登录用户不能为空")
	}
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("WebView 地址必须是有效的 HTTP(S) 地址")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成 WebView 票据失败: %w", err)
	}
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	ticket := &models.WebViewTicket{
		TokenHash: hex.EncodeToString(hash[:]),
		AppID:     appID,
		UserID:    userID,
		TargetURL: targetURL,
		ExpiresAt: time.Now().Add(webViewTicketTTL),
		CreatedAt: time.Now(),
	}
	if db.Mysql == nil {
		return "", errors.New("数据库未连接，无法创建 WebView 票据")
	}
	if err := db.Mysql.Create(ticket).Error; err != nil {
		return "", fmt.Errorf("保存 WebView 票据失败: %w", err)
	}
	q := parsed.Query()
	q.Set("webview_ticket", token)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// ConsumeTicket 原子消费票据并返回关联用户和目标地址。
func (s *WebViewTicketService) ConsumeTicket(token string) (*models.WebViewTicket, error) {
	if strings.TrimSpace(token) == "" || db.Mysql == nil {
		return nil, errors.New("WebView 票据无效")
	}
	hash := sha256.Sum256([]byte(token))
	hashText := hex.EncodeToString(hash[:])
	now := time.Now()
	var ticket models.WebViewTicket
	tx := db.Mysql.Begin()
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", hashText, now).First(&ticket).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("WebView 票据不存在、已使用或已过期")
	}
	if err := tx.Model(&ticket).Updates(map[string]interface{}{"used_at": now}).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("WebView 票据消费失败")
	}
	if err := tx.Commit().Error; err != nil {
		return nil, errors.New("WebView 票据提交失败")
	}
	return &ticket, nil
}
