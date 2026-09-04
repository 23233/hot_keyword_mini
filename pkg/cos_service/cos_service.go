// Package cos_service cos_service.go
package cos_service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// Config 定义腾讯云 COS 客户端配置。
type Config struct {
	// BucketURL 是存储桶 HTTPS 根地址。
	BucketURL string
	// SecretID 是腾讯云 API SecretId。
	SecretID string
	// SecretKey 是腾讯云 API SecretKey。
	SecretKey string
	// CDNURL 是未配置小程序专属域名时使用的默认 CDN 地址。
	CDNURL string
	// CdnURL 兼容参考项目的字段命名。
	CdnURL string
}

// Service 封装 COS 预签名上传能力。
type Service struct {
	client *cos.Client
	config Config
}

// NewCOSService 创建 COS 服务。
func NewCOSService(cfg Config) (*Service, error) {
	if strings.TrimSpace(cfg.BucketURL) == "" || strings.TrimSpace(cfg.SecretID) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, errors.New("COS 配置不完整")
	}
	bucketURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.BucketURL), "/"))
	if err != nil || bucketURL.Scheme != "https" || bucketURL.Host == "" {
		return nil, errors.New("COS 存储桶地址无效")
	}
	if cfg.CDNURL == "" {
		cfg.CDNURL = cfg.CdnURL
	}
	cfg.CdnURL = cfg.CDNURL

	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &Service{client: client, config: cfg}, nil
}

// IsConfigured 判断 COS 服务是否可用。
func (s *Service) IsConfigured() bool {
	return s != nil && s.client != nil
}

// GeneratePresignedUploadURL 生成指定对象的预签名上传地址。
func (s *Service) GeneratePresignedUploadURL(ctx context.Context, fileKey, httpMethod, contentType string, durationMinutes int) (string, error) {
	if !s.IsConfigured() {
		return "", errors.New("COS 服务未初始化")
	}
	fileKey = strings.TrimLeft(strings.TrimSpace(fileKey), "/")
	if fileKey == "" {
		return "", errors.New("COS 文件键不能为空")
	}
	if httpMethod == "" {
		httpMethod = http.MethodPut
	}
	if durationMinutes <= 0 {
		durationMinutes = 10
	}

	headers := &http.Header{}
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	signedURL, err := s.client.Object.GetPresignedURL(ctx, httpMethod, fileKey, s.config.SecretID, s.config.SecretKey, time.Duration(durationMinutes)*time.Minute, &cos.PresignedURLOptions{Header: headers})
	if err != nil {
		return "", fmt.Errorf("生成 COS 上传地址失败: %w", err)
	}
	return signedURL.String(), nil
}

// GetConfig 返回当前 COS 配置，用于需要读取 CDN 根地址的调用方。
func (s *Service) GetConfig() Config {
	if s == nil {
		return Config{}
	}
	return s.config
}

// UploadObject 将服务端生成的文件写入指定 COS 对象键。
func (s *Service) UploadObject(ctx context.Context, fileKey string, content io.Reader, fileSize int64, contentType string) error {
	if !s.IsConfigured() {
		return errors.New("COS 服务未初始化")
	}
	fileKey = strings.TrimLeft(strings.TrimSpace(fileKey), "/")
	if fileKey == "" || content == nil {
		return errors.New("COS 文件参数不完整")
	}
	_, err := s.client.Object.Put(ctx, fileKey, content, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentLength: fileSize,
			ContentType:   contentType,
		},
	})
	if err != nil {
		return fmt.Errorf("上传文件到 COS 失败: %w", err)
	}
	return nil
}

// UploadFile 将文件上传到 COS 并返回 CDN 地址和对象键。
func (s *Service) UploadFile(ctx context.Context, content io.Reader, fileSize int64, originalFileName string) (string, string, error) {
	extension := ""
	if dot := strings.LastIndex(originalFileName, "."); dot >= 0 {
		extension = originalFileName[dot:]
	}
	fileKey := "miniapps/resources/" + uuid.NewString() + extension
	if err := s.UploadObject(ctx, fileKey, content, fileSize, "application/octet-stream"); err != nil {
		return "", "", err
	}
	fileURL, err := s.FileURL(fileKey, "")
	if err != nil {
		return "", "", err
	}
	return fileURL, fileKey, nil
}

// DeleteObject 删除指定 COS 对象。
func (s *Service) DeleteObject(ctx context.Context, fileKey string) error {
	if !s.IsConfigured() {
		return errors.New("COS 服务未初始化")
	}
	fileKey = strings.TrimLeft(strings.TrimSpace(fileKey), "/")
	if fileKey == "" {
		return errors.New("COS 文件键不能为空")
	}
	_, err := s.client.Object.Delete(ctx, fileKey)
	if err != nil {
		return fmt.Errorf("删除 COS 对象失败: %w", err)
	}
	return nil
}

// FileURL 根据对象键生成小程序可访问的 CDN 地址。
func (s *Service) FileURL(fileKey, cdnURL string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(cdnURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(s.config.CDNURL), "/")
	}
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(s.config.BucketURL), "/")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("当前小程序未配置有效的 HTTPS CDN 或 COS 地址")
	}
	return base + "/" + strings.TrimLeft(fileKey, "/"), nil
}
