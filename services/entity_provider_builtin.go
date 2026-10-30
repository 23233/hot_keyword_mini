// Package services entity_provider_builtin.go
package services

import (
	"hot_keyword/db"
	"hot_keyword/models"
)

func init() {
	RegisterPageEntityAssembler("drama", assembleDramaEntity)
	RegisterPageEntityAssembler("game", assembleGameEntity)
	RegisterPageEntityAssembler("query", assembleQueryEntity)
	RegisterPageEntityAssembler("download", assembleDownloadEntity)
}

// assembleDramaEntity 装配短剧实体，优先按 id/title 查询，未命中回退热门剧。
func assembleDramaEntity(appID, keyword string, query map[string]string, entity map[string]interface{}) {
	if db.DB == nil {
		return
	}
	var drama models.Drama
	queryID := ""
	if query != nil {
		queryID = query["id"]
	}
	var err error
	if queryID != "" {
		err = db.DB.Where("id = ?", queryID).First(&drama).Error
	} else if keyword != "" {
		err = db.DB.Where("title = ?", keyword).First(&drama).Error
	}
	if err != nil || drama.ID == 0 {
		if errFallback := db.DB.Order("hot_score desc, id asc").First(&drama).Error; errFallback != nil {
			_ = db.DB.Where("title = ?", "猴王下山").First(&drama).Error
		}
	}
	if drama.ID == 0 {
		return
	}
	entity["id"] = drama.ID
	entity["title"] = drama.Title
	entity["subtitle"] = drama.Subtitle
	entity["cover_url"] = drama.CoverUrl
	entity["banner_url"] = drama.BannerUrl
	entity["rating"] = drama.Rating
	entity["hot_score"] = drama.HotScore
	entity["total_episodes"] = drama.TotalEpisodes
	entity["updated_episodes"] = drama.UpdatedEpisodes
	entity["tags"] = drama.Tags
	entity["description"] = drama.Description
	entity["is_locked"] = false
}

// assembleGameEntity 装配游戏礼包实体，未领取前不下发兑换码。
func assembleGameEntity(appID, keyword string, query map[string]string, entity map[string]interface{}) {
	if db.DB == nil {
		return
	}
	var pkg models.GameRedeemPackage
	queryPkgID := ""
	if query != nil {
		queryPkgID = query["package_id"]
		if queryPkgID == "" {
			queryPkgID = query["game_id"]
		}
	}
	var err error
	if queryPkgID != "" {
		err = db.DB.Where("app_id = ? AND (package_id = ? OR game_id = ?)", appID, queryPkgID, queryPkgID).First(&pkg).Error
	}
	if err != nil || pkg.ID == 0 {
		_ = db.DB.Where("app_id = ?", appID).First(&pkg).Error
	}
	if pkg.ID == 0 {
		return
	}
	entity["package_id"] = pkg.PackageID
	entity["game_id"] = pkg.GameID
	entity["title"] = pkg.Title
	entity["description"] = pkg.Description
	entity["total_stock"] = pkg.TotalStock
	entity["remaining_stock"] = pkg.RemainingStock
	entity["claim_status"] = "unclaimed"
}

// assembleQueryEntity 装配通用查询参数与就绪状态。
func assembleQueryEntity(appID, keyword string, query map[string]string, entity map[string]interface{}) {
	if query != nil && query["query_value"] != "" {
		entity["query_value"] = query["query_value"]
	}
	entity["status"] = "ready"
}

// assembleDownloadEntity 装配下载平台信息。
func assembleDownloadEntity(appID, keyword string, query map[string]string, entity map[string]interface{}) {
	entity["platform"] = "Android / iOS"
	entity["status"] = "verified"
}
