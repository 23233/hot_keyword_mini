// Package services mcp_token.go
package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

var allowedMCPTokenScopes = map[string]bool{
	"read":        true,
	"write:draft": true,
	"release":     true,
}

// CreateMCPAccessToken 创建全局 MCP 访问令牌，明文仅返回一次。
func CreateMCPAccessToken(name string, scopes []string) (*models.MCPAccessToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", errors.New("令牌名称不能为空")
	}

	cleanScopes := normalizeMCPScopes(scopes)
	if len(cleanScopes) == 0 {
		return nil, "", errors.New("至少选择一个 MCP 权限")
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, "", err
	}
	rawToken := "mcp_" + hex.EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(rawToken))
	now := time.Now()
	record := &models.MCPAccessToken{
		Name:        name,
		TokenHash:   hex.EncodeToString(hash[:]),
		TokenPrefix: rawToken[:12],
		AppID:       "",
		Scopes:      strings.Join(cleanScopes, ","),
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Mysql.Create(record).Error; err != nil {
		return nil, "", err
	}
	return record, rawToken, nil
}

// ListMCPAccessTokens 查询全局 MCP 令牌。
func ListMCPAccessTokens() ([]models.MCPAccessToken, error) {
	var list []models.MCPAccessToken
	err := db.Mysql.Order("id desc").Find(&list).Error
	return list, err
}

// DeleteMCPAccessToken 删除指定 MCP 令牌。
func DeleteMCPAccessToken(id int64) error {
	return db.Mysql.Delete(&models.MCPAccessToken{}, id).Error
}

// AuthenticateMCPAccessToken 校验数据库 MCP 令牌并更新最后使用时间。
func AuthenticateMCPAccessToken(rawToken string) (*models.MCPAccessToken, error) {
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化")
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	var record models.MCPAccessToken
	err := db.Mysql.Where("token_hash = ? AND status = 'active'", hex.EncodeToString(hash[:])).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("MCP 访问令牌无效")
		}
		return nil, err
	}
	now := time.Now()
	_ = db.Mysql.Model(&record).Update("last_used_at", now).Error
	record.LastUsedAt = &now
	return &record, nil
}

// ParseMCPTokenScopes 解析数据库保存的 MCP 权限范围。
func ParseMCPTokenScopes(value string) []string {
	return normalizeMCPScopes(strings.Split(value, ","))
}

func normalizeMCPScopes(scopes []string) []string {
	result := make([]string, 0, len(scopes))
	seen := make(map[string]bool)
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if allowedMCPTokenScopes[scope] && !seen[scope] {
			seen[scope] = true
			result = append(result, scope)
		}
	}
	return result
}
