// Package services wechat_audit.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type wechatAccessToken struct {
	Token     string
	ExpiresAt time.Time
}

var wechatAuditTokenCache = struct {
	sync.Mutex
	Items map[string]wechatAccessToken
}{Items: make(map[string]wechatAccessToken)}

// WechatAuditResult 微信内容安全审核结果。
type WechatAuditResult struct {
	Status  string
	TraceID string
	Reason  string
}

// WechatAuditService 微信文本与图片内容安全审核适配器。
type WechatAuditService struct {
	client *http.Client
}

// NewWechatAuditService 创建微信内容安全审核服务。
func NewWechatAuditService() *WechatAuditService {
	return &WechatAuditService{client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *WechatAuditService) accessToken(ctx context.Context, appID string) (string, error) {
	wechatAuditTokenCache.Lock()
	cached := wechatAuditTokenCache.Items[appID]
	wechatAuditTokenCache.Unlock()
	if cached.Token != "" && time.Now().Before(cached.ExpiresAt) { return cached.Token, nil }
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil { return "", errors.New("小程序配置不存在") }
	if strings.TrimSpace(app.AppSecret) == "" { return "", errors.New("当前小程序未配置 AppSecret，无法调用微信内容审核") }
	endpoint := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(appID) + "&secret=" + url.QueryEscape(app.AppSecret)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	resp, err := s.client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	var result struct { AccessToken string `json:"access_token"`; ExpiresIn int `json:"expires_in"`; ErrCode int `json:"errcode"`; ErrMsg string `json:"errmsg"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return "", err }
	if result.ErrCode != 0 || result.AccessToken == "" { return "", fmt.Errorf("微信 access_token 获取失败: %d %s", result.ErrCode, result.ErrMsg) }
	expires := time.Duration(result.ExpiresIn-120) * time.Second
	if expires <= 0 { expires = time.Hour }
	wechatAuditTokenCache.Lock()
	wechatAuditTokenCache.Items[appID] = wechatAccessToken{Token: result.AccessToken, ExpiresAt: time.Now().Add(expires)}
	wechatAuditTokenCache.Unlock()
	return result.AccessToken, nil
}

// CheckText 同步调用微信文本安全审核。
func (s *WechatAuditService) CheckText(ctx context.Context, appID, openID, content string) (*WechatAuditResult, error) {
	token, err := s.accessToken(ctx, appID)
	if err != nil { return nil, err }
	body, _ := json.Marshal(map[string]interface{}{"content": content, "version": 2, "scene": 2, "openid": openID})
	endpoint := "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=" + url.QueryEscape(token)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	var result struct { ErrCode int `json:"errcode"`; ErrMsg string `json:"errmsg"`; TraceID string `json:"trace_id"`; Result struct { Suggest string `json:"suggest"`; Label int `json:"label"` } `json:"result"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
	if result.ErrCode != 0 { return nil, fmt.Errorf("微信文本审核失败: %d %s", result.ErrCode, result.ErrMsg) }
	status := "rejected"
	if result.Result.Suggest == "pass" { status = "approved" } else if result.Result.Suggest == "review" { status = "pending" }
	return &WechatAuditResult{Status: status, TraceID: result.TraceID, Reason: fmt.Sprintf("suggest=%s,label=%d", result.Result.Suggest, result.Result.Label)}, nil
}

// CheckImageAsync 调用微信图片异步安全审核，回调前保持 pending。
func (s *WechatAuditService) CheckImageAsync(ctx context.Context, appID, openID, imageURL string) (*WechatAuditResult, error) {
	token, err := s.accessToken(ctx, appID)
	if err != nil { return nil, err }
	body, _ := json.Marshal(map[string]interface{}{"media_url": imageURL, "media_type": 2, "version": 2, "scene": 2, "openid": openID})
	endpoint := "https://api.weixin.qq.com/wxa/media_check_async?access_token=" + url.QueryEscape(token)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	var result struct { ErrCode int `json:"errcode"`; ErrMsg string `json:"errmsg"`; TraceID string `json:"trace_id"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
	if result.ErrCode != 0 || result.TraceID == "" { return nil, fmt.Errorf("微信图片审核失败: %d %s", result.ErrCode, result.ErrMsg) }
	return &WechatAuditResult{Status: "pending", TraceID: result.TraceID}, nil
}
