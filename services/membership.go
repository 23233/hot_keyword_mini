// Package services membership.go
package services

import (
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MembershipService 会员套餐、权益和购买后续处理服务。
type MembershipService struct{}

// NewMembershipService 创建会员服务。
func NewMembershipService() *MembershipService { return &MembershipService{} }

// ListPlans 返回当前小程序启用的会员等级。
func (s *MembershipService) ListPlans(appID string) ([]models.MembershipLevel, error) {
	var plans []models.MembershipLevel
	err := db.Mysql.Where("app_id = ? AND status = ?", appID, "active").Order("level asc").Find(&plans).Error
	return plans, err
}

// GetMembership 返回用户当前会员权益；没有权益时返回 nil、nil。
func (s *MembershipService) GetMembership(appID string, userID int64) (*models.UserMembership, error) {
	var membership models.UserMembership
	err := db.Mysql.Where("app_id = ? AND user_id = ?", appID, userID).First(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

// SavePlan 保存会员套餐并同步可售商品。
func (s *MembershipService) SavePlan(plan *models.MembershipLevel) error {
	if plan == nil || strings.TrimSpace(plan.AppID) == "" || plan.Level <= 0 || strings.TrimSpace(plan.Name) == "" || strings.TrimSpace(plan.SKU) == "" || plan.PriceFen <= 0 || plan.DurationDays <= 0 {
		return errors.New("会员等级参数不完整")
	}
	plan.AppID, plan.Name, plan.SKU = strings.TrimSpace(plan.AppID), strings.TrimSpace(plan.Name), strings.TrimSpace(plan.SKU)
	plan.Status = strings.TrimSpace(plan.Status)
	if plan.Status == "" {
		plan.Status = "active"
	}
	if plan.Status != "active" && plan.Status != "inactive" {
		return errors.New("会员等级状态必须为 active 或 inactive")
	}
	var article models.Article
	if err := db.Mysql.Where("app_id = ? AND is_paid = ? AND pay_sku = ?", plan.AppID, true, plan.SKU).First(&article).Error; err == nil {
		return errors.New("会员套餐 SKU 不能与单篇文章重复")
	}
	now := time.Now()
	plan.UpdatedAt = now
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = now
	}
	return db.Mysql.Transaction(func(tx *gorm.DB) error {
		var existing models.MembershipLevel
		oldSKU := ""
		query := tx.Where("app_id = ?", plan.AppID)
		if plan.ID > 0 {
			query = query.Where("id = ?", plan.ID)
		} else {
			query = query.Where("level = ?", plan.Level)
		}
		err := query.First(&existing).Error
		updates := map[string]interface{}{"level": plan.Level, "name": plan.Name, "sku": plan.SKU, "price_fen": plan.PriceFen, "duration_days": plan.DurationDays, "description": plan.Description, "status": plan.Status, "updated_at": now}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if plan.ID > 0 {
				return errors.New("会员等级不存在或不属于当前小程序")
			}
			if err := tx.Create(plan).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			oldSKU = existing.SKU
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			plan.ID, plan.CreatedAt = existing.ID, existing.CreatedAt
		}
		if oldSKU != "" && oldSKU != plan.SKU {
			if err := tx.Model(&models.Product{}).Where("app_id = ? AND sku = ?", plan.AppID, oldSKU).Update("status", models.ProductStatusInactive).Error; err != nil {
				return err
			}
		}
		product := models.Product{AppID: plan.AppID, SKU: plan.SKU, Name: plan.Name, Description: plan.Description, PriceFen: plan.PriceFen, Status: models.ProductStatusActive, CreatedAt: now, UpdatedAt: now}
		productStatus := models.ProductStatusActive
		if plan.Status == "inactive" {
			productStatus = models.ProductStatusInactive
		}
		if err := tx.Where("app_id = ? AND sku = ?", plan.AppID, plan.SKU).Assign(map[string]interface{}{"name": product.Name, "description": product.Description, "price_fen": product.PriceFen, "status": productStatus, "updated_at": now}).FirstOrCreate(&product).Error; err != nil {
			return err
		}
		return nil
	})
}

// ApplyPaidOrder 在事务中按订单发放权益，订单重放不重复续期。
func (s *MembershipService) ApplyPaidOrder(order *models.PaymentOrder) error {
	return db.Mysql.Transaction(func(tx *gorm.DB) error { return applyOrderGrant(tx, order, false) })
}

// ReversePaidOrder 在事务中撤销指定订单并重算剩余权益。
func (s *MembershipService) ReversePaidOrder(order *models.PaymentOrder) error {
	return db.Mysql.Transaction(func(tx *gorm.DB) error { return applyOrderGrant(tx, order, true) })
}

// applyOrderGrant 与支付状态共用事务；用户权益行锁串行化同一用户的并发支付。
func applyOrderGrant(tx *gorm.DB, order *models.PaymentOrder, reverse bool) error {
	if order == nil || order.AppID == "" || order.UserID <= 0 {
		return errors.New("会员权益订单参数不完整")
	}
	if (!reverse && order.Status != models.PaymentOrderPaid) || (reverse && order.Status != models.PaymentOrderRefunded) {
		return errors.New("订单状态与权益操作不一致")
	}
	now := time.Now()
	seed := models.UserMembership{AppID: order.AppID, UserID: order.UserID, StartedAt: now, ExpiresAt: now, CreatedAt: now, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	var current models.UserMembership
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("app_id = ? AND user_id = ?", order.AppID, order.UserID).First(&current).Error; err != nil {
		return err
	}
	var grant models.MembershipGrant
	err := tx.Where("order_id = ? AND app_id = ? AND user_id = ?", order.ID, order.AppID, order.UserID).First(&grant).Error
	if reverse {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("退款订单缺少权益记录，须先迁移历史权益")
		}
		if err != nil {
			return err
		}
		if grant.ReversedAt != nil {
			return nil
		}
		if err := tx.Model(&grant).Update("reversed_at", now).Error; err != nil {
			return err
		}
	} else {
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var product models.Product
		if err := tx.Where("id = ? AND app_id = ?", order.ProductID, order.AppID).First(&product).Error; err != nil {
			return err
		}
		grant = models.MembershipGrant{AppID: order.AppID, UserID: order.UserID, OrderID: order.ID, GrantedAt: now}
		if order.PaidAt != nil {
			grant.GrantedAt = *order.PaidAt
		}
		var articles []models.Article
		if err := tx.Where("app_id = ? AND is_paid = ? AND pay_sku = ? AND status IN ?", order.AppID, true, product.SKU, []string{"published", "archived"}).Find(&articles).Error; err != nil {
			return err
		}
		if len(articles) > 1 {
			return errors.New("商品绑定多个文章权益")
		}
		if len(articles) == 1 {
			grant.ArticleID = articles[0].ID
		} else {
			var plan models.MembershipLevel
			if err := tx.Where("app_id = ? AND sku = ?", order.AppID, product.SKU).First(&plan).Error; err != nil {
				return fmt.Errorf("商品未绑定权益: %w", err)
			}
			if plan.Level <= 0 || plan.DurationDays <= 0 {
				return errors.New("会员套餐无效")
			}
			grant.Level, grant.DurationDays = plan.Level, plan.DurationDays
		}
		if err := tx.Create(&grant).Error; err != nil {
			return err
		}
	}
	if grant.ArticleID > 0 {
		if reverse {
			return tx.Where("app_id = ? AND user_id = ? AND order_id = ?", order.AppID, order.UserID, order.ID).Delete(&models.ArticlePurchase{}).Error
		}
		purchase := models.ArticlePurchase{AppID: order.AppID, UserID: order.UserID, ArticleID: grant.ArticleID, OrderID: order.ID, PurchasedAt: grant.GrantedAt, CreatedAt: now, UpdatedAt: now}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&purchase).Error
	}
	var grants []models.MembershipGrant
	if err := tx.Where("app_id = ? AND user_id = ? AND article_id = 0 AND reversed_at IS NULL", order.AppID, order.UserID).Order("granted_at asc, id asc").Find(&grants).Error; err != nil {
		return err
	}
	var calculated *models.UserMembership
	for _, entry := range grants {
		plan := models.MembershipLevel{Level: entry.Level, DurationDays: entry.DurationDays}
		snapshot := models.PaymentOrder{ID: entry.OrderID, AppID: entry.AppID, UserID: entry.UserID}
		next, _, err := calculateMembershipGrant(calculated, plan, &snapshot, entry.GrantedAt)
		if err != nil {
			return err
		}
		calculated = &next
	}
	if calculated == nil {
		calculated = &models.UserMembership{StartedAt: now, ExpiresAt: now}
	}
	return tx.Model(&current).Updates(map[string]interface{}{"level": calculated.Level, "started_at": calculated.StartedAt, "expires_at": calculated.ExpiresAt, "last_order_id": calculated.LastOrderID, "updated_at": now}).Error
}

// calculateMembershipGrant 纯计算会员顺延、升级折算和订单幂等结果。
func calculateMembershipGrant(current *models.UserMembership, plan models.MembershipLevel, order *models.PaymentOrder, now time.Time) (models.UserMembership, bool, error) {
	if order == nil || plan.Level <= 0 || plan.DurationDays <= 0 {
		return models.UserMembership{}, false, errors.New("会员权益参数无效")
	}
	if current == nil {
		return models.UserMembership{AppID: order.AppID, UserID: order.UserID, Level: plan.Level, StartedAt: now, ExpiresAt: now.AddDate(0, 0, plan.DurationDays), LastOrderID: order.ID, CreatedAt: now, UpdatedAt: now}, true, nil
	}
	result := *current
	if result.LastOrderID == order.ID {
		return result, false, nil
	}
	if result.Level > plan.Level {
		return result, false, fmt.Errorf("当前会员等级为 level %d，不能购买较低等级套餐", result.Level)
	}
	base := now
	durationDays := plan.DurationDays
	if result.Level == plan.Level && result.ExpiresAt.After(now) {
		base = result.ExpiresAt
	} else if result.Level < plan.Level && result.ExpiresAt.After(now) {
		remainingDays := result.ExpiresAt.Sub(now).Hours() / 24
		durationDays += int(math.Floor(remainingDays * float64(result.Level) / float64(plan.Level)))
	}
	result.Level = plan.Level
	result.ExpiresAt = base.AddDate(0, 0, durationDays)
	result.LastOrderID = order.ID
	result.UpdatedAt = now
	return result, true, nil
}

// SeedDefaultPlans 创建 AI 破甲的三档会员套餐，并同步支付商品。
func (s *MembershipService) SeedDefaultPlans(appID string) error {
	plans := []models.MembershipLevel{
		{Level: 1, Name: "普通会员", SKU: "ai_breakthrough_member_1", PriceFen: 990, DurationDays: 30, Description: "解锁 level 1 会员文章", Status: "active"},
		{Level: 2, Name: "高级会员", SKU: "ai_breakthrough_member_2", PriceFen: 2990, DurationDays: 90, Description: "解锁 level 1-2 会员文章", Status: "active"},
		{Level: 3, Name: "年度会员", SKU: "ai_breakthrough_member_3", PriceFen: 9900, DurationDays: 365, Description: "解锁全部会员文章", Status: "active"},
	}
	for _, plan := range plans {
		plan.AppID = appID
		if err := db.Mysql.Where("app_id = ? AND level = ?", appID, plan.Level).Assign(map[string]interface{}{
			"name": plan.Name, "sku": plan.SKU, "price_fen": plan.PriceFen, "duration_days": plan.DurationDays, "description": plan.Description, "status": plan.Status, "updated_at": time.Now(),
		}).FirstOrCreate(&plan).Error; err != nil {
			return err
		}
		product := models.Product{AppID: appID, SKU: plan.SKU, Name: plan.Name, Description: plan.Description, PriceFen: plan.PriceFen, Status: models.ProductStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := db.Mysql.Where("app_id = ? AND sku = ?", appID, plan.SKU).Assign(map[string]interface{}{
			"name": product.Name, "description": product.Description, "price_fen": product.PriceFen, "status": product.Status, "updated_at": product.UpdatedAt,
		}).FirstOrCreate(&product).Error; err != nil {
			return err
		}
	}
	return nil
}

// NormalizeArticleTags 规范化文章标签文本。
func NormalizeArticleTags(tags []string) string {
	clean := make([]string, 0, len(tags))
	for _, tag := range tags {
		if value := strings.TrimSpace(tag); value != "" {
			clean = append(clean, value)
		}
	}
	return strings.Join(clean, ",")
}
