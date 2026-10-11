// Package system migrate_test.go
package system

import (
	"hot_keyword/config"
	"hot_keyword/db"
	"strings"
	"testing"
)

// legacyArticlesDDL 模拟未升级的旧生产表结构（无组合唯一索引，允许写入脏数据），按验收方言返回对应 DDL。
func legacyArticlesDDL(dialect string) string {
	if dialect == config.DBDriverPostgres {
		return `CREATE TABLE articles (
	id BIGSERIAL PRIMARY KEY,
	app_id VARCHAR(64) NOT NULL DEFAULT '',
	slug VARCHAR(128) NOT NULL DEFAULT '',
	title VARCHAR(255) NOT NULL DEFAULT '',
	markdown TEXT,
	status VARCHAR(16) DEFAULT 'draft',
	created_at TIMESTAMP,
	updated_at TIMESTAMP
)`
	}
	return `CREATE TABLE articles (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	app_id VARCHAR(64) NOT NULL DEFAULT '',
	slug VARCHAR(128) NOT NULL DEFAULT '',
	title VARCHAR(255) NOT NULL DEFAULT '',
	markdown LONGTEXT,
	status VARCHAR(16) DEFAULT 'draft',
	created_at DATETIME,
	updated_at DATETIME
)`
}

// TestPreflightCompositeUniqueConflicts 在真实数据库 (ACCEPTANCE_DB=mysql|pg) 上验证组合唯一索引预检：
// 空库跳过、干净存量表通过、同租户重复与空 app_id 遗留行被拦截且错误信息可读。
// 只使用随机命名的临时库，结束后删除。
func TestPreflightCompositeUniqueConflicts(t *testing.T) {
	dialect, err := db.AcceptanceDialect()
	if err != nil {
		t.Fatal(err)
	}
	if dialect == "" {
		t.Skip("设置 ACCEPTANCE_DB=mysql 或 ACCEPTANCE_DB=pg 执行独立数据库验收")
	}
	cfg, err := db.AcceptanceTestConfig("../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	testDB, cleanup, err := db.OpenAcceptanceTestDB(cfg, dialect)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	sqlDB, _ := testDB.DB()
	defer sqlDB.Close()

	// 场景一：空库（表不存在）应直接跳过预检
	original := db.DB
	db.DB = testDB
	defer func() { db.DB = original }()
	if err := preflightCompositeUniqueConflicts(); err != nil {
		t.Fatalf("空库预检应直接跳过: %v", err)
	}

	// 场景二：干净的旧结构存量表应通过预检
	if err := testDB.Exec(legacyArticlesDDL(dialect)).Error; err != nil {
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
