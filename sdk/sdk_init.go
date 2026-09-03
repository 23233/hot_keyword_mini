package sdk

import (
	"github.com/23233/ggg/logger"
	"github.com/go-pay/wechat-sdk/mini"
)

// MiniSdk 微信小程序sdk 从总sdk中实例化
var MiniSdk *mini.SDK

// 微信小程序
var (
	WechatMiniAppId  = "wx516563cfe994bbc6"               // 小程序ID
	WechatMiniSecret = "673ac42e8aaec2f9cfa5547e780f7658" // 小程序secret
)

func init() {
	var err error
	MiniSdk, err = mini.New(WechatMiniAppId, WechatMiniSecret, true)
	if err != nil {
		logger.JM.ErrEf(err, "初始化小程序sdk失败")
		return
	}
}
