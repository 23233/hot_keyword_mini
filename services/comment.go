// Package services comment.go
package services

import (
	"context"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

// CommentCreateInput 创建评论参数。
type CommentCreateInput struct {
	ArticleID int64
	ParentID  int64
	Content   string
	ImageURL  string
}

// CommentDTO 评论安全返回结构。
type CommentDTO struct {
	ID              int64     `json:"id"`
	ArticleID       int64     `json:"article_id"`
	ParentID        int64     `json:"parent_id"`
	RootID          int64     `json:"root_id"`
	Level           int       `json:"level"`
	ContentType     string    `json:"content_type"`
	Content         string    `json:"content"`
	ImageURL        string    `json:"image_url,omitempty"`
	UserID          int64     `json:"user_id"`
	Nickname        string    `json:"nickname"`
	ReplyToNickname string    `json:"reply_to_nickname,omitempty"`
	ReplyCount      int       `json:"reply_count"`
	CreatedAt       time.Time `json:"created_at"`
}

// CommentService 两级评论和内容审核服务。
type CommentService struct{ audit *WechatAuditService }

// NewCommentService 创建评论服务。
func NewCommentService() *CommentService { return &CommentService{audit: NewWechatAuditService()} }

func isTrustedCOSURL(base, imageURL string) bool {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	parsed, err := url.Parse(strings.TrimSpace(imageURL))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && base != "" && strings.HasPrefix(strings.TrimSpace(imageURL), base+"/")
}

func commentDTO(comment models.ArticleComment) CommentDTO {
	var user models.User
	_ = db.Mysql.Select("id", "nickname").First(&user, comment.UserID).Error
	return CommentDTO{ID: comment.ID, ArticleID: comment.ArticleID, ParentID: comment.ParentID, RootID: comment.RootID, Level: comment.Level, ContentType: comment.ContentType, Content: comment.Content, ImageURL: comment.ImageURL, UserID: comment.UserID, Nickname: user.NickName, ReplyToNickname: comment.ReplyToNickname, ReplyCount: comment.ReplyCount, CreatedAt: comment.CreatedAt}
}

// ListRoots 仅返回一级评论和二级数量。
func (s *CommentService) ListRoots(appID string, articleID int64, limit, offset int) ([]CommentDTO, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var comments []models.ArticleComment
	if err := db.Mysql.Where("app_id = ? AND article_id = ? AND level = 1 AND audit_status = ?", appID, articleID, "approved").Order("id desc").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		return nil, err
	}
	result := make([]CommentDTO, 0, len(comments))
	for _, comment := range comments {
		result = append(result, commentDTO(comment))
	}
	return result, nil
}

// ListReplies 按一级评论懒加载二级回复。
func (s *CommentService) ListReplies(appID string, rootID int64, limit, offset int) ([]CommentDTO, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var comments []models.ArticleComment
	if err := db.Mysql.Where("app_id = ? AND root_id = ? AND level = 2 AND audit_status = ?", appID, rootID, "approved").Order("id asc").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		return nil, err
	}
	result := make([]CommentDTO, 0, len(comments))
	for _, comment := range comments {
		result = append(result, commentDTO(comment))
	}
	return result, nil
}

// Create 创建评论并调用微信安全审核。
func (s *CommentService) Create(ctx context.Context, appID string, user *models.User, input CommentCreateInput) (*models.ArticleComment, error) {
	if user == nil || user.ID <= 0 {
		return nil, errors.New("请先完成微信登录")
	}
	var article models.Article
	if err := db.Mysql.Where("app_id = ? AND id = ? AND status = ?", appID, input.ArticleID, "published").First(&article).Error; err != nil {
		return nil, errors.New("文章不存在")
	}
	if !article.AllowComments {
		return nil, errors.New("当前文章未开放评论")
	}
	content := strings.TrimSpace(input.Content)
	imageURL := strings.TrimSpace(input.ImageURL)
	if content == "" && imageURL == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if len([]rune(content)) > 500 {
		return nil, errors.New("评论文字不能超过 500 字")
	}
	if imageURL != "" {
		var app models.MiniApp
		if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
			return nil, errors.New("小程序配置不存在")
		}
		if !isTrustedCOSURL(app.CosCdnUrl, imageURL) {
			return nil, errors.New("评论图片必须来自当前小程序 COS CDN")
		}
	}

	comment := &models.ArticleComment{AppID: appID, ArticleID: input.ArticleID, UserID: user.ID, ParentID: input.ParentID, Level: 1, ContentType: "text", Content: content, ImageURL: imageURL, AuditStatus: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if imageURL != "" {
		comment.ContentType = "image"
	}
	if input.ParentID > 0 {
		var parent models.ArticleComment
		if err := db.Mysql.Where("app_id = ? AND article_id = ? AND id = ? AND audit_status = ?", appID, input.ArticleID, input.ParentID, "approved").First(&parent).Error; err != nil {
			return nil, errors.New("回复目标不存在")
		}
		comment.Level = 2
		if parent.Level == 1 {
			comment.RootID = parent.ID
		} else {
			comment.RootID = parent.RootID
		}
		comment.ReplyToUserID = parent.UserID
		var target models.User
		_ = db.Mysql.Select("nickname").First(&target, parent.UserID).Error
		comment.ReplyToNickname = target.NickName
	}
	if err := db.Mysql.Create(comment).Error; err != nil {
		return nil, err
	}

	var audit *WechatAuditResult
	var err error
	if imageURL != "" {
		audit, err = s.audit.CheckImageAsync(ctx, appID, user.WechatOpenID, imageURL)
	} else {
		audit, err = s.audit.CheckText(ctx, appID, user.WechatOpenID, content)
	}
	if err != nil {
		_ = db.Mysql.Model(comment).Updates(map[string]interface{}{"audit_status": "failed", "audit_reason": err.Error(), "updated_at": time.Now()}).Error
		return nil, err
	}
	comment.AuditStatus = audit.Status
	comment.AuditReason = audit.Reason
	if err := db.Mysql.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(comment).Error; err != nil {
			return err
		}
		record := models.ContentAuditRecord{AppID: appID, CommentID: comment.ID, MediaType: comment.ContentType, Provider: "wechat", Status: audit.Status, TraceID: audit.TraceID, Reason: audit.Reason, CreatedAt: time.Now()}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if comment.Level == 2 && audit.Status == "approved" {
			return tx.Model(&models.ArticleComment{}).Where("id = ?", comment.RootID).UpdateColumn("reply_count", gorm.Expr("reply_count + 1")).Error
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return comment, nil
}

// CompleteImageAudit 根据微信异步回调更新图片评论状态。
func (s *CommentService) CompleteImageAudit(appID, traceID, suggest string, label int) error {
	appID = strings.TrimSpace(appID)
	traceID = strings.TrimSpace(traceID)
	suggest = strings.ToLower(strings.TrimSpace(suggest))
	if appID == "" || traceID == "" {
		return errors.New("审核回调租户或 trace_id 无效")
	}
	if suggest != "pass" && suggest != "review" && suggest != "reject" && suggest != "block" {
		return errors.New("审核回调结果无效")
	}
	var record models.ContentAuditRecord
	if err := db.Mysql.Where("app_id = ? AND trace_id = ? AND media_type = ?", appID, traceID, "image").First(&record).Error; err != nil {
		return err
	}
	status := "rejected"
	if suggest == "pass" {
		status = "approved"
	} else if suggest == "review" {
		status = "pending"
	}
	return db.Mysql.Transaction(func(tx *gorm.DB) error {
		// 微信可能重复投递回调，已结束的审核记录保持终态，避免旧结果覆盖新结果。
		if record.Status == "approved" || record.Status == "rejected" {
			return nil
		}
		if err := tx.Model(&record).Updates(map[string]interface{}{"status": status, "reason": fmt.Sprintf("suggest=%s,label=%d", suggest, label)}).Error; err != nil {
			return err
		}
		var comment models.ArticleComment
		if err := tx.Where("app_id = ? AND id = ?", appID, record.CommentID).First(&comment).Error; err != nil {
			return err
		}
		wasApproved := comment.AuditStatus == "approved"
		if err := tx.Model(&comment).Updates(map[string]interface{}{"audit_status": status, "audit_reason": fmt.Sprintf("suggest=%s,label=%d", suggest, label), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if comment.Level == 2 && !wasApproved && status == "approved" {
			return tx.Model(&models.ArticleComment{}).Where("id = ?", comment.RootID).UpdateColumn("reply_count", gorm.Expr("reply_count + 1")).Error
		}
		return nil
	})
}

// Moderate 管理员审核、隐藏或恢复评论，并同步二级回复数量。
func (s *CommentService) Moderate(appID string, commentID int64, status, reason string) error {
	appID, status = strings.TrimSpace(appID), strings.ToLower(strings.TrimSpace(status))
	if appID == "" || commentID <= 0 {
		return errors.New("评论参数无效")
	}
	if status != "approved" && status != "rejected" && status != "pending" && status != "deleted" {
		return errors.New("评论状态无效")
	}
	return db.Mysql.Transaction(func(tx *gorm.DB) error {
		var comment models.ArticleComment
		if err := tx.Where("app_id = ? AND id = ?", appID, commentID).First(&comment).Error; err != nil {
			return err
		}
		wasApproved := comment.AuditStatus == "approved"
		if err := tx.Model(&comment).Updates(map[string]interface{}{"audit_status": status, "audit_reason": strings.TrimSpace(reason), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if comment.Level == 2 && wasApproved != (status == "approved") {
			delta := 1
			if wasApproved {
				delta = -1
			}
			return tx.Model(&models.ArticleComment{}).Where("app_id = ? AND id = ?", appID, comment.RootID).UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count + ?, 0)", delta)).Error
		}
		return nil
	})
}
