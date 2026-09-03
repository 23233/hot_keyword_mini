// Package services drama.go
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"time"

	"github.com/23233/ggg/logger"
	"gorm.io/gorm"
)

// DramaHomeResponse 首页聚合响应结构体
type DramaHomeResponse struct {
	// 小程序页面主标题(接口动态驱动)
	PageTitle string `json:"page_title"`
	// 小程序页面副标题(接口动态驱动)
	PageSubtitle string `json:"page_subtitle"`
	// 剧集详细信息
	Drama *models.Drama `json:"drama"`
	// 选集列表
	Episodes []*models.DramaEpisode `json:"episodes"`
	// 当前页面展示模式 (immersive_video / episode_grid / direct_portal / gallery_matrix / webview)
	DisplayMode string `json:"display_mode"`
	// Webview模式目标跳转网页链接(当 display_mode == "webview" 时生效)
	WebviewUrl string `json:"webview_url"`
	// 看后续承接渠道列表
	ActionChannels []models.ActionChannel `json:"action_channels"`
	// 首页浮动按钮配置(可选，有则浮动显示)
	FloatingButton *models.FloatingButton `json:"floating_button,omitempty"`
	// 顶部通告
	Announcement string `json:"announcement"`
	// 分享标题
	ShareTitle string `json:"share_title"`
	// 分享描述
	ShareDesc string `json:"share_desc"`
	// 分享封面
	ShareCover string `json:"share_cover"`
	// 同类热门推荐列表
	Recommendations []models.Drama `json:"recommendations"`
	// 短剧画廊海量精选列表(用于 gallery_matrix 画廊模式)
	GalleryList []*models.Drama `json:"gallery_list"`
}

// DramaService 短剧业务服务结构体
type DramaService struct{}

// NewDramaService 创建短剧服务实例
func NewDramaService() *DramaService {
	return &DramaService{}
}

// GetDefaultDrama 获取默认《猴王下山》短剧信息并确保种子数据已初始化
func (s *DramaService) GetDefaultDrama() (*models.Drama, error) {
	var drama models.Drama
	err := db.Mysql.Where("title = ?", "猴王下山").First(&drama).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 数据库为空时自动初始化种子数据
			logger.JM.Infof("未检测到《猴王下山》数据，开始执行种子数据自举初始化...")
			return s.seedDefaultDramaData()
		}
		return nil, err
	}
	return &drama, nil
}

// GetHomeData 获取小程序首页所需的所有驱动数据
// 支持通过 modeOverride 参数临时预览不同模式
func (s *DramaService) GetHomeData(modeOverride string) (*DramaHomeResponse, error) {
	// 1. 获取《猴王下山》核心数据
	drama, err := s.GetDefaultDrama()
	if err != nil {
		return nil, fmt.Errorf("获取短剧信息失败: %w", err)
	}

	// 2. 查询全部选集列表(按集数升序排列)
	var episodes []*models.DramaEpisode
	err = db.Mysql.Where("drama_id = ?", drama.ID).Order("episode_num asc").Find(&episodes).Error
	if err != nil {
		return nil, fmt.Errorf("获取剧集选集失败: %w", err)
	}

	// 3. 读取页面布局配置
	var pageConfig models.PageConfig
	err = db.Mysql.Where("drama_id = ?", drama.ID).First(&pageConfig).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("读取页面配置失败: %w", err)
	}

	// 确定当前生效的展示模式
	activeMode := pageConfig.DisplayMode
	if activeMode == "" {
		activeMode = "immersive_video"
	}
	// 支持请求参数中临时指定模式进行体验与切换
	if modeOverride != "" {
		activeMode = modeOverride
	}

	// 解析渠道配置列表
	var channels []models.ActionChannel
	if pageConfig.ActionChannels != "" {
		_ = json.Unmarshal([]byte(pageConfig.ActionChannels), &channels)
	}

	// 4. 获取同类热门推荐短剧与画廊短剧列表
	recommendations := s.getMockRecommendations()
	galleryList := s.getMockGalleryList(drama)

	// 5. 组装动态页面标题与副标题
	pageTitle := pageConfig.PageTitle
	if pageTitle == "" {
		pageTitle = drama.Title
	}
	pageSubtitle := pageConfig.PageSubtitle
	if pageSubtitle == "" {
		pageSubtitle = drama.Subtitle
	}

	// 6. 组装浮动按钮(若有网盘等转化渠道，下发浮动快捷入口)
	var floatingBtn *models.FloatingButton
	hasPan := false
	for _, c := range channels {
		if c.Type == "pan" {
			hasPan = true
			break
		}
	}
	if hasPan {
		floatingBtn = &models.FloatingButton{
			Text:       "领取全集网盘资源",
			Icon:       "🎁",
			ActionType: "open_modal",
			Badge:      "免费",
			IsVisible:  true,
		}
	}

	webviewUrl := pageConfig.WebviewUrl
	if webviewUrl == "" {
		webviewUrl = drama.WebUrl
	}

	// 7. 组合并返回响应数据
	return &DramaHomeResponse{
		PageTitle:       pageTitle,
		PageSubtitle:    pageSubtitle,
		Drama:           drama,
		Episodes:        episodes,
		DisplayMode:     activeMode,
		WebviewUrl:      webviewUrl,
		ActionChannels:  channels,
		FloatingButton:  floatingBtn,
		Announcement:    pageConfig.Announcement,
		ShareTitle:      pageConfig.ShareTitle,
		ShareDesc:       pageConfig.ShareDesc,
		ShareCover:      pageConfig.ShareCover,
		Recommendations: recommendations,
		GalleryList:     galleryList,
	}, nil
}

// SwitchDisplayMode 切换小程序的展示模式
func (s *DramaService) SwitchDisplayMode(mode string) error {
	drama, err := s.GetDefaultDrama()
	if err != nil {
		return err
	}
	return db.Mysql.Model(&models.PageConfig{}).
		Where("drama_id = ?", drama.ID).
		Update("display_mode", mode).Error
}

// seedDefaultDramaData 初始化默认《猴王下山》完整业务种子数据
func (s *DramaService) seedDefaultDramaData() (*models.Drama, error) {
	now := time.Now()

	// 1. 初始化短剧主表
	drama := models.Drama{
		CreatedAt:       now,
		UpdatedAt:       now,
		Title:           "猴王下山",
		Subtitle:        "齐天战神下山，都市风云再起！全网爆款热播爽剧",
		CoverUrl:        "https://images.unsplash.com/photo-1579783900882-c0d3dad7b119?w=800&auto=format&fit=crop&q=80",
		BannerUrl:       "https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1200&auto=format&fit=crop&q=80",
		TotalEpisodes:   80,
		UpdatedEpisodes: 80,
		Rating:          9.9,
		HotScore:        998820,
		Tags:            "逆袭,战神,爽剧,高能反转,神话都市",
		Description:     "三千年前，猴王大闹天宫傲视群仙；三千年后，化身林家弃婿下山入凡尘！本想隐姓埋名守护娇妻，奈何各方恶势力步步紧逼。今日，齐天令出，万佛臣服，都市各路豪门皆尽俯首称臣！看齐天大圣如何横扫都市，血战八方！",
		Highlights:      "【高能预警】第3集金箍棒初露锋芒震碎豪车！第12集猴王大寿亮出齐天金令，四大家族家主齐下跪！第45集十万妖王云集江城，迎战域外战神！",
	}

	if err := db.Mysql.Create(&drama).Error; err != nil {
		return nil, err
	}

	// 2. 初始化 80 集选集列表(第1~3集提供试看源，其余标注需解锁观看)
	sampleVideos := []string{
		"https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4",
		"https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerEscapes.mp4",
		"https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerFun.mp4",
	}

	var episodes []models.DramaEpisode
	for i := 1; i <= 80; i++ {
		isFree := i <= 3
		videoUrl := ""
		if isFree {
			videoUrl = sampleVideos[(i-1)%len(sampleVideos)]
		}

		episodes = append(episodes, models.DramaEpisode{
			CreatedAt:  now,
			UpdatedAt:  now,
			DramaID:    drama.ID,
			EpisodeNum: i,
			Title:      fmt.Sprintf("第%d集: %s", i, getEpisodeTitle(i)),
			CoverUrl:   drama.CoverUrl,
			VideoUrl:   videoUrl,
			IsFree:     isFree,
			Duration:   115 + (i % 30),
		})
	}

	if err := db.Mysql.CreateInBatches(episodes, 40).Error; err != nil {
		logger.JM.Errorf("批量插入剧集选集失败: %v", err)
	}

	// 3. 初始化看后续承接渠道配置
	defaultChannels := []models.ActionChannel{
		{
			Type:      "pan",
			Name:      "网盘极速全集(无删减完整版)",
			Icon:      "cloud-download",
			Desc:      "4K蓝光画质 包含1-80集全集及大结局",
			BtnText:   "一键复制网盘链接与提取码",
			Content:   "https://pan.quark.cn/s/monkey_king_full_80episodes",
			FetchCode: "8888",
			TipNotice: "网盘链接已复制！请打开浏览器或网盘APP粘贴保存即可直接观看完整版！",
		},
		{
			Type:      "mp",
			Name:      "官方剧场公众号(防走丢通道)",
			Icon:      "chat-bubble-left-ellipsis",
			Desc:      "发送【猴王下山】即刻获取更新与隐藏番外篇",
			BtnText:   "一键复制公众号名称",
			Content:   "爆笑追剧社",
			TipNotice: "公众号名称已复制！打开微信搜一搜关注并回复【猴王下山】即可！",
		},
		{
			Type:      "customer",
			Name:      "剧迷专属微信客服/追剧群",
			Icon:      "user-group",
			Desc:      "添加客服微信进入【猴王下山】剧迷催更大群",
			BtnText:   "复制客服微信号",
			Content:   "houwang_vip999",
			TipNotice: "微信号已复制，打开微信添加好友即可免费领取后续剧集！",
		},
	}

	channelsJSON, _ := json.Marshal(defaultChannels)

	// 4. 初始化页面配置
	pageConfig := models.PageConfig{
		CreatedAt:      now,
		UpdatedAt:      now,
		DramaID:        drama.ID,
		DisplayMode:    "immersive_video", // 默认沉浸影音模式
		ActionChannels: string(channelsJSON),
		Announcement:   "🔥《猴王下山》全网热度突破99万！1~80集大结局已更新，点击选集即可观看后续！",
		ShareTitle:     "【热播短剧】《猴王下山》全集在线看！战神归来横扫都市！",
		ShareDesc:      "刷到停不下来！大圣下山护妻，各路豪门俯首称臣，点击立即免费看全集！",
		ShareCover:     drama.CoverUrl,
	}

	if err := db.Mysql.Create(&pageConfig).Error; err != nil {
		logger.JM.Errorf("创建默认页面配置失败: %v", err)
	}

	return &drama, nil
}

// getEpisodeTitle 模拟生成前几十集的精彩吸睛标题
func getEpisodeTitle(num int) string {
	titles := map[int]string{
		1:  "潜龙出渊，猴王降世",
		2:  "入赘林家，受尽冷眼",
		3:  "金箍初现，一指断豪车",
		4:  "欺我妻子者，虽远必诛",
		5:  "齐天令出，江城震荡",
		6:  "岳母逼离，豪门上门求见",
		7:  "打脸二叔，真相大白",
		8:  "万宝阁主躬身相迎",
		9:  "暗夜袭杀，猴王显威",
		10: "生死擂台，一拳破宗师",
		11: "林家老太君悔不当初",
		12: "齐天金令现世，四大家族跪迎",
		13: "娇妻受辱，雷霆之怒",
		14: "战神归来，横推敌巢",
		15: "昔日仇敌，不过草芥",
		16: "神医妙手，逆天改命",
		17: "大闹江城，无人敢挡",
		18: "四大家族全军覆没",
		19: "域外杀手，弹指灰飞烟灭",
		20: "大结局前篇：猴王登顶都市至尊",
	}
	if t, ok := titles[num]; ok {
		return t
	}
	return fmt.Sprintf("都市风云变色 第%d章惊天对决", num)
}

// getMockRecommendations 模拟同类热门爆款推荐短剧
func (s *DramaService) getMockRecommendations() []models.Drama {
	return []models.Drama{
		{
			ID:              2,
			Title:           "天王殿之龙王归来",
			Subtitle:        "九星战神卸甲，十万将士相送",
			CoverUrl:        "https://images.unsplash.com/photo-1534447677768-be436bb09401?w=600&auto=format&fit=crop&q=80",
			Rating:          9.7,
			HotScore:        886000,
			Tags:            "战神,热血,逆袭",
			TotalEpisodes:   75,
			UpdatedEpisodes: 75,
		},
		{
			ID:              3,
			Title:           "绝世武神在都市",
			Subtitle:        "修仙万载归来，弹指遮天",
			CoverUrl:        "https://images.unsplash.com/photo-1514565131-fce0801e5785?w=600&auto=format&fit=crop&q=80",
			Rating:          9.6,
			HotScore:        765000,
			Tags:            "修真,无敌,都市",
			TotalEpisodes:   68,
			UpdatedEpisodes: 68,
		},
		{
			ID:              4,
			Title:           "千亿豪婿",
			Subtitle:        "装穷三年，今日首富身份曝光",
			CoverUrl:        "https://images.unsplash.com/photo-1492691527719-9d1e07e534b4?w=600&auto=format&fit=crop&q=80",
			Rating:          9.5,
			HotScore:        692000,
			Tags:            "神豪,爽文,打脸",
			TotalEpisodes:   85,
			UpdatedEpisodes: 85,
		},
	}
}

// GetDramaDetail 获取指定短剧的详情与选集列表
func (s *DramaService) GetDramaDetail(dramaID int64) (*models.Drama, []*models.DramaEpisode, error) {
	// 若请求的是默认《猴王下山》
	defaultDrama, err := s.GetDefaultDrama()
	if err == nil && (dramaID == defaultDrama.ID || dramaID == 0) {
		var episodes []*models.DramaEpisode
		_ = db.Mysql.Where("drama_id = ?", defaultDrama.ID).Order("episode_num asc").Find(&episodes).Error
		return defaultDrama, episodes, nil
	}

	// 匹配画廊中的短剧
	galleryList := s.getMockGalleryList(defaultDrama)
	for _, d := range galleryList {
		if d.ID == dramaID {
			episodes := s.generateEpisodesForDrama(d)
			return d, episodes, nil
		}
	}

	// 默认回退
	var episodes []*models.DramaEpisode
	_ = db.Mysql.Where("drama_id = ?", defaultDrama.ID).Order("episode_num asc").Find(&episodes).Error
	return defaultDrama, episodes, nil
}

// getMockGalleryList 获取画廊多剧库列表，包含丰富短剧及多种播放模式配置
func (s *DramaService) getMockGalleryList(currentDrama *models.Drama) []*models.Drama {
	var list []*models.Drama

	// 1. 首位加入当前核心推广的《猴王下山》
	if currentDrama != nil {
		currentDrama.PlayMode = "direct_video"
		list = append(list, currentDrama)
	}

	// 2. 其它各类型短剧（涵盖 direct_video, channels_video, web_view, none）
	list = append(list, []*models.Drama{
		{
			ID:              2,
			Title:           "天王殿之龙王归来",
			Subtitle:        "九星战神卸甲，十万将士相送",
			CoverUrl:        "https://images.unsplash.com/photo-1534447677768-be436bb09401?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.8,
			HotScore:        886000,
			Tags:            "战神,热血,逆袭",
			TotalEpisodes:   80,
			UpdatedEpisodes: 80,
			PlayMode:        "channels_video", // 视频号跳转模式
			FinderUserName:  "gh_drama_official",
			ChannelsFeedID:  "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
			Description:     "九星战神卸甲还乡，只为履行三年之约，却遭岳母与情敌百般羞辱。一声令下，十万战部将士星夜疾驰奔赴江城！",
			Highlights:      "第1集战神归来！第6集天王令出震撼全城！",
		},
		{
			ID:              3,
			Title:           "绝世武神在都市",
			Subtitle:        "修仙万载归来，弹指遮天",
			CoverUrl:        "https://images.unsplash.com/photo-1514565131-fce0801e5785?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1579783900882-c0d3dad7b119?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.7,
			HotScore:        765000,
			Tags:            "修真,无敌,都市",
			TotalEpisodes:   68,
			UpdatedEpisodes: 68,
			PlayMode:        "direct_video", // 播放源直接播放
			Description:     "渡劫失败重回少年时代，这一世，他不仅要弥补所有前世遗憾，更要将欺辱至亲之人狠狠踩在脚下！",
			Highlights:      "第3集弹指破宗师，第15集横压江南百家！",
		},
		{
			ID:              4,
			Title:           "千亿豪婿",
			Subtitle:        "装穷三年，今日首富身份曝光",
			CoverUrl:        "https://images.unsplash.com/photo-1492691527719-9d1e07e534b4?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1509198397868-475647b2a1e5?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.6,
			HotScore:        692000,
			Tags:            "神豪,爽文,打脸",
			TotalEpisodes:   85,
			UpdatedEpisodes: 85,
			PlayMode:        "web_view", // 外部网页播放模式
			WebUrl:          "https://example.com/drama/rich_groom",
			Description:     "入赘豪门三年任劳任怨，今日家族禁令解除，万亿资产解冻，龙归大海！",
			Highlights:      "第4集随手买下整栋金融大厦打脸岳母！",
		},
		{
			ID:              5,
			Title:           "大夏镇国战神",
			Subtitle:        "国士无双，一人镇守一国国门",
			CoverUrl:        "https://images.unsplash.com/photo-1509198397868-475647b2a1e5?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1514565131-fce0801e5785?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.5,
			HotScore:        654000,
			Tags:            "国战,热血,英雄",
			TotalEpisodes:   90,
			UpdatedEpisodes: 90,
			PlayMode:        "none", // 无播放源/筹备中
			Description:     "大夏第一战神隐退江湖，边境却突生变故，强敌压境！战神披甲再战，誓守家国每一寸山河！",
			Highlights:      "热播转码中，点击一键领取全集网盘资源！",
		},
		{
			ID:              6,
			Title:           "龙门战神",
			Subtitle:        "龙门之主，号令天下莫敢不从",
			CoverUrl:        "https://images.unsplash.com/photo-1563089145-599997674d42?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1534447677768-be436bb09401?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.7,
			HotScore:        812000,
			Tags:            "龙门,暗黑,爽剧",
			TotalEpisodes:   72,
			UpdatedEpisodes: 72,
			PlayMode:        "channels_embedded", // 微信视频号视频内嵌播放模式
			FinderUserName:  "gh_drama_official",
			ChannelsFeedID:  "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
			Description:     "五年前惨遭暗害家破人亡，五年后创立龙门执掌天下权柄强势归来！血债血偿，一个都不放过！",
			Highlights:      "第2集龙门令一出诸王叩拜！",
		},
		{
			ID:              7,
			Title:           "狂飙龙王",
			Subtitle:        "潜龙在渊，一朝狂飙九万里",
			CoverUrl:        "https://images.unsplash.com/photo-1518709268805-4e9042af9f23?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1579783900882-c0d3dad7b119?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.6,
			HotScore:        720000,
			Tags:            "打脸,反转,高能",
			TotalEpisodes:   65,
			UpdatedEpisodes: 65,
			PlayMode:        "channels_video", // 视频号跳转
			ChannelsFeedID:  "export/UzFfdHQ5M1F2cTVXWll4eW1GZz09",
			Description:     "小摊贩竟是隐匿战神，面对恶霸欺凌，这一次不再隐忍！",
			Highlights:      "全程高能无尿点，爽感拉满！",
		},
		{
			ID:              8,
			Title:           "无敌天尊在都市",
			Subtitle:        "天尊出关，横推八荒六合",
			CoverUrl:        "https://images.unsplash.com/photo-1579783900882-c0d3dad7b119?w=600&auto=format&fit=crop&q=80",
			BannerUrl:       "https://images.unsplash.com/photo-1514565131-fce0801e5785?w=1200&auto=format&fit=crop&q=80",
			Rating:          9.9,
			HotScore:        935000,
			Tags:            "无敌,修真,爽剧",
			TotalEpisodes:   88,
			UpdatedEpisodes: 88,
			PlayMode:        "direct_video", // 播放源播放
			Description:     "闭关万载的天尊降临现代都市，随手炼丹救活千亿老总，抬手御雷惩戒豪门纨绔！",
			Highlights:      "第1集直接召唤天雷击碎豪门宗师！",
		},
	}...)

	return list
}

// generateEpisodesForDrama 针对画廊中的任意短剧快速生成选集列表
func (s *DramaService) generateEpisodesForDrama(drama *models.Drama) []*models.DramaEpisode {
	sampleVideos := []string{
		"https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4",
		"https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerEscapes.mp4",
		"https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerFun.mp4",
	}

	total := drama.TotalEpisodes
	if total <= 0 {
		total = 60
	}
	if total > 80 {
		total = 80
	}

	var episodes []*models.DramaEpisode
	for i := 1; i <= total; i++ {
		isFree := i <= 3
		videoUrl := ""
		if drama.PlayMode == "direct_video" && isFree {
			videoUrl = sampleVideos[(i-1)%len(sampleVideos)]
		}

		episodes = append(episodes, &models.DramaEpisode{
			ID:             int64(drama.ID*1000 + int64(i)),
			DramaID:        drama.ID,
			EpisodeNum:     i,
			Title:          fmt.Sprintf("第%d集: %s", i, getEpisodeTitle(i)),
			CoverUrl:       drama.CoverUrl,
			VideoUrl:       videoUrl,
			IsFree:         isFree,
			Duration:       120,
			PlayMode:       drama.PlayMode,
			FinderUserName: drama.FinderUserName,
			ChannelsFeedID: drama.ChannelsFeedID,
			WebUrl:         drama.WebUrl,
		})
	}
	return episodes
}
