// Package validator mini.go
package validator

// WeCodeReq 微信小程序临时登录凭证换取请求结构
type WeCodeReq struct {
	// 微信登录换取 session_key 和 openid 的临时 code 凭证
	Code string `json:"code" form:"code" validate:"required,max=64"`
}
