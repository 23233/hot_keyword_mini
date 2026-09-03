package jwtToken

import (
	"errors"
	"gorm_template/db"
	"gorm_template/models"

	"github.com/iris-contrib/middleware/jwt"
	"github.com/kataras/iris/v12"

	"time"
)

var MySecret = []byte("HefNcCJPz2eT7rq2eW7L9WaFLYO4zZOy")

// 自定义JWT
// 使用办法 中间层 handler.CustomJwt.Serve, handler.TokenToUserUidMiddleware,user handler
var jwtConfig = jwt.Config{
	//Extractor : jwtToken.FromParameter("token")
	//Extractor : jwtToken.FromAuthHeader // default
	ErrorHandler: func(ctx iris.Context, err error) {
		if err == nil {
			return
		}
		ctx.StopExecution()
		ctx.StatusCode(iris.StatusUnauthorized)
		_ = ctx.JSON(iris.Map{
			"detail": err.Error(),
		})
	},
	ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
		return MySecret, nil
	},
	Expiration:          true,
	CredentialsOptional: false,
	SigningMethod:       jwt.SigningMethodHS256,
}

var CustomJwt = jwt.New(jwtConfig)

var CtJwt = jwt.New(jwtConfig)

// ContextGetUser ctx上下文中获取用户信息
func ContextGetUser(ctx iris.Context) (*models.User, error) {
	openId, _ := ContextGetOpenId(ctx)
	var user models.User
	err := db.Mysql.Where("wechat_openid = ?", openId).First(&user).Error
	return &user, err
}

// ContextGetOpenId 上下文中获取用户唯一标识
func ContextGetOpenId(ctx iris.Context) (string, error) {
	if ctx.Values().Exists("jwt") {
		info := ctx.Values().Get("jwt").(*jwt.Token)
		jwtData := info.Claims.(jwt.MapClaims)
		openid := jwtData["openId"].(string)
		return openid, nil
	}
	return "", errors.New("上下文中未获取到token ")
}

// TokenToUserUidMiddleware 登录token存储信息 记录到上下文中
func TokenToUserUidMiddleware(ctx iris.Context) {
	// 这里可以遍历所有的token信息
	//for key, value := range jwtData {
	//	_, _ = ctx.Writef("%s = %s", key, value)
	//}
	user, err := ContextGetUser(ctx)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		ctx.Values().Set("errorText", "获取用户失败")
		return
	}
	ctx.Values().Set(JwtUserModel, user)
	ctx.Values().Set(JwtUserId, user.ID)
	ctx.Values().Set(JwtUserOpenId, user.WechatOpenID)
	ctx.Next()
}

// GenJwtToken 生成token
func GenJwtToken(openId string) string {
	token := jwt.NewTokenWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"openId":    openId,
		"version":   1,
		"loginTime": time.Now().Format("2006-01-02"),
		"exp":       time.Now().Add(JwtExpired).Unix(), //过期时间
	})
	tokenString, _ := token.SignedString(MySecret)
	return tokenString
}

func init() {
	CtJwt.Config.Extractor = jwt.FromParameter("c_t")
}
