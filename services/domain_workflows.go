// Package services domain_workflows.go
package services

import (
	"context"
	"hot_keyword/models"
	"hot_keyword/sdk"
	"strings"
	"time"
)

func (c *domainContext) order(endpoint string) (interface{}, error) {
	p := c.payload
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
	// 支付订单只允许绑定一个领域实体，绑定记录与状态在同一事务提交。
	var count int64
	if err := c.tx.Model(&models.DomainRecord{}).Where("app_id = ? AND id <> ? AND JSON_UNQUOTE(JSON_EXTRACT(data, '$.out_trade_no')) = ?", c.app, r.ID, payment.OutTradeNo).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return domainError("PAYMENT_REUSED", "付款已绑定其他订单")
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
