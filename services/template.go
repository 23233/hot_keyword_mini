// Package services template.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/models"
	"time"
)

// SDUITemplate 定义 SDUI 行业模板包元数据
type SDUITemplate struct {
	// 模板全局唯一标识 (如 tpl_drama_standard / tpl_game_redeem)
	TemplateID string `json:"template_id"`
	// 模板版本号 (如 1.0.0)
	TemplateVersion string `json:"template_version"`
	// 模板展示名称
	Name string `json:"name"`
	// 适用业务类型 (drama / game / query / download / custom)
	BusinessType string `json:"business_type"`
	// 适用搜索意图 (watch / redeem / query / download)
	Intent string `json:"intent"`
	// 模板使用场景说明
	Description string `json:"description"`
	// 默认主题风格 (dark_glass / light_clean / cyber_neon)
	DefaultTheme string `json:"default_theme"`
	// 默认高光强调色
	DefaultAccentColor string `json:"default_accent_color"`
	// 默认原子积木组件列表
	DefaultBlocks []models.BlockItem `json:"default_blocks"`
	// 默认分享配置
	DefaultShare *models.PageShareConfig `json:"default_share,omitempty"`
}

// TemplateRegistry 行业模板注册中心
type TemplateRegistry struct {
	templates map[string]*SDUITemplate
}

var globalRegistry *TemplateRegistry

func init() {
	globalRegistry = &TemplateRegistry{
		templates: make(map[string]*SDUITemplate),
	}
	initDefaultTemplates(globalRegistry)
}

// GetGlobalTemplateRegistry 获取全局模板注册中心实例
func GetGlobalTemplateRegistry() *TemplateRegistry {
	return globalRegistry
}

// ListTemplates 获取指定业务类型或全部模板列表
func (r *TemplateRegistry) ListTemplates(businessType string) []*SDUITemplate {
	var list []*SDUITemplate
	for _, tpl := range r.templates {
		if businessType == "" || tpl.BusinessType == businessType {
			list = append(list, tpl)
		}
	}
	return list
}

// GetTemplate 根据 TemplateID 查询特定模板
func (r *TemplateRegistry) GetTemplate(templateID string) (*SDUITemplate, error) {
	tpl, ok := r.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("未找到行业模板: %s", templateID)
	}
	return tpl, nil
}

// ApplyTemplateToPage 从模板一键派生为标准 DynamicPage 实体 (不产生模板专属私有协议)
func (r *TemplateRegistry) ApplyTemplateToPage(templateID, appID, pageID, title string) (*models.DynamicPage, error) {
	tpl, err := r.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	if appID == "" || pageID == "" {
		return nil, errors.New("appID 与 pageID 不能为空")
	}

	if title == "" {
		title = tpl.Name
	}

	blocksJSON, _ := json.Marshal(tpl.DefaultBlocks)
	shareJSON, _ := json.Marshal(tpl.DefaultShare)

	page := &models.DynamicPage{
		AppID:        appID,
		PageID:       pageID,
		Revision:     1,
		Status:       "published",
		Title:        title,
		BusinessType: tpl.BusinessType,
		Intent:       tpl.Intent,
		Theme:        tpl.DefaultTheme,
		AccentColor:  tpl.DefaultAccentColor,
		RequireAuth:  false,
		ShareConfig:  string(shareJSON),
		Blocks:       string(blocksJSON),
		Keyword:      title,
		Source:       "wechat_search",
		CampaignID:   "template_derived",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return page, nil
}

// initDefaultTemplates 初始化四大核心行业爆款模板包
func initDefaultTemplates(r *TemplateRegistry) {
	// 1. 短剧爆款承接模板 (drama)
	dramaBlocks := []models.BlockItem{
		{
			ID:   "block_hero_drama",
			Type: "media_hero",
			Props: map[string]interface{}{
				"title":     "猴王下山",
				"subtitle":  "第 1 集试看 · 爆火全网都市神话",
				"cover_url": "",
				"video_url": "https://sample-videos.com/video321/mp4/720/big_buck_bunny_720p_1mb.mp4",
				"rating":    9.9,
				"badge":     "🎬 高清试看",
			},
			Style: &models.BlockStyle{
				BorderRadius: "28rpx",
				GlassBlur:    true,
				AccentColor:  "#FF9F0A",
			},
			Action: &models.BlockAction{
				Type: "open_channels_activity",
				Payload: map[string]interface{}{
					"feed_id":          "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
					"finder_user_name": "gh_drama_official",
				},
			},
		},
		{
			ID:   "block_resource_drama",
			Type: "resource_card",
			Props: map[string]interface{}{
				"title":      "夸克网盘极速看全集",
				"desc":       "超清 4K 无删减版 免费自取",
				"pan_name":   "夸克网盘",
				"fetch_code": "hwxs88",
			},
			Style: &models.BlockStyle{
				BorderRadius: "24rpx",
				GlassBlur:    true,
			},
			Action: &models.BlockAction{
				Type: "copy_text",
				Payload: map[string]interface{}{
					"text":  "https://pan.quark.cn/s/monkey_king_full_888 提取码: hwxs88",
					"toast": "网盘链接已复制，打开夸克 APP 即可看全集",
				},
			},
		},
		{
			ID:   "block_btn_drama",
			Type: "action_button",
			Props: map[string]interface{}{
				"text":  "⚡ 获取 1-80 集完整大结局",
				"badge": "免费未删减",
			},
			Style: &models.BlockStyle{
				BorderRadius: "999rpx",
				AccentColor:  "#FF9F0A",
			},
			Action: &models.BlockAction{
				Type: "copy_text",
				Payload: map[string]interface{}{
					"text":  "关注官方公众号回复【猴王下山】获取完整版",
					"toast": "公众号信息已复制，微信搜一搜即可直达",
				},
			},
		},
	}

	r.templates["tpl_drama_standard"] = &SDUITemplate{
		TemplateID:         "tpl_drama_standard",
		TemplateVersion:    "1.0.0",
		Name:               "短剧爆款承接模板",
		BusinessType:       "drama",
		Intent:             "watch",
		Description:        "专为热播短剧搜索流量打造，包含大焦点试看视频、网盘多渠道提取与公众号防走丢闭环",
		DefaultTheme:       "dark_glass",
		DefaultAccentColor: "#FF9F0A",
		DefaultBlocks:      dramaBlocks,
	}

	// 2. 游戏礼包与兑换码模板 (game)
	gameBlocks := []models.BlockItem{
		{
			ID:   "block_notice_game",
			Type: "notice",
			Props: map[string]interface{}{
				"icon": "🎁",
				"text": "本周最新公测兑换码已更新，限时限量先到先得！",
			},
		},
		{
			ID:   "block_game_card",
			Type: "game_card",
			Props: map[string]interface{}{
				"title":        "绝地天王：觉醒",
				"subtitle":     "年度硬核魔幻 3D 手游",
				"cover_url":    "",
				"version":      "v2.5.0 全新公测",
				"package_id":   "pkg_game_novice_888",
				"claim_status": "unclaimed",
				"redeem_code":  "点击立即授权领取",
				"remaining":    "仅剩 12% 剩余",
			},
			Style: &models.BlockStyle{
				BorderRadius: "28rpx",
				GlassBlur:    true,
			},
			Action: &models.BlockAction{
				Type:        "request_data",
				RequireAuth: true,
				Payload: map[string]interface{}{
					"endpoint": "game.redeem",
					"body": map[string]interface{}{
						"package_id": "pkg_game_novice_888",
					},
					"response": map[string]interface{}{
						"data_path": "data",
						"save_as":   "redeem_result",
					},
					"on_success": []map[string]interface{}{
						{
							"type": "copy_text",
							"payload": map[string]interface{}{
								"path":  "$result.code",
								"toast": "公测专属礼包码已成功领取并复制！",
							},
						},
					},
				},
			},
		},
		{
			ID:   "block_btn_game",
			Type: "action_button",
			Props: map[string]interface{}{
				"text":  "🎮 登录领取独家公测礼包码",
				"badge": "限量礼包",
			},
			Style: &models.BlockStyle{
				BorderRadius: "999rpx",
				AccentColor:  "#30D158",
			},
			Action: &models.BlockAction{
				Type:        "request_data",
				RequireAuth: true,
				Payload: map[string]interface{}{
					"endpoint": "game.redeem",
					"body": map[string]interface{}{
						"package_id": "pkg_game_novice_888",
					},
					"response": map[string]interface{}{
						"data_path": "data",
						"save_as":   "redeem_result",
					},
					"on_success": []map[string]interface{}{
						{
							"type": "copy_text",
							"payload": map[string]interface{}{
								"path":  "$result.code",
								"toast": "礼包码已复制，请进入游戏兑换！",
							},
						},
					},
				},
			},
		},
	}

	r.templates["tpl_game_redeem"] = &SDUITemplate{
		TemplateID:         "tpl_game_redeem",
		TemplateVersion:    "1.0.0",
		Name:               "游戏礼包兑换码模板",
		BusinessType:       "game",
		Intent:             "redeem",
		Description:        "用于新游开服、礼包兑换码、激活码等热词收割，一键复制兑换码",
		DefaultTheme:       "cyber_neon",
		DefaultAccentColor: "#30D158",
		DefaultBlocks:      gameBlocks,
	}

	// 3. 通用信息与考分查询模板 (query)
	queryBlocks := []models.BlockItem{
		{
			ID:   "block_notice_query",
			Type: "notice",
			Props: map[string]interface{}{
				"icon": "🔍",
				"text": "2026 年度最新成绩查询通道已开启，请输入准考证号查询",
			},
		},
		{
			ID:   "block_form_query",
			Type: "form",
			Props: map[string]interface{}{
				"title":       "官方成绩极速查询入口",
				"input_label": "准考证号 / 身份证号",
				"placeholder": "请输入 15 位准考证号或证件号",
				"btn_text":    "立即查询结果",
			},
			Style: &models.BlockStyle{
				BorderRadius: "24rpx",
				GlassBlur:    true,
			},
			Action: &models.BlockAction{
				Type: "toast",
				Payload: map[string]interface{}{
					"text": "查询服务通道已受理，正在实时拉取中...",
				},
			},
		},
	}

	r.templates["tpl_query_result"] = &SDUITemplate{
		TemplateID:         "tpl_query_result",
		TemplateVersion:    "1.0.0",
		Name:               "通用信息查询结果模板",
		BusinessType:       "query",
		Intent:             "query",
		Description:        "用于各类考试成绩、物流、真伪验证、证书查询等热词意图承接",
		DefaultTheme:       "dark_glass",
		DefaultAccentColor: "#0A84FF",
		DefaultBlocks:      queryBlocks,
	}

	// 4. 极速软件与资源下载模板 (download)
	downloadBlocks := []models.BlockItem{
		{
			ID:   "block_hero_download",
			Type: "media_hero",
			Props: map[string]interface{}{
				"title":     "极速应用安装包",
				"subtitle":  "官方绿色完整版 · 纯净无广告 · 45.8 MB",
				"cover_url": "",
				"badge":     "⚡ 正版高速",
			},
			Style: &models.BlockStyle{
				BorderRadius: "28rpx",
				GlassBlur:    true,
			},
		},
		{
			ID:   "block_resource_download",
			Type: "resource_card",
			Props: map[string]interface{}{
				"title":      "夸克不限速通道",
				"desc":       "支持多线程极速下载",
				"pan_name":   "夸克网盘",
				"fetch_code": "apk888",
			},
			Style: &models.BlockStyle{
				BorderRadius: "24rpx",
				GlassBlur:    true,
			},
			Action: &models.BlockAction{
				Type: "copy_text",
				Payload: map[string]interface{}{
					"text":  "https://pan.quark.cn/s/download_apk 提取码: apk888",
					"toast": "下载链接与提取码已复制，请在浏览器中打开",
				},
			},
		},
	}

	r.templates["tpl_download_resource"] = &SDUITemplate{
		TemplateID:         "tpl_download_resource",
		TemplateVersion:    "1.0.0",
		Name:               "极速软件/资源下载模板",
		BusinessType:       "download",
		Intent:             "download",
		Description:        "用于软件包、电子书、高清壁纸等资源搜索流量，提供夸克/百度网盘多通道",
		DefaultTheme:       "light_clean",
		DefaultAccentColor: "#5E5CE6",
		DefaultBlocks:      downloadBlocks,
	}
}
