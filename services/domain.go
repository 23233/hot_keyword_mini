// Package services domain.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DomainError 为 HTTP、MCP 和客户端提供稳定的领域错误码。
type DomainError struct {
	// 机器可读错误码。
	Code string `json:"code"`
	// 错误说明。
	Message string `json:"message"`
}

func (e *DomainError) Error() string         { return e.Code + ": " + e.Message }
func domainError(code, message string) error { return &DomainError{code, message} }

// domainContext 在单个数据库事务中执行权限、状态和账本更新。
type domainContext struct {
	tx      *gorm.DB
	app     string
	user    int64
	actor   string
	admin   bool
	payload map[string]interface{}
}

func domainText(p map[string]interface{}, key string) string {
	value, _ := p[key].(string)
	return strings.TrimSpace(value)
}
func domainNumber(p map[string]interface{}, key string) (int64, error) {
	v, ok := p[key].(float64)
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v < 0 || v > 9007199254740991 {
		return 0, domainError("INVALID_ARGUMENT", key+" 必须为非负安全整数")
	}
	return int64(v), nil
}
func recordData(r *models.DomainRecord) map[string]interface{} {
	p := map[string]interface{}{}
	_ = json.Unmarshal([]byte(r.Data), &p)
	return p
}
func (c *domainContext) load(id, kind string) (*models.DomainRecord, error) {
	var r models.DomainRecord
	if err := c.tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND app_id = ? AND kind = ?", id, c.app, kind).First(&r).Error; err != nil {
		return nil, domainError("NOT_FOUND", "业务记录不存在")
	}
	if !c.admin && r.UserID != c.user && r.PeerID != c.user {
		return nil, domainError("FORBIDDEN", "无权访问业务记录")
	}
	return &r, nil
}
func (c *domainContext) save(r *models.DomainRecord, p map[string]interface{}) error {
	encoded, err := json.Marshal(p)
	if err != nil {
		return err
	}
	r.Data = string(encoded)
	r.Revision++
	r.UpdatedAt = time.Now()
	return c.tx.Save(r).Error
}
func (c *domainContext) create(kind, status string, p map[string]interface{}) (*models.DomainRecord, error) {
	r := &models.DomainRecord{ID: uuid.NewString(), AppID: c.app, UserID: c.user, Kind: kind, Status: status, CreatedAt: time.Now()}
	return r, c.save(r, p)
}
func publicDomain(r *models.DomainRecord) map[string]interface{} {
	p := recordData(r)
	for _, key := range []string{"account", "identity", "contact", "qualifications", "payment_details", "file_key"} {
		delete(p, key)
	}
	return map[string]interface{}{"id": r.ID, "status": r.Status, "revision": r.Revision, "user_id": r.UserID, "peer_id": r.PeerID, "data": p}
}

// ExecuteDomainOperation 统一的后台、MCP 和用户业务入口。admin 仅由认证后的服务端入口设置。
func ExecuteDomainOperation(app string, user int64, actor, endpoint string, p map[string]interface{}, idem string, admin bool) (interface{}, error) {
	if db.Mysql == nil {
		return nil, domainError("UNAVAILABLE", "数据库未初始化")
	}
	if app == "" || actor == "" || (!admin && user <= 0) {
		return nil, domainError("UNAUTHORIZED", "身份不完整")
	}
	if p == nil {
		p = map[string]interface{}{}
	}
	read := endpoint == "chat.poll" || endpoint == "wallet.refresh" || endpoint == "logistics.refresh" || endpoint == "domain.list" || endpoint == "domain.get"
	if !read && strings.TrimSpace(idem) == "" {
		return nil, domainError("INVALID_ARGUMENT", "写操作必须携带幂等键")
	}
	if len(idem) > 128 {
		return nil, domainError("INVALID_ARGUMENT", "幂等键过长")
	}
	var result interface{}
	err := db.Mysql.Transaction(func(tx *gorm.DB) error {
		var appRow models.MiniApp
		if err := tx.Where("app_id = ?", app).First(&appRow).Error; err != nil {
			return domainError("NOT_FOUND", "租户不存在")
		}
		if admin {
			allowed := map[string]bool{"domain.list": true, "order.fulfill": true, "after_sale.review": true, "service.offer": true, "service.profile.review": true, "service.configure": true, "chat.assign": true, "chat.send": true, "chat.block": true, "chat.recall": true, "media.review": true, "media.delete": true, "wallet.adjust": true, "wallet.review": true, "wallet.refresh": true, "ads.configure": true, "membership.status": true}
			if !allowed[endpoint] {
				return domainError("FORBIDDEN", "管理员入口不允许冒充用户发起业务")
			}
		}
		if !admin {
			var u models.User
			if err := tx.Where("app_id = ? AND id = ?", app, user).First(&u).Error; err != nil {
				return domainError("UNAUTHORIZED", "用户不存在")
			}
		}
		payloadJSON, err := json.Marshal(p)
		if err != nil {
			return err
		}
		op := models.PlatformOperation{AppID: app, UserID: user, OpenID: actor, Kind: endpoint, IdempotencyKey: idem, Payload: string(payloadJSON), CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if !read {
			if err := tx.Create(&op).Error; err != nil {
				if !isDuplicateKeyError(err) {
					return err
				}
				var previous models.PlatformOperation
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("app_id = ? AND user_id = ? AND open_id = ? AND kind = ? AND idempotency_key = ?", app, user, actor, endpoint, idem).First(&previous).Error; err != nil {
					return err
				}
				if previous.Payload != string(payloadJSON) {
					return domainError("IDEMPOTENCY_CONFLICT", "同一幂等键不能用于不同请求")
				}
				return json.Unmarshal([]byte(previous.Result), &result)
			}
		}
		c := &domainContext{tx: tx, app: app, user: user, actor: actor, admin: admin, payload: p}
		result, err = c.execute(endpoint)
		if err != nil {
			return err
		}
		if !read {
			encoded, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return tx.Model(&op).Updates(map[string]interface{}{"result": string(encoded), "status": "completed"}).Error
		}
		return nil
	})
	return result, err
}

func (c *domainContext) execute(endpoint string) (interface{}, error) {
	switch {
	case strings.HasPrefix(endpoint, "chat."):
		return c.chat(endpoint)
	case strings.HasPrefix(endpoint, "wallet."):
		return c.wallet(endpoint)
	case strings.HasPrefix(endpoint, "order."), strings.HasPrefix(endpoint, "after_sale."), endpoint == "logistics.refresh":
		return c.order(endpoint)
	case strings.HasPrefix(endpoint, "service."):
		return c.service(endpoint)
	case strings.HasPrefix(endpoint, "media."):
		return c.media(endpoint)
	case endpoint == "domain.list":
		kind := domainText(c.payload, "kind")
		allowed := map[string]bool{"order": true, "after_sale": true, "service_profile": true, "task": true, "thread": true, "withdraw": true, "media": true}
		if !allowed[kind] {
			return nil, domainError("INVALID_ARGUMENT", "未知领域类型")
		}
		var items []models.DomainRecord
		query := c.tx.Where("app_id = ? AND kind = ?", c.app, kind)
		if !c.admin {
			query = query.Where("user_id = ? OR peer_id = ?", c.user, c.user)
		}
		if err := query.Order("created_at desc, id desc").Limit(100).Find(&items).Error; err != nil {
			return nil, err
		}
		out := []map[string]interface{}{}
		for i := range items {
			out = append(out, publicDomain(&items[i]))
		}
		return map[string]interface{}{"items": out}, nil
	default:
		return nil, domainError("UNKNOWN_ENDPOINT", "未登记领域操作")
	}
}

func (c *domainContext) chat(endpoint string) (interface{}, error) {
	id := domainText(c.payload, "thread_id")
	if id == "" {
		id = domainText(c.payload, "id")
	}
	if endpoint == "chat.connect" && id == "" {
		r, err := c.create("thread", "open", map[string]interface{}{"source": domainText(c.payload, "source")})
		if err != nil {
			return nil, err
		}
		out := publicDomain(r)
		out["thread_id"] = r.ID
		return out, nil
	}
	r, err := c.load(id, "thread")
	if err != nil {
		return nil, err
	}
	data := recordData(r)
	if endpoint == "chat.assign" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可指派客服")
		}
		peer, err := domainNumber(c.payload, "peer_id")
		if err != nil || peer <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "客服用户无效")
		}
		var user models.User
		if err := c.tx.Where("id = ? AND app_id = ?", peer, c.app).First(&user).Error; err != nil {
			return nil, domainError("NOT_FOUND", "客服用户不存在")
		}
		r.PeerID = peer
		if err := c.save(r, data); err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	if endpoint == "chat.send" || endpoint == "chat.upload_media" {
		if r.Status != "open" {
			return nil, domainError("INVALID_STATE", "会话已关闭")
		}
		content := domainText(c.payload, "content")
		mediaID := domainText(c.payload, "media_id")
		if (content == "" && mediaID == "") || len([]rune(content)) > 2000 {
			return nil, domainError("INVALID_ARGUMENT", "消息为空或超过2000字")
		}
		if mediaID != "" {
			media, err := c.load(mediaID, "media")
			if err != nil {
				return nil, err
			}
			if media.Status != "approved" || media.UserID != c.user {
				return nil, domainError("FORBIDDEN", "媒体未审核或非本人所有")
			}
		}
		message := models.ChatMessage{AppID: c.app, ThreadID: r.ID, SenderID: c.user, Content: content, MediaID: mediaID, Status: "sent", CreatedAt: time.Now()}
		if err := c.tx.Create(&message).Error; err != nil {
			return nil, err
		}
		return message, nil
	}
	if endpoint == "chat.recall" || endpoint == "chat.block" {
		messageID, err := domainNumber(c.payload, "message_id")
		if err != nil {
			return nil, err
		}
		var message models.ChatMessage
		if err := c.tx.Where("app_id = ? AND thread_id = ? AND id = ?", c.app, r.ID, messageID).First(&message).Error; err != nil {
			return nil, domainError("NOT_FOUND", "消息不存在")
		}
		if !c.admin && (endpoint == "chat.block" || message.SenderID != c.user) {
			return nil, domainError("FORBIDDEN", "不能撤回他人消息")
		}
		status := "recalled"
		if endpoint == "chat.block" {
			status = "blocked"
		}
		if err := c.tx.Model(&message).Updates(map[string]interface{}{"status": status, "content": "", "media_id": ""}).Error; err != nil {
			return nil, err
		}
		return map[string]interface{}{"id": messageID, "status": status}, nil
	}
	if endpoint == "chat.mark_read" {
		cursor, err := domainNumber(c.payload, "cursor")
		if err != nil {
			return nil, err
		}
		var max int64
		c.tx.Model(&models.ChatMessage{}).Where("app_id = ? AND thread_id = ?", c.app, r.ID).Select("COALESCE(MAX(id),0)").Scan(&max)
		if cursor > max {
			return nil, domainError("INVALID_ARGUMENT", "游标超过最新消息")
		}
		key := fmt.Sprintf("read_%d", c.user)
		previous, _ := data[key].(float64)
		if float64(cursor) > previous {
			data[key] = cursor
			if err := c.save(r, data); err != nil {
				return nil, err
			}
		}
		return map[string]interface{}{"status": "read", "cursor": cursor}, nil
	}
	if endpoint == "chat.poll" || endpoint == "chat.connect" {
		cursor := int64(0)
		if _, ok := c.payload["cursor"]; ok {
			cursor, err = domainNumber(c.payload, "cursor")
			if err != nil {
				return nil, err
			}
		}
		var messages []models.ChatMessage
		if err := c.tx.Where("app_id = ? AND thread_id = ? AND id > ?", c.app, r.ID, cursor).Order("id asc").Limit(50).Find(&messages).Error; err != nil {
			return nil, err
		}
		last := cursor
		if len(messages) > 0 {
			last = messages[len(messages)-1].ID
		}
		read, _ := data[fmt.Sprintf("read_%d", c.user)].(float64)
		var unread int64
		if err := c.tx.Model(&models.ChatMessage{}).Where("app_id = ? AND thread_id = ? AND sender_id <> ? AND id > ? AND status = ?", c.app, r.ID, c.user, int64(read), "sent").Count(&unread).Error; err != nil {
			return nil, err
		}
		return map[string]interface{}{"thread_id": r.ID, "messages": messages, "cursor": last, "unread_count": unread, "status": r.Status}, nil
	}
	return nil, domainError("UNKNOWN_ENDPOINT", "未登记聊天操作")
}

func (c *domainContext) account(user int64) (*models.WalletAccount, error) {
	a := models.WalletAccount{AppID: c.app, UserID: user}
	if err := c.tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
		return nil, err
	}
	if err := c.tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("app_id = ? AND user_id = ?", c.app, user).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}
func (c *domainContext) ledger(a *models.WalletAccount, available, frozen int64, reference, reason string) error {
	if reason == "" || a.BalanceFen+available < 0 || a.FrozenFen+frozen < 0 {
		return domainError("INSUFFICIENT_BALANCE", "余额不足或缺少记账理由")
	}
	if available > 0 && a.BalanceFen > 9007199254740991-available {
		return domainError("INVALID_ARGUMENT", "余额超出范围")
	}
	a.BalanceFen += available
	a.FrozenFen += frozen
	if err := c.tx.Save(a).Error; err != nil {
		return err
	}
	return c.tx.Create(&models.WalletEntry{AppID: c.app, UserID: a.UserID, AvailableDelta: available, FrozenDelta: frozen, ReferenceID: reference, Reason: reason, Actor: c.actor, CreatedAt: time.Now()}).Error
}
func (c *domainContext) wallet(endpoint string) (interface{}, error) {
	user := c.user
	if c.admin {
		var err error
		user, err = domainNumber(c.payload, "user_id")
		if err != nil || user <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "目标用户无效")
		}
		var u models.User
		if err := c.tx.Where("app_id = ? AND id = ?", c.app, user).First(&u).Error; err != nil {
			return nil, domainError("NOT_FOUND", "用户不存在")
		}
	}
	a, err := c.account(user)
	if err != nil {
		return nil, err
	}
	if endpoint == "wallet.refresh" {
		return a, nil
	}
	if endpoint == "wallet.adjust" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可调整余额")
		}
		amount, err := domainNumber(c.payload, "amount")
		if err != nil || amount == 0 {
			return nil, domainError("INVALID_ARGUMENT", "金额无效")
		}
		if domainText(c.payload, "direction") == "debit" {
			amount = -amount
		}
		if err := c.ledger(a, amount, 0, "adjustment", domainText(c.payload, "reason")); err != nil {
			return nil, err
		}
		return a, nil
	}
	if endpoint == "wallet.withdraw" {
		amount, err := domainNumber(c.payload, "amount")
		if err != nil || amount <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "提现金额必须为正整数分")
		}
		account := domainText(c.payload, "account")
		if account == "" || len(account) > 256 {
			return nil, domainError("INVALID_ARGUMENT", "必须绑定收款账户")
		}
		r, err := c.create("withdraw", "pending", map[string]interface{}{"amount_fen": amount, "account": account})
		if err != nil {
			return nil, err
		}
		if err := c.ledger(a, -amount, amount, r.ID, "申请提现冻结"); err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	if endpoint == "wallet.review" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可审核提现")
		}
		r, err := c.load(domainText(c.payload, "id"), "withdraw")
		if err != nil {
			return nil, err
		}
		if r.UserID != user {
			return nil, domainError("FORBIDDEN", "提现用户不匹配")
		}
		data := recordData(r)
		amount, err := domainNumber(data, "amount_fen")
		if err != nil {
			return nil, err
		}
		next := domainText(c.payload, "status")
		reason := domainText(c.payload, "reason")
		if reason == "" {
			return nil, domainError("INVALID_ARGUMENT", "审核理由必填")
		}
		valid := (r.Status == "pending" && (next == "processing" || next == "rejected")) || (r.Status == "processing" && (next == "paid" || next == "failed"))
		if !valid {
			return nil, domainError("INVALID_STATE", "提现状态迁移无效")
		}
		if next == "rejected" || next == "failed" {
			if err := c.ledger(a, amount, -amount, r.ID, reason); err != nil {
				return nil, err
			}
		}
		if next == "paid" {
			if domainText(c.payload, "transfer_reference") == "" {
				return nil, domainError("INVALID_ARGUMENT", "付款凭证必填")
			}
			if err := c.ledger(a, 0, -amount, r.ID, reason); err != nil {
				return nil, err
			}
		}
		r.Status = next
		data["reason"] = reason
		data["transfer_reference"] = domainText(c.payload, "transfer_reference")
		if err := c.save(r, data); err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	return nil, domainError("UNKNOWN_ENDPOINT", "未登记钱包操作")
}
