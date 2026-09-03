// Package services admin_auth.go
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"time"

	"github.com/23233/ggg/ut"
	"github.com/golang-jwt/jwt/v4"
)

// AdminSecretKey 管理员独立签发密钥
var AdminSecretKey = []byte("SDUI_Admin_Secure_Key_2026_x86")

// AdminAuthService 管理员认证与账户生命周期管理服务
type AdminAuthService struct{}

// NewAdminAuthService 构造函数
func NewAdminAuthService() *AdminAuthService {
	return &AdminAuthService{}
}

// HashPassword 计算加盐哈希
func (s *AdminAuthService) HashPassword(password, salt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password + salt))
	return hex.EncodeToString(hasher.Sum(nil))
}

// VerifyPassword 校验密码一致性
func (s *AdminAuthService) VerifyPassword(password, salt, expectedHash string) bool {
	return s.HashPassword(password, salt) == expectedHash
}

// GenerateAdminToken 为管理员签发专用的 24 小时访问凭证
func (s *AdminAuthService) GenerateAdminToken(user *models.AdminUser) (string, error) {
	claims := jwt.MapClaims{
		"admin_id":  user.ID,
		"username":  user.Username,
		"role":      user.Role,
		"real_name": user.RealName,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"iss":       "sdui_admin_auth",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(AdminSecretKey)
}

// ParseAdminToken 解析并验证管理员凭证
func (s *AdminAuthService) ParseAdminToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return AdminSecretKey, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("无效或已过期的管理员凭证")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.New("凭证 Claims 解析失败")
}

// AdminLogin 管理员账号密码认证登录
func (s *AdminAuthService) AdminLogin(username, password string) (*models.AdminUser, string, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, "", errors.New("用户名与密码不能为空")
	}

	var user models.AdminUser
	if db.Mysql != nil {
		if err := db.Mysql.Where("username = ?", username).First(&user).Error; err != nil {
			return nil, "", errors.New("管理员账户不存在或密码错误")
		}
	} else {
		// 内存自举兜底模式
		if username == "admin" && password == "admin123456" {
			user = models.AdminUser{
				ID:        1,
				Username:  "admin",
				RealName:  "系统超级管理员",
				Role:      "super_admin",
				Status:    "active",
				CreatedAt: time.Now(),
			}
			token, _ := s.GenerateAdminToken(&user)
			return &user, token, nil
		}
		return nil, "", errors.New("管理员账户不存在或密码错误")
	}

	if user.Status == "disabled" {
		return nil, "", errors.New("该管理员账户已被停用禁用，请联系超级管理员")
	}

	if !s.VerifyPassword(password, user.Salt, user.PasswordHash) {
		return nil, "", errors.New("管理员账户不存在或密码错误")
	}

	// 记录最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	if db.Mysql != nil {
		_ = db.Mysql.Model(&user).Update("last_login_at", now)
	}

	token, err := s.GenerateAdminToken(&user)
	if err != nil {
		return nil, "", fmt.Errorf("签发凭证失败: %w", err)
	}

	return &user, token, nil
}

// ListAdminUsers 获取管理员列表
func (s *AdminAuthService) ListAdminUsers() ([]models.AdminUser, error) {
	var list []models.AdminUser
	if db.Mysql != nil {
		if err := db.Mysql.Order("id asc").Find(&list).Error; err != nil {
			return nil, err
		}
	} else {
		list = []models.AdminUser{
			{
				ID:        1,
				Username:  "admin",
				RealName:  "系统超级管理员",
				Role:      "super_admin",
				Status:    "active",
				CreatedAt: time.Now(),
			},
		}
	}
	return list, nil
}

// CreateAdminUser 创建新管理员账户
func (s *AdminAuthService) CreateAdminUser(username, password, realName, role string) (*models.AdminUser, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	realName = strings.TrimSpace(realName)
	if username == "" || password == "" {
		return nil, errors.New("登录账号和初始密码不能为空")
	}

	if len(password) < 6 {
		return nil, errors.New("密码长度不能低于6位")
	}

	if role == "" {
		role = "editor"
	}

	if db.Mysql != nil {
		var count int64
		db.Mysql.Model(&models.AdminUser{}).Where("username = ?", username).Count(&count)
		if count > 0 {
			return nil, fmt.Errorf("账号 '%s' 已存在，请使用其他用户名", username)
		}

		salt := "admin_salt_" + ut.RandomStr(16)
		pwdHash := s.HashPassword(password, salt)

		user := models.AdminUser{
			Username:     username,
			PasswordHash: pwdHash,
			Salt:         salt,
			RealName:     realName,
			Role:         role,
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := db.Mysql.Create(&user).Error; err != nil {
			return nil, fmt.Errorf("创建管理员失败: %w", err)
		}
		return &user, nil
	}

	return &models.AdminUser{
		ID:        999,
		Username:  username,
		RealName:  realName,
		Role:      role,
		Status:    "active",
		CreatedAt: time.Now(),
	}, nil
}

// UpdateAdminUser 修改管理员基本信息、角色、启禁用状态或重置密码
func (s *AdminAuthService) UpdateAdminUser(id int64, realName, role, status, newPassword string) (*models.AdminUser, error) {
	if db.Mysql == nil {
		return nil, errors.New("无数据库环境")
	}

	var user models.AdminUser
	if err := db.Mysql.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("未找到指定管理员 ID: %d", id)
	}

	updates := make(map[string]interface{})
	if realName != "" {
		updates["real_name"] = realName
		user.RealName = realName
	}
	if role != "" {
		updates["role"] = role
		user.Role = role
	}
	if status != "" {
		updates["status"] = status
		user.Status = status
	}
	if newPassword != "" {
		if len(newPassword) < 6 {
			return nil, errors.New("新密码长度不能少于6位")
		}
		salt := "admin_salt_" + ut.RandomStr(16)
		updates["salt"] = salt
		updates["password_hash"] = s.HashPassword(newPassword, salt)
	}

	updates["updated_at"] = time.Now()

	if err := db.Mysql.Model(&user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新管理员信息失败: %w", err)
	}

	return &user, nil
}

// DeleteAdminUser 删除管理员 (具备防自杀与防最后超管删除双重保护)
func (s *AdminAuthService) DeleteAdminUser(id int64, operatorAdminID int64) error {
	if id == operatorAdminID {
		return errors.New("安全保护拦截: 禁止删除当前正在登录的自身账户")
	}

	if db.Mysql == nil {
		return nil
	}

	var user models.AdminUser
	if err := db.Mysql.First(&user, id).Error; err != nil {
		return errors.New("管理员不存在")
	}

	// 若为超级管理员，检查是否是最后一位超级管理员
	if user.Role == "super_admin" {
		var superCount int64
		db.Mysql.Model(&models.AdminUser{}).Where("role = ? AND status = 'active'", "super_admin").Count(&superCount)
		if superCount <= 1 {
			return errors.New("安全保护拦截: 系统必须至少保留一位启用的超级管理员，无法删除")
		}
	}

	return db.Mysql.Delete(&user).Error
}
