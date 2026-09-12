// Package services production_readiness.go
package services

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/url"
	"sort"
	"strings"
)

// ProductionReadinessReport 描述小程序生产配置是否完整，不包含任何密钥原文。
type ProductionReadinessReport struct {
	AppID   string   `json:"app_id"`
	Ready   bool     `json:"ready"`
	Missing []string `json:"missing"`
	Invalid []string `json:"invalid"`
}

// CheckProductionReadiness 检查指定租户上线前必须配置的生产参数。
func CheckProductionReadiness(appID string, cfg *config.Config) (ProductionReadinessReport, error) {
	report := ProductionReadinessReport{AppID: strings.TrimSpace(appID), Ready: true, Missing: []string{}, Invalid: []string{}}
	if report.AppID == "" || db.Mysql == nil {
		return report, fmt.Errorf("AppID 不能为空且数据库必须已初始化")
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", report.AppID).First(&app).Error; err != nil {
		return report, fmt.Errorf("读取小程序失败: %w", err)
	}
	matrix, err := CapabilityMatrixForApp(&app)
	if err != nil {
		return report, err
	}
	require := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			report.Missing = append(report.Missing, name)
		}
	}
	require("app_secret", app.AppSecret)
	if cfg == nil {
		report.Missing = append(report.Missing, "public_base_url")
	} else if _, err := cfg.PaymentNotifyURL(app.AppID); err != nil {
		report.Invalid = append(report.Invalid, "public_base_url")
	}
	if capabilityEnabled(matrix, "payment") || capabilityEnabled(matrix, "membership") {
		require("payment_mch_id", app.PaymentMchID)
		require("payment_mch_serial_no", app.PaymentMchSerialNo)
		require("payment_api_v3_key", app.PaymentAPIv3Key)
		require("payment_private_key", app.PaymentPrivateKey)
		if app.PaymentAPIv3Key != "" && len(app.PaymentAPIv3Key) != 32 {
			report.Invalid = append(report.Invalid, "payment_api_v3_key")
		}
		if app.PaymentPrivateKey != "" && !validRSAPrivateKey(app.PaymentPrivateKey) {
			report.Invalid = append(report.Invalid, "payment_private_key")
		}
	}
	if capabilityEnabled(matrix, "media_upload") {
		require("cos_cdn_url", app.CosCdnUrl)
		if !validHTTPSRoot(app.CosCdnUrl) {
			report.Invalid = append(report.Invalid, "cos_cdn_url")
		}
		if cfg == nil || cfg.ValidateCOS() != nil {
			report.Invalid = append(report.Invalid, "cos_service")
		}
	}
	if capabilityEnabled(matrix, "webview") {
		entries, err := ListWebViews(app.AppID)
		if err != nil || len(entries) == 0 {
			report.Missing = append(report.Missing, "webview_registry")
		}
	}
	if capabilityEnabled(matrix, "location") {
		require("tencent_map_key", app.TencentMapKey)
	}
	if capabilityEnabled(matrix, "wechat_customer_service") {
		require("customer_service_corp_id", app.CustomerServiceCorpID)
		require("customer_service_url", app.CustomerServiceURL)
		if app.CustomerServiceURL != "" && !validHTTPSRoot(app.CustomerServiceURL) {
			report.Invalid = append(report.Invalid, "customer_service_url")
		}
	}
	if capabilityEnabled(matrix, "subscribe_message") {
		require("subscribe_template_ids", app.SubscribeTemplateIDs)
	}
	if capabilityEnabled(matrix, "ads") {
		require("ad_unit_ids", app.AdUnitIDs)
	}
	sort.Strings(report.Missing)
	sort.Strings(report.Invalid)
	report.Ready = len(report.Missing) == 0 && len(report.Invalid) == 0
	return report, nil
}

func capabilityEnabled(matrix map[string]models.CapabilityMatrixEntry, key string) bool {
	return matrix[key].State == "enabled"
}

func validHTTPSRoot(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validRSAPrivateKey(value string) bool {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return false
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		_, ok := key.(*rsa.PrivateKey)
		return ok
	}
	_, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	return err == nil
}
