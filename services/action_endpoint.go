// Package services action_endpoint.go
package services

import (
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"strings"
	"sync"
	"time"

	"github.com/23233/ggg/ut"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ActionEndpointHandler 受控端点处理函数原型
type ActionEndpointHandler func(appID, openID string, payload map[string]interface{}, idempotencyKey string) (interface{}, error)

// ActionEndpointMeta 端点元信息
type ActionEndpointMeta struct {
	// 端点唯一名称 (如 game.redeem)
	Name string
	// 端点描述
	Description string
	// 是否强制要求用户登录态
	RequireAuth bool
	// 执行处理器
	Handler ActionEndpointHandler
}

// ActionEndpointService 受控业务端点调度服务
type ActionEndpointService struct {
	mu        sync.RWMutex
	endpoints map[string]ActionEndpointMeta
}

// NewActionEndpointService 创建受控业务端点调度服务并初始化注册表
func NewActionEndpointService() *ActionEndpointService {
	s := &ActionEndpointService{
		endpoints: make(map[string]ActionEndpointMeta),
	}

	// 注册官方内置受控端点: 游戏礼包兑换码核销与发放
	s.RegisterEndpoint(ActionEndpointMeta{
		Name:        "game.redeem",
		Description: "游戏独家礼包兑换码防超发事务领取",
		RequireAuth: false, // 允许匿名试玩，但在真实业务可开启
		Handler:     s.handleGameRedeem,
	})

	return s
}

// RegisterEndpoint 向注册表登记新的受控端点 (防止开放代理攻击)
func (s *ActionEndpointService) RegisterEndpoint(meta ActionEndpointMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[meta.Name] = meta
}

// ExecuteActionEndpoint 执行受控业务端点
func (s *ActionEndpointService) ExecuteActionEndpoint(appID, openID, endpoint string, payload map[string]interface{}, idempotencyKey string) (interface{}, error) {
	if appID == "" {
		return nil, errors.New("app_id 不能为空")
	}
	if endpoint == "" {
		return nil, errors.New("endpoint 不能为空")
	}

	s.mu.RLock()
	meta, exists := s.endpoints[endpoint]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("非法未登记的受控端点: %s (禁止任意URL代理)", endpoint)
	}

	// 强制要求登录态时校验 openID
	if meta.RequireAuth && openID == "" {
		return nil, errors.New("此业务动作必须登录后方可执行")
	}

	// 若未传入 openID，生成临时访客标识
	if openID == "" {
		openID = "guest_" + ut.RandomStr(8)
	}

	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("idem_%s_%d", ut.RandomStr(12), time.Now().UnixNano())
	}

	return meta.Handler(appID, openID, payload, idempotencyKey)
}

// handleGameRedeem 游戏礼包领取事务实现 (行级悲观锁、库存扣减、幂等防刷)
func (s *ActionEndpointService) handleGameRedeem(appID, openID string, payload map[string]interface{}, idempotencyKey string) (interface{}, error) {
	packageID := "pkg_game_novice_888"
	if payload != nil {
		if pid, ok := payload["package_id"].(string); ok && pid != "" {
			packageID = pid
		}
	}

	// 兜底模式: 当数据库未初始化时 (如单元测试环境)
	if db.Mysql == nil {
		fakeCode := "VIP888-MOCK" + strings.ToUpper(ut.RandomStr(4))
		return map[string]interface{}{
			"package_id": packageID,
			"title":      "绝地突围公测独家礼包",
			"code":       fakeCode,
			"is_mock":    true,
		}, nil
	}

	// 1. 幂等检查: 若相同幂等键曾请求过，直接返回先前生成的兑换码
	var idemRecord models.GameRedeemRecord
	if err := db.Mysql.Where("app_id = ? AND idempotency_key = ?", appID, idempotencyKey).First(&idemRecord).Error; err == nil {
		return map[string]interface{}{
			"package_id": idemRecord.PackageID,
			"code":       idemRecord.RedeemCode,
			"idempotent": true,
		}, nil
	}

	// 2. 防重检查: 同一用户对同一礼包限领一次
	var userRecord models.GameRedeemRecord
	if err := db.Mysql.Where("app_id = ? AND package_id = ? AND open_id = ?", appID, packageID, openID).First(&userRecord).Error; err == nil {
		// 已领过，返回之前领取的兑换码，不额外扣减库存
		return map[string]interface{}{
			"package_id": userRecord.PackageID,
			"code":       userRecord.RedeemCode,
			"already":    true,
			"msg":        "您已领取过该礼包",
		}, nil
	}

	// 3. 开启数据库事务，并采用行级排他锁 (SELECT ... FOR UPDATE) 防超发
	tx := db.Mysql.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var pkg models.GameRedeemPackage
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("app_id = ? AND package_id = ?", appID, packageID).
		First(&pkg).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("礼包套餐 %s 不存在", packageID)
		}
		return nil, fmt.Errorf("查询礼包库存失败: %w", err)
	}

	if pkg.RemainingStock <= 0 {
		tx.Rollback()
		return nil, errors.New("手慢了，该独家礼包已被全部领完")
	}

	// 4. 派生唯一兑换码并扣减库存
	prefix := pkg.CodePrefix
	if prefix == "" {
		prefix = "VIP888-"
	}
	realCode := fmt.Sprintf("%s%s", prefix, strings.ToUpper(ut.RandomStr(6)))

	pkg.RemainingStock -= 1
	if err := tx.Save(&pkg).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("更新库存失败: %w", err)
	}

	// 5. 写入用户领取记录
	newRecord := models.GameRedeemRecord{
		AppID:          appID,
		PackageID:      packageID,
		OpenID:         openID,
		RedeemCode:     realCode,
		IdempotencyKey: idempotencyKey,
		ClaimedAt:      time.Now(),
	}
	if err := tx.Create(&newRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("写入领取记录失败: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return map[string]interface{}{
		"package_id": packageID,
		"title":      pkg.Title,
		"code":       realCode,
		"remaining":  pkg.RemainingStock,
	}, nil
}
