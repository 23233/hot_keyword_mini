// Package services cos_upload.go
package services

import (
	"context"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// COSUploadRequest 申请 COS 预签名上传地址的统一请求。
type COSUploadRequest struct {
	AppID       string
	FileName    string
	FileSize    int64
	ContentType string
	OwnerType   string
}

// COSUploadResult 返回预签名地址和最终 CDN 地址。
type COSUploadResult struct {
	PresignedURL    string            `json:"presignedUrl"`
	FinalCosFileURL string            `json:"finalCosFileUrl"`
	FileKey         string            `json:"fileKey"`
	ContentType     string            `json:"contentType"`
	UploadHeaders   map[string]string `json:"uploadHeaders"`
	ExpiresIn       int               `json:"expiresIn"`
}

// PrepareCOSUpload 生成统一的 miniapps/{app_id}/ COS 预签名上传链路。
func PrepareCOSUpload(ctx context.Context, req COSUploadRequest) (*COSUploadResult, error) {
	req.AppID = strings.TrimSpace(req.AppID)
	req.FileName = strings.TrimSpace(req.FileName)
	req.ContentType = strings.ToLower(strings.TrimSpace(req.ContentType))
	if req.AppID == "" || req.FileName == "" || req.FileSize <= 0 {
		return nil, errors.New("图片上传参数不完整")
	}
	if sdk.CosService == nil {
		return nil, errors.New("COS 服务未配置")
	}

	allowedTypes := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
		"image/gif":  ".gif",
	}
	extension, ok := allowedTypes[req.ContentType]
	if !ok {
		return nil, errors.New("仅支持 JPG、PNG、WebP 或 GIF 图片")
	}
	if req.FileSize > 10*1024*1024 {
		return nil, errors.New("图片大小不能超过 10MB")
	}
	if originalExt := strings.ToLower(path.Ext(req.FileName)); originalExt == ".jpeg" || originalExt == extension {
		extension = originalExt
	}
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化")
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", req.AppID).First(&app).Error; err != nil {
		return nil, fmt.Errorf("小程序不存在: %w", err)
	}

	ownerType := strings.ToLower(strings.TrimSpace(req.OwnerType))
	switch ownerType {
	case "drama", "sdui", "share":
	default:
		ownerType = "resources"
	}
	fileKey := fmt.Sprintf("miniapps/%s/%s/%s/%s%s", app.AppID, ownerType, time.Now().Format("2006/01"), uuid.NewString(), extension)
	const expiresInMinutes = 10
	presignedURL, err := sdk.CosService.GeneratePresignedUploadURL(ctx, fileKey, http.MethodPut, req.ContentType, expiresInMinutes)
	if err != nil {
		return nil, err
	}
	fileURL, err := sdk.CosService.FileURL(fileKey, app.CosCdnUrl)
	if err != nil {
		return nil, err
	}
	return &COSUploadResult{
		PresignedURL:    presignedURL,
		FinalCosFileURL: fileURL,
		FileKey:         fileKey,
		ContentType:     req.ContentType,
		UploadHeaders:   map[string]string{"Content-Type": req.ContentType},
		ExpiresIn:       expiresInMinutes * 60,
	}, nil
}
