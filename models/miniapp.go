// Package models miniapp.go
package models

import (
	"time"
)

// MiniApp 表示多小程序应用主体配置 (支持一脑多控与多租户隔离)
type MiniApp struct {
	// 小程序 AppID (主键，如 wx516563cfe994bbc6)
	AppID string `gorm:"column:app_id;primaryKey;size:64;comment:小程序AppID" json:"app_id"`
	// 小程序密钥 Secret (换取 openid 与 session_key)
	AppSecret string `gorm:"column:app_secret;size:64;not null;comment:小程序密钥Secret" json:"-"`
	// 小程序名称 (当前蹭的上升指数词/主体名称)
	AppName string `gorm:"column:app_name;size:128;not null;comment:小程序名称" json:"app_name"`
	// 当前线上激活展示的主页ID (默认 home)
	CurrentPage string `gorm:"column:current_page;size:64;default:'home';comment:当前线上激活的主页ID" json:"current_page"`
	// 发布模式: normal(正常模式) / gray(灰度体验) / fallback(应急兜底)
	ReleaseMode string `gorm:"column:release_mode;size:16;default:'normal';comment:发布模式" json:"release_mode"`
	// 故障或过期时的兜底页面ID (默认 home)
	FallbackPageID string `gorm:"column:fallback_page_id;size:64;default:'home';comment:兜底页面ID" json:"fallback_page_id"`
	// 租户能力矩阵 JSON；空值兼容旧租户，配置后启用严格发布门禁。
	CapabilityMatrix string `gorm:"column:capability_matrix;type:text;comment:SDUI租户能力矩阵JSON" json:"capability_matrix,omitempty"`
	// 已登记 WebView 入口 JSON；仅保存 url_key、HTTPS 地址、用途、版本和启用状态。
	WebViewRegistry string `gorm:"column:webview_registry;type:text;comment:WebView登记入口JSON" json:"webview_registry,omitempty"`
	// 小程序图片访问 CDN 根地址；需加入微信 downloadFile 合法域名
	CosCdnUrl string `gorm:"column:cos_cdn_url;size:255;comment:小程序COS CDN访问根地址" json:"cos_cdn_url"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	// 微信支付普通商户号
	PaymentMchID string `gorm:"column:payment_mch_id;size:32;comment:微信支付商户号" json:"-"`
	// 微信支付商户证书序列号
	PaymentMchSerialNo string `gorm:"column:payment_mch_serial_no;size:64;comment:微信支付商户证书序列号" json:"-"`
	// 微信支付 API v3 密钥
	PaymentAPIv3Key string `gorm:"column:payment_api_v3_key;size:64;comment:微信支付APIv3密钥" json:"-"`
	// 微信支付商户 API 私钥 PEM 内容
	PaymentPrivateKey string `gorm:"column:payment_private_key;type:text;comment:微信支付商户私钥" json:"-"`
	// 腾讯地图 WebService Key。
	TencentMapKey string `gorm:"column:tencent_map_key;size:128;comment:腾讯地图Key" json:"-"`
	// 微信客服企业 ID。
	CustomerServiceCorpID string `gorm:"column:customer_service_corp_id;size:128;comment:微信客服企业ID" json:"-"`
	// 微信客服链接。
	CustomerServiceURL string `gorm:"column:customer_service_url;size:512;comment:微信客服链接" json:"-"`
	// 订阅消息模板 ID，多个值使用英文逗号分隔。
	SubscribeTemplateIDs string `gorm:"column:subscribe_template_ids;type:text;comment:订阅消息模板ID" json:"-"`
	// 微信流量主广告位 ID，多个值使用英文逗号分隔。
	AdUnitIDs string `gorm:"column:ad_unit_ids;type:text;comment:微信广告位ID" json:"-"`
}

// TableName 自定义 MiniApp 模型的表名
func (m *MiniApp) TableName() string {
	return "mini_apps"
}
