// Package config config.go
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/23233/ggg/logger"
	"github.com/spf13/viper"
)

// Config 定义了应用的配置结构体
type Config struct {
	// 数据库主机地址
	DBHost string `yaml:"db_host" mapstructure:"db_host"`
	// 数据库端口
	DBPort string `yaml:"db_port" mapstructure:"db_port"`
	// 数据库用户名
	DBUser string `yaml:"db_user" mapstructure:"db_user"`
	// 数据库密码 (支持 DB_PASSWORD 环境变量覆盖)
	DBPassword string `yaml:"db_password" mapstructure:"db_password"`
	// 数据库名称
	DBName string `yaml:"db_name" mapstructure:"db_name"`

	// Redis 主机地址
	RedisHost string `yaml:"redis_host" mapstructure:"redis_host"`
	// Redis 端口
	RedisPort string `yaml:"redis_port" mapstructure:"redis_port"`
	// Redis 访问密码 (支持 REDIS_PASSWORD 环境变量覆盖)
	RedisPassword string `yaml:"redis_password" mapstructure:"redis_password"`
	// Redis 数据库序号
	RedisDB int `yaml:"redis_db" mapstructure:"redis_db"`
	// 是否开启 Redis
	RedisEnable bool `yaml:"redis_enable" mapstructure:"redis_enable"`

	// 应用运行环境 (development / staging / production，默认 development)
	AppEnv string `yaml:"app_env" mapstructure:"app_env"`
	// 服务对外访问的公共根地址；生产环境必须配置 HTTPS 地址
	PublicBaseURL string `yaml:"public_base_url" mapstructure:"public_base_url"`

	// Google 客户端配置 (可选)
	GoogleClientID     string `yaml:"google_client_id" mapstructure:"google_client_id"`
	GoogleClientSecret string `yaml:"google_client_secret" mapstructure:"google_client_secret"`
	GoogleRedirectURL  string `yaml:"google_redirect_url" mapstructure:"google_redirect_url"`
}

func (c *Config) GetDefaultLang() string {
	return "zh-CN"
}

// IsProduction 判断当前是否为生产发布环境
func (c *Config) IsProduction() bool {
	if c == nil {
		return false
	}
	env := c.AppEnv
	return env == "production" || env == "prod" || env == "release"
}

// PaymentNotifyURL 根据当前公共域名和小程序 AppID 生成微信支付回调地址。
func (c *Config) PaymentNotifyURL(appID string) (string, error) {
	if c == nil || strings.TrimSpace(appID) == "" {
		return "", errors.New("支付回调地址参数不完整")
	}
	base := strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("公共服务地址必须是 HTTPS 且不能包含查询参数")
	}
	return fmt.Sprintf("%s/api/v1/payment/notify/%s", base, url.PathEscape(appID)), nil
}

var Cfg *Config

// isWeakOrPlaceholderCredential 检查给定的凭据是否为空、弱密码或占位符
func isWeakOrPlaceholderCredential(val string) bool {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return true
	}
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "CHANGEME") || strings.HasPrefix(upper, "YOUR_SECURE") ||
		strings.Contains(upper, "CHANGEME") || strings.Contains(upper, "REPLACE_IN_PRODUCTION") ||
		upper == "ROOT" || upper == "ADMIN" || upper == "123456" || upper == "PASSWORD" {
		return true
	}
	return false
}

// CheckSecurityCredentials 检查关键凭据安全性，生产环境缺失或使用弱凭据则强制阻断启动
func CheckSecurityCredentials() error {
	if Cfg == nil {
		return nil
	}
	if Cfg.IsProduction() {
		if _, err := Cfg.PaymentNotifyURL("wx2e8feeb13a20fb1b"); err != nil {
			return fmt.Errorf("【生产环境致命配置错误】PUBLIC_BASE_URL 必须配置当前线上 HTTPS 域名: %w", err)
		}
		jwtSecret := os.Getenv("JWT_SECRET")
		if isWeakOrPlaceholderCredential(jwtSecret) || len(jwtSecret) < 16 {
			return errors.New("【生产环境致命安全错误】环境变量 JWT_SECRET 必须配置且长度不少于16位，禁止使用 CHANGEME 或默认弱密钥")
		}

		adminJwtSecret := os.Getenv("ADMIN_JWT_SECRET")
		if isWeakOrPlaceholderCredential(adminJwtSecret) || len(adminJwtSecret) < 16 {
			return errors.New("【生产环境致命安全错误】环境变量 ADMIN_JWT_SECRET 必须配置且长度不少于16位，禁止使用 CHANGEME 或默认弱密钥")
		}

		mcpAuthKey := os.Getenv("MCP_AUTH_KEY")
		if isWeakOrPlaceholderCredential(mcpAuthKey) || len(mcpAuthKey) < 16 {
			return errors.New("【生产环境致命安全错误】环境变量 MCP_AUTH_KEY 必须配置且长度不少于16位，禁止使用 CHANGEME 或默认弱密钥")
		}

		// 生产环境强制校验数据库密码
		if isWeakOrPlaceholderCredential(Cfg.DBPassword) {
			return errors.New("【生产环境致命安全错误】数据库密码 DB_PASSWORD 必须配置真实生产密码，禁止使用 CHANGEME 或占位默认值")
		}

		// 生产环境若开启 Redis，强制校验 Redis 密码
		if Cfg.RedisEnable && isWeakOrPlaceholderCredential(Cfg.RedisPassword) {
			return errors.New("【生产环境致命安全错误】Redis 密码 REDIS_PASSWORD 必须配置真实生产密码，禁止使用 CHANGEME 或占位默认值")
		}
	}
	return nil
}

// LoadConfig 从指定路径的 YAML 文件加载配置，优先采用系统环境变量
func LoadConfig() error {
	Cfg = new(Config)

	viper.AutomaticEnv()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.JM.Infof("无法读取配置文件: %v\n", err)
	}

	_ = viper.Unmarshal(Cfg)

	// 环境变量优先级覆盖环境与敏感凭据
	if envAppEnv := os.Getenv("APP_ENV"); envAppEnv != "" {
		Cfg.AppEnv = envAppEnv
	} else if env := os.Getenv("ENV"); env != "" {
		Cfg.AppEnv = env
	}
	if Cfg.AppEnv == "" {
		Cfg.AppEnv = "development"
	}

	if envDBHost := os.Getenv("DB_HOST"); envDBHost != "" {
		Cfg.DBHost = envDBHost
	}
	if envDBPort := os.Getenv("DB_PORT"); envDBPort != "" {
		Cfg.DBPort = envDBPort
	}
	if envDBUser := os.Getenv("DB_USER"); envDBUser != "" {
		Cfg.DBUser = envDBUser
	}
	if envDBPass := os.Getenv("DB_PASSWORD"); envDBPass != "" {
		Cfg.DBPassword = envDBPass
	}
	if envDBName := os.Getenv("DB_NAME"); envDBName != "" {
		Cfg.DBName = envDBName
	}
	if envPublicBaseURL := os.Getenv("PUBLIC_BASE_URL"); envPublicBaseURL != "" {
		Cfg.PublicBaseURL = envPublicBaseURL
	}
	if envRedisHost := os.Getenv("REDIS_HOST"); envRedisHost != "" {
		Cfg.RedisHost = envRedisHost
	}
	if envRedisPort := os.Getenv("REDIS_PORT"); envRedisPort != "" {
		Cfg.RedisPort = envRedisPort
	}
	if envRedisPass := os.Getenv("REDIS_PASSWORD"); envRedisPass != "" {
		Cfg.RedisPassword = envRedisPass
	}

	// 生产环境凭据合规门禁检查
	if err := CheckSecurityCredentials(); err != nil {
		logger.JM.Errorf("%s", err.Error())
		return err
	}

	return nil
}
