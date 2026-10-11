// Package db acceptance.go
package db

import (
	"errors"
	"fmt"
	"hot_keyword/config"
	"net"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// AcceptanceDialect 返回集成验收测试要求的目标数据库方言 (mysql / pg)；未要求执行验收时返回空串。
// 兼容旧的 ACCEPTANCE_MYSQL=1 写法，等价于 ACCEPTANCE_DB=mysql。
func AcceptanceDialect() (string, error) {
	dialect := strings.ToLower(strings.TrimSpace(os.Getenv("ACCEPTANCE_DB")))
	if dialect == "" {
		if os.Getenv("ACCEPTANCE_MYSQL") == "1" {
			return config.DBDriverMySQL, nil
		}
		return "", nil
	}
	if dialect != config.DBDriverMySQL && dialect != config.DBDriverPostgres {
		return "", fmt.Errorf("验收环境变量 ACCEPTANCE_DB 仅支持 %s 或 %s，当前值: %s", config.DBDriverMySQL, config.DBDriverPostgres, dialect)
	}
	return dialect, nil
}

// AcceptanceTestConfig 从指定 YAML 配置文件读取数据库连接参数，并应用标准 DB_* 环境变量覆盖，
// 供集成验收测试在不修改配置文件的情况下切换目标数据库实例。非 development 配置拒绝执行验收。
func AcceptanceTestConfig(configFile string) (*config.Config, error) {
	v := viper.New()
	v.SetConfigFile(configFile)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(v.GetString("app_env"))) != "development" {
		return nil, errors.New("仅允许开发配置执行数据库验收")
	}
	cfg := &config.Config{
		DBHost:     v.GetString("db_host"),
		DBPort:     v.GetString("db_port"),
		DBUser:     v.GetString("db_user"),
		DBPassword: v.GetString("db_password"),
		DBName:     v.GetString("db_name"),
	}
	for _, item := range []struct {
		name string
		dst  *string
	}{
		{"DB_HOST", &cfg.DBHost},
		{"DB_PORT", &cfg.DBPort},
		{"DB_USER", &cfg.DBUser},
		{"DB_PASSWORD", &cfg.DBPassword},
	} {
		if value := os.Getenv(item.name); value != "" {
			*item.dst = value
		}
	}
	return cfg, nil
}

// OpenAcceptanceTestDB 集成验收测试专用支撑：按方言在真实数据库实例上创建随机命名的临时库，
// 返回临时库的 GORM 连接与清理函数 (删除临时库)，调用方必须 defer 清理。业务代码禁止调用。
func OpenAcceptanceTestDB(cfg *config.Config, dialect string) (*gorm.DB, func(), error) {
	// 与生产连接一致的 TranslateError 保证验收能覆盖 gorm.ErrDuplicatedKey 等统一错误判断
	silentConfig := func() *gorm.Config {
		return &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true}
	}
	name := fmt.Sprintf("hk_acceptance_%d", time.Now().UnixNano())
	switch dialect {
	case config.DBDriverMySQL:
		c := mysqlDriver.NewConfig()
		c.User, c.Passwd = cfg.DBUser, cfg.DBPassword
		c.Net, c.Addr = "tcp", net.JoinHostPort(cfg.DBHost, cfg.DBPort)
		c.ParseTime = true
		c.Timeout = 10 * time.Second
		admin, err := gorm.Open(gormmysql.Open(c.FormatDSN()), silentConfig())
		if err != nil {
			return nil, nil, fmt.Errorf("连接验收 MySQL 实例失败: %w", err)
		}
		if err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4").Error; err != nil {
			return nil, nil, fmt.Errorf("创建 MySQL 临时验收库失败: %w", err)
		}
		c.DBName = name
		testDB, err := gorm.Open(gormmysql.Open(c.FormatDSN()), silentConfig())
		if err != nil {
			_ = admin.Exec("DROP DATABASE `" + name + "`")
			return nil, nil, fmt.Errorf("连接 MySQL 临时验收库失败: %w", err)
		}
		return testDB, func() { _ = admin.Exec("DROP DATABASE `" + name + "`") }, nil
	case config.DBDriverPostgres:
		credentials := fmt.Sprintf("host=%s port=%s user=%s password=%s", quotePGParam(cfg.DBHost), quotePGParam(cfg.DBPort), quotePGParam(cfg.DBUser), quotePGParam(cfg.DBPassword))
		admin, err := gorm.Open(postgres.Open(credentials+" dbname=postgres sslmode=disable TimeZone=UTC"), silentConfig())
		if err != nil {
			return nil, nil, fmt.Errorf("连接验收 PostgreSQL 实例失败: %w", err)
		}
		// 临时库名为小写字母数字下划线，无需引号即可保证双方言合法
		if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
			return nil, nil, fmt.Errorf("创建 PostgreSQL 临时验收库失败: %w", err)
		}
		testDB, err := gorm.Open(postgres.Open(credentials+" dbname="+quotePGParam(name)+" sslmode=disable TimeZone=UTC"), silentConfig())
		if err != nil {
			_ = admin.Exec("DROP DATABASE " + name)
			return nil, nil, fmt.Errorf("连接 PostgreSQL 临时验收库失败: %w", err)
		}
		return testDB, func() { _ = admin.Exec("DROP DATABASE " + name) }, nil
	default:
		return nil, nil, fmt.Errorf("不支持的验收数据库方言: %s", dialect)
	}
}
