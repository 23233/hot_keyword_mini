// Package routers drama.go
package routers

import (
	"hot_keyword/services"

	"github.com/23233/ggg/ut"
	"github.com/kataras/iris/v12"
)

// RegisterDramaRoutes 注册短剧相关 API 路由
func RegisterDramaRoutes(party iris.Party) {
	dramaParty := party.Party("/drama")
	{
		// 获取小程序首页数据(短剧详情、选集、当前展示模式、看后续渠道配置)
		dramaParty.Get("/home", GetDramaHomeHandler)
		// 获取单个短剧详情及选集列表
		dramaParty.Get("/detail", GetDramaDetailHandler)
		// 切换小程序首页展示模式
		dramaParty.Post("/switch_mode", SwitchDramaModeHandler)
	}
}

// GetDramaDetailHandler 获取单个短剧详情与选集
func GetDramaDetailHandler(ctx iris.Context) {
	id, _ := ctx.URLParamInt64("id")
	srv := services.NewDramaService()
	drama, episodes, err := srv.GetDramaDetail(id)
	if err != nil {
		ut.IrisErrLog(ctx, err, "获取短剧详情失败")
		_ = ctx.JSON(iris.Map{
			"code": 500,
			"msg":  "获取短剧详情失败",
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": iris.Map{
			"drama":    drama,
			"episodes": episodes,
		},
	})
}

// GetDramaHomeHandler 获取短剧首页驱动数据
func GetDramaHomeHandler(ctx iris.Context) {
	// 可通过 ?mode=immersive_video / episode_grid / direct_portal 临时切换体验
	mode := ctx.URLParam("mode")

	srv := services.NewDramaService()
	data, err := srv.GetHomeData(mode)
	if err != nil {
		ut.IrisErrLog(ctx, err, "获取短剧首页数据失败")
		_ = ctx.JSON(iris.Map{
			"code": 500,
			"msg":  "获取短剧首页数据失败: " + err.Error(),
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// SwitchDramaModeReq 切换展示模式请求体
type SwitchDramaModeReq struct {
	// 展示模式: immersive_video / episode_grid / direct_portal
	Mode string `json:"mode"`
}

// SwitchDramaModeHandler 切换展示模式接口
func SwitchDramaModeHandler(ctx iris.Context) {
	var req SwitchDramaModeReq
	if err := ctx.ReadJSON(&req); err != nil || req.Mode == "" {
		_ = ctx.JSON(iris.Map{
			"code": 400,
			"msg":  "请传入合法的 mode 参数",
		})
		return
	}

	srv := services.NewDramaService()
	if err := srv.SwitchDisplayMode(req.Mode); err != nil {
		ut.IrisErrLog(ctx, err, "切换展示模式失败")
		_ = ctx.JSON(iris.Map{
			"code": 500,
			"msg":  "切换展示模式失败: " + err.Error(),
		})
		return
	}

	_ = ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "模式切换成功",
		"data": iris.Map{
			"mode": req.Mode,
		},
	})
}
