// Package services template_service.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TemplateService 统一管理内置行业模板和同租户的用户自定义模板。
type TemplateService struct {
	registry *TemplateRegistry
}

// NewTemplateService 创建模板服务实例。
func NewTemplateService() *TemplateService {
	return &TemplateService{registry: GetGlobalTemplateRegistry()}
}

// ListTemplates 返回内置模板与指定小程序可复用的用户模板。
func (s *TemplateService) ListTemplates(appID, businessType string) ([]*SDUITemplate, error) {
	templates := s.registry.ListTemplates(businessType)
	if strings.TrimSpace(appID) == "" || db.Mysql == nil {
		return templates, nil
	}

	var records []models.DynamicPageTemplate
	query := db.Mysql.Where("app_id = ?", appID)
	if strings.TrimSpace(businessType) != "" {
		query = query.Where("business_type = ?", businessType)
	}
	if err := query.Order("template_id asc").Find(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		template, err := templateFromRecord(&record)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	sort.SliceStable(templates, func(i, j int) bool {
		if templates[i].Builtin != templates[j].Builtin {
			return templates[i].Builtin
		}
		return templates[i].TemplateID < templates[j].TemplateID
	})
	return templates, nil
}

// GetTemplate 读取内置模板或同租户用户模板。
func (s *TemplateService) GetTemplate(appID, templateID string) (*SDUITemplate, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return nil, errors.New("template_id 不能为空")
	}
	if template, err := s.registry.GetTemplate(templateID); err == nil {
		return template, nil
	}
	if strings.TrimSpace(appID) == "" {
		return nil, fmt.Errorf("未找到行业模板: %s", templateID)
	}
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化，无法读取用户模板")
	}

	var record models.DynamicPageTemplate
	if err := db.Mysql.Where("app_id = ? AND template_id = ?", appID, templateID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到模板: %s", templateID)
		}
		return nil, err
	}
	return templateFromRecord(&record)
}

// SaveCustomTemplate 新建或更新用户模板，并在持久化前复用页面协议校验。
func (s *TemplateService) SaveCustomTemplate(template *SDUITemplate, operator string, expectedRevision int) (*SDUITemplate, error) {
	if db.Mysql == nil {
		return nil, errors.New("数据库未初始化，无法保存用户模板")
	}
	if template == nil {
		return nil, errors.New("模板不能为空")
	}
	if template.Builtin || s.isBuiltin(template.TemplateID) {
		return nil, errors.New("内置行业模板只读，请另存为新的自定义模板")
	}

	prepared, err := prepareCustomTemplate(template)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateProtocol(prepared); err != nil {
		return nil, err
	}
	if strings.TrimSpace(operator) == "" {
		operator = "admin"
	}

	var saved models.DynamicPageTemplate
	err = db.Mysql.Transaction(func(tx *gorm.DB) error {
		var existing models.DynamicPageTemplate
		findErr := tx.Where("app_id = ? AND template_id = ?", prepared.AppID, prepared.TemplateID).First(&existing).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if expectedRevision != 0 {
				return errors.New("模板不存在，不能使用非零 expected_revision 更新")
			}
			created := templateToRecord(prepared, operator)
			created.Revision = 1
			if err := tx.Create(&created).Error; err != nil {
				return err
			}
			saved = created
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if expectedRevision <= 0 || existing.Revision != expectedRevision {
			return fmt.Errorf("模板乐观锁 CAS 拦截：当前版本为 v%d，请刷新后重试", existing.Revision)
		}

		updated := templateToRecord(prepared, operator)
		updated.ID = existing.ID
		updated.Revision = existing.Revision + 1
		updated.CreatedAt = existing.CreatedAt
		if err := tx.Save(&updated).Error; err != nil {
			return err
		}
		saved = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return templateFromRecord(&saved)
}

// DeleteCustomTemplate 按修订版本删除指定用户模板，防止编辑窗口覆盖后的误删。
func (s *TemplateService) DeleteCustomTemplate(appID, templateID string, expectedRevision int) error {
	if s.isBuiltin(templateID) {
		return errors.New("内置行业模板只读，不能删除")
	}
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(templateID) == "" {
		return errors.New("app_id 与 template_id 不能为空")
	}
	if expectedRevision <= 0 {
		return errors.New("删除模板必须提供有效的 expected_revision")
	}
	if db.Mysql == nil {
		return errors.New("数据库未初始化，无法删除用户模板")
	}

	result := db.Mysql.Where("app_id = ? AND template_id = ? AND revision = ?", appID, templateID, expectedRevision).Delete(&models.DynamicPageTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("模板不存在或版本已变化，请刷新后重试")
	}
	return nil
}

// ApplyTemplateToPage 将内置或用户模板转换为待保存的标准动态页面。
func (s *TemplateService) ApplyTemplateToPage(templateID, appID, pageID, title string) (*models.DynamicPage, error) {
	template, err := s.GetTemplate(appID, templateID)
	if err != nil {
		return nil, err
	}
	return applySDUITemplateToPage(template, appID, pageID, title)
}

// isBuiltin 判断模板标识是否已被内置行业模板占用。
func (s *TemplateService) isBuiltin(templateID string) bool {
	_, err := s.registry.GetTemplate(strings.TrimSpace(templateID))
	return err == nil
}

// prepareCustomTemplate 规范化用户模板中的默认字段与空数组。
func prepareCustomTemplate(source *SDUITemplate) (*SDUITemplate, error) {
	template := cloneSDUITemplate(source)
	template.AppID = strings.TrimSpace(template.AppID)
	template.TemplateID = strings.TrimSpace(template.TemplateID)
	template.Name = strings.TrimSpace(template.Name)
	template.Description = strings.TrimSpace(template.Description)
	if template.AppID == "" || template.TemplateID == "" || template.Name == "" {
		return nil, errors.New("app_id、template_id 与 name 不能为空")
	}
	if template.BusinessType == "" {
		template.BusinessType = "custom"
	}
	if template.Intent == "" {
		template.Intent = "watch"
	}
	if template.DefaultTheme == "" {
		template.DefaultTheme = "dark_glass"
	}
	if template.DefaultAccentColor == "" {
		template.DefaultAccentColor = "#FF9F0A"
	}
	if template.DefaultBlocks == nil {
		template.DefaultBlocks = []models.BlockItem{}
	}
	template.Builtin = false
	return template, nil
}

// validateTemplateProtocol 将模板映射为临时页面，复用同一份 SDUI 协议校验规则。
func validateTemplateProtocol(template *SDUITemplate) error {
	blocks, err := json.Marshal(template.DefaultBlocks)
	if err != nil {
		return fmt.Errorf("模板积木序列化失败: %w", err)
	}
	page := &models.DynamicPage{
		AppID:        template.AppID,
		PageID:       "template_preview",
		Status:       "draft",
		Title:        template.Name,
		BusinessType: template.BusinessType,
		Intent:       template.Intent,
		Theme:        template.DefaultTheme,
		AccentColor:  template.DefaultAccentColor,
		Blocks:       string(blocks),
	}
	if report := ValidateDynamicPage(page); !report.IsValid {
		return fmt.Errorf("模板 SDUI 协议校验未通过: %s", strings.Join(report.Errors, "; "))
	}
	if report := ValidatePageAgainstSchema(page); !report.IsValid {
		return fmt.Errorf("模板 Schema 校验未通过: %s", strings.Join(report.Errors, "; "))
	}
	return nil
}

// templateFromRecord 将数据库记录转换为统一模板 DTO。
func templateFromRecord(record *models.DynamicPageTemplate) (*SDUITemplate, error) {
	if record == nil {
		return nil, errors.New("模板记录不能为空")
	}
	template := &SDUITemplate{
		ID:                 record.ID,
		AppID:              record.AppID,
		TemplateID:         record.TemplateID,
		TemplateVersion:    fmt.Sprintf("custom.%d", record.Revision),
		Revision:           record.Revision,
		Name:               record.Name,
		BusinessType:       record.BusinessType,
		Intent:             record.Intent,
		Description:        record.Description,
		DefaultTheme:       record.Theme,
		DefaultAccentColor: record.AccentColor,
		DefaultBlocks:      []models.BlockItem{},
	}
	if strings.TrimSpace(record.Blocks) != "" {
		if err := json.Unmarshal([]byte(record.Blocks), &template.DefaultBlocks); err != nil {
			return nil, fmt.Errorf("模板积木解析失败: %w", err)
		}
	}
	if strings.TrimSpace(record.ShareConfig) != "" && record.ShareConfig != "null" {
		var share models.PageShareConfig
		if err := json.Unmarshal([]byte(record.ShareConfig), &share); err != nil {
			return nil, fmt.Errorf("模板分享配置解析失败: %w", err)
		}
		template.DefaultShare = &share
	}
	return template, nil
}

// templateToRecord 将统一模板 DTO 转换为持久化实体。
func templateToRecord(template *SDUITemplate, operator string) models.DynamicPageTemplate {
	blocks, _ := json.Marshal(template.DefaultBlocks)
	share, _ := json.Marshal(template.DefaultShare)
	now := time.Now()
	return models.DynamicPageTemplate{
		AppID:        template.AppID,
		TemplateID:   template.TemplateID,
		Name:         template.Name,
		BusinessType: template.BusinessType,
		Intent:       template.Intent,
		Description:  template.Description,
		Theme:        template.DefaultTheme,
		AccentColor:  template.DefaultAccentColor,
		Blocks:       string(blocks),
		ShareConfig:  string(share),
		UpdatedBy:    operator,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
