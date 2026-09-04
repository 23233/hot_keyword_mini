// Package services payment.go
package services

import (
	"context"
	"errors"
	"fmt"
	"hot_keyword/config"
	"hot_keyword/db"
	"hot_keyword/models"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/23233/ggg/ut"
	mysqlDriver "github.com/go-sql-driver/mysql"
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
func (s *PaymentService) CreateJSAPIOrder(ctx context.Context, appID string, userID int64, openID, productSKU, idempotencyKey string) (*models.PaymentOrder, *JSAPIPaymentParams, error) {
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
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil, nil, fmt.Errorf("小程序配置不存在: %w", err)
	}
	var product models.Product
	if err := db.Mysql.Where("app_id = ? AND sku = ? AND status = ?", appID, productSKU, models.ProductStatusActive).First(&product).Error; err != nil {
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
	if _, err := config.Cfg.PaymentNotifyURL(app.AppID); err != nil {
		return nil, nil, err
	}
	var existing models.PaymentOrder
	if err := db.Mysql.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; err == nil {
		return s.reusePaymentOrder(ctx, &app, &existing)
	}
	outTradeNo := fmt.Sprintf("%s%d%s", time.Now().Format("20060102150405"), userID%100000, strings.ToUpper(ut.RandomStr(8)))
	order := &models.PaymentOrder{AppID: appID, UserID: userID, ProductID: product.ID, OutTradeNo: outTradeNo, OpenID: openID, Description: product.Name, AmountFen: product.PriceFen, Status: models.PaymentOrderPending, Attach: attach, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Mysql.Create(order).Error; err != nil {
		if isDuplicateKeyError(err) {
			if findErr := db.Mysql.Where("app_id = ? AND user_id = ? AND attach = ?", appID, userID, attach).First(&existing).Error; findErr == nil {
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
		_ = db.Mysql.Model(order).Update("status", models.PaymentOrderFailed).Error
		return nil, nil, err
	}
	notifyURL, err := config.Cfg.PaymentNotifyURL(app.AppID)
	if err != nil {
		return nil, nil, err
	}
	svc := jsapi.JsapiApiService{Client: client}
	resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{Appid: core.String(app.AppID), Mchid: core.String(app.PaymentMchID), Description: core.String(order.Description), OutTradeNo: core.String(order.OutTradeNo), Attach: core.String(order.Attach), NotifyUrl: core.String(notifyURL), Amount: &jsapi.Amount{Total: core.Int64(order.AmountFen), Currency: core.String("CNY")}, Payer: &jsapi.Payer{Openid: core.String(order.OpenID)}})
	if err != nil {
		_ = db.Mysql.Model(order).Update("status", models.PaymentOrderFailed).Error
		return nil, nil, fmt.Errorf("微信支付预下单失败: %w", err)
	}
	if resp == nil || resp.PrepayId == nil {
		_ = db.Mysql.Model(order).Update("status", models.PaymentOrderFailed).Error
		return nil, nil, errors.New("微信支付未返回 prepay_id")
	}
	order.PrepayID = *resp.PrepayId
	if err := db.Mysql.Model(order).Updates(map[string]interface{}{"prepay_id": order.PrepayID, "updated_at": time.Now()}).Error; err != nil {
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

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
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
	if err := db.Mysql.Where("app_id = ? AND out_trade_no = ?", app.AppID, *transaction.OutTradeNo).First(&order).Error; err != nil {
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
	if order.Status == models.PaymentOrderPaid {
		return nil
	}
	updates := map[string]interface{}{"status": models.PaymentOrderPaid, "updated_at": time.Now()}
	if transaction.TransactionId != nil {
		updates["transaction_id"] = *transaction.TransactionId
	}
	if transaction.SuccessTime != nil {
		if paidAt, parseErr := time.Parse(time.RFC3339, *transaction.SuccessTime); parseErr == nil {
			updates["paid_at"] = paidAt
		}
	}
	result := db.Mysql.Model(&models.PaymentOrder{}).Where("app_id = ? AND out_trade_no = ? AND status <> ?", app.AppID, *transaction.OutTradeNo, models.PaymentOrderPaid).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("支付订单状态更新失败")
	}
	return nil
}

// GetOrderStatus 查询本地订单，并在未完成时向微信主动查单。
func (s *PaymentService) GetOrderStatus(ctx context.Context, appID string, userID int64, outTradeNo string) (*models.PaymentOrder, error) {
	if appID == "" || userID <= 0 || strings.TrimSpace(outTradeNo) == "" {
		return nil, errors.New("订单查询参数不完整")
	}
	var app models.MiniApp
	if err := db.Mysql.Where("app_id = ?", appID).First(&app).Error; err != nil {
		return nil, errors.New("小程序配置不存在")
	}
	var order models.PaymentOrder
	if err := db.Mysql.Where("app_id = ? AND user_id = ? AND out_trade_no = ?", appID, userID, outTradeNo).First(&order).Error; err != nil {
		return nil, errors.New("支付订单不存在")
	}
	if order.Status == models.PaymentOrderPaid || order.Status == models.PaymentOrderClosed || order.Status == models.PaymentOrderFailed {
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
	updates := map[string]interface{}{"updated_at": time.Now()}
	switch valueOrEmpty(resp.TradeState) {
	case "SUCCESS":
		updates["status"] = models.PaymentOrderPaid
		if resp.TransactionId != nil {
			updates["transaction_id"] = *resp.TransactionId
		}
		if resp.SuccessTime != nil {
			if paidAt, parseErr := time.Parse(time.RFC3339, *resp.SuccessTime); parseErr == nil {
				updates["paid_at"] = paidAt
			}
		}
	case "CLOSED":
		updates["status"] = models.PaymentOrderClosed
	case "PAYERROR":
		updates["status"] = models.PaymentOrderFailed
	default:
		return &order, nil
	}
	if err := db.Mysql.Model(&order).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = db.Mysql.First(&order, order.ID).Error
	return &order, nil
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
