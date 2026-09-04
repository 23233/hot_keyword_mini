// Package services payment_test.go
package services

import (
	"errors"
	"hot_keyword/config"
	"hot_keyword/models"
	"sync"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm/schema"
)

func TestValidatePaymentNotifyURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "https", url: "https://pay.example.com", ok: true},
		{name: "http", url: "http://pay.example.com", ok: false},
		{name: "query", url: "https://pay.example.com/notify?app_id=wx-demo", ok: false},
		{name: "fragment", url: "https://pay.example.com/notify#x", ok: false},
		{name: "empty", url: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{PublicBaseURL: tt.url}
			got, err := cfg.PaymentNotifyURL("wx-demo")
			if (err == nil) != tt.ok || (tt.ok && got != tt.url+"/api/v1/payment/notify/wx-demo") {
				t.Fatalf("PaymentNotifyURL(%q) = %q, error = %v, want valid = %v", tt.url, got, err, tt.ok)
			}
		})
	}
}

func TestIsDuplicateKeyError(t *testing.T) {
	if !isDuplicateKeyError(&mysqlDriver.MySQLError{Number: 1062}) {
		t.Fatal("1062 应识别为重复键错误")
	}
	if isDuplicateKeyError(errors.New("duplicate")) {
		t.Fatal("普通错误不应识别为重复键错误")
	}
}

func TestProductSKUIndexIsPerApp(t *testing.T) {
	parsed, err := schema.Parse(&models.Product{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range parsed.ParseIndexes() {
		if index.Name != "idx_product_app_sku" {
			continue
		}
		if index.Class != "UNIQUE" || len(index.Fields) != 2 {
			t.Fatalf("商品 SKU 索引必须是 app_id + sku 联合唯一索引，实际为 class=%s fields=%d", index.Class, len(index.Fields))
		}
		return
	}
	t.Fatal("未找到商品联合唯一索引")
}
