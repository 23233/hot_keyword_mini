// Package sdk cos.go
package sdk

import (
	"hot_keyword/config"
	"hot_keyword/pkg/cos_service"

	"github.com/23233/ggg/logger"
)

// CosService 是全局 COS 服务实例。
var CosService *cos_service.Service

// InitCos 根据系统配置初始化 COS 服务。
func InitCos() {
	service, err := cos_service.NewCOSService(cos_service.Config{
		BucketURL: config.Cfg.CosBucketUrl,
		SecretID:  config.Cfg.CosSecretId,
		SecretKey: config.Cfg.CosSecretKey,
		CdnURL:    config.Cfg.CosCdnUrl,
	})
	if err != nil {
		logger.JM.Warnf("COS 服务未启用: %v", err)
		CosService = nil
		return
	}
	CosService = service
	logger.JM.Info("COS 服务初始化成功")
}
