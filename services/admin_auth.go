// Package services admin_auth.go
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/23233/ggg/ut"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

const (
	// adminPasswordSaltMarker 标记使用 bcrypt 的管理员密码记录。
	adminPasswordSaltMarker = "bcrypt"
	// adminPasswordMinLength 管理员密码最小长度。
	adminPasswordMinLength = 8
)

// adminUsernameRegexp 管理员用户名格式安全正则（防注入与异常控制字符）
var adminUsernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// AdminSecretKey 管理员独立签发密钥 (优先通过环境变量 ADMIN_JWT_SECRET 注入，非生产环境生成一次性随机安全密钥)
var AdminSecretKey = func() []byte {
	if s := os.Getenv("ADMIN_JWT_SECRET"); s != "" {
		return []byte(s)
	}
	fmt.Fprintln(os.Stderr, "【安全提示】环境变量 ADMIN_JWT_SECRET 未配置，已生成运行时一次性随机安全密钥")
	return []byte("sdui_admin_jwt_" + ut.RandomStr(32))
}()

// AdminAuthService 管理员认证与账户生命周期管理服务
type AdminAuthService struct{}

// NewAdminAuthService 构造函数
func NewAdminAuthService() *AdminAuthService {
	return &AdminAuthService{}
}

// HashPassword 计算历史 SHA-256 加盐哈希，仅用于兼容既有账户与迁移校验。
func (s *AdminAuthService) HashPassword(password, salt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password + salt))
	return hex.EncodeToString(hasher.Sum(nil))
}

// HashPasswordSecure 使用 bcrypt 计算管理员密码哈希，新账户与重置密码统一使用该格式。
func (s *AdminAuthService) HashPasswordSecure(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成安全密码哈希失败: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword 校验密码；bcrypt 记录走 bcrypt 比对，历史记录回退 SHA-256。
func (s *AdminAuthService) VerifyPassword(password, salt, expectedHash string) bool {
	if strings.HasPrefix(expectedHash, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(expectedHash), []byte(password)) == nil
	}
	legacy := s.HashPassword(password, salt)
	// 使用固定长度比较，降低哈希比对时序侧信道风险。
	return subtleCompare(legacy, expectedHash)
}

// needsPasswordUpgrade 判断历史密码记录是否需要在登录成功后升级为 bcrypt。
func needsPasswordUpgrade(expectedHash string) bool {
	return !strings.HasPrefix(expectedHash, "$2")
}

// subtleCompare 以常量时间比较两个等长字符串。
func subtleCompare(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for index := 0; index < len(left); index++ {
		diff |= left[index] ^ right[index]
	}
	return diff == 0
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

// ParseAdminToken 解析并验证管理员凭证 (同时强制复验数据库账户启用状态与最新角色)
func (s *AdminAuthService) ParseAdminToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return AdminSecretKey, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("无效或已过期的管理员凭证")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("凭证 Claims 解析失败")
	}

	// 实时强校验数据库账户活跃状态与最新角色权限 (杜绝封禁账号继续使用或降权延迟)
	if db.DB != nil {
		var adminUser models.AdminUser
		var findErr error
		if adminID, ok := claims["admin_id"].(float64); ok && adminID > 0 {
			findErr = db.DB.Where("id = ?", int64(adminID)).First(&adminUser).Error
		} else if username, ok := claims["username"].(string); ok && username != "" {
			findErr = db.DB.Where("username = ?", username).First(&adminUser).Error
		}
		if findErr != nil {
			return nil, errors.New("管理员账户不存在或已被删除")
		}
		if adminUser.Status == "disabled" || adminUser.Status != "active" {
			return nil, errors.New("管理员账户已被冻结禁用，禁止访问")
		}
		// 动态同步数据库中的最新角色，杜绝权限降级后旧 Token 越权
		claims["role"] = adminUser.Role
	}

	return claims, nil
}

// AdminLogin 管理员账号密码认证登录
func (s *AdminAuthService) AdminLogin(username, password string) (*models.AdminUser, string, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, "", errors.New("用户名与密码不能为空")
	}

	var user models.AdminUser
	if db.DB != nil {
		if err := db.DB.Where("username = ?", username).First(&user).Error; err != nil {
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

	// 历史 SHA-256 账户在登录成功后透明升级为 bcrypt，避免一次性强制全员改密。
	if needsPasswordUpgrade(user.PasswordHash) {
		if upgraded, err := s.HashPasswordSecure(password); err == nil {
			user.PasswordHash, user.Salt = upgraded, adminPasswordSaltMarker
			_ = db.DB.Model(&user).Updates(map[string]interface{}{
				"password_hash": upgraded,
				"salt":          adminPasswordSaltMarker,
				"updated_at":    time.Now(),
			}).Error
		}
	}

	// 记录最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	if db.DB != nil {
		_ = db.DB.Model(&user).Update("last_login_at", now)
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
	if db.DB != nil {
		if err := db.DB.Order("id asc").Find(&list).Error; err != nil {
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

	if !adminUsernameRegexp.MatchString(username) {
		return nil, errors.New("用户名仅支持3-32位字母、数字、下划线和减号")
	}

	if len(password) < adminPasswordMinLength {
		return nil, fmt.Errorf("密码长度不能低于%d位", adminPasswordMinLength)
	}

	if role == "" {
		role = "editor"
	}

	if db.DB != nil {
		var count int64
		db.DB.Model(&models.AdminUser{}).Where("username = ?", username).Count(&count)
		if count > 0 {
			return nil, fmt.Errorf("账号 '%s' 已存在，请使用其他用户名", username)
		}

		pwdHash, err := s.HashPasswordSecure(password)
		if err != nil {
			return nil, err
		}

		user := models.AdminUser{
			Username:     username,
			PasswordHash: pwdHash,
			Salt:         adminPasswordSaltMarker,
			RealName:     realName,
			Role:         role,
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := db.DB.Create(&user).Error; err != nil {
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
	if db.DB == nil {
		return nil, errors.New("无数据库环境")
	}

	var user models.AdminUser
	if err := db.DB.First(&user, id).Error; err != nil {
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
		if len(newPassword) < adminPasswordMinLength {
			return nil, fmt.Errorf("新密码长度不能少于%d位", adminPasswordMinLength)
		}
		pwdHash, err := s.HashPasswordSecure(newPassword)
		if err != nil {
			return nil, err
		}
		updates["salt"] = adminPasswordSaltMarker
		updates["password_hash"] = pwdHash
	}

	updates["updated_at"] = time.Now()

	if err := db.DB.Model(&user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新管理员信息失败: %w", err)
	}

	return &user, nil
}

// DeleteAdminUser 删除管理员 (具备防自杀与防最后超管删除双重保护)
func (s *AdminAuthService) DeleteAdminUser(id int64, operatorAdminID int64) error {
	if id == operatorAdminID {
		return errors.New("安全保护拦截: 禁止删除当前正在登录的自身账户")
	}

	if db.DB == nil {
		return nil
	}

	var user models.AdminUser
	if err := db.DB.First(&user, id).Error; err != nil {
		return errors.New("管理员不存在")
	}

	// 若为超级管理员，检查是否是最后一位超级管理员
	if user.Role == "super_admin" {
		var superCount int64
		db.DB.Model(&models.AdminUser{}).Where("role = ? AND status = 'active'", "super_admin").Count(&superCount)
		if superCount <= 1 {
			return errors.New("安全保护拦截: 系统必须至少保留一位启用的超级管理员，无法删除")
		}
	}

	return db.DB.Delete(&user).Error
}
