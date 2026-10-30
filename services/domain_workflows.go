// Package services domain_workflows.go
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// membershipOpen 返回当前用户的会员状态与可售套餐，供 open_membership 动作使用。
// 该端点只读，不发放权益；真正的开通必须在支付回调事务中完成。
func (c *domainContext) membershipOpen() (interface{}, error) {
	var plans []models.MembershipLevel
	if err := c.tx.Where("app_id = ? AND status = ?", c.app, "active").Order("level asc").Find(&plans).Error; err != nil {
		return nil, err
	}

	var membership models.UserMembership
	status := "inactive"
	level := 0
	var expiresAt interface{}
	err := c.tx.Where("app_id = ? AND user_id = ?", c.app, c.user).First(&membership).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil && membership.ExpiresAt.After(time.Now()) {
		status, level, expiresAt = "active", membership.Level, membership.ExpiresAt
	}

	return map[string]interface{}{
		"status":     status,
		"level":      level,
		"expires_at": expiresAt,
		"plans":      plans,
		"message":    "会员状态已刷新",
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// profileData 管理当前用户的地址簿与发票资料；数据按用户隔离，管理员不代填。
func (c *domainContext) profileData(endpoint string) (interface{}, error) {
	switch endpoint {
	case "address.list", "invoice.get":
		kind := "address"
		if endpoint == "invoice.get" {
			kind = "invoice"
		}
		var records []models.DomainRecord
		if err := c.tx.Where("app_id = ? AND user_id = ? AND kind = ?", c.app, c.user, kind).
			Order("created_at desc, id desc").Find(&records).Error; err != nil {
			return nil, err
		}
		items := make([]map[string]interface{}, 0, len(records))
		for index := range records {
			items = append(items, publicDomain(&records[index]))
		}
		if endpoint == "invoice.get" {
			if len(items) == 0 {
				return map[string]interface{}{"invoice": nil}, nil
			}
			return map[string]interface{}{"invoice": items[0]}, nil
		}
		return map[string]interface{}{"items": items}, nil

	case "address.save":
		name := domainText(c.payload, "name")
		phone := domainText(c.payload, "phone")
		region := domainText(c.payload, "region")
		detail := domainText(c.payload, "detail")
		if name == "" || phone == "" || region == "" || detail == "" {
			return nil, domainError("INVALID_ARGUMENT", "地址信息不完整")
		}
		data := map[string]interface{}{
			"name": name, "phone": phone, "region": region, "detail": detail,
			"is_default": c.payload["is_default"] == true,
		}
		var record *models.DomainRecord
		if id := domainText(c.payload, "id"); id != "" {
			loaded, err := c.load(id, "address")
			if err != nil {
				return nil, err
			}
			if loaded.UserID != c.user {
				return nil, domainError("FORBIDDEN", "不能修改他人地址")
			}
			if err := c.save(loaded, data); err != nil {
				return nil, err
			}
			record = loaded
		} else {
			created, err := c.create("address", "active", data)
			if err != nil {
				return nil, err
			}
			record = created
		}
		if data["is_default"] == true {
			c.clearOtherAddressDefaults(record.ID)
		}
		return publicDomain(record), nil

	case "address.delete":
		id := domainText(c.payload, "id")
		if id == "" {
			return nil, domainError("INVALID_ARGUMENT", "地址标识不能为空")
		}
		record, err := c.load(id, "address")
		if err != nil {
			return nil, err
		}
		if record.UserID != c.user {
			return nil, domainError("FORBIDDEN", "不能删除他人地址")
		}
		if err := c.tx.Delete(record).Error; err != nil {
			return nil, err
		}
		return map[string]interface{}{"id": record.ID, "status": "deleted"}, nil

	case "invoice.save":
		invoiceType := domainText(c.payload, "type")
		if invoiceType == "" {
			invoiceType = "personal"
		}
		title := domainText(c.payload, "title")
		taxNo := domainText(c.payload, "tax_no")
		if title == "" {
			return nil, domainError("INVALID_ARGUMENT", "发票抬头不能为空")
		}
		if invoiceType == "company" && taxNo == "" {
			return nil, domainError("INVALID_ARGUMENT", "企业发票税号不能为空")
		}
		data := map[string]interface{}{"type": invoiceType, "title": title, "tax_no": taxNo, "email": domainText(c.payload, "email")}
		var existing models.DomainRecord
		err := c.tx.Where("app_id = ? AND user_id = ? AND kind = ?", c.app, c.user, "invoice").First(&existing).Error
		if err == nil {
			if err := c.save(&existing, data); err != nil {
				return nil, err
			}
			return publicDomain(&existing), nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		created, err := c.create("invoice", "active", data)
		if err != nil {
			return nil, err
		}
		return publicDomain(created), nil

	default:
		return nil, domainError("UNKNOWN_ENDPOINT", "未登记地址或发票操作")
	}
}

// clearOtherAddressDefaults 将同一用户的其他地址取消默认标记。
func (c *domainContext) clearOtherAddressDefaults(keepID string) {
	var records []models.DomainRecord
	if err := c.tx.Where("app_id = ? AND user_id = ? AND kind = ? AND id <> ?", c.app, c.user, "address", keepID).
		Find(&records).Error; err != nil {
		return
	}
	for index := range records {
		data := recordData(&records[index])
		if isDefault, _ := data["is_default"].(bool); !isDefault {
			continue
		}
		data["is_default"] = false
		_ = c.save(&records[index], data)
	}
}

func (c *domainContext) order(endpoint string) (interface{}, error) {
	p := c.payload
	if endpoint == "order.list" {
		limit := int64(20)
		if value, err := domainNumber(p, "limit"); err == nil && value > 0 {
			limit = value
		}
		if limit > 50 {
			limit = 50
		}
		offset := int64(0)
		if value, err := domainNumber(p, "offset"); err == nil && value > 0 {
			offset = value
		}
		var total int64
		if err := c.tx.Model(&models.DomainRecord{}).
			Where("app_id = ? AND user_id = ? AND kind = ?", c.app, c.user, "order").Count(&total).Error; err != nil {
			return nil, err
		}
		var records []models.DomainRecord
		if err := c.tx.Where("app_id = ? AND user_id = ? AND kind = ?", c.app, c.user, "order").
			Order("created_at desc, id desc").Limit(int(limit)).Offset(int(offset)).Find(&records).Error; err != nil {
			return nil, err
		}
		items := make([]map[string]interface{}, 0, len(records))
		for index := range records {
			items = append(items, publicDomain(&records[index]))
		}
		return map[string]interface{}{"items": items, "total": total, "has_more": offset+int64(len(items)) < total}, nil
	}
	if endpoint == "order.create" {
		var product models.Product
		if err := c.tx.Where("app_id = ? AND sku = ? AND status = ?", c.app, domainText(p, "sku"), models.ProductStatusActive).First(&product).Error; err != nil {
			return nil, domainError("NOT_FOUND", "商品不存在")
		}
		if product.PriceFen <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "商品价格无效")
		}
		r, err := c.create("order", "created", map[string]interface{}{"sku": product.SKU, "product_id": product.ID, "amount_fen": product.PriceFen, "title": product.Name, "tracks": []interface{}{}})
		if err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	id := domainText(p, "id")
	kind := "order"
	if strings.HasPrefix(endpoint, "after_sale.") && endpoint != "after_sale.apply" {
		kind = "after_sale"
	}
	if endpoint == "after_sale.apply" {
		id = domainText(p, "order_id")
	}
	r, err := c.load(id, kind)
	if err != nil {
		return nil, err
	}
	data := recordData(r)
	if endpoint == "logistics.refresh" {
		return map[string]interface{}{"id": r.ID, "status": r.Status, "tracks": data["tracks"]}, nil
	}
	if endpoint == "order.confirm" {
		if r.Status != "created" {
			return nil, domainError("INVALID_STATE", "订单不可确认")
		}
		if err := c.checkPayment(r, data); err != nil {
			return nil, err
		}
		r.Status = "confirmed"
	} else if endpoint == "order.cancel" {
		if r.Status != "created" {
			return nil, domainError("INVALID_STATE", "已付款订单须申请售后")
		}
		r.Status = "cancelled"
	} else if endpoint == "order.confirm_receipt" {
		if r.UserID != c.user || r.Status != "returning" {
			return nil, domainError("INVALID_STATE", "仅购买者可确认已回寄订单")
		}
		r.Status = "completed"
		data["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	} else if endpoint == "order.fulfill" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可更新履约")
		}
		next := domainText(p, "status")
		allowed := map[string]string{"confirmed": "received", "received": "in_service", "in_service": "awaiting_confirmation", "awaiting_confirmation": "returning"}
		if allowed[r.Status] != next {
			return nil, domainError("INVALID_STATE", "履约状态迁移无效")
		}
		if next == "returning" && (domainText(p, "tracking_no") == "" || domainText(p, "carrier") == "") {
			return nil, domainError("INVALID_ARGUMENT", "回寄必须有物流公司与单号")
		}
		r.Status = next
		tracks, _ := data["tracks"].([]interface{})
		data["tracks"] = append(tracks, map[string]interface{}{"status": next, "time": time.Now().UTC().Format(time.RFC3339), "carrier": domainText(p, "carrier"), "tracking_no": domainText(p, "tracking_no")})
	} else if endpoint == "after_sale.apply" {
		if r.UserID != c.user {
			return nil, domainError("FORBIDDEN", "仅购买者可申请售后")
		}
		if r.Status == "created" || r.Status == "cancelled" || r.Status == "refunded" {
			return nil, domainError("INVALID_STATE", "订单未付款或已关闭")
		}
		typeName := domainText(p, "type")
		valid := map[string]bool{"cancel": true, "refund": true, "dispute": true, "logistics": true, "quality": true}
		if !valid[typeName] || domainText(p, "reason") == "" {
			return nil, domainError("INVALID_ARGUMENT", "售后类型或理由无效")
		}
		if existing := domainText(data, "after_sale_id"); existing != "" {
			return nil, domainError("CONFLICT", "订单已有售后单")
		}
		after, err := c.create("after_sale", "requested", map[string]interface{}{"order_id": r.ID, "type": typeName, "reason": domainText(p, "reason"), "evidence": []interface{}{}})
		if err != nil {
			return nil, err
		}
		data["after_sale_id"] = after.ID
		if err := c.save(r, data); err != nil {
			return nil, err
		}
		return publicDomain(after), nil
	} else if endpoint == "after_sale.upload_evidence" {
		if r.UserID != c.user || (r.Status != "requested" && r.Status != "evidence_required") {
			return nil, domainError("INVALID_STATE", "当前不可提交证据")
		}
		media, err := c.load(domainText(p, "media_id"), "media")
		if err != nil {
			return nil, err
		}
		if media.UserID != c.user || media.Status != "approved" {
			return nil, domainError("FORBIDDEN", "证据必须为本人审核通过的媒体")
		}
		items, _ := data["evidence"].([]interface{})
		data["evidence"] = append(items, media.ID)
		r.Status = "under_review"
	} else if endpoint == "after_sale.review" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可裁决售后")
		}
		next := domainText(p, "status")
		valid := map[string]map[string]bool{"requested": {"evidence_required": true, "under_review": true}, "evidence_required": {"under_review": true}, "under_review": {"approved": true, "rejected": true}, "approved": {"refunded": true}, "rejected": {"closed": true}, "refunded": {"closed": true}}
		if !valid[r.Status][next] || domainText(p, "reason") == "" {
			return nil, domainError("INVALID_STATE", "裁决状态无效或缺少理由")
		}
		if next == "refunded" {
			order, err := c.load(domainText(data, "order_id"), "order")
			if err != nil {
				return nil, err
			}
			orderData := recordData(order)
			var payment models.PaymentOrder
			if err := c.tx.Where("app_id = ? AND out_trade_no = ? AND status = ?", c.app, domainText(orderData, "out_trade_no"), models.PaymentOrderRefunded).First(&payment).Error; err != nil {
				return nil, domainError("INVALID_STATE", "须先完成支付退款回调")
			}
			order.Status = "refunded"
			if err := c.save(order, orderData); err != nil {
				return nil, err
			}
		}
		r.Status = next
		data["review_reason"] = domainText(p, "reason")
	} else {
		return nil, domainError("UNKNOWN_ENDPOINT", "未登记订单操作")
	}
	if err := c.save(r, data); err != nil {
		return nil, err
	}
	return publicDomain(r), nil
}

func (c *domainContext) checkPayment(r *models.DomainRecord, data map[string]interface{}) error {
	var payment models.PaymentOrder
	if err := c.tx.Where("app_id = ? AND user_id = ? AND out_trade_no = ? AND status = ?", c.app, r.UserID, domainText(c.payload, "out_trade_no"), models.PaymentOrderPaid).First(&payment).Error; err != nil {
		return domainError("PAYMENT_REQUIRED", "缺少已支付的本人订单")
	}
	amount, err := domainNumber(data, "amount_fen")
	if err != nil || amount != payment.AmountFen {
		return domainError("PAYMENT_MISMATCH", "支付金额不匹配")
	}
	// 支付订单只允许绑定一个领域实体。这里按租户与用户读取候选订单后在 Go 内比较，
	// 不使用 MySQL 专有的 JSON_EXTRACT，保证 MySQL 与 PostgreSQL 的行为完全一致。
	// 支付单本身已通过 user_id 限定归属，因此只需在同一用户自己的订单中查重。
	var boundOrders []models.DomainRecord
	if err := c.tx.Select("id", "data").
		Where("app_id = ? AND user_id = ? AND kind = ? AND id <> ?", c.app, r.UserID, "order", r.ID).
		Find(&boundOrders).Error; err != nil {
		return err
	}
	for index := range boundOrders {
		if bound, _ := recordData(&boundOrders[index])["out_trade_no"].(string); bound == payment.OutTradeNo {
			return domainError("PAYMENT_REUSED", "付款已绑定其他订单")
		}
	}
	data["out_trade_no"] = payment.OutTradeNo
	return nil
}

func (c *domainContext) service(endpoint string) (interface{}, error) {
	p := c.payload
	if endpoint == "service.apply" {
		if domainText(p, "identity") == "" || domainText(p, "contact") == "" {
			return nil, domainError("INVALID_ARGUMENT", "认证身份和联系方式必填")
		}
		var count int64
		if err := c.tx.Model(&models.DomainRecord{}).Where("app_id = ? AND kind = ? AND user_id = ?", c.app, "service_profile", c.user).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, domainError("CONFLICT", "已存在认证申请")
		}
		r, err := c.create("service_profile", "submitted", map[string]interface{}{"identity": domainText(p, "identity"), "contact": domainText(p, "contact"), "accepting": false, "max_concurrent": 1})
		if err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	if endpoint == "service.profile.review" || endpoint == "service.configure" {
		r, err := c.load(domainText(p, "id"), "service_profile")
		if err != nil {
			return nil, err
		}
		data := recordData(r)
		if endpoint == "service.profile.review" {
			if !c.admin {
				return nil, domainError("FORBIDDEN", "认证审核仅管理员可执行")
			}
			next := domainText(p, "status")
			allowed := map[string]map[string]bool{"draft": {"submitted": true}, "submitted": {"under_review": true}, "under_review": {"approved": true, "rejected": true}, "rejected": {"submitted": true}, "approved": {"suspended": true, "closed": true}, "suspended": {"approved": true, "closed": true}}
			if !allowed[r.Status][next] || domainText(p, "reason") == "" {
				return nil, domainError("INVALID_STATE", "认证状态或审核理由无效")
			}
			if next == "approved" {
				for _, key := range []string{"identity_approved", "contact_approved", "qualifications_approved", "payment_approved"} {
					if p[key] != true {
						return nil, domainError("REVIEW_REQUIRED", "认证材料须分别审核")
					}
					data[key] = true
				}
			}
			r.Status = next
			data["review_reason"] = domainText(p, "reason")
		} else {
			if r.Status != "approved" {
				return nil, domainError("REVIEW_REQUIRED", "仅认证通过斗师可接单")
			}
			max, err := domainNumber(p, "max_concurrent")
			if err != nil || max < 1 || max > 100 {
				return nil, domainError("INVALID_ARGUMENT", "接单上限应为1至100")
			}
			if domainText(p, "region") == "" || domainText(p, "service_type") == "" {
				return nil, domainError("INVALID_ARGUMENT", "服务区域和类型必填")
			}
			data["accepting"] = p["accepting"] == true
			data["max_concurrent"] = max
			data["region"] = domainText(p, "region")
			data["service_type"] = domainText(p, "service_type")
		}
		if err := c.save(r, data); err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	if endpoint == "service.request" {
		start, err := time.Parse(time.RFC3339, domainText(p, "starts_at"))
		if err != nil {
			return nil, domainError("INVALID_ARGUMENT", "排期开始时间无效")
		}
		end, err := time.Parse(time.RFC3339, domainText(p, "ends_at"))
		if err != nil || !end.After(start) || !start.After(time.Now()) {
			return nil, domainError("INVALID_ARGUMENT", "排期结束时间无效")
		}
		r, err := c.create("task", "pending", map[string]interface{}{"starts_at": start.UTC().Format(time.RFC3339), "ends_at": end.UTC().Format(time.RFC3339), "region": domainText(p, "region"), "service_type": domainText(p, "service_type")})
		if err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	if endpoint == "service.offer" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可分配任务")
		}
		r, err := c.load(domainText(p, "id"), "task")
		if err != nil {
			return nil, err
		}
		if r.Status != "pending" {
			return nil, domainError("INVALID_STATE", "任务不可分配")
		}
		peer, err := domainNumber(p, "peer_id")
		if err != nil || peer <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "斗师用户无效")
		}
		r.PeerID = peer
		r.Status = "offered"
		if err := c.save(r, recordData(r)); err != nil {
			return nil, err
		}
		return publicDomain(r), nil
	}
	r, err := c.load(domainText(p, "id"), "task")
	if err != nil {
		return nil, err
	}
	data := recordData(r)
	if endpoint == "service.accept_task" || endpoint == "service.reject_task" {
		if r.Status != "offered" || r.PeerID != c.user {
			return nil, domainError("FORBIDDEN", "仅被指派斗师可处理待接任务")
		}
		if endpoint == "service.reject_task" {
			r.Status = "pending"
			r.PeerID = 0
			data["rejection_reason"] = domainText(p, "reason")
		} else {
			var profile models.DomainRecord
			if err := c.tx.Where("app_id = ? AND kind = ? AND user_id = ? AND status = ?", c.app, "service_profile", c.user, "approved").First(&profile).Error; err != nil {
				return nil, domainError("REVIEW_REQUIRED", "斗师未认证")
			}
			settings := recordData(&profile)
			if settings["accepting"] != true || settings["region"] != data["region"] || settings["service_type"] != data["service_type"] {
				return nil, domainError("UNAVAILABLE", "斗师未开启该地区或类型服务")
			}
			var active []models.DomainRecord
			if err := c.tx.Where("app_id = ? AND kind = ? AND peer_id = ? AND status IN ?", c.app, "task", c.user, []string{"accepted", "quoted", "paid", "in_service", "awaiting_confirmation"}).Find(&active).Error; err != nil {
				return nil, err
			}
			max, _ := domainNumber(settings, "max_concurrent")
			if int64(len(active)) >= max {
				return nil, domainError("SCHEDULE_CONFLICT", "已达并发接单上限")
			}
			for _, other := range active {
				d := recordData(&other)
				if domainText(d, "starts_at") < domainText(data, "ends_at") && domainText(d, "ends_at") > domainText(data, "starts_at") {
					return nil, domainError("SCHEDULE_CONFLICT", "排期冲突")
				}
			}
			r.Status = "accepted"
		}
	} else if endpoint == "service.submit_quote" {
		if r.PeerID != c.user || r.Status != "accepted" {
			return nil, domainError("FORBIDDEN", "仅接单斗师可报价")
		}
		amount, err := domainNumber(p, "amount")
		if err != nil || amount <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "报价必须为正整数分")
		}
		expires, err := time.Parse(time.RFC3339, domainText(p, "expires_at"))
		if err != nil || !expires.After(time.Now()) || domainText(p, "items") == "" || domainText(p, "cancel_policy") == "" {
			return nil, domainError("INVALID_ARGUMENT", "报价有效期、项目和取消规则必填")
		}
		data["amount_fen"] = amount
		data["expires_at"] = expires.UTC().Format(time.RFC3339)
		data["items"] = domainText(p, "items")
		data["cancel_policy"] = domainText(p, "cancel_policy")
		r.Status = "quoted"
	} else if endpoint == "service.update_status" {
		next := domainText(p, "status")
		if next == "paid" {
			if r.UserID != c.user || r.Status != "quoted" {
				return nil, domainError("FORBIDDEN", "仅委托人可确认报价")
			}
			if domainText(data, "expires_at") < time.Now().UTC().Format(time.RFC3339) {
				return nil, domainError("EXPIRED", "报价已过期")
			}
			if err := c.checkPayment(r, data); err != nil {
				return nil, err
			}
		} else {
			allowed := (r.Status == "paid" && next == "in_service" && r.PeerID == c.user) || (r.Status == "in_service" && next == "awaiting_confirmation" && r.PeerID == c.user) || (r.Status == "awaiting_confirmation" && (next == "completed" || next == "disputed") && r.UserID == c.user)
			if !allowed {
				return nil, domainError("INVALID_STATE", "服务状态迁移无效")
			}
		}
		r.Status = next
	} else {
		return nil, domainError("UNKNOWN_ENDPOINT", "未登记服务操作")
	}
	if err := c.save(r, data); err != nil {
		return nil, err
	}
	return publicDomain(r), nil
}

func (c *domainContext) media(endpoint string) (interface{}, error) {
	if endpoint == "media.prepare" {
		size, err := domainNumber(c.payload, "file_size")
		if err != nil {
			return nil, err
		}
		prepared, err := PrepareCOSUpload(context.Background(), COSUploadRequest{AppID: c.app, FileName: domainText(c.payload, "file_name"), FileSize: size, ContentType: domainText(c.payload, "content_type"), OwnerType: "resources"})
		if err != nil {
			return nil, err
		}
		r, err := c.create("media", "pending", map[string]interface{}{"file_key": prepared.FileKey, "url": prepared.FinalCosFileURL, "size": size, "content_type": prepared.ContentType})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"id": r.ID, "upload": prepared, "status": r.Status}, nil
	}
	r, err := c.load(domainText(c.payload, "id"), "media")
	if err != nil {
		return nil, err
	}
	data := recordData(r)
	if endpoint == "media.review" {
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可审核媒体")
		}
		next := domainText(c.payload, "status")
		if r.Status != "pending" || (next != "approved" && next != "rejected") || domainText(c.payload, "reason") == "" {
			return nil, domainError("INVALID_STATE", "媒体审核状态或理由无效")
		}
		r.Status = next
		data["review_reason"] = domainText(c.payload, "reason")
	} else if endpoint == "media.delete" {
		if r.UserID != c.user && !c.admin {
			return nil, domainError("FORBIDDEN", "不能删除他人媒体")
		}
		if r.Status == "deleted" {
			return publicDomain(r), nil
		}
		if sdk.CosService == nil {
			return nil, domainError("UNAVAILABLE", "COS服务未配置")
		}
		if err := sdk.CosService.DeleteObject(context.Background(), domainText(data, "file_key")); err != nil {
			return nil, err
		}
		r.Status = "deleted"
	} else {
		return nil, domainError("UNKNOWN_ENDPOINT", "未登记媒体操作")
	}
	if err := c.save(r, data); err != nil {
		return nil, err
	}
	return publicDomain(r), nil
}

// review 处理评价提交与商品评价读取；评价必须绑定本人已完成的订单。
func (c *domainContext) review(endpoint string) (interface{}, error) {
	if endpoint == "review.list" {
		productID, err := domainNumber(c.payload, "product_id")
		if err != nil || productID <= 0 {
			sku := domainText(c.payload, "sku")
			if sku == "" {
				return nil, domainError("INVALID_ARGUMENT", "product_id 或 sku 不能为空")
			}
			var product models.Product
			if err := c.tx.Where("app_id = ? AND sku = ? AND status = ?", c.app, sku, models.ProductStatusActive).First(&product).Error; err != nil {
				return nil, domainError("NOT_FOUND", "商品不存在或已下架")
			}
			productID = product.ID
		}
		limit := 20
		if value, err := domainNumber(c.payload, "limit"); err == nil && value > 0 {
			limit = int(value)
		}
		offset := 0
		if value, err := domainNumber(c.payload, "offset"); err == nil && value > 0 {
			offset = int(value)
		}
		return ListProductReviews(c.app, productID, limit, offset)
	}
	if endpoint != "review.submit" {
		return nil, domainError("UNKNOWN_ENDPOINT", "未登记评价操作")
	}

	orderID := domainText(c.payload, "order_id")
	if orderID == "" {
		return nil, domainError("INVALID_ARGUMENT", "order_id 不能为空")
	}
	rating, err := domainNumber(c.payload, "rating")
	if err != nil || rating < 1 || rating > 5 {
		return nil, domainError("INVALID_ARGUMENT", "评分必须为 1-5")
	}
	content := strings.TrimSpace(domainText(c.payload, "content"))
	if length := utf8.RuneCountInString(content); length < 5 || length > 500 {
		return nil, domainError("INVALID_ARGUMENT", "评价内容需为 5-500 字")
	}

	order, err := c.load(orderID, "order")
	if err != nil {
		return nil, err
	}
	switch order.Status {
	case "created", "cancelled", "refunded":
		return nil, domainError("INVALID_STATE", "订单未完成，暂不可评价")
	}
	orderData := recordData(order)
	productID := recordInt(orderData["product_id"])
	if productID <= 0 {
		return nil, domainError("INVALID_ARGUMENT", "订单缺少商品信息")
	}
	sku, _ := orderData["sku"].(string)

	// 同一订单只允许一条评价；评价量有限，逐条扫描订单号以规避 JSON SQL 的双方言差异。
	var reviews []models.DomainRecord
	if err := c.tx.Where("app_id = ? AND kind = ? AND user_id = ?", c.app, "review", c.user).Find(&reviews).Error; err != nil {
		return nil, err
	}
	for index := range reviews {
		if recordData(&reviews[index])["order_id"] == orderID {
			return nil, domainError("CONFLICT", "该订单已评价")
		}
	}

	nickname := "匿名用户"
	var user models.User
	if err := c.tx.Where("app_id = ? AND id = ?", c.app, c.user).First(&user).Error; err == nil && strings.TrimSpace(user.NickName) != "" {
		nickname = strings.TrimSpace(user.NickName)
	}

	record, err := c.create("review", "published", map[string]interface{}{
		"order_id":   orderID,
		"product_id": productID,
		"sku":        sku,
		"rating":     rating,
		"content":    content,
		"nickname":   nickname,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	// PeerID 用于按商品聚合评价，Rating 用于 SQL 聚合均分与分布；写入后更新以复用统一创建流程。
	record.PeerID = int64(productID)
	record.Rating = int(rating)
	if err := c.tx.Model(record).Updates(map[string]interface{}{"peer_id": record.PeerID, "rating": record.Rating}).Error; err != nil {
		return nil, err
	}
	return publicDomain(record), nil
}

// coupon 处理优惠券定义、领取与用户券列表。
func (c *domainContext) coupon(endpoint string) (interface{}, error) {
	switch endpoint {
	case "coupon.define":
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可定义优惠券")
		}
		title := domainText(c.payload, "title")
		amount, err := domainNumber(c.payload, "amount_fen")
		if err != nil || amount <= 0 || amount > couponMaxAmountFen {
			return nil, domainError("INVALID_ARGUMENT", "优惠金额无效")
		}
		minSpend, err := domainNumber(c.payload, "min_spend_fen")
		if err != nil {
			minSpend = 0
		}
		stock, err := domainNumber(c.payload, "total_stock")
		if err != nil || stock <= 0 {
			return nil, domainError("INVALID_ARGUMENT", "发行数量必须大于 0")
		}
		validUntilText := domainText(c.payload, "valid_until")
		validUntil, err := time.Parse(time.RFC3339, validUntilText)
		if err != nil || !validUntil.After(time.Now()) {
			return nil, domainError("INVALID_ARGUMENT", "有效期必须晚于当前时间")
		}
		if title == "" {
			return nil, domainError("INVALID_ARGUMENT", "优惠券标题不能为空")
		}
		data := map[string]interface{}{
			"title": title, "amount_fen": amount, "min_spend_fen": minSpend,
			"total_stock": stock, "valid_until": validUntil.UTC().Format(time.RFC3339),
		}
		if validFromText := domainText(c.payload, "valid_from"); validFromText != "" {
			validFrom, err := time.Parse(time.RFC3339, validFromText)
			if err != nil || validFrom.After(validUntil) {
				return nil, domainError("INVALID_ARGUMENT", "生效时间无效")
			}
			data["valid_from"] = validFrom.UTC().Format(time.RFC3339)
		}
		if description := domainText(c.payload, "description"); description != "" {
			data["description"] = description
		}
		record, err := c.create(couponKindDefinition, "active", data)
		if err != nil {
			return nil, err
		}
		return publicDomain(record), nil

	case "coupon.claim":
		definitionID := domainText(c.payload, "definition_id")
		if definitionID == "" {
			return nil, domainError("INVALID_ARGUMENT", "definition_id 不能为空")
		}
		var definition models.DomainRecord
		if err := c.tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND app_id = ? AND kind = ?", definitionID, c.app, couponKindDefinition).
			First(&definition).Error; err != nil {
			return nil, domainError("NOT_FOUND", "优惠券不存在")
		}
		if definition.Status != "active" {
			return nil, domainError("INVALID_STATE", "优惠券已停止发放")
		}
		defData := recordData(&definition)
		amount, minSpend, validUntil, title, err := couponRules(defData)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		if now.After(validUntil) {
			return nil, domainError("INVALID_STATE", "优惠券已过期")
		}
		if validFromText := couponString(defData["valid_from"]); validFromText != "" {
			if validFrom, parseErr := time.Parse(time.RFC3339, validFromText); parseErr == nil && now.Before(validFrom) {
				return nil, domainError("INVALID_STATE", "优惠券尚未生效")
			}
		}
		couponID := fmt.Sprintf("coupon_%s_%d", definitionID, c.user)
		var existing models.DomainRecord
		if err := c.tx.Where("id = ? AND app_id = ?", couponID, c.app).First(&existing).Error; err == nil {
			return publicDomain(&existing), nil
		}
		stock := couponInt(defData["total_stock"])
		var claimed int64
		if err := c.tx.Model(&models.DomainRecord{}).
			Where("app_id = ? AND kind = ? AND id LIKE ?", c.app, couponKindUser, "coupon_"+definitionID+"_%").
			Count(&claimed).Error; err != nil {
			return nil, err
		}
		if stock > 0 && claimed >= stock {
			return nil, domainError("CONFLICT", "优惠券已领完")
		}
		data := map[string]interface{}{
			"definition_id": definitionID, "title": title, "amount_fen": amount,
			"min_spend_fen": minSpend, "valid_until": validUntil.UTC().Format(time.RFC3339),
			"claimed_at": now.UTC().Format(time.RFC3339),
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		record := models.DomainRecord{
			ID: couponID, AppID: c.app, UserID: c.user, Kind: couponKindUser, Status: "available",
			Data: string(encoded), Revision: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := c.tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
			return nil, err
		}
		if err := c.tx.Where("id = ?", couponID).First(&record).Error; err != nil {
			return nil, err
		}
		return publicDomain(&record), nil

	case "coupon.cleanup":
		if !c.admin {
			return nil, domainError("FORBIDDEN", "仅管理员可回收优惠券锁定")
		}
		released, err := cleanupCouponReservations(c.tx, c.app)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"released": released}, nil

	case "coupon.list":
		var records []models.DomainRecord
		if err := c.tx.Where("app_id = ? AND user_id = ? AND kind = ?", c.app, c.user, couponKindUser).
			Order("created_at desc, id desc").Limit(100).Find(&records).Error; err != nil {
			return nil, err
		}
		now := time.Now()
		items := make([]map[string]interface{}, 0, len(records))
		for index := range records {
			item := publicDomain(&records[index])
			item["status"] = couponStatus(&records[index], now)
			items = append(items, item)
		}
		return map[string]interface{}{"items": items}, nil
	}
	return nil, domainError("UNKNOWN_ENDPOINT", "未登记优惠券操作")
}
