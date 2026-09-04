// Package services auth.go
package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/jwtToken"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"time"

	"github.com/23233/ggg/logger"
	"github.com/23233/ggg/ut"
	"gorm.io/gorm"
)

// WechatLoginResponse 微信免密登录统一响应实体 (包含双 Token 与会话信息)
type WechatLoginResponse struct {
	// 短期访问令牌 (用于业务接口请求 Header)
	AccessToken string `json:"access_token"`
	// 访问令牌过期时间 (RFC3339)
	AccessExpiresAt string `json:"access_expires_at"`
	// 长期刷新令牌 (用于无感续期，客户端妥善存储)
	RefreshToken string `json:"refresh_token"`
	// 刷新令牌过期时间 (RFC3339)
	RefreshExpiresAt string `json:"refresh_expires_at"`
	// 会话全局唯一ID
	SessionID string `json:"session_id"`
	// 脱敏用户信息
	User *models.User `json:"user"`
}

// SessionInfoResponse 会话状态检查响应
type SessionInfoResponse struct {
	// 会话是否有效
	IsValid bool `json:"is_valid"`
	// 会话唯一ID
	SessionID string `json:"session_id"`
	// 所属小程序
	AppID string `json:"app_id"`
	// 过期时间
	ExpiresAt string `json:"expires_at"`
	// 关联用户
	User *models.User `json:"user"`
}

// AuthService 用户鉴权与会话管理服务
type AuthService struct{}

// NewAuthService 创建鉴权服务实例
func NewAuthService() *AuthService {
	return &AuthService{}
}

// HashToken 计算 Refresh Token 的 SHA256 散列值 (防数据库明文泄露)
func (s *AuthService) HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateRandomToken 生成高熵安全随机令牌字符串
func (s *AuthService) GenerateRandomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GetMiniAppSecret 根据 AppID 动态获取小程序的 Secret 配置 (实现多租户解耦)
func (s *AuthService) GetMiniAppSecret(appID string) (string, error) {
	var app models.MiniApp
	err := db.Mysql.Where("app_id = ?", appID).First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("未注册的小程序 AppID: %s", appID)
		}
		return "", err
	}
	return app.AppSecret, nil
}

// WechatLogin 微信小程序 Code 换取 OpenID 并创建双 Token 会话
func (s *AuthService) WechatLogin(ctx context.Context, appID, code string) (*WechatLoginResponse, error) {
	if code == "" {
		return nil, errors.New("微信登录 code 不能为空")
	}
	if appID == "" {
		return nil, errors.New("小程序 AppID 不能为空")
	}

	// 1. 动态获取小程序密钥
	secret, err := s.GetMiniAppSecret(appID)
	if err != nil {
		return nil, fmt.Errorf("获取小程序配置失败: %w", err)
	}

	// 2. 动态创建微信小程序 SDK 实例并换取 session
	miniClient, err := sdk.GetMiniSdkByAppID(appID, secret)
	if err != nil {
		return nil, fmt.Errorf("初始化微信客户端失败: %w", err)
	}

	sessionRsp, err := miniClient.Code2Session(ctx, code)
	if err != nil || sessionRsp.Errcode != 0 {
		return nil, fmt.Errorf("微信登录凭证校验异常: err=%v, code=%d, msg=%s", err, sessionRsp.Errcode, sessionRsp.Errmsg)
	}

	openID := sessionRsp.Openid
	if openID == "" {
		return nil, errors.New("微信服务返回的 openid 为空")
	}

	// 3. 多租户物理隔离查询或创建用户实体
	var user models.User
	err = db.Mysql.Where("app_id = ? AND wechat_openid = ?", appID, openID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = models.User{
				AppID:        appID,
				WechatOpenID: openID,
				NickName:     "微信用户" + ut.RandomStr(6),
				AvatarType:   0,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if err := db.Mysql.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("创建新用户失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("查询用户信息失败: %w", err)
		}
	}

	// 4. 创建全局会话并签发双 Token
	sessionID, err := s.GenerateRandomToken(16)
	if err != nil {
		sessionID = ut.RandomStr(32)
	}

	refreshTokenPlain, err := s.GenerateRandomToken(32)
	if err != nil {
		refreshTokenPlain = ut.RandomStr(64)
	}
	refreshHash := s.HashToken(refreshTokenPlain)
	refreshExpiresAt := time.Now().Add(jwtToken.RefreshTokenExpired)

	session := models.UserSession{
		SessionID:        sessionID,
		AppID:            appID,
		UserID:           user.ID,
		RefreshTokenHash: refreshHash,
		Revoked:          false,
		ExpiresAt:        refreshExpiresAt,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := db.Mysql.Create(&session).Error; err != nil {
		return nil, fmt.Errorf("创建用户会话失败: %w", err)
	}

	// 5. 签发 Access Token
	accessToken, accessExpiresAt := jwtToken.GenAccessToken(user.WechatOpenID, appID, sessionID, user.ID)

	return &WechatLoginResponse{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt.Format(time.RFC3339),
		RefreshToken:     refreshTokenPlain,
		RefreshExpiresAt: refreshExpiresAt.Format(time.RFC3339),
		SessionID:        sessionID,
		User:             user.SimpleUser(),
	}, nil
}

// // RefreshSession 刷新令牌换取新的双 Token (实现事务 CAS 轮换、重放攻击防御与并发宽限期)
func (s *AuthService) RefreshSession(appID, refreshTokenPlain string) (*WechatLoginResponse, error) {
	if refreshTokenPlain == "" {
		return nil, errors.New("刷新凭证 refresh_token 不能为空")
	}

	hash := s.HashToken(refreshTokenPlain)

	var session models.UserSession
	err := db.Mysql.Where("refresh_token_hash = ?", hash).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("无效的刷新凭证")
		}
		return nil, err
	}

	// 1. 重放攻击检测 (Replay Attack Detection): 旧 Refresh Token 轮换后立即失效，一旦再次被使用立即判定为重放攻击并吊销
	if session.Revoked {
		logger.JM.Warnf("【安全警报】检测到 Refresh Token 重放攻击！SessionID: %s, UserID: %d, 原失效原因: %s", session.SessionID, session.UserID, session.RevokedReason)
		// 吊销该用户在该小程序租户下的所有会话凭证，彻底阻断攻击者
		db.Mysql.Model(&models.UserSession{}).
			Where("session_id = ? OR (user_id = ? AND app_id = ?)", session.SessionID, session.UserID, session.AppID).
			Updates(map[string]interface{}{
				"revoked":        true,
				"revoked_reason": "replay_detected",
				"updated_at":     time.Now(),
			})
		return nil, errors.New("检测到凭证重放安全异常，为保障账户安全已强制退出所有会话，请重新登录")
	}

	// 2. 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		db.Mysql.Model(&session).Updates(map[string]interface{}{
			"revoked":        true,
			"revoked_reason": "expired",
			"updated_at":     time.Now(),
		})
		return nil, errors.New("刷新凭证已过期，请重新登录")
	}

	// 3. 校验租户匹配
	if appID != "" && session.AppID != appID {
		return nil, errors.New("小程序应用租户不匹配")
	}

	// 4. 查询关联用户
	var user models.User
	if err := db.Mysql.Where("id = ?", session.UserID).First(&user).Error; err != nil {
		return nil, errors.New("关联用户不存在")
	}

	// 5. 在原子事务中执行 CAS 状态标记与新 Session 签发
	newSessionID, err := s.GenerateRandomToken(16)
	if err != nil {
		newSessionID = ut.RandomStr(32)
	}
	newRefreshTokenPlain, err := s.GenerateRandomToken(32)
	if err != nil {
		newRefreshTokenPlain = ut.RandomStr(64)
	}
	newRefreshHash := s.HashToken(newRefreshTokenPlain)
	newRefreshExpiresAt := time.Now().Add(jwtToken.RefreshTokenExpired)

	newSession := models.UserSession{
		SessionID:        newSessionID,
		AppID:            session.AppID,
		UserID:           user.ID,
		RefreshTokenHash: newRefreshHash,
		Revoked:          false,
		ExpiresAt:        newRefreshExpiresAt,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err = db.Mysql.Transaction(func(tx *gorm.DB) error {
		// CAS 原子抢占：仅当原 session 仍未被 revoked 时更新成功
		res := tx.Model(&models.UserSession{}).
			Where("id = ? AND revoked = false", session.ID).
			Updates(map[string]interface{}{
				"revoked":        true,
				"revoked_reason": "token_rotated",
				"updated_at":     time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("会话并发抢占刷新中，请稍后重试")
		}

		// 写入新会话
		if err := tx.Create(&newSession).Error; err != nil {
			return fmt.Errorf("写入新会话失败: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 6. 签发全新 Access Token
	newAccessToken, newAccessExpiresAt := jwtToken.GenAccessToken(user.WechatOpenID, session.AppID, newSessionID, user.ID)

	return &WechatLoginResponse{
		AccessToken:      newAccessToken,
		AccessExpiresAt:  newAccessExpiresAt.Format(time.RFC3339),
		RefreshToken:     newRefreshTokenPlain,
		RefreshExpiresAt: newRefreshExpiresAt.Format(time.RFC3339),
		SessionID:        newSessionID,
		User:             user.SimpleUser(),
	}, nil

}

// GetSessionInfo 查询当前指定会话的存活状态
func (s *AuthService) GetSessionInfo(sessionID, appID string) (*SessionInfoResponse, error) {
	if sessionID == "" {
		return nil, errors.New("会话 ID 不能为空")
	}

	var session models.UserSession
	query := db.Mysql.Where("session_id = ?", sessionID)
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}
	if err := query.First(&session).Error; err != nil {
		return nil, errors.New("会话不存在")
	}

	isValid := !session.Revoked && time.Now().Before(session.ExpiresAt)

	var user models.User
	_ = db.Mysql.Where("id = ?", session.UserID).First(&user).Error

	return &SessionInfoResponse{
		IsValid:   isValid,
		SessionID: session.SessionID,
		AppID:     session.AppID,
		ExpiresAt: session.ExpiresAt.Format(time.RFC3339),
		User:      user.SimpleUser(),
	}, nil
}

// RevokeSession 撤销指定会话 (用户登出或管理员下线)
func (s *AuthService) RevokeSession(sessionID, reason string) error {
	if sessionID == "" {
		return errors.New("会话 ID 不能为空")
	}
	if reason == "" {
		reason = "user_logout"
	}

	return db.Mysql.Model(&models.UserSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"revoked":        true,
			"revoked_reason": reason,
			"updated_at":     time.Now(),
		}).Error
}
