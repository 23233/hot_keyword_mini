package system

import (
	"gorm_template/db"
	"gorm_template/models"
	"log"
)

// Migrate 数据库迁移
func Migrate() error {

	migrateList := []any{
		&models.User{},
	}

	err := db.Mysql.AutoMigrate(migrateList...)
	if err != nil {
		log.Fatalf("无法自动迁移数据库: %v", err)
		return err
	}

	return nil
}
