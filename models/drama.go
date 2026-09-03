// Package models drama.go
package models

import (
	"time"
)

// Drama 表示短剧基础信息
type Drama struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement;comment:剧集ID" json:"id"`
	CreatedAt       time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	Title           string    `gorm:"column:title;type:varchar(128);not null;comment:短剧名称" json:"title"`
	Subtitle        string    `gorm:"column:subtitle;type:varchar(255);comment:宣传副标题" json:"subtitle"`
	CoverUrl        string    `gorm:"column:cover_url;type:varchar(512);comment:竖版封面海报" json:"cover_url"`
	BannerUrl       string    `gorm:"column:banner_url;type:varchar(512);comment:横版背景宣传图" json:"banner_url"`
	TotalEpisodes   int       `gorm:"column:total_episodes;default:80;comment:总集数" json:"total_episodes"`
	UpdatedEpisodes int       `gorm:"column:updated_episodes;default:80;comment:已更新集数" json:"updated_episodes"`
	Rating          float64   `gorm:"column:rating;type:decimal(3,1);default:9.8;comment:短剧评分" json:"rating"`
	HotScore        int64     `gorm:"column:hot_score;default:990000;comment:全网热度指数" json:"hot_score"`
	Tags            string    `gorm:"column:tags;type:varchar(255);comment:短剧分类标签(逗号隔开)" json:"tags"`
	Description     string    `gorm:"column:description;type:text;comment:剧情详细简介" json:"description"`
	Highlights      string    `gorm:"column:highlights;type:varchar(512);comment:爆款高潮看点说明" json:"highlights"`
	PlayMode        string    `gorm:"column:play_mode;type:varchar(32);default:'direct_video';comment:默认播放模式(direct_video/channels_embedded/channels_video/web_view/none)" json:"play_mode"`
	FinderUserName  string    `gorm:"column:finder_user_name;type:varchar(128);comment:微信视频号ID(用于内嵌或跳转)" json:"finder_user_name"`
	ChannelsFeedID  string    `gorm:"column:channels_feed_id;type:varchar(128);comment:微信视频号动态ID/视频源" json:"channels_feed_id"`
	WebUrl          string    `gorm:"column:web_url;type:varchar(512);comment:网页播放链接" json:"web_url"`
}

// DramaEpisode 表示短剧的单个剧集信息
type DramaEpisode struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement;comment:集数ID" json:"id"`
	CreatedAt      time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	DramaID        int64     `gorm:"column:drama_id;index;not null;comment:所属短剧ID" json:"drama_id"`
	EpisodeNum     int       `gorm:"column:episode_num;index;not null;comment:集数序号" json:"episode_num"`
	Title          string    `gorm:"column:title;type:varchar(128);comment:单集标题" json:"title"`
	CoverUrl       string    `gorm:"column:cover_url;type:varchar(512);comment:单集封面" json:"cover_url"`
	VideoUrl       string    `gorm:"column:video_url;type:varchar(512);comment:播放源地址(试看用)" json:"video_url"`
	IsFree         bool      `gorm:"column:is_free;default:false;comment:是否支持免费试看" json:"is_free"`
	Duration       int       `gorm:"column:duration;default:120;comment:单集时长(秒)" json:"duration"`
	PlayMode       string    `gorm:"column:play_mode;type:varchar(32);default:'direct_video';comment:单集播放模式(direct_video/channels_embedded/channels_video/web_view/none)" json:"play_mode"`
	FinderUserName string    `gorm:"column:finder_user_name;type:varchar(128);comment:微信视频号ID(用于内嵌或跳转)" json:"finder_user_name"`
	ChannelsFeedID string    `gorm:"column:channels_feed_id;type:varchar(128);comment:微信视频号动态ID" json:"channels_feed_id"`
	WebUrl         string    `gorm:"column:web_url;type:varchar(512);comment:单集网页播放链接" json:"web_url"`
}

// ActionChannel 看后续转化渠道配置项
type ActionChannel struct {
	Type        string `json:"type"`        // 渠道类型: pan(网盘), mp(公众号), customer(微信客服), mini(正版小程序)
	Name        string `json:"name"`        // 渠道名称: 如 "夸克网盘极速看全集" / "官方公众号"
	Icon        string `json:"icon"`        // 图标标识
	Desc        string `json:"desc"`        // 说明文案: 如 "高清4K未删减版 免费自取"
	BtnText     string `json:"btn_text"`    // 按钮文案: 如 "一键复制网盘链接"
	Content     string `json:"content"`     // 核心复制内容: 网盘链接、微信号或公众号名称
	FetchCode   string `json:"fetch_code"`  // 提取码(如有)
	TargetAppID string `json:"target_appid"`// 目标小程序AppID(如有)
	TargetPath  string `json:"target_path"` // 目标小程序页面路径(如有)
	TipNotice   string `json:"tip_notice"`  // 复制后的操作指引提示
}

// FloatingButton 首页悬浮按钮配置(可选，接口有值才展示)
type FloatingButton struct {
	Text       string `json:"text"`        // 浮动按钮文字
	Icon       string `json:"icon"`        // 浮动图标
	ActionType string `json:"action_type"` // 交互行为: open_modal(打开网盘弹窗), copy_pan(直接复制网盘)
	ActionData string `json:"action_data"` // 额外透传数据
	Badge      string `json:"badge"`       // 悬浮微标提示(如"免费")
	IsVisible  bool   `json:"is_visible"`  // 是否可见
}

// PageConfig 表示小程序首页布局与渠道配置
type PageConfig struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement;comment:配置ID" json:"id"`
	CreatedAt      time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	DramaID        int64     `gorm:"column:drama_id;index;unique;not null;comment:关联短剧ID" json:"drama_id"`
	PageTitle      string    `gorm:"column:page_title;type:varchar(128);comment:小程序页面主标题" json:"page_title"`
	PageSubtitle   string    `gorm:"column:page_subtitle;type:varchar(255);comment:小程序页面副标题" json:"page_subtitle"`
	DisplayMode    string    `gorm:"column:display_mode;type:varchar(32);default:'immersive_video';comment:展示模式(immersive_video/episode_grid/direct_portal/gallery_matrix/webview)" json:"display_mode"`
	WebviewUrl     string    `gorm:"column:webview_url;type:varchar(512);comment:Webview模式目标跳转链接" json:"webview_url"`
	ActionChannels string    `gorm:"column:action_channels;type:text;comment:看后续渠道配置JSON" json:"action_channels"`
	Announcement   string    `gorm:"column:announcement;type:varchar(255);comment:顶部横幅公告" json:"announcement"`
	ShareTitle     string    `gorm:"column:share_title;type:varchar(128);comment:分享标题" json:"share_title"`
	ShareDesc      string    `gorm:"column:share_desc;type:varchar(255);comment:分享描述" json:"share_desc"`
	ShareCover     string    `gorm:"column:share_cover;type:varchar(512);comment:分享封面图" json:"share_cover"`
}
