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

	// 注册官方内置受控端点: 游戏礼包兑换码领取与发放（首发允许匿名访问）
	s.RegisterEndpoint(ActionEndpointMeta{
		Name:        "game.redeem",
		Description: "游戏独家礼包兑换码匿名领取与防超发事务",
		RequireAuth: false,
		Handler:     s.handleGameRedeem,
	})

	// 注册官方内置受控端点: 考分与通用数据查询 (支持免登录或受控查询)
	s.RegisterEndpoint(ActionEndpointMeta{
		Name:        "query.score",
		Description: "官方成绩与通用数据受控查询端点",
		RequireAuth: false,
		Handler:     s.handleQueryScore,
	})

	// 通用领域沙箱端点：用于本地协议、Block、MCP 和微信运行时验收。
	// 这些端点只返回受控状态数据，不连接生产支付、资金、即时通讯或物流服务。
	for _, meta := range []ActionEndpointMeta{
		{Name: "chat.send", Description: "发送聊天消息沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "chat.poll", Description: "轮询聊天消息沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "chat.mark_read", Description: "标记聊天已读沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "chat.connect", Description: "建立聊天连接沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "chat.upload_media", Description: "上传聊天媒体沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "media.delete", Description: "删除当前用户媒体沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "order.create", Description: "创建订单沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "order.confirm", Description: "确认订单沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "order.cancel", Description: "取消订单沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "order.confirm_receipt", Description: "确认收货沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "logistics.refresh", Description: "刷新物流沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "after_sale.apply", Description: "申请售后沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "after_sale.upload_evidence", Description: "上传售后证据沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "service.accept_task", Description: "斗师接受任务沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "service.reject_task", Description: "斗师拒绝任务沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "service.submit_quote", Description: "斗师提交报价沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "service.update_status", Description: "更新服务状态沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "membership.open", Description: "开通会员沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "ads.load", Description: "加载受控广告位沙箱动作", RequireAuth: false, Handler: s.handlePlatformSandbox},
		{Name: "wallet.refresh", Description: "刷新钱包沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
		{Name: "wallet.withdraw", Description: "申请提现沙箱动作", RequireAuth: true, Handler: s.handlePlatformSandbox},
	} {
		s.RegisterEndpoint(meta)
	}
	for _, name := range []string{"domain.list", "service.apply", "service.configure", "service.request", "chat.recall", "media.prepare"} {
		s.RegisterEndpoint(ActionEndpointMeta{Name: name, Description: "通用领域持久化操作", RequireAuth: true, Handler: func(appID, openID string, p map[string]interface{}, key string) (interface{}, error) {
			copy := map[string]interface{}{}
			for k, v := range p {
				copy[k] = v
			}
			copy["_endpoint"] = name
			return s.handlePlatformSandbox(appID, openID, copy, key)
		}})
	}

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

	// 强制要求登录态时校验 openID，拒绝未授权访问
	if meta.RequireAuth {
		if openID == "" || strings.HasPrefix(openID, "guest_") {
			return nil, errors.New("此业务动作必须在微信授权登录后方可执行")
		}
	} else if openID == "" {
		// 仅对明确允许匿名的端点分配只读访客标识
		openID = "guest_" + ut.RandomStr(8)
	}

	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("idem_%s_%d", ut.RandomStr(12), time.Now().UnixNano())
	}

	// 将端点名以内部字段传给通用沙箱处理器，复制 Map 避免污染调用方状态。
	if strings.HasPrefix(endpoint, "chat.") || strings.HasPrefix(endpoint, "media.") || strings.HasPrefix(endpoint, "order.") || strings.HasPrefix(endpoint, "logistics.") || strings.HasPrefix(endpoint, "after_sale.") || strings.HasPrefix(endpoint, "service.") || strings.HasPrefix(endpoint, "membership.") || strings.HasPrefix(endpoint, "ads.") || strings.HasPrefix(endpoint, "wallet.") {
		cloned := make(map[string]interface{}, len(payload)+1)
		for key, value := range payload {
			cloned[key] = value
		}
		cloned["_endpoint"] = endpoint
		payload = cloned
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

	// 2. 防重检查: 同一访客标识对同一礼包限领一次；匿名访客由上层生成短期标识。
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

// handleQueryScore 官方考分与信息查询受控端点处理实现 (支持免登录或受控查询)
func (s *ActionEndpointService) handleQueryScore(appID, openID string, payload map[string]interface{}, idempotencyKey string) (interface{}, error) {
	queryVal := ""
	if payload != nil {
		if v, ok := payload["query_value"].(string); ok && v != "" {
			queryVal = strings.TrimSpace(v)
		} else if v, ok := payload["code"].(string); ok && v != "" {
			queryVal = strings.TrimSpace(v)
		}
	}
	if queryVal == "" {
		return nil, errors.New("查询关键词或准考证号不能为空")
	}

	// 模拟返回结构化考分/信息查询结果
	return map[string]interface{}{
		"query_value": queryVal,
		"status":      "success",
		"title":       "官方成绩查询结果",
		"subject":     "全国统一认证测试",
		"score":       628,
		"rank":        "前 5%",
		"passed":      true,
		"remark":      "成绩合格，恭喜通过！",
		"query_time":  time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// handlePlatformSandbox 兼容既有入口；数据库可用时执行真实持久化领域逻辑。
func (s *ActionEndpointService) handlePlatformSandbox(appID, openID string, payload map[string]interface{}, idempotencyKey string) (interface{}, error) {
	endpoint := domainText(payload, "_endpoint")
	if db.Mysql == nil {
		status, err := nextPlatformStatus(endpoint, "", payload)
		if err != nil {
			return nil, err
		}
		return platformSandboxResult(appID, openID, endpoint, domainText(payload, "id"), idempotencyKey, payload, status), nil
	}
	var user models.User
	if err := db.Mysql.Where("app_id = ? AND wechat_openid = ?", appID, openID).First(&user).Error; err != nil {
		return nil, domainError("UNAUTHORIZED", "用户尚未登录")
	}
	clean := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		if k != "_endpoint" {
			clean[k] = v
		}
	}
	return ExecuteDomainOperation(appID, user.ID, openID, endpoint, clean, idempotencyKey, false)
}

func endpointFamily(endpoint string) string {
	if index := strings.IndexByte(endpoint, '.'); index > 0 {
		return endpoint[:index]
	}
	return endpoint
}

func nextPlatformStatus(endpoint, current string, payload map[string]interface{}) (string, error) {
	if strings.HasPrefix(endpoint, "chat.") {
		switch endpoint {
		case "chat.send":
			return "sent", nil
		case "chat.poll":
			return "connected", nil
		case "chat.mark_read":
			return "read", nil
		case "chat.connect":
			return "connected", nil
		default:
			return "uploaded", nil
		}
	}
	if strings.HasPrefix(endpoint, "order.") {
		next := map[string]string{"order.create": "created", "order.confirm": "confirmed", "order.cancel": "cancelled", "order.confirm_receipt": "completed"}[endpoint]
		if next == "" {
			return "accepted", nil
		}
		valid := (endpoint == "order.create" && current == "") || (endpoint == "order.confirm" && current == "created") || (endpoint == "order.cancel" && (current == "created" || current == "confirmed")) || (endpoint == "order.confirm_receipt" && current == "confirmed")
		if !valid && current != next {
			return "", fmt.Errorf("订单状态 %s 不能执行 %s", current, endpoint)
		}
		return next, nil
	}
	switch {
	case endpoint == "logistics.refresh":
		return "ready", nil
	case endpoint == "media.delete":
		return "deleted", nil
	case endpoint == "after_sale.apply":
		if current != "" && current != "completed" {
			return "", errors.New("当前订单状态不允许申请售后")
		}
		return "requested", nil
	case endpoint == "after_sale.upload_evidence":
		if current != "requested" && current != "evidence_required" && current != "evidence_uploaded" {
			return "", errors.New("售后单当前状态不允许上传证据")
		}
		return "evidence_uploaded", nil
	case endpoint == "service.accept_task":
		if current != "" && current != "pending" {
			return "", errors.New("斗师任务当前状态不可接单")
		}
		return "accepted", nil
	case endpoint == "service.reject_task":
		if current != "" && current != "pending" && current != "offered" {
			return "", errors.New("斗师任务当前状态不可拒绝")
		}
		return "rejected", nil
	case endpoint == "service.submit_quote":
		if current != "accepted" && current != "quoted" {
			return "", errors.New("斗师任务必须接单后才能报价")
		}
		if _, ok := payload["price"]; !ok {
			if _, ok = payload["amount"]; !ok {
				return "", errors.New("报价金额不能为空")
			}
		}
		return "quoted", nil
	case endpoint == "service.update_status":
		value := strings.TrimSpace(fmt.Sprint(payload["status"]))
		allowed := map[string]map[string]bool{
			"quoted": {"paid": true}, "paid": {"in_service": true},
			"in_service": {"awaiting_confirmation": true}, "awaiting_confirmation": {"completed": true, "disputed": true},
		}
		if value != "" && allowed[current][value] {
			return value, nil
		}
		return "", fmt.Errorf("斗师任务状态 %s 不能更新为 %s", current, value)
	case endpoint == "membership.open":
		return "pending", nil
	case endpoint == "ads.load":
		return "ready", nil
	case endpoint == "wallet.refresh":
		return "ready", nil
	case endpoint == "wallet.withdraw":
		raw, exists := payload["amount"]
		amount, ok := raw.(float64)
		if !exists || !ok || amount <= 0 {
			return "", errors.New("提现金额必须大于 0")
		}
		return "pending", nil
	default:
		return "accepted", nil
	}
}

func platformSandboxResult(appID, openID, endpoint, entityID, idempotencyKey string, payload map[string]interface{}, status string) map[string]interface{} {
	if status == "" {
		status = "accepted"
	}
	result := map[string]interface{}{"sandbox": true, "app_id": appID, "open_id": openID, "id": entityID, "status": status, "endpoint": endpoint, "idempotency": idempotencyKey, "message": "本地通用能力沙箱已执行", "updated_at": time.Now().Format(time.RFC3339)}
	switch endpoint {
	case "chat.send":
		result["message_id"] = "msg-" + strings.ToLower(ut.RandomStr(8))
	case "logistics.refresh":
		result["tracks"] = []map[string]interface{}{{"status": "已创建物流单", "time": time.Now().Format(time.RFC3339)}}
	case "wallet.refresh":
		result["balance_fen"] = int64(0)
	}
	if len(payload) > 0 {
		result["payload"] = payload
	}
	return result
}
