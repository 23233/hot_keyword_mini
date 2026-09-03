package config

import (
	"github.com/23233/ggg/logger"
	"github.com/spf13/viper"
)

// Config 定义了应用的配置结构体
type Config struct {
	DBHost     string `yaml:"db_host" mapstructure:"db_host"`
	DBPort     string `yaml:"db_port" mapstructure:"db_port"`
	DBUser     string `yaml:"db_user" mapstructure:"db_user"`
	DBPassword string `yaml:"db_password" mapstructure:"db_password"`
	DBName     string `yaml:"db_name" mapstructure:"db_name"`

	RedisHost     string `yaml:"redis_host" mapstructure:"redis_host"`
	RedisPort     string `yaml:"redis_port" mapstructure:"redis_port"`
	RedisPassword string `yaml:"redis_password" mapstructure:"redis_password"`
	RedisDB       int    `yaml:"redis_db" mapstructure:"redis_db"`
	RedisEnable   bool   `yaml:"redis_enable" mapstructure:"redis_enable"`

	GoogleClientID     string `yaml:"google_client_id" mapstructure:"google_client_id"`
	GoogleClientSecret string `yaml:"google_client_secret" mapstructure:"google_client_secret"`
	GoogleRedirectURL  string `yaml:"google_redirect_url" mapstructure:"google_redirect_url"`
}

func (c *Config) GetDefaultLang() string {
	return "zh-CN"
}

var Cfg *Config

// LoadConfig 从指定路径的 YAML 文件加载配置
// 如果文件不存在或无法解析，则返回错误
func LoadConfig() error {
	// 文件存在，解析配置
	Cfg = new(Config)

	viper.AutomaticEnv()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.JM.Infof("无法读取配置文件: %v\n", err)
		return err
	}

	return viper.Unmarshal(Cfg)
}
