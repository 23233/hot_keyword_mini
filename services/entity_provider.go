// Package services entity_provider.go
package services

import "hot_keyword/models"

// PageEntityAssembler 在基础实体上追加某一业务领域的受控数据。
type PageEntityAssembler func(appID, keyword string, query map[string]string, entity map[string]interface{})

// PageEnvelopeEnricher 在页面信封装配完成后追加访问者相关数据。
type PageEnvelopeEnricher func(envelope *models.PageResponseEnvelope, appID string, authenticated bool, userID int64) error

var (
	pageEntityAssemblers  = map[string]PageEntityAssembler{}
	pageEnvelopeEnrichers = map[string]PageEnvelopeEnricher{}
	pageShareModeDefaults = map[string]string{}
)

// RegisterPageEntityAssembler 注册业务领域实体装配器；内核只按 business_type 调度。
func RegisterPageEntityAssembler(businessType string, assembler PageEntityAssembler) {
	if businessType == "" || assembler == nil {
		return
	}
	pageEntityAssemblers[businessType] = assembler
}

// RegisterPageEnvelopeEnricher 注册业务领域信封装配器（如会员权益、文章摘要）。
func RegisterPageEnvelopeEnricher(businessType string, enricher PageEnvelopeEnricher) {
	if businessType == "" || enricher == nil {
		return
	}
	pageEnvelopeEnrichers[businessType] = enricher
}

// AssemblePageEntity 按业务类型装配基础实体，未知类型只返回标题等通用字段。
func AssemblePageEntity(appID, businessType, keyword string, query map[string]string) map[string]interface{} {
	entity := make(map[string]interface{})
	if keyword != "" {
		entity["title"] = keyword
	}
	if assembler, ok := pageEntityAssemblers[businessType]; ok {
		assembler(appID, keyword, query, entity)
	}
	return entity
}

// applyPageEnvelopeEnrichers 调用业务领域信封装配器；内核不感知具体业务实现。
func applyPageEnvelopeEnrichers(envelope *models.PageResponseEnvelope, appID string, authenticated bool, userID int64) {
	if envelope == nil {
		return
	}
	if enricher, ok := pageEnvelopeEnrichers[envelope.Page.BusinessType]; ok {
		_ = enricher(envelope, appID, authenticated, userID)
	}
}

// RegisterDefaultShareMode 注册业务类型的默认分享访问模式，由业务 Provider 自行声明。
func RegisterDefaultShareMode(businessType, mode string) {
	if businessType == "" || mode == "" {
		return
	}
	pageShareModeDefaults[businessType] = mode
}

// DefaultShareMode 返回业务类型声明的默认分享访问模式。
func DefaultShareMode(businessType string) string {
	return pageShareModeDefaults[businessType]
}
