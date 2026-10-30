// Package db database.go
package db

import (
	"fmt"
	"hot_keyword/config"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/23233/ggg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB 全局数据库句柄，具体指向的数据库类型由配置 db_driver 决定 (mysql / pg)
var DB *gorm.DB

// InitDB 按配置的数据库驱动初始化连接，业务层统一通过 DB 句柄访问，对具体数据库类型无感
func InitDB(cfg *config.Config) error {
	var err error
	var dialector gorm.Dialector

	// 主流程：按驱动类型构建方言，再以统一的 GORM 配置打开连接
	switch cfg.DBDriver {
	case config.DBDriverPostgres:
		dialector = openPostgres(cfg)
	case config.DBDriverMySQL:
		dialector = openMySQL(cfg)
	default:
		return fmt.Errorf("不支持的数据库驱动类型: %s", cfg.DBDriver)
	}

	// TranslateError 开启后，各驱动会把方言错误翻译为统一 GORM 错误
	// (MySQL 1062 与 PG 23505 均映射为 gorm.ErrDuplicatedKey)，业务层判断错误无需关心具体数据库
	DB, err = gorm.Open(dialector, &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatalf("无法连接到数据库 (driver=%s): %v", cfg.DBDriver, err)
		return err
	}

	// 统一配置连接池，避免默认无上限连接拖垮数据库。
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", 30))
	sqlDB.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", 10))
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	logger.JM.Infof("%s 连接成功", cfg.DBDriver)
	return nil
}

// envInt 读取正整数环境变量，非法或未配置时返回默认值。
func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// openMySQL 构建 MySQL 方言连接配置，保持原有 DSN 格式与时区 (UTC) 行为不变
func openMySQL(cfg *config.Config) gorm.Dialector {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)
	return mysql.Open(dsn)
}

// openPostgres 构建 PostgreSQL 方言连接配置；TimeZone=UTC 与 MySQL 的 loc=UTC 语义对齐
func openPostgres(cfg *config.Config) gorm.Dialector {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		quotePGParam(cfg.DBHost),
		quotePGParam(cfg.DBPort),
		quotePGParam(cfg.DBUser),
		quotePGParam(cfg.DBPassword),
		quotePGParam(cfg.DBName),
	)
	return postgres.New(postgres.Config{DSN: dsn})
}

// quotePGParam 对 PG 关键字值连接串中的参数做单引号转义，兼容密码中的空格、引号与反斜杠
func quotePGParam(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}
