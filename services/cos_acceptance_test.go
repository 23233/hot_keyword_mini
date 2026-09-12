// Package services cos_acceptance_test.go
package services

import (
	"bytes"
	"context"
	"hot_keyword/pkg/cos_service"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealCOSAcceptance 使用真实 COS 密钥执行上传、URL 生成和删除；默认跳过。
func TestRealCOSAcceptance(t *testing.T) {
	if os.Getenv("ACCEPTANCE_COS") != "1" {
		t.Skip("设置 ACCEPTANCE_COS=1 执行真实 COS 验收")
	}
	service, err := cos_service.NewCOSService(cos_service.Config{SecretID: os.Getenv("COS_SECRET_ID"), SecretKey: os.Getenv("COS_SECRET_KEY"), BucketURL: os.Getenv("COS_BUCKET_URL"), CdnURL: os.Getenv("COS_CDN_URL")})
	if err != nil {
		t.Fatal(err)
	}
	appID := os.Getenv("ACCEPTANCE_APP_ID")
	if appID == "" {
		appID = "wx7a779add6a689881"
	}
	key := "miniapps/" + appID + "/acceptance/" + time.Now().Format("20060102150405.000000000") + ".txt"
	body := []byte("hot-keyword-cos-acceptance")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := service.UploadObject(ctx, key, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	defer service.DeleteObject(context.Background(), key)
	url, err := service.FileURL(key, os.Getenv("COS_CDN_URL"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "/miniapps/"+appID+"/") {
		t.Fatal("COS 对象路径未按租户隔离")
	}
	if err := service.DeleteObject(ctx, key); err != nil {
		t.Fatal(err)
	}
}
