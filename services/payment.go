// Package services payment.go
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/23233/ggg/ut"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// JSAPIPaymentParams 小程序 wx.requestPayment 所需参数。
type JSAPIPaymentParams struct {
	AppID     string `json:"appId"`
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

// PaymentService 微信支付普通商户服务。
type PaymentService struct{}

var paymentClientCache = struct {
	sync.Mutex
	items map[string]paymentClientEntry
}{items: make(map[string]paymentClientEntry)}

type paymentClientEntry struct {
	client *core.Client
	mchID  string
}

// NewPaymentService 创建支付服务。
func NewPaymentService() *PaymentService { return &PaymentService{} }

// CreateSandboxOrder 创建本地支付沙箱订单。沙箱不访问微信支付，也不要求商户参数。
func (s *PaymentService) CreateSandboxOrder(appID string, userID int64, openID, productSKU, idempotencyKey string) (*models.PaymentOrder, error) {
	return s.CreateSandboxOrderWithCoupon(appID, userID, openID, productSKU, idempotencyKey, "")
}

func (s *PaymentService) CreateSandboxOrderWithCoupon(appID string, userID int64, openID, productSKU, idempotencyKey, couponID string) (*models.PaymentOrder, error) {
	if db.DB == nil || appID == "" || userID <= 0 || openID == "" || strings.TrimSpace(productSKU) == "" {
		return nil, errors.New("支付沙箱订单参数不完整")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("sandbox-%s-%d", productSKU, time.Now().UnixNano())
	}
	attach := "sandbox:" + idempotencyKey
	var existing models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; err == nil {
		return &existing, nil
	}
	var product models.Product
	if err := db.DB.Where("app_id = ? AND sku = ? AND status = ?", appID, productSKU, models.ProductStatusActive).First(&product).Error; err != nil {
		return nil, errors.New("商品不存在或已下架")
	}
	discount, reserveErr := ReserveCouponForOrder(appID, userID, couponID, product.PriceFen)
	if reserveErr != nil {
		return nil, reserveErr
	}
	amountFen := product.PriceFen - discount
	now := time.Now()
	order := &models.PaymentOrder{AppID: appID, UserID: userID, ProductID: product.ID, OutTradeNo: "S" + time.Now().Format("20060102150405") + strings.ToUpper(ut.RandomStr(10)), OpenID: openID, Description: product.Name, AmountFen: amountFen, CouponID: couponID, DiscountFen: discount, Status: models.PaymentOrderPending, Attach: attach, Sandbox: true, CreatedAt: now, UpdatedAt: now}
	if err := db.DB.Create(order).Error; err != nil {
		_ = ReleaseReservedCoupon(appID, userID, couponID)
		if isDuplicateKeyError(err) && db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("创建支付沙箱订单失败: %w", err)
	}
	return order, nil
}

// ApplySandboxTransition 本地回调与正式回调共用事务和权益逻辑。
func (s *PaymentService) ApplySandboxTransition(appID string, userID int64, outTradeNo, transition string) (*models.PaymentOrder, error) {
	if db.DB == nil || appID == "" || userID <= 0 || strings.TrimSpace(outTradeNo) == "" {
		return nil, errors.New("支付沙箱参数不完整")
	}
	var order models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND user_id = ? AND out_trade_no = ? AND sandbox = ?", appID, userID, outTradeNo, true).First(&order).Error; err != nil {
		return nil, errors.New("支付沙箱订单不存在")
	}
	return commitPaymentTransition(&order, transition, "sandbox-tx-"+strings.ToLower(ut.RandomStr(10)), nil)
}

// commitPaymentTransition 原子提交订单与权益，行锁防止回调、查单、退款互相覆盖。
func commitPaymentTransition(input *models.PaymentOrder, transition, transactionID string, paidAt *time.Time) (*models.PaymentOrder, error) {
	var order models.PaymentOrder
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND app_id = ? AND user_id = ?", input.ID, input.AppID, input.UserID).First(&order).Error; err != nil {
			return err
		}
		next, idempotent, err := nextSandboxPaymentStatus(order.Status, transition)
		if err != nil {
			return err
		}
		if !idempotent {
			now := time.Now()
			order.Status, order.UpdatedAt = next, now
			if next == models.PaymentOrderPaid {
				order.TransactionID = transactionID
				order.PaidAt = paidAt
				if paidAt == nil {
					order.PaidAt = &now
				}
			}
			if next == models.PaymentOrderRefunded {
				order.RefundedAt = &now
			}
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
		}
		if next == models.PaymentOrderPaid {
			if err := applyCartOrder(tx, &order, false); err != nil {
				return err
			}
			if err := consumeCouponTx(tx, &order); err != nil {
				return err
			}
			return applyOrderGrant(tx, &order, false)
		}
		if next == models.PaymentOrderRefunded {
			if err := applyCartOrder(tx, &order, true); err != nil {
				return err
			}
			if err := releaseCouponTx(tx, &order, "refund"); err != nil {
				return err
			}
			return applyOrderGrant(tx, &order, true)
		}
		if next == models.PaymentOrderClosed || next == models.PaymentOrderFailed {
			if err := releaseCouponTx(tx, &order, next); err != nil {
				return err
			}
		}
		return nil
	})
	return &order, err
}

// applyCartOrder 为购物车支付订单落地领域订单；退款时同步更新订单状态。
func applyCartOrder(tx *gorm.DB, order *models.PaymentOrder, reverse bool) error {
	if order == nil || order.Kind != models.PaymentOrderKindCart {
		return nil
	}
	recordID := "cart_" + order.OutTradeNo
	now := time.Now()
	if !reverse {
		var items []map[string]interface{}
		if err := json.Unmarshal([]byte(order.Items), &items); err != nil {
			return err
		}
		data := map[string]interface{}{
			"items":            items,
			"amount_fen":       order.AmountFen,
			"out_trade_no":     order.OutTradeNo,
			"payment_order_id": order.ID,
			"source":           "cart",
			"description":      order.Description,
		}
		if order.PaidAt != nil {
			data["paid_at"] = order.PaidAt.UTC().Format(time.RFC3339)
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		record := models.DomainRecord{
			ID: recordID, AppID: order.AppID, UserID: order.UserID, Kind: "order", Status: "confirmed",
			Data: string(encoded), Revision: 1, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error
	}
	result := tx.Model(&models.DomainRecord{}).
		Where("id = ? AND app_id = ? AND kind = ? AND status <> ?", recordID, order.AppID, "order", "refunded").
		Updates(map[string]interface{}{"status": "refunded", "revision": gorm.Expr("revision + 1"), "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 已退款订单重复回调保持幂等；只有记录完全缺失时才报错。
		var exists int64
		if err := tx.Model(&models.DomainRecord{}).Where("id = ? AND app_id = ? AND kind = ?", recordID, order.AppID, "order").Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return errors.New("购物车订单记录不存在")
		}
	}
	return nil
}

// nextSandboxPaymentStatus 校验支付沙箱状态迁移，并标记重复回调。
func nextSandboxPaymentStatus(current, transition string) (string, bool, error) {
	transition = strings.ToLower(strings.TrimSpace(transition))
	targets := map[string]string{
		"success": models.PaymentOrderPaid, "pay": models.PaymentOrderPaid, "paid": models.PaymentOrderPaid,
		"cancel": models.PaymentOrderClosed, "closed": models.PaymentOrderClosed,
		"fail": models.PaymentOrderFailed, "failed": models.PaymentOrderFailed,
		"refund": models.PaymentOrderRefunded, "refunded": models.PaymentOrderRefunded,
	}
	target, ok := targets[transition]
	if !ok {
		return "", false, errors.New("支付沙箱状态仅支持 success、cancel、fail 或 refund")
	}
	if current == target {
		return target, true, nil
	}
	allowed := current == models.PaymentOrderPending && target != models.PaymentOrderRefunded || current == models.PaymentOrderPaid && target == models.PaymentOrderRefunded
	if !allowed {
		return "", false, fmt.Errorf("订单状态 %s 不允许迁移为 %s", current, target)
	}
	return target, false, nil
}

// ApplySandboxNotify 模拟微信支付回调，保持订单更新与正式回调相同的幂等语义。
func (s *PaymentService) ApplySandboxNotify(appID, outTradeNo string, success bool) (*models.PaymentOrder, error) {
	var order models.PaymentOrder
	if db.DB == nil || appID == "" || outTradeNo == "" {
		return nil, errors.New("支付沙箱回调参数不完整")
	}
	if err := db.DB.Where("app_id = ? AND out_trade_no = ? AND sandbox = ?", appID, outTradeNo, true).First(&order).Error; err != nil {
		return nil, errors.New("支付沙箱订单不存在")
	}
	if success {
		return s.ApplySandboxTransition(appID, order.UserID, outTradeNo, "success")
	}
	return s.ApplySandboxTransition(appID, order.UserID, outTradeNo, "fail")
}

// clientForApp 按小程序 AppID 构造并缓存独立微信支付客户端。
func (s *PaymentService) clientForApp(ctx context.Context, app *models.MiniApp) (*core.Client, error) {
	if app == nil || app.AppID == "" {
		return nil, errors.New("小程序配置不能为空")
	}
	if app.PaymentMchID == "" || app.PaymentMchSerialNo == "" || app.PaymentAPIv3Key == "" || app.PaymentPrivateKey == "" {
		return nil, errors.New("当前小程序未完整配置微信支付商户参数")
	}
	paymentClientCache.Lock()
	defer paymentClientCache.Unlock()
	if entry := paymentClientCache.items[app.AppID]; entry.client != nil {
		return entry.client, nil
	}
	privateKey, err := utils.LoadPrivateKey(app.PaymentPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("加载微信支付商户私钥失败: %w", err)
	}
	client, err := core.NewClient(ctx, option.WithWechatPayAutoAuthCipher(app.PaymentMchID, app.PaymentMchSerialNo, privateKey, app.PaymentAPIv3Key))
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付客户端失败: %w", err)
	}
	paymentClientCache.items[app.AppID] = paymentClientEntry{client: client, mchID: app.PaymentMchID}
	return client, nil
}

// InvalidatePaymentClient 清除指定小程序支付客户端缓存。
func InvalidatePaymentClient(appID string) {
	paymentClientCache.Lock()
	entry := paymentClientCache.items[appID]
	delete(paymentClientCache.items, appID)
	paymentClientCache.Unlock()
	if entry.mchID == "" {
		return
	}
	paymentClientCache.Lock()
	shared := false
	for _, other := range paymentClientCache.items {
		if other.mchID == entry.mchID {
			shared = true
			break
		}
	}
	paymentClientCache.Unlock()
	if !shared {
		downloader.MgrInstance().RemoveDownloader(context.Background(), entry.mchID)
	}
}

// CreateJSAPIOrder 创建商品订单并返回前端调起支付参数。
// prepareCartPayment 校验购物车并生成订单快照、金额与描述。
func prepareCartPayment(appID string, items []map[string]interface{}) (string, int64, int64, error) {
	if err := validateCartBenefits(appID, items); err != nil {
		return "", 0, 0, err
	}
	quoted, err := QuoteCart(appID, items)
	if err != nil {
		return "", 0, 0, err
	}
	totalFen, _ := quoted["total_fen"].(int64)
	count, _ := quoted["count"].(int64)
	quotedItems, _ := quoted["items"].([]map[string]interface{})
	encoded, err := json.Marshal(quotedItems)
	if err != nil {
		return "", 0, 0, err
	}
	description := fmt.Sprintf("购物车结算（%d 件）", count)
	if len([]byte(description)) > 127 {
		return "", 0, 0, errors.New("购物车订单描述过长")
	}
	return string(encoded), totalFen, count, nil
}

// validateCartBenefits 拒绝把会员套餐或文章权益商品混入购物车结算。
// 权益按单商品订单发放，购物车订单无法拆分多份权益，必须在支付前明确拒绝。
func validateCartBenefits(appID string, items []map[string]interface{}) error {
	skus := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		sku := strings.TrimSpace(domainText(item, "sku"))
		if sku == "" || seen[sku] {
			continue
		}
		seen[sku] = true
		skus = append(skus, sku)
	}
	if len(skus) == 0 {
		return nil
	}
	var plan models.MembershipLevel
	if err := db.DB.Where("app_id = ? AND sku IN ?", appID, skus).First(&plan).Error; err == nil {
		return domainError("INVALID_ARGUMENT", "会员套餐请单独购买: "+plan.SKU)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var article models.Article
	if err := db.DB.Where("app_id = ? AND is_paid = ? AND pay_sku IN ?", appID, true, skus).First(&article).Error; err == nil {
		return domainError("INVALID_ARGUMENT", "文章权益商品请单独购买: "+article.PaySKU)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

// CreateSandboxCartOrder 创建本地多 SKU 支付沙箱订单。
func (s *PaymentService) CreateSandboxCartOrder(appID string, userID int64, openID string, items []map[string]interface{}, idempotencyKey string) (*models.PaymentOrder, error) {
	return s.CreateSandboxCartOrderWithCoupon(appID, userID, openID, items, idempotencyKey, "")
}

func (s *PaymentService) CreateSandboxCartOrderWithCoupon(appID string, userID int64, openID string, items []map[string]interface{}, idempotencyKey, couponID string) (*models.PaymentOrder, error) {
	if db.DB == nil || appID == "" || userID <= 0 || openID == "" {
		return nil, errors.New("支付沙箱订单参数不完整")
	}
	itemsJSON, totalFen, count, err := prepareCartPayment(appID, items)
	if err != nil {
		return nil, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("cart-sandbox-%d", time.Now().UnixNano())
	}
	attach := "sandbox:" + idempotencyKey
	var existing models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; err == nil {
		return &existing, nil
	}
	discount, reserveErr := ReserveCouponForOrder(appID, userID, couponID, totalFen)
	if reserveErr != nil {
		return nil, reserveErr
	}
	amountFen := totalFen - discount
	now := time.Now()
	order := &models.PaymentOrder{
		AppID: appID, UserID: userID, ProductID: 0, Kind: models.PaymentOrderKindCart, Items: itemsJSON,
		OutTradeNo: "S" + time.Now().Format("20060102150405") + strings.ToUpper(ut.RandomStr(10)), OpenID: openID,
		Description: fmt.Sprintf("购物车结算（%d 件）", count), AmountFen: amountFen, CouponID: couponID, DiscountFen: discount, Status: models.PaymentOrderPending,
		Attach: attach, Sandbox: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.DB.Create(order).Error; err != nil {
		_ = ReleaseReservedCoupon(appID, userID, couponID)
		if isDuplicateKeyError(err) && db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("创建购物车支付沙箱订单失败: %w", err)
	}
	return order, nil
}

// CreateCartJSAPIOrder 创建多 SKU 支付订单并返回 wx.requestPayment 参数。
func (s *PaymentService) CreateCartJSAPIOrder(ctx context.Context, appID string, userID int64, openID string, items []map[string]interface{}, idempotencyKey string) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
	return s.CreateCartJSAPIOrderWithCoupon(ctx, appID, userID, openID, items, idempotencyKey, "")
}

func (s *PaymentService) CreateCartJSAPIOrderWithCoupon(ctx context.Context, appID string, userID int64, openID string, items []map[string]interface{}, idempotencyKey, couponID string) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
	if appID == "" || userID <= 0 || openID == "" {
		return nil, nil, errors.New("支付订单参数不完整")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("cart-%s-%d", ut.RandomStr(12), time.Now().UnixNano())
	}
	attach := "idem:" + idempotencyKey
	if len(attach) > 128 {
		return nil, nil, errors.New("幂等键长度不能超过 123 个字符")
	}
	var app models.MiniApp
	if err := db.DB.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil, nil, fmt.Errorf("小程序配置不存在: %w", err)
	}
	if config.Cfg == nil {
		return nil, nil, errors.New("服务公共域名未配置")
	}
	if app.PaymentMchID == "" || app.PaymentMchSerialNo == "" || app.PaymentAPIv3Key == "" || app.PaymentPrivateKey == "" {
		return nil, nil, errors.New("当前小程序未完整配置微信支付商户参数")
	}
	if _, err := config.Cfg.PaymentNotifyURL(app.AppID); err != nil {
		return nil, nil, err
	}
	itemsJSON, totalFen, count, err := prepareCartPayment(appID, items)
	if err != nil {
		return nil, nil, err
	}
	var existing models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; err == nil {
		return s.reusePaymentOrder(ctx, &app, &existing)
	}
	discount, reserveErr := ReserveCouponForOrder(appID, userID, couponID, totalFen)
	if reserveErr != nil {
		return nil, nil, reserveErr
	}
	amountFen := totalFen - discount
	outTradeNo := fmt.Sprintf("%s%d%s", time.Now().Format("20060102150405"), userID%100000, strings.ToUpper(ut.RandomStr(8)))
	order := &models.PaymentOrder{
		AppID: appID, UserID: userID, ProductID: 0, Kind: models.PaymentOrderKindCart, Items: itemsJSON,
		OutTradeNo: outTradeNo, OpenID: openID, Description: fmt.Sprintf("购物车结算（%d 件）", count),
		AmountFen: amountFen, CouponID: couponID, DiscountFen: discount, Status: models.PaymentOrderPending, Attach: attach, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.DB.Create(order).Error; err != nil {
		_ = ReleaseReservedCoupon(appID, userID, couponID)
		if isDuplicateKeyError(err) {
			if findErr := db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; findErr == nil {
				return s.reusePaymentOrder(ctx, &app, &existing)
			}
		}
		return nil, nil, fmt.Errorf("创建购物车支付订单失败: %w", err)
	}
	return s.prepayOrder(ctx, &app, order)
}

func (s *PaymentService) CreateJSAPIOrder(ctx context.Context, appID string, userID int64, openID, productSKU, idempotencyKey string) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
	return s.CreateJSAPIOrderWithCoupon(ctx, appID, userID, openID, productSKU, idempotencyKey, "")
}

func (s *PaymentService) CreateJSAPIOrderWithCoupon(ctx context.Context, appID string, userID int64, openID, productSKU, idempotencyKey, couponID string) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
	if appID == "" || userID <= 0 || openID == "" || productSKU == "" {
		return nil, nil, errors.New("支付订单参数不完整")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("%s-%s-%d", productSKU, ut.RandomStr(12), time.Now().UnixNano())
	}
	attach := "idem:" + idempotencyKey
	if len(attach) > 128 {
		return nil, nil, errors.New("幂等键长度不能超过 123 个字符")
	}
	var app models.MiniApp
	if err := db.DB.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil, nil, fmt.Errorf("小程序配置不存在: %w", err)
	}
	var product models.Product
	if err := db.DB.Where("app_id = ? AND sku = ? AND status = ?", appID, productSKU, models.ProductStatusActive).First(&product).Error; err != nil {
		return nil, nil, errors.New("商品不存在或已下架")
	}
	if product.PriceFen <= 0 {
		return nil, nil, errors.New("商品金额必须大于 0")
	}
	if len([]byte(product.Name)) > 127 {
		return nil, nil, errors.New("商品名称不能超过 127 个字节")
	}
	if config.Cfg == nil {
		return nil, nil, errors.New("服务公共域名未配置")
	}
	if app.PaymentMchID == "" || app.PaymentMchSerialNo == "" || app.PaymentAPIv3Key == "" || app.PaymentPrivateKey == "" {
		return nil, nil, errors.New("当前小程序未完整配置微信支付商户参数")
	}
	if _, err := config.Cfg.PaymentNotifyURL(app.AppID); err != nil {
		return nil, nil, err
	}
	var existing models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; err == nil {
		return s.reusePaymentOrder(ctx, &app, &existing)
	}
	discount, reserveErr := ReserveCouponForOrder(appID, userID, couponID, product.PriceFen)
	if reserveErr != nil {
		return nil, nil, reserveErr
	}
	amountFen := product.PriceFen - discount
	outTradeNo := fmt.Sprintf("%s%d%s", time.Now().Format("20060102150405"), userID%100000, strings.ToUpper(ut.RandomStr(8)))
	order := &models.PaymentOrder{AppID: appID, UserID: userID, ProductID: product.ID, OutTradeNo: outTradeNo, OpenID: openID, Description: product.Name, AmountFen: amountFen, CouponID: couponID, DiscountFen: discount, Status: models.PaymentOrderPending, Attach: attach, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.DB.Create(order).Error; err != nil {
		_ = ReleaseReservedCoupon(appID, userID, couponID)
		if isDuplicateKeyError(err) {
			if findErr := db.DB.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; findErr == nil {
				return s.reusePaymentOrder(ctx, &app, &existing)
			}
		}
		return nil, nil, fmt.Errorf("创建支付订单失败: %w", err)
	}
	return s.prepayOrder(ctx, &app, order)
}

func (s *PaymentService) prepayOrder(ctx context.Context, app *models.MiniApp, order *models.PaymentOrder) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
	client, err := s.clientForApp(ctx, app)
	if err != nil {
		_ = db.DB.Model(order).Update("status", models.PaymentOrderFailed).Error
		_ = releaseCouponOrder(order)
		return nil, nil, err
	}
	notifyURL, err := config.Cfg.PaymentNotifyURL(app.AppID)
	if err != nil {
		return nil, nil, err
	}
	svc := jsapi.JsapiApiService{Client: client}
	resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{Appid: core.String(app.AppID), Mchid: core.String(app.PaymentMchID), Description: core.String(order.Description), OutTradeNo: core.String(order.OutTradeNo), Attach: core.String(order.Attach), NotifyUrl: core.String(notifyURL), Amount: &jsapi.Amount{Total: core.Int64(order.AmountFen), Currency: core.String("CNY")}, Payer: &jsapi.Payer{Openid: core.String(order.OpenID)}})
	if err != nil {
		_ = db.DB.Model(order).Update("status", models.PaymentOrderFailed).Error
		_ = releaseCouponOrder(order)
		return nil, nil, fmt.Errorf("微信支付预下单失败: %w", err)
	}
	if resp == nil || resp.PrepayId == nil {
		_ = db.DB.Model(order).Update("status", models.PaymentOrderFailed).Error
		_ = releaseCouponOrder(order)
		return nil, nil, errors.New("微信支付未返回 prepay_id")
	}
	order.PrepayID = *resp.PrepayId
	if err := db.DB.Model(order).Updates(map[string]interface{}{"prepay_id": order.PrepayID, "updated_at": time.Now()}).Error; err != nil {
		return nil, nil, err
	}
	return order, &JSAPIPaymentParams{AppID: valueOrEmpty(resp.Appid), TimeStamp: valueOrEmpty(resp.TimeStamp), NonceStr: valueOrEmpty(resp.NonceStr), Package: valueOrEmpty(resp.Package), SignType: valueOrEmpty(resp.SignType), PaySign: valueOrEmpty(resp.PaySign)}, nil
}

func (s *PaymentService) reusePaymentOrder(ctx context.Context, app *models.MiniApp, order *models.PaymentOrder) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
	if order.Status == models.PaymentOrderPaid {
		return nil, nil, errors.New("幂等键对应的订单已支付")
	}
	if order.Status == models.PaymentOrderFailed || order.Status == models.PaymentOrderClosed {
		return nil, nil, errors.New("幂等键对应的订单已结束，请使用新的幂等键")
	}
	if order.PrepayID == "" {
		return s.prepayOrder(ctx, app, order)
	}
	params, err := s.signPaymentParams(ctx, app, order.PrepayID)
	return order, params, err
}

// isDuplicateKeyError 判断是否为唯一键冲突错误。
// 依赖 GORM TranslateError：MySQL 1062 与 PostgreSQL 23505 均被翻译为 gorm.ErrDuplicatedKey，双数据库通用。
func isDuplicateKeyError(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

func (s *PaymentService) signPaymentParams(ctx context.Context, app *models.MiniApp, prepayID string) (*JSAPIPaymentParams, error) {
	if prepayID == "" {
		return nil, errors.New("订单缺少 prepay_id")
	}
	client, err := s.clientForApp(ctx, app)
	if err != nil {
		return nil, err
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce, err := utils.GenerateNonce()
	if err != nil {
		return nil, err
	}
	packageValue := "prepay_id=" + prepayID
	signature, err := client.Sign(ctx, fmt.Sprintf("%s\n%s\n%s\n%s\n", app.AppID, timestamp, nonce, packageValue))
	if err != nil {
		return nil, err
	}
	return &JSAPIPaymentParams{AppID: app.AppID, TimeStamp: timestamp, NonceStr: nonce, Package: packageValue, SignType: "RSA", PaySign: signature.Signature}, nil
}

// ApplyNotify 幂等处理微信支付成功通知。
func (s *PaymentService) ApplyNotify(ctx context.Context, app *models.MiniApp, request *http.Request) error {
	if app == nil || request == nil || app.PaymentAPIv3Key == "" || app.PaymentMchID == "" || app.PaymentMchSerialNo == "" || app.PaymentPrivateKey == "" {
		return errors.New("支付通知配置不完整")
	}
	// 确保平台证书下载器已按当前商户初始化，多实例各自维护本地证书缓存。
	if _, err := s.clientForApp(ctx, app); err != nil {
		return err
	}
	visitor := downloader.MgrInstance().GetCertificateVisitor(app.PaymentMchID)
	handler, err := notify.NewRSANotifyHandler(app.PaymentAPIv3Key, verifiers.NewSHA256WithRSAVerifier(visitor))
	if err != nil {
		return err
	}
	transaction := new(payments.Transaction)
	if _, err := handler.ParseNotifyRequest(ctx, request, transaction); err != nil {
		return err
	}
	if transaction.Appid == nil || *transaction.Appid != app.AppID || transaction.Mchid == nil || *transaction.Mchid != app.PaymentMchID {
		return errors.New("支付通知商户信息不匹配")
	}
	if transaction.OutTradeNo == nil || transaction.TradeState == nil || *transaction.TradeState != "SUCCESS" || transaction.TradeType == nil || *transaction.TradeType != "JSAPI" {
		return errors.New("支付通知状态无效")
	}
	var order models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND out_trade_no = ?", app.AppID, *transaction.OutTradeNo).First(&order).Error; err != nil {
		return errors.New("支付订单不存在")
	}
	if transaction.Amount == nil || transaction.Amount.Total == nil || *transaction.Amount.Total != order.AmountFen {
		return errors.New("支付通知金额不匹配")
	}
	if transaction.Amount.Currency != nil && *transaction.Amount.Currency != "CNY" {
		return errors.New("支付通知币种不匹配")
	}
	if transaction.Attach == nil || *transaction.Attach != order.Attach {
		return errors.New("支付通知订单绑定信息不匹配")
	}
	if transaction.Payer == nil || transaction.Payer.Openid == nil || *transaction.Payer.Openid != order.OpenID {
		return errors.New("支付通知用户不匹配")
	}
	if transaction.TransactionId == nil || strings.TrimSpace(*transaction.TransactionId) == "" {
		return errors.New("支付通知缺少微信交易单号")
	}
	var paidAt *time.Time
	if transaction.SuccessTime != nil {
		value, err := time.Parse(time.RFC3339, *transaction.SuccessTime)
		if err != nil {
			return errors.New("支付成功时间无效")
		}
		paidAt = &value
	}
	if order.Sandbox {
		return errors.New("正式支付通知不能修改沙箱订单")
	}
	_, err = commitPaymentTransition(&order, "success", *transaction.TransactionId, paidAt)
	return err
}

// GetOrderStatus 查询本地订单，并在未完成时向微信主动查单。
func (s *PaymentService) GetOrderStatus(ctx context.Context, appID string, userID int64, outTradeNo string) (*models.PaymentOrder, error) {
	if appID == "" || userID <= 0 || strings.TrimSpace(outTradeNo) == "" {
		return nil, errors.New("订单查询参数不完整")
	}
	var app models.MiniApp
	if err := db.DB.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil, errors.New("小程序配置不存在")
	}
	var order models.PaymentOrder
	if err := db.DB.Where("app_id = ? AND user_id = ? AND out_trade_no = ?", appID, userID, outTradeNo).First(&order).Error; err != nil {
		return nil, errors.New("支付订单不存在")
	}
	if order.Status == models.PaymentOrderPaid {
		if err := NewMembershipService().ApplyPaidOrder(&order); err != nil {
			return nil, err
		}
		return &order, nil
	}
	if order.Status == models.PaymentOrderClosed || order.Status == models.PaymentOrderFailed || order.Status == models.PaymentOrderRefunded {
		return &order, nil
	}
	if order.Sandbox {
		return &order, nil
	}
	client, err := s.clientForApp(ctx, &app)
	if err != nil {
		return &order, nil
	}
	resp, _, err := (&jsapi.JsapiApiService{Client: client}).QueryOrderByOutTradeNo(ctx, jsapi.QueryOrderByOutTradeNoRequest{OutTradeNo: core.String(order.OutTradeNo), Mchid: core.String(app.PaymentMchID)})
	if err != nil || resp == nil || resp.Appid == nil || *resp.Appid != app.AppID || resp.Mchid == nil || *resp.Mchid != app.PaymentMchID {
		return &order, nil
	}
	if resp.TradeType != nil && *resp.TradeType != "JSAPI" {
		return &order, nil
	}
	if resp.Amount != nil && resp.Amount.Total != nil && *resp.Amount.Total != order.AmountFen {
		return &order, nil
	}
	if resp.Attach == nil || *resp.Attach != order.Attach || resp.Payer == nil || resp.Payer.Openid == nil || *resp.Payer.Openid != order.OpenID {
		return &order, nil
	}
	transition := ""
	switch valueOrEmpty(resp.TradeState) {
	case "SUCCESS":
		transition = "success"
	case "CLOSED":
		transition = "cancel"
	case "PAYERROR":
		transition = "fail"
	default:
		return &order, nil
	}
	var paidAt *time.Time
	if resp.SuccessTime != nil {
		value, err := time.Parse(time.RFC3339, *resp.SuccessTime)
		if err != nil {
			return nil, err
		}
		paidAt = &value
	}
	return commitPaymentTransition(&order, transition, valueOrEmpty(resp.TransactionId), paidAt)
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
