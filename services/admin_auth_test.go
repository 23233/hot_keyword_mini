// Package services admin_auth_test.go
package services

import (
	"hot_keyword/models"
	"testing"
)

// TestAdminPasswordHashAndVerify 测试管理员加盐哈希计算与校验
func TestAdminPasswordHashAndVerify(t *testing.T) {
	service := NewAdminAuthService()
	salt := "test_salt_123456"
	pwd := "MyStrongAdminPwd!2026"

	hash := service.HashPassword(pwd, salt)
	if hash == "" {
		t.Fatalf("计算哈希不应为空")
	}

	if !service.VerifyPassword(pwd, salt, hash) {
		t.Fatalf("正确密码哈希比效应为 true")
	}

	if service.VerifyPassword("wrong_password", salt, hash) {
		t.Fatalf("错误密码不应通过哈希核验")
	}
}

// TestAdminTokenGenerationAndParse 测试管理员专用 JWT 凭证生成与解析
func TestAdminTokenGenerationAndParse(t *testing.T) {
	service := NewAdminAuthService()
	admin := &models.AdminUser{
		ID:       101,
		Username: "ops_editor",
		Role:     "editor",
		RealName: "运营专员",
	}

	tokenStr, err := service.GenerateAdminToken(admin)
	if err != nil {
		t.Fatalf("生成 AdminToken 失败: %v", err)
	}

	claims, err := service.ParseAdminToken(tokenStr)
	if err != nil {
		t.Fatalf("解析 AdminToken 失败: %v", err)
	}

	if claims["username"] != "ops_editor" {
		t.Fatalf("Claims 用户名不符: %v", claims["username"])
	}

	if claims["role"] != "editor" {
		t.Fatalf("Claims 角色不符: %v", claims["role"])
	}
}

// TestAdminLoginValidation 测试登录空参数校验与内存兜底
func TestAdminLoginValidation(t *testing.T) {
	service := NewAdminAuthService()

	_, _, err := service.AdminLogin("", "")
	if err == nil {
		t.Fatalf("空用户名密码应当被拦截报错")
	}

	// 内存模式下使用初始超管账号
	user, token, err := service.AdminLogin("admin", "admin123456")
	if err != nil {
		t.Fatalf("兜底超级管理员登录失败: %v", err)
	}
	if user.Username != "admin" || token == "" {
		t.Fatalf("登录返回结果不完整: %v", user)
	}
}
