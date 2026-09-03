package db

import (
	"fmt"
	"gorm_template/config"
	"log"

	"github.com/23233/ggg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Mysql *gorm.DB

// InitDB 初始化数据库连接并执行自动迁移
func InitDB(cfg *config.Config) error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	Mysql, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("无法连接到数据库 at %s:%s/%s: %v", cfg.DBHost, cfg.DBPort, cfg.DBName, err)
		return err
	}

	logger.JM.Infof("mysql 连接成功")
	return nil
}
