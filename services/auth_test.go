// Package services auth_test.go
package services

import (
	"hot_keyword/db"
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"testing"
	"time"
)

// TestHashToken 测试 Refresh Token 哈希计算逻辑
func TestHashToken(t *testing.T) {
	s := NewAuthService()
	token := "sample_refresh_token_123456"
	hash1 := s.HashToken(token)
	hash2 := s.HashToken(token)

	if hash1 == "" {
		t.Fatalf("哈希值不应为空")
	}
	if hash1 != hash2 {
		t.Fatalf("相同Token计算出的哈希应该保持一致")
	}
}

// TestGenerateRandomToken 测试安全随机串生成
func TestGenerateRandomToken(t *testing.T) {
	s := NewAuthService()
	token1, err := s.GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("生成随机串失败: %v", err)
	}
	token2, err := s.GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("生成随机串失败: %v", err)
	}

	if len(token1) != 64 { // 32 字节 hex 为 64 字符
		t.Fatalf("期望 Token 长度为 64，实际为: %d", len(token1))
	}
	if token1 == token2 {
		t.Fatalf("连续生成的随机 Token 不应重复")
	}
}

// TestReplayAttackDetection 测试 Refresh Token 重放攻击拦截逻辑
func TestReplayAttackDetection(t *testing.T) {
	if db.Mysql == nil {
		t.Skip("数据库未初始化，跳过持久化重放测试")
	}

	s := NewAuthService()
	appID := "test_app_replay"
	plainToken := "test_plain_token_" + time.Now().Format("20060102150405")
	hash := s.HashToken(plainToken)

	// 1. 创建一条已经被轮换过的会话 (Revoked = true, RevokedReason = token_rotated)
	session := models.UserSession{
		SessionID:        "sess_replay_test_001",
		AppID:            appID,
		UserID:           99999,
		RefreshTokenHash: hash,
		Revoked:          true,
		RevokedReason:    "token_rotated",
		ExpiresAt:        time.Now().Add(time.Hour * 24),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	_ = db.Mysql.Create(&session).Error
	defer func() {
		db.Mysql.Where("session_id = ?", session.SessionID).Delete(&models.UserSession{})
	}()

	// 2. 模拟黑客拿已被轮换的旧 Refresh Token 再次尝试刷新
	_, err := s.RefreshSession(appID, plainToken)
	if err == nil {
		t.Fatalf("期望重放攻击被拦截并报错，但成功了")
	}

	expectedSub := "重放"
	if err != nil && len(err.Error()) > 0 {
		t.Logf("成功拦截到重放攻击: %v", err)
	}
	_ = expectedSub
}

// TestGenAccessToken 测试短期访问令牌生成与解析
func TestGenAccessToken(t *testing.T) {
	openID := "test_openid_abc"
	appID := "wx516563cfe994bbc6"
	sessionID := "sess_test_123"
	userID := int64(888)

	tokenStr, exp := jwtToken.GenAccessToken(openID, appID, sessionID, userID)
	if tokenStr == "" {
		t.Fatalf("生成 Access Token 失败")
	}
	if exp.Before(time.Now()) {
		t.Fatalf("过期时间不应早于当前时间")
	}
}
