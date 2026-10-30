// Package services reviews.go
package services

import (
	"hot_keyword/db"
	"hot_keyword/models"
	"math"
	"strconv"
	"time"
)

// reviewListMaxLimit 单次评价列表最大返回条数。
const reviewListMaxLimit = 50

// reviewRatingDistribution 是评价星级分布的聚合结果。
type reviewRatingDistribution struct {
	Rating int
	Count  int64
}

// ListProductReviews 返回指定商品已发布评价与统计，供公开页面和动作端点使用。
// 均分与分布直接由 SQL 聚合，避免评价量增长后把全部记录读入内存。
func ListProductReviews(appID string, productID int64, limit, offset int) (map[string]interface{}, error) {
	if db.DB == nil {
		return nil, domainError("UNAVAILABLE", "数据库未初始化")
	}
	if appID == "" || productID <= 0 {
		return nil, domainError("INVALID_ARGUMENT", "商品参数无效")
	}
	if limit <= 0 || limit > reviewListMaxLimit {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	base := db.DB.Model(&models.DomainRecord{}).
		Where("app_id = ? AND kind = ? AND status = ? AND peer_id = ?", appID, "review", "published", productID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var average float64
	if err := db.DB.Model(&models.DomainRecord{}).
		Where("app_id = ? AND kind = ? AND status = ? AND peer_id = ?", appID, "review", "published", productID).
		Select("COALESCE(AVG(rating), 0)").Scan(&average).Error; err != nil {
		return nil, err
	}
	var distributionRows []reviewRatingDistribution
	if err := db.DB.Model(&models.DomainRecord{}).
		Where("app_id = ? AND kind = ? AND status = ? AND peer_id = ?", appID, "review", "published", productID).
		Select("rating, COUNT(*) AS count").Group("rating").Scan(&distributionRows).Error; err != nil {
		return nil, err
	}
	distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	for _, row := range distributionRows {
		if row.Rating >= 1 && row.Rating <= 5 {
			distribution[strconv.Itoa(row.Rating)] = int(row.Count)
		}
	}

	var records []models.DomainRecord
	if err := db.DB.
		Where("app_id = ? AND kind = ? AND status = ? AND peer_id = ?", appID, "review", "published", productID).
		Order("created_at desc, id desc").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(records))
	for index := range records {
		data := recordData(&records[index])
		author, _ := data["nickname"].(string)
		content, _ := data["content"].(string)
		items = append(items, map[string]interface{}{
			"id":         records[index].ID,
			"author":     author,
			"rating":     records[index].Rating,
			"content":    content,
			"time":       records[index].CreatedAt.UTC().Format(time.RFC3339),
			"avatar_url": "",
		})
	}

	average = math.Round(average*10) / 10
	return map[string]interface{}{
		"items":        items,
		"total":        total,
		"average":      average,
		"distribution": distribution,
		"has_more":     int64(offset+len(items)) < total,
	}, nil
}
