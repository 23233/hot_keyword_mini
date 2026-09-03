// Package models sdui.go
package models

import (
	"time"
)

// BlockAction 定义原子积木组件被触发时的标准动作行为
type BlockAction struct {
	// 动作类型: copy_text / navigate_page / open_channels_activity / open_mini_program / preview_image / open_webview / request_data / require_auth / toast / refresh / share
	Type string `json:"type"`
	// 是否必须登录后方可触发
	RequireAuth bool `json:"require_auth,omitempty"`
	// 动作载荷参数字典
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// BlockStyle 定义积木组件视觉渲染样式 (严格遵循苹果 HIG 规范)
type BlockStyle struct {
	// 垂直外边距
	MarginY string `json:"margin_y,omitempty"`
	// 水平外边距
	MarginX string `json:"margin_x,omitempty"`
	// 内边距
	Padding string `json:"padding,omitempty"`
	// 圆角梯度
	BorderRadius string `json:"border_radius,omitempty"`
	// 是否开启苹果高斯模糊毛玻璃
	GlassBlur bool `json:"glass_blur,omitempty"`
	// 主题高光强调色
	AccentColor string `json:"accent_color,omitempty"`
	// 自定义背景色/渐变
	Background string `json:"background,omitempty"`
}

// BlockItem 定义原子积木组件核心结构
type BlockItem struct {
	// 积木唯一ID (如 block_hero_101)
	ID string `json:"id"`
	// 积木类型 (media_hero / resource_card / action_button / episode_list / item_grid / notice / timeline 等)
	Type string `json:"type"`
	// 组件属性字典 (title, subtitle, cover_url, video_url 等)
	Props map[string]interface{} `json:"props,omitempty"`
	// 组件样式配置
	Style *BlockStyle `json:"style,omitempty"`
	// 绑定的单一交互动作 (兼容简易场景)
	Action *BlockAction `json:"action,omitempty"`
	// 绑定的多事件流动作列表 (如 tap: [Action1, Action2])
	Events map[string][]BlockAction `json:"events,omitempty"`
	// 显示条件表达式 (受控操作符: eq, neq, exists, and, or)
	VisibleWhen map[string]interface{} `json:"visible_when,omitempty"`
	// 列表循环配置
	Repeat map[string]interface{} `json:"repeat,omitempty"`
	// 兜底降级组件
	Fallback *BlockItem `json:"fallback,omitempty"`
}

// ShareItem 微信分享单一配置 (朋友或朋友圈)
type ShareItem struct {
	// 是否开启该分享渠道
	Enabled bool `json:"enabled"`
	// 分享标题
	Title string `json:"title,omitempty"`
	// 分享目标路径 (仅分享给朋友生效)
	Path string `json:"path,omitempty"`
	// 分享查询参数 (分享到朋友圈时生效)
	Query string `json:"query,omitempty"`
	// 分享卡片海报图
	ImageUrl string `json:"image_url,omitempty"`
}

// PageShareConfig 页面级微信分享整体配置
type PageShareConfig struct {
	// 默认兜底分享图片
	DefaultImageUrl string `json:"default_image_url,omitempty"`
	// 分享给朋友配置
	Friend *ShareItem `json:"friend,omitempty"`
	// 分享到朋友圈配置
	Timeline *ShareItem `json:"timeline,omitempty"`
}

// DynamicPage 表示服务端驱动的动态页面配置实体 (多租户物理隔离)
type DynamicPage struct {
	// 自增ID
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:自增ID" json:"id"`
	// 所属小程序AppID (多租户物理隔离)
	AppID string `gorm:"column:app_id;size:64;not null;uniqueIndex:idx_app_page;comment:小程序AppID" json:"app_id"`
	// 页面唯一ID (如 home / drama_detail / game_redeem)
	PageID string `gorm:"column:page_id;size:64;not null;uniqueIndex:idx_app_page;comment:页面唯一ID" json:"page_id"`
	// 修订版本号
	Revision int `gorm:"column:revision;default:1;comment:修订版本号" json:"revision"`
	// 状态: published(已发布) / draft(草稿) / archived(归档)
	Status string `gorm:"column:status;size:32;default:'published';comment:页面状态" json:"status"`
	// 页面标题 (如: 猴王下山 - 精选剧场)
	Title string `gorm:"column:title;size:128;not null;comment:页面标题" json:"title"`
	// 业务领域类型: drama(短剧) / game(游戏) / query(查询) / download(资源下载) / custom(自定义)
	BusinessType string `gorm:"column:business_type;size:64;default:'drama';comment:业务类型" json:"business_type"`
	// 用户搜索意图: watch(观看) / redeem(领兑换码) / query(查询) / download(下载)
	Intent string `gorm:"column:intent;size:64;default:'watch';comment:用户搜索意图" json:"intent"`
	// 主题风格: dark_glass(深黑磨砂) / light_clean(极简冷白) / cyber_neon(赛博霓虹)
	Theme string `gorm:"column:theme;size:64;default:'dark_glass';comment:主题基调" json:"theme"`
	// 主题高光强调色 (如 #FF9F0A)
	AccentColor string `gorm:"column:accent_color;size:32;default:'#FF9F0A';comment:强调色" json:"accent_color"`
	// 页面级强制登录拦截
	RequireAuth bool `gorm:"column:require_auth;default:false;comment:是否需要强制登录" json:"require_auth"`
	// 分享协议 (JSON序列化存储)
	ShareConfig string `gorm:"column:share_config;type:text;comment:页面分享配置JSON" json:"share_config"`
	// 原子积木组件树列表 (JSON序列化存储)
	Blocks string `gorm:"column:blocks;type:longtext;comment:积木树JSON" json:"blocks"`
	// 当前蹭的上升指数词
	Keyword string `gorm:"column:keyword;size:128;comment:蹭的热门关键词" json:"keyword"`
	// 流量来源渠道标识
	Source string `gorm:"column:source;size:64;comment:渠道来源" json:"source"`
	// 营销归因活动ID
	CampaignID string `gorm:"column:campaign_id;size:64;comment:活动ID" json:"campaign_id"`
	// 页面/活动到期时间
	ExpiresAt *time.Time `gorm:"column:expires_at;comment:过期时间" json:"expires_at"`
	// 创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

// TableName 自定义 DynamicPage 模型的表名
func (p *DynamicPage) TableName() string {
	return "dynamic_pages"
}

// DynamicPageRevision 动态页面版本历史快照模型 (用于原子回滚与审计)
type DynamicPageRevision struct {
	// 自增主键ID
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:快照ID" json:"id"`
	// 所属小程序 AppID
	AppID string `gorm:"column:app_id;size:64;not null;index:idx_app_page_rev;comment:小程序AppID" json:"app_id"`
	// 页面唯一ID
	PageID string `gorm:"column:page_id;size:64;not null;index:idx_app_page_rev;comment:页面唯一ID" json:"page_id"`
	// 修订版本号
	Revision int `gorm:"column:revision;not null;index:idx_app_page_rev;comment:修订版本号" json:"revision"`
	// 页面标题
	Title string `gorm:"column:title;size:128;comment:页面标题" json:"title"`
	// 业务类型
	BusinessType string `gorm:"column:business_type;size:64;comment:业务类型" json:"business_type"`
	// 主题风格
	Theme string `gorm:"column:theme;size:64;comment:主题基调" json:"theme"`
	// 强调色
	AccentColor string `gorm:"column:accent_color;size:32;comment:强调色" json:"accent_color"`
	// 原子积木组件树列表快照 (JSON)
	Blocks string `gorm:"column:blocks;type:longtext;comment:积木树快照JSON" json:"blocks"`
	// 分享协议快照 (JSON)
	ShareConfig string `gorm:"column:share_config;type:text;comment:分享协议快照JSON" json:"share_config"`
	// 发布变更备注
	Remark string `gorm:"column:remark;size:255;comment:发布备注" json:"remark"`
	// 操作人
	CreatedBy string `gorm:"column:created_by;size:64;default:'admin';comment:操作人" json:"created_by"`
	// 快照创建时间
	CreatedAt time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
}

// TableName 自定义 DynamicPageRevision 模型的表名
func (r *DynamicPageRevision) TableName() string {
	return "dynamic_page_revisions"
}

// EnvelopeCache 响应信封中的缓存策略
type EnvelopeCache struct {
	// 实体标签ETag (用于条件请求比对)
	ETag string `json:"etag"`
	// 客户端缓存最大有效期(秒)
	MaxAge int `json:"max_age"`
}

// EnvelopeFallback 响应信封中的降级配置
type EnvelopeFallback struct {
	// 兜底页面ID
	PageID string `json:"page_id"`
	// 兜底模式 (如 static_safe)
	Mode string `json:"mode"`
}

// DynamicPageDTO 传输到前端的页面对象结构
type DynamicPageDTO struct {
	// 页面ID
	PageID string `json:"page_id"`
	// 修订版本号
	Revision int `json:"revision"`
	// 页面状态
	Status string `json:"status"`
	// 页面标题
	Title string `json:"title"`
	// 业务领域类型
	BusinessType string `json:"business_type"`
	// 搜索意图
	Intent string `json:"intent"`
	// 主题基调
	Theme string `json:"theme"`
	// 强调色
	AccentColor string `json:"accent_color"`
	// 是否强制鉴权
	RequireAuth bool `json:"require_auth"`
	// 分享配置
	ShareConfig *PageShareConfig `json:"share_config,omitempty"`
	// 积木组件列表
	Blocks []BlockItem `json:"blocks"`
}

// PageResponseEnvelope 服务端下发 SDUI 页面的统一响应信封
type PageResponseEnvelope struct {
	// 协议主版本号 (如 "1.1")
	ProtocolVersion string `json:"protocol_version"`
	// 模式Schema版本号 (如 3)
	SchemaVersion int `json:"schema_version"`
	// 请求追踪ID
	RequestID string `json:"request_id"`
	// 动态页面主体
	Page DynamicPageDTO `json:"page"`
	// 受控附加数据
	Data map[string]interface{} `json:"data"`
	// 渲染此页必需的客户端能力声明 (如 ["video", "clipboard"])
	CapabilitiesRequired []string `json:"capabilities_required"`
	// 缓存控制元数据
	Cache EnvelopeCache `json:"cache"`
	// 异常时的降级路由方案
	Fallback EnvelopeFallback `json:"fallback"`
}
