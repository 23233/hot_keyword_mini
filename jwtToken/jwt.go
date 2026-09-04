// Package jwtToken jwt.go
package jwtToken

import (
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"os"
	"time"

	"github.com/23233/ggg/logger"
	"github.com/23233/ggg/ut"
	golangjwt "github.com/golang-jwt/jwt/v4"
	"github.com/iris-contrib/middleware/jwt"
	"github.com/kataras/iris/v12"
)

// MySecret 用户JWT签名密钥 (优先通过环境变量 JWT_SECRET 注入，非生产环境生成一次性随机安全密钥)
var MySecret = func() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	logger.JM.Warn("【安全提示】环境变量 JWT_SECRET 未配置，已生成运行时一次性随机安全密钥")
	return []byte("sdui_user_jwt_" + ut.RandomStr(32))
}()

// 自定义JWT配置
var jwtConfig = jwt.Config{
	ErrorHandler: func(ctx iris.Context, err error) {
		if err == nil {
			return
		}
		ctx.StopExecution()
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{
			"code":   401,
			"detail": err.Error(),
			"msg":    "身份认证失败或凭证已过期",
		})
	},
	ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
		return MySecret, nil
	},
	Expiration:          true,
	CredentialsOptional: false,
	SigningMethod:       jwt.SigningMethodHS256,
}

// CustomJwt 默认通过 Authorization Bearer 头提取 JWT
var CustomJwt = jwt.New(jwtConfig)

// CtJwt 通过 Query 参数 c_t 提取 JWT
var CtJwt = jwt.New(jwtConfig)

func init() {
	CtJwt.Config.Extractor = jwt.FromParameter("c_t")
}

// ContextGetClaims 从当前请求上下文中获取 JWT Claims 数据
// ParseTokenString 解析任意 Token 字符串并验证签名与时效性
func ParseTokenString(tokenString string) (golangjwt.MapClaims, error) {
	token, err := golangjwt.Parse(tokenString, func(t *golangjwt.Token) (interface{}, error) {
		return MySecret, nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, errors.New("无效或过期的访问凭证")
	}
	if claims, ok := token.Claims.(golangjwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.New("无法解析凭证 Claims 数据")
}

func ContextGetClaims(ctx iris.Context) (jwt.MapClaims, error) {
	if ctx.Values().Exists("jwt") {
		info := ctx.Values().Get("jwt").(*jwt.Token)
		if claims, ok := info.Claims.(jwt.MapClaims); ok {
			return claims, nil
		}
	}
	return nil, errors.New("上下文中未获取到有效凭证")
}

// ContextGetOpenId 上下文中获取用户微信 OpenID
func ContextGetOpenId(ctx iris.Context) (string, error) {
	claims, err := ContextGetClaims(ctx)
	if err != nil {
		return "", err
	}
	if openid, ok := claims["openId"].(string); ok {
		return openid, nil
	}
	return "", errors.New("凭证中未包含用户标识")
}

// ContextGetUser 从上下文获取当前登录用户信息并验证会话有效性 (支持多租户与会话撤销检测)
func ContextGetUser(ctx iris.Context) (*models.User, error) {
	claims, err := ContextGetClaims(ctx)
	if err != nil {
		return nil, err
	}

	// 1. 若携带 sessionId，执行强校验检测是否已被主动撤销或踢出
	if sid, ok := claims[JwtSessionId].(string); ok && sid != "" {
		var session models.UserSession
		if err := db.Mysql.Where("session_id = ?", sid).First(&session).Error; err != nil {
			return nil, errors.New("会话不存在或已过期")
		}
		if session.Revoked {
			return nil, fmt.Errorf("会话已失效 (%s)，请重新登录", session.RevokedReason)
		}
		if time.Now().After(session.ExpiresAt) {
			return nil, errors.New("会话已过期，请重新登录")
		}
	}

	// 2. 根据 userId 或 openid + app_id 查询对应用户实体
	var user models.User
	if uidVal, ok := claims[JwtUserId]; ok {
		var uid int64
		switch v := uidVal.(type) {
		case float64:
			uid = int64(v)
		case int64:
			uid = v
		}
		if uid > 0 {
			if err := db.Mysql.Where("id = ?", uid).First(&user).Error; err == nil {
				return &user, nil
			}
		}
	}

	// 降级使用 openid 与 app_id 查询
	openId, _ := claims["openId"].(string)
	appId, _ := claims[JwtAppId].(string)
	query := db.Mysql.Where("wechat_openid = ?", openId)
	if appId != "" {
		query = query.Where("app_id = ?", appId)
	}
	if err := query.First(&user).Error; err != nil {
		return nil, errors.New("未能找到对应的有效用户")
	}

	return &user, nil
}

// TokenToUserUidMiddleware 登录 token 信息校验并注入请求上下文
func TokenToUserUidMiddleware(ctx iris.Context) {
	claims, err := ContextGetClaims(ctx)
	if err != nil {
		ctx.StopExecution()
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": "未登录或登录态无效"})
		return
	}

	user, err := ContextGetUser(ctx)
	if err != nil {
		ctx.StopExecution()
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{"code": 401, "msg": err.Error()})
		return
	}

	ctx.Values().Set(JwtUserModel, user)
	ctx.Values().Set(JwtUserId, user.ID)
	ctx.Values().Set(JwtUserOpenId, user.WechatOpenID)
	if appId, ok := claims[JwtAppId].(string); ok {
		ctx.Values().Set(JwtAppId, appId)
	}
	if sid, ok := claims[JwtSessionId].(string); ok {
		ctx.Values().Set(JwtSessionId, sid)
	}

	ctx.Next()
}

// GenAccessToken 生成短期访问令牌 Access Token (2小时)
func GenAccessToken(openId, appId, sessionId string, userId int64) (string, time.Time) {
	expireTime := time.Now().Add(AccessTokenExpired)
	token := jwt.NewTokenWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"openId":     openId,
		JwtAppId:     appId,
		JwtSessionId: sessionId,
		JwtUserId:    userId,
		"version":    2,
		"loginTime":  time.Now().Format("2006-01-02 15:04:05"),
		"exp":        expireTime.Unix(),
	})
	tokenString, _ := token.SignedString(MySecret)
	return tokenString, expireTime
}

// GenJwtToken 生成兼容旧版的长期单 Token
func GenJwtToken(openId string) string {
	token := jwt.NewTokenWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"openId":    openId,
		"version":   1,
		"loginTime": time.Now().Format("2006-01-02"),
		"exp":       time.Now().Add(JwtExpired).Unix(),
	})
	tokenString, _ := token.SignedString(MySecret)
	return tokenString
}

// ValidateTokenSessionAndTenant 解析并严密校验访问凭证、所属租户匹配性与用户会话有效性
// 杜绝已登出、已轮换撤销或跨小程序租户的凭据越权访问
func ValidateTokenSessionAndTenant(tokenStr string, expectedAppID string) (*models.UserSession, *models.User, golangjwt.MapClaims, error) {
	if tokenStr == "" {
		return nil, nil, nil, errors.New("缺少身份访问凭证")
	}

	claims, err := ParseTokenString(tokenStr)
	if err != nil || claims == nil {
		return nil, nil, nil, fmt.Errorf("凭证无效或已过期: %w", err)
	}

	// 1. 强校验多租户隔离: 凭证内嵌入的 appId 必须存在且与当前请求的 expectedAppID 完全一致
	tokenAppID, _ := claims[JwtAppId].(string)
	if tokenAppID == "" {
		return nil, nil, nil, errors.New("凭据格式不合规: 缺少 appId 租户信息，禁止未隔离访问")
	}
	if expectedAppID != "" && tokenAppID != expectedAppID {
		return nil, nil, nil, fmt.Errorf("多租户凭证越权拦截: 凭据签发给 %s，无法访问租户 %s", tokenAppID, expectedAppID)
	}

	// 2. 强校验会话状态与生命周期 (杜绝已登出、被踢或重放攻击吊销后继续使用)
	sid, _ := claims[JwtSessionId].(string)
	if sid == "" {
		return nil, nil, nil, errors.New("凭据格式不合规: 缺少 sessionId 会话信息，禁止无状态绕过撤销检查")
	}
	var session models.UserSession
	if db.Mysql != nil {
		if err := db.Mysql.Where("session_id = ?", sid).First(&session).Error; err != nil {
			return nil, nil, nil, errors.New("会话不存在或已失效")
		}
		if session.Revoked {
			return nil, nil, nil, fmt.Errorf("会话已被注销或吊销 (%s)，请重新登录", session.RevokedReason)
		}
		if time.Now().After(session.ExpiresAt) {
			return nil, nil, nil, errors.New("会话已过期，请重新登录")
		}
		if expectedAppID != "" && session.AppID != "" && session.AppID != expectedAppID {
			return nil, nil, nil, fmt.Errorf("会话租户不匹配: 会话绑定 %s", session.AppID)
		}
	}

	// 3. 提取关联用户
	var user models.User
	if db.Mysql != nil && session.UserID > 0 {
		_ = db.Mysql.Where("id = ?", session.UserID).First(&user)
	}

	return &session, &user, claims, nil
}
