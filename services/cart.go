// Package services cart.go
package services

import (
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
)

const (
	// cartQuoteMaxItems 单次报价允许的最大 SKU 数。
	cartQuoteMaxItems = 20
	// cartQuoteMaxQuantity 单个 SKU 允许的最大数量。
	cartQuoteMaxQuantity = 99
	// productMaxPriceFen 商品单价上限（100 万元），防止异常价格导致金额溢出。
	productMaxPriceFen = 100000000
)

// QuoteCart 按服务端商品配置重算购物车价格与合计，忽略客户端传入的任何金额。
func QuoteCart(appID string, items []map[string]interface{}) (map[string]interface{}, error) {
	if db.DB == nil {
		return nil, domainError("UNAVAILABLE", "数据库未初始化")
	}
	if appID == "" {
		return nil, domainError("INVALID_ARGUMENT", "租户不能为空")
	}
	if len(items) == 0 {
		return nil, domainError("INVALID_ARGUMENT", "购物车不能为空")
	}
	if len(items) > cartQuoteMaxItems {
		return nil, domainError("INVALID_ARGUMENT", fmt.Sprintf("购物车最多支持 %d 个 SKU", cartQuoteMaxItems))
	}

	seen := make(map[string]bool, len(items))
	quoted := make([]map[string]interface{}, 0, len(items))
	var totalFen int64
	var count int64
	for _, item := range items {
		sku := strings.TrimSpace(domainText(item, "sku"))
		if sku == "" {
			return nil, domainError("INVALID_ARGUMENT", "购物车条目缺少 sku")
		}
		if seen[sku] {
			return nil, domainError("INVALID_ARGUMENT", "购物车存在重复 SKU")
		}
		seen[sku] = true
		quantity, err := domainNumber(item, "quantity")
		if err != nil || quantity < 1 || quantity > cartQuoteMaxQuantity {
			return nil, domainError("INVALID_ARGUMENT", fmt.Sprintf("数量必须为 1-%d", cartQuoteMaxQuantity))
		}

		var product models.Product
		if err := db.DB.Where("app_id = ? AND sku = ? AND status = ?", appID, sku, models.ProductStatusActive).First(&product).Error; err != nil {
			return nil, domainError("NOT_FOUND", "商品不存在或已下架: "+sku)
		}
		if product.PriceFen <= 0 || product.PriceFen > productMaxPriceFen {
			return nil, domainError("INVALID_ARGUMENT", "商品价格无效: "+sku)
		}
		lineFen := product.PriceFen * quantity
		if totalFen > 9000000000000000-lineFen {
			return nil, domainError("INVALID_ARGUMENT", "购物车金额超出上限")
		}
		totalFen += lineFen
		count += quantity
		quoted = append(quoted, map[string]interface{}{
			"sku":        product.SKU,
			"title":      product.Name,
			"price_fen":  product.PriceFen,
			"price_text": fmt.Sprintf("¥%.2f", float64(product.PriceFen)/100),
			"quantity":   quantity,
			"line_fen":   lineFen,
		})
	}

	return map[string]interface{}{
		"items":      quoted,
		"count":      count,
		"total_fen":  totalFen,
		"total_text": fmt.Sprintf("¥%.2f", float64(totalFen)/100),
	}, nil
}
