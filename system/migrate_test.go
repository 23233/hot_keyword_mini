// Package system migrate_test.go
package system

import (
	"database/sql"
	"fmt"
	"hot_keyword/db"
	"os"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// legacyArticlesDDL 模拟未升级的旧生产表结构（无组合唯一索引，允许写入脏数据）
const legacyArticlesDDL = `
CREATE TABLE articles (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	app_id VARCHAR(64) NOT NULL DEFAULT '',
	slug VARCHAR(128) NOT NULL DEFAULT '',
	title VARCHAR(255) NOT NULL DEFAULT '',
	markdown LONGTEXT,
	status VARCHAR(16) DEFAULT 'draft',
	created_at DATETIME,
	updated_at DATETIME
)`

// TestPreflightCompositeUniqueConflicts 在真实 MySQL 上验证组合唯一索引预检：
// 空库跳过、干净存量表通过、同租户重复与空 app_id 遗留行被拦截且错误信息可读。
// 设置 ACCEPTANCE_MYSQL=1 执行；只使用随机命名的临时库，结束后删除。
func TestPreflightCompositeUniqueConflicts(t *testing.T) {
	if os.Getenv("ACCEPTANCE_MYSQL") != "1" {
		t.Skip("设置 ACCEPTANCE_MYSQL=1 执行独立数据库验收")
	}
	v := viper.New()
	v.SetConfigFile("../config.yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if v.GetString("app_env") != "development" {
		t.Fatal("仅允许开发配置执行验收")
	}
	c := mysqlDriver.NewConfig()
	c.User, c.Passwd = v.GetString("db_user"), v.GetString("db_password")
	c.Net, c.Addr = "tcp", v.GetString("db_host")+":"+v.GetString("db_port")
	c.ParseTime = true
	c.Timeout = 10 * time.Second

	// 1. 使用无库名连接创建随机临时验收库，结束后删除，不触碰已有业务库
	conn, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		t.Fatal("创建数据库连接失败")
	}
	defer conn.Close()
	name := fmt.Sprintf("hk_preflight_%d", time.Now().UnixNano())
	if _, err := conn.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		t.Fatalf("创建随机验收库失败: %v", err)
	}
	defer func() {
		_, _ = conn.Exec("DROP DATABASE `" + name + "`")
	}()

	c.DBName = name
	testDB, err := gorm.Open(gormmysql.Open(c.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("连接随机验收库失败: %v", err)
	}

	// 场景一：空库（表不存在）应直接跳过预检
	original := db.Mysql
	db.Mysql = testDB
	defer func() { db.Mysql = original }()
	if err := preflightCompositeUniqueConflicts(); err != nil {
		t.Fatalf("空库预检应直接跳过: %v", err)
	}

	// 场景二：干净的旧结构存量表应通过预检
	if err := testDB.Exec(legacyArticlesDDL).Error; err != nil {
		t.Fatalf("初始化旧结构表失败: %v", err)
	}
	if err := testDB.Exec("INSERT INTO articles (app_id, slug, title, status) VALUES ('wx516563cfe994bbc6', 'only-one', '正常行', 'published')").Error; err != nil {
		t.Fatalf("写入正常数据失败: %v", err)
	}
	if err := preflightCompositeUniqueConflicts(); err != nil {
		t.Fatalf("干净存量表预检应通过: %v", err)
	}

	// 场景三：同租户重复 slug 应被拦截且错误信息包含表名
	if err := testDB.Exec("INSERT INTO articles (app_id, slug, title, status) VALUES ('wx516563cfe994bbc6', 'dup', '重复一', 'published')").Error; err != nil {
		t.Fatalf("写入重复脏数据失败: %v", err)
	}
	if err := testDB.Exec("INSERT INTO articles (app_id, slug, title, status) VALUES ('wx516563cfe994bbc6', 'dup', '重复二', 'published')").Error; err != nil {
		t.Fatalf("写入重复脏数据失败: %v", err)
	}
	err = preflightCompositeUniqueConflicts()
	if err == nil {
		t.Fatal("同租户重复数据应被预检拦截")
	}
	if !strings.Contains(err.Error(), "articles") || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("错误信息应指出具体表名与重复问题: %v", err)
	}

	// 场景四：app_id 为空的遗留行应被拦截
	if err := testDB.Exec("DELETE FROM articles").Error; err != nil {
		t.Fatalf("清理重复数据失败: %v", err)
	}
	if err := testDB.Exec("INSERT INTO articles (app_id, slug, title, status) VALUES ('', 'legacy', '遗留行', 'published')").Error; err != nil {
		t.Fatalf("写入空 app_id 遗留行失败: %v", err)
	}
	err = preflightCompositeUniqueConflicts()
	if err == nil {
		t.Fatal("app_id 为空的遗留行应被预检拦截")
	}
	if !strings.Contains(err.Error(), "app_id 为空") {
		t.Fatalf("错误信息应指出 app_id 空值问题: %v", err)
	}
}
