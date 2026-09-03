// Package jwtToken jwt.go
package jwtToken

import (
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"time"

	"github.com/iris-contrib/middleware/jwt"
	"github.com/kataras/iris/v12"
)

var MySecret = []byte("HefNcCJPz2eT7rq2eW7L9WaFLYO4zZOy")

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
		"openId":      openId,
		JwtAppId:      appId,
		JwtSessionId:  sessionId,
		JwtUserId:     userId,
		"version":     2,
		"loginTime":   time.Now().Format("2006-01-02 15:04:05"),
		"exp":         expireTime.Unix(),
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
