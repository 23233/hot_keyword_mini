// Package models payment.go
package models

import "time"

// Product 后台可售商品。
type Product struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID       string    `gorm:"column:app_id;size:64;index;not null;uniqueIndex:idx_product_app_sku" json:"app_id"`
	SKU         string    `gorm:"column:sku;size:64;not null;uniqueIndex:idx_product_app_sku" json:"sku"`
	Name        string    `gorm:"column:name;size:128;not null" json:"name"`
	Description string    `gorm:"column:description;size:255" json:"description"`
	PriceFen    int64     `gorm:"column:price_fen;not null" json:"price_fen"`
	Status      string    `gorm:"column:status;size:16;default:'active';index" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProductStatus 商品状态常量。
const (
	ProductStatusActive   = "active"
	ProductStatusInactive = "inactive"
)

// TableName 返回商品表名。
func (Product) TableName() string { return "products" }

// PaymentOrder 微信支付订单。
type PaymentOrder struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppID         string     `gorm:"column:app_id;size:64;index;not null;uniqueIndex:idx_payment_idem" json:"app_id"`
	UserID        int64      `gorm:"column:user_id;index;not null;uniqueIndex:idx_payment_idem" json:"user_id"`
	ProductID     int64      `gorm:"column:product_id;index;not null" json:"product_id"`
	OutTradeNo    string     `gorm:"column:out_trade_no;size:64;uniqueIndex;not null" json:"out_trade_no"`
	TransactionID string     `gorm:"column:transaction_id;size:64;index" json:"transaction_id,omitempty"`
	OpenID        string     `gorm:"column:openid;size:128;not null" json:"-"`
	Description   string     `gorm:"column:description;size:127;not null" json:"description"`
	AmountFen     int64      `gorm:"column:amount_fen;not null" json:"amount_fen"`
	Status        string     `gorm:"column:status;size:16;index;not null" json:"status"`
	PrepayID      string     `gorm:"column:prepay_id;size:128" json:"-"`
	Attach        string     `gorm:"column:attach;size:128;uniqueIndex:idx_payment_idem" json:"attach,omitempty"`
	PaidAt        *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
	// 支付订单类型：product 单商品、cart 多 SKU 购物车。
	Kind string `gorm:"column:kind;size:16;not null;default:'product';index" json:"kind"`
	// 购物车订单的商品明细快照（JSON），单商品订单为空。
	Items string `gorm:"column:items" json:"items,omitempty"`
	// 使用的优惠券记录 ID，未使用为空。
	CouponID string `gorm:"column:coupon_id;size:128;index" json:"coupon_id,omitempty"`
	// 优惠券抵扣金额（分）。
	DiscountFen int64 `gorm:"column:discount_fen;not null;default:0" json:"discount_fen"`
	// 是否由本地支付沙箱创建。
	Sandbox bool `gorm:"column:sandbox;not null;default:false;index" json:"sandbox"`
	// 退款完成时间。
	RefundedAt *time.Time `gorm:"column:refunded_at" json:"refunded_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// PaymentOrderKind 支付订单类型常量。
const (
	// PaymentOrderKindProduct 单商品支付订单。
	PaymentOrderKindProduct = "product"
	// PaymentOrderKindCart 多 SKU 购物车支付订单。
	PaymentOrderKindCart = "cart"
)

// PaymentOrderStatus 支付订单状态常量。
const (
	PaymentOrderPending  = "pending"
	PaymentOrderPaid     = "paid"
	PaymentOrderClosed   = "closed"
	PaymentOrderFailed   = "failed"
	PaymentOrderRefunded = "refunded"
)

// TableName 返回支付订单表名。
func (PaymentOrder) TableName() string { return "payment_orders" }
