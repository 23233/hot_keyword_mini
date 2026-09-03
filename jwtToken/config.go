// Package jwtToken config.go
package jwtToken

import "time"

var (
	// JwtUserModel 上下文中存储 User 对象的键名
	JwtUserModel = "userModel"
	// JwtUserId 上下文中存储 UserID 的键名
	JwtUserId = "userId"
	// JwtUserOpenId 上下文中存储 WechatOpenID 的键名
	JwtUserOpenId = "userOpenId"
	// JwtAppId 上下文中存储 AppID 的键名
	JwtAppId = "appId"
	// JwtSessionId 上下文中存储 SessionID 的键名
	JwtSessionId = "sessionId"

	// AccessTokenExpired 短期访问令牌有效时长 (2小时)
	AccessTokenExpired = time.Hour * 2
	// RefreshTokenExpired 长期刷新令牌有效时长 (30天)
	RefreshTokenExpired = time.Hour * 24 * 30

	// JwtExpired 保持对旧代码的变量兼容
	JwtExpired = time.Hour * 24 * 30
)
