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
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PaymentOrderStatus 支付订单状态常量。
const (
	PaymentOrderPending = "pending"
	PaymentOrderPaid    = "paid"
	PaymentOrderClosed  = "closed"
	PaymentOrderFailed  = "failed"
)

// TableName 返回支付订单表名。
func (PaymentOrder) TableName() string { return "payment_orders" }
