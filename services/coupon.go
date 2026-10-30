// Package services coupon.go
package services

import (
	"encoding/json"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// couponKindDefinition 优惠券定义记录类型。
	couponKindDefinition = "coupon_definition"
	// couponKindUser 用户已领取优惠券记录类型。
	couponKindUser = "coupon"
	// couponMaxAmountFen 单张优惠券最大抵扣金额（1 万元）。
	couponMaxAmountFen = 1000000
	// couponReservationTTL 优惠券支付锁定超时时间。
	// 必须大于微信预支付订单有效期（默认 2 小时），避免旧支付单仍可支付时优惠券已被回收。
	couponReservationTTL = 3 * time.Hour
)

// couponRules 解析优惠券快照中的金额、门槛、有效期与标题。
func couponRules(data map[string]interface{}) (int64, int64, time.Time, string, error) {
	amount := couponInt(data["amount_fen"])
	minSpend := couponInt(data["min_spend_fen"])
	title := strings.TrimSpace(couponString(data["title"]))
	validUntilText := couponString(data["valid_until"])
	if amount <= 0 || amount > couponMaxAmountFen || minSpend < 0 || title == "" || validUntilText == "" {
		return 0, 0, time.Time{}, "", domainError("INVALID_ARGUMENT", "优惠券快照无效")
	}
	validUntil, err := time.Parse(time.RFC3339, validUntilText)
	if err != nil {
		return 0, 0, time.Time{}, "", domainError("INVALID_ARGUMENT", "优惠券有效期格式无效")
	}
	return amount, minSpend, validUntil, title, nil
}

// couponInt 将 JSON 数值安全转换为 int64。
func couponInt(value interface{}) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case int:
		return int64(number)
	case int64:
		return number
	}
	return 0
}

// couponString 将 JSON 字符串安全转换。
func couponString(value interface{}) string {
	text, _ := value.(string)
	return text
}

// couponStatus 计算优惠券展示状态（可用券超过有效期时显示 expired）。
func couponStatus(record *models.DomainRecord, now time.Time) string {
	data := recordData(record)
	switch record.Status {
	case "reserved":
		if reservedAt, err := time.Parse(time.RFC3339, couponString(data["reserved_at"])); err == nil && now.Sub(reservedAt) >= couponReservationTTL {
			return "available"
		}
		return "locked"
	case "available":
		if validUntil, err := time.Parse(time.RFC3339, couponString(data["valid_until"])); err == nil && now.After(validUntil) {
			return "expired"
		}
		return "available"
	}
	return record.Status
}

// ReserveCouponForOrder 原子锁定用户可用优惠券并返回抵扣金额。
func ReserveCouponForOrder(appID string, userID int64, couponID string, totalFen int64) (int64, error) {
	couponID = strings.TrimSpace(couponID)
	if couponID == "" {
		return 0, nil
	}
	if db.DB == nil {
		return 0, domainError("UNAVAILABLE", "数据库未初始化")
	}
	var discount int64
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var record models.DomainRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND app_id = ? AND user_id = ? AND kind = ?", couponID, appID, userID, couponKindUser).
			First(&record).Error; err != nil {
			return domainError("NOT_FOUND", "优惠券不存在")
		}
		data := recordData(&record)
		now := time.Now()
		if record.Status == "reserved" {
			reservedAt, err := time.Parse(time.RFC3339, couponString(data["reserved_at"]))
			if err != nil || now.Sub(reservedAt) < couponReservationTTL {
				return domainError("CONFLICT", "优惠券正在使用中")
			}
			// 超过锁定时间视为支付流程中断，允许重新使用。
		} else if record.Status != "available" {
			return domainError("CONFLICT", "优惠券不可用")
		}
		amount, minSpend, validUntil, title, err := couponRules(data)
		if err != nil {
			return err
		}
		if now.After(validUntil) {
			return domainError("INVALID_STATE", "优惠券已过期")
		}
		if totalFen < minSpend {
			return domainError("INVALID_ARGUMENT", "未达到优惠券使用门槛")
		}
		discount = amount
		if discount >= totalFen {
			discount = totalFen - 1
		}
		if discount <= 0 {
			return domainError("INVALID_ARGUMENT", "优惠券不适用于当前订单")
		}
		data["title"] = title
		data["reserved_at"] = now.UTC().Format(time.RFC3339)
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return tx.Model(&models.DomainRecord{}).Where("id = ?", record.ID).
			Updates(map[string]interface{}{"status": "reserved", "revision": gorm.Expr("revision + 1"), "data": string(encoded), "updated_at": now}).Error
	})
	if err != nil {
		return 0, err
	}
	return discount, nil
}

// ReleaseReservedCoupon 在支付单创建失败时释放已锁定的优惠券。
func ReleaseReservedCoupon(appID string, userID int64, couponID string) error {
	couponID = strings.TrimSpace(couponID)
	if couponID == "" || db.DB == nil {
		return nil
	}
	return db.DB.Transaction(func(tx *gorm.DB) error {
		return setCouponStatus(tx, appID, userID, couponID, "available", "reserved", "created_failed")
	})
}

// consumeCouponTx 支付成功后把优惠券标记为已使用。
func consumeCouponTx(tx *gorm.DB, order *models.PaymentOrder) error {
	if order == nil || strings.TrimSpace(order.CouponID) == "" {
		return nil
	}
	return setCouponStatus(tx, order.AppID, order.UserID, order.CouponID, "used", "reserved", order.OutTradeNo)
}

// releaseCouponTx 退款、关闭或失败时恢复优惠券可用状态。
func releaseCouponTx(tx *gorm.DB, order *models.PaymentOrder, reason string) error {
	if order == nil || strings.TrimSpace(order.CouponID) == "" {
		return nil
	}
	return setCouponStatus(tx, order.AppID, order.UserID, order.CouponID, "available", "reserved|used", reason)
}

// releaseCouponOrder 在支付预下单失败后释放订单锁定的优惠券。
func releaseCouponOrder(order *models.PaymentOrder) error {
	if order == nil || strings.TrimSpace(order.CouponID) == "" || db.DB == nil {
		return nil
	}
	return db.DB.Transaction(func(tx *gorm.DB) error {
		return releaseCouponTx(tx, order, "prepay_failed")
	})
}

// CleanupStaleCouponReservations 回收所有租户中超过锁定 TTL 的优惠券，返回回收数量。
// 供进程内定时任务调用；重复执行与多实例并发均安全。
func CleanupStaleCouponReservations() (int, error) {
	if db.DB == nil {
		return 0, nil
	}
	return cleanupCouponReservations(db.DB, "")
}

// cleanupCouponReservations 回收指定租户（appID 为空表示全部租户）的超时锁定优惠券。
func cleanupCouponReservations(tx *gorm.DB, appID string) (int, error) {
	query := tx.Where("kind = ? AND status = ?", couponKindUser, "reserved")
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}
	var locked []models.DomainRecord
	if err := query.Find(&locked).Error; err != nil {
		return 0, err
	}
	now := time.Now()
	released := 0
	for index := range locked {
		data := recordData(&locked[index])
		reservedAt, err := time.Parse(time.RFC3339, couponString(data["reserved_at"]))
		if err != nil || now.Sub(reservedAt) < couponReservationTTL {
			continue
		}
		delete(data, "reserved_at")
		data["status"] = "available"
		data["status_reason"] = "reservation_expired"
		encoded, err := json.Marshal(data)
		if err != nil {
			return released, err
		}
		if err := tx.Model(&models.DomainRecord{}).Where("id = ?", locked[index].ID).
			Updates(map[string]interface{}{"status": "available", "revision": gorm.Expr("revision + 1"), "data": string(encoded), "updated_at": now}).Error; err != nil {
			return released, err
		}
		released++
	}
	return released, nil
}

// setCouponStatus 以条件更新切换优惠券状态，兼容重复回调。
func setCouponStatus(tx *gorm.DB, appID string, userID int64, couponID, next, from, reason string) error {
	var record models.DomainRecord
	if err := tx.Where("id = ? AND app_id = ? AND user_id = ? AND kind = ?", couponID, appID, userID, couponKindUser).First(&record).Error; err != nil {
		return domainError("NOT_FOUND", "优惠券记录不存在")
	}
	if record.Status == next {
		return nil
	}
	allowed := false
	for _, current := range strings.Split(from, "|") {
		if record.Status == current {
			allowed = true
			break
		}
	}
	if !allowed {
		return domainError("INVALID_STATE", fmt.Sprintf("优惠券状态 %s 不能切换为 %s", record.Status, next))
	}
	data := recordData(&record)
	data["status"] = next
	data["status_reason"] = reason
	if next == "used" {
		data["used_at"] = time.Now().UTC().Format(time.RFC3339)
		data["payment_order"] = reason
	} else {
		delete(data, "reserved_at")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return tx.Model(&models.DomainRecord{}).Where("id = ?", record.ID).
		Updates(map[string]interface{}{"status": next, "revision": gorm.Expr("revision + 1"), "data": string(encoded), "updated_at": time.Now()}).Error
}
