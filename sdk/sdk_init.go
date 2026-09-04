// Package sdk sdk_init.go
package sdk

import (
	"errors"
	"os"
	"sync"

	"github.com/23233/ggg/logger"
	"github.com/go-pay/wechat-sdk/mini"
)

// MiniSdk 全局默认微信小程序 SDK 句柄
var MiniSdk *mini.SDK

// 默认小程序标识 (生产环境推荐通过环境变量注入)
var (
	// 可选的单租户默认小程序 AppID；多租户场景从数据库动态传入。
	WechatMiniAppId = ""
	// 小程序 AppSecret (禁止硬编码，支持 WECHAT_MINI_SECRET 环境变量覆盖)
	WechatMiniSecret = ""
)

var (
	sdkCacheMu sync.RWMutex
	sdkCache   = make(map[string]*mini.SDK)
)

func init() {
	if envAppId := os.Getenv("WECHAT_MINI_APP_ID"); envAppId != "" {
		WechatMiniAppId = envAppId
	}
	if envSecret := os.Getenv("WECHAT_MINI_SECRET"); envSecret != "" {
		WechatMiniSecret = envSecret
	}

	if WechatMiniAppId == "" || WechatMiniSecret == "" || WechatMiniSecret == "CHANGEME_WECHAT_MINI_SECRET" {
		logger.JM.Warnf("未配置默认小程序 SDK；多租户请求将按 mini_apps 数据库配置动态初始化")
		return
	}

	var err error
	MiniSdk, err = mini.New(WechatMiniAppId, WechatMiniSecret, true)
	if err != nil {
		logger.JM.Warnf("默认小程序 SDK 初始化提示: %v (未配置合法 Secret 时微信免密换取不可用)", err)
		return
	}
	sdkCacheMu.Lock()
	sdkCache[WechatMiniAppId] = MiniSdk
	sdkCacheMu.Unlock()
}

// GetMiniSdkByAppID 根据小程序 AppID 与 Secret 动态获取或构建专用的 SDK 实例 (支持多租户独立密钥)
func GetMiniSdkByAppID(appID, appSecret string) (*mini.SDK, error) {
	if appID == "" {
		appID = WechatMiniAppId
	}
	if appID == "" {
		return nil, errors.New("小程序 AppID 未提供")
	}

	sdkCacheMu.RLock()
	cached, ok := sdkCache[appID]
	sdkCacheMu.RUnlock()
	if ok && cached != nil {
		return cached, nil
	}

	secret := appSecret
	if secret == "" && appID == WechatMiniAppId {
		secret = WechatMiniSecret
	}
	if secret == "" || secret == "CHANGEME_WECHAT_MINI_SECRET" {
		return nil, errors.New("小程序 Secret 未配置，无法调用微信接口")
	}

	// 持写锁覆盖实例创建，避免同一 AppID 并发初始化多个自动刷新协程。
	sdkCacheMu.Lock()
	defer sdkCacheMu.Unlock()
	if cached, exists := sdkCache[appID]; exists && cached != nil {
		return cached, nil
	}
	instance, err := mini.New(appID, secret, true)
	if err != nil {
		return nil, err
	}

	sdkCache[appID] = instance

	return instance, nil
}

// InvalidateMiniSdk 清理指定 AppID 的进程内 SDK 缓存。
// 小程序 Secret 变更后必须调用，确保下一次调用使用新凭据并重新维护 AccessToken。
func InvalidateMiniSdk(appID string) {
	if appID == "" {
		return
	}
	sdkCacheMu.Lock()
	delete(sdkCache, appID)
	if WechatMiniAppId == appID {
		MiniSdk = nil
	}
	sdkCacheMu.Unlock()
}
