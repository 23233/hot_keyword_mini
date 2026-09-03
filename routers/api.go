// Package routers/api.go
package routers

import (
	"gorm_template/db"
	"gorm_template/jwtToken"
	"gorm_template/models"
	"gorm_template/sdk"
	"gorm_template/validator"

	"github.com/23233/ggg/sv"
	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"

	"time"
)

func registerAPIRoutes(party iris.Party) {
	api := party.Party("/api/v1")

	// 通过code获取 openid
	api.Post("/code", sv.Run(new(validator.WeCodeReq)), WeCodeGetInfo)

	// Protected routes
	userParty := api.Party("/user")
	userParty.Use(jwtToken.SidAndJwtMiddleware, jwtToken.TokenToUserUidMiddleware)
	userParty.Get("/info", GetNewInfo)

}

// WeCodeGetInfo 小程序通过wx.login获取code后 获取openid
func WeCodeGetInfo(ctx iris.Context) {
	req := ctx.Values().Get(sv.GlobalContextKey).(*validator.WeCodeReq)
	var err error
	sessionRsp, err := sdk.MiniSdk.Code2Session(ctx, req.Code)
	if err != nil || sessionRsp.Errcode != 0 {
		ut.IrisErrLog(ctx, err, "获取用户信息异常")
		return
	}
	// 通过openid 获取或注册用户
	var user = models.User{}

	// 获取或创建
	err = db.Mysql.Where(models.User{WechatOpenID: sessionRsp.Openid}).Assign(models.User{
		NickName:   "微信用户" + ut.RandomStr(6),
		AvatarType: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}).FirstOrCreate(&user).Error
	if err != nil {
		ut.IrisErrLog(ctx, err, "获取用户信息异常")
		return
	}

	token := jwtToken.GenJwtToken(user.WechatOpenID)
	_ = ctx.JSON(iris.Map{
		"code": 0,
		"data": iris.Map{
			"token": token,
			"user":  user.SimpleUser(),
		},
	})
	return
}

// GetNewInfo 获取最新信息
func GetNewInfo(ctx iris.Context) {
	userModel := ctx.Values().Get(jwtToken.JwtUserModel).(*models.User)
	_ = ctx.JSON(iris.Map{
		"code": 0,
		"data": userModel.SimpleUser(),
	})
}
