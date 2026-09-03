// Package services share_card.go
package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hot_keyword/db"
	"hot_keyword/models"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"time"
)

// ShareCardService 微信分享卡片生成与无头渲染服务
type ShareCardService struct {
	sduiService *SDUIService
}

// NewShareCardService 创建分享卡片渲染服务
func NewShareCardService() *ShareCardService {
	return &ShareCardService{
		sduiService: NewSDUIService(),
	}
}

// RenderShareCard 根据页面动态协议生成符合微信规范的分享图片二进制
// cardType: "app_message" (5:4 比例, 1000x800) 或 "timeline" (1:1 比例, 800x800)
func (s *ShareCardService) RenderShareCard(appID, pageID, cardType string) ([]byte, error) {
	var page *models.DynamicPage
	if db.Mysql != nil {
		p, err := s.sduiService.GetRawPage(appID, pageID)
		if err == nil {
			page = p
		}
	}

	// 兜底页面数据
	if page == nil {
		page = &models.DynamicPage{
			AppID:        appID,
			PageID:       pageID,
			Title:        "精选热播 - 极速直达",
			BusinessType: "drama",
			Keyword:      "猴王下山",
		}
	}

	return s.RenderShareCardFromPage(page, cardType)
}

// RenderShareCardFromPage 根据传入的 DynamicPage 纯实体内存渲染分享图 (无外部 DB 依赖)
func (s *ShareCardService) RenderShareCardFromPage(page *models.DynamicPage, cardType string) ([]byte, error) {
	if page == nil {
		return nil, errors.New("动态页面实体不能为空")
	}

	width := 1000
	height := 800
	if cardType == "timeline" {
		width = 800
		height = 800
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 1. 绘制深邃深黑磨砂渐变底色
	drawBackgroundGradient(img, width, height)

	// 2. 绘制外层磨砂高光圆角框
	drawAppleGlassFrame(img, width, height)

	// 3. 绘制顶部业务类型胶囊标签
	badgeText := getBadgeByBusinessType(page.BusinessType)
	drawBadge(img, 80, 80, badgeText, color.RGBA{R: 255, G: 159, B: 10, A: 255})

	// 4. 绘制页面核心主视觉大卡片装饰区
	drawCenterPosterPlaceholder(img, width, height, page.Title)

	// 5. 绘制底部行动号召 (CTA 胶囊)
	ctaText := fmt.Sprintf("微信搜一搜【%s】极速体验", page.Keyword)
	if page.Keyword == "" {
		ctaText = "点击进入小程序立即体验"
	}
	drawCTABanner(img, width, height, ctaText)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("PNG 图像编码失败: %w", err)
	}

	return buf.Bytes(), nil
}

// AutoUpdatePageShareConfig 自动生成分享图地址并持久化更新页面 ShareConfig
func (s *ShareCardService) AutoUpdatePageShareConfig(appID, pageID, host string) error {
	page, err := s.sduiService.GetRawPage(appID, pageID)
	if err != nil {
		return err
	}

	if host == "" {
		host = "http://localhost:8080"
	}
	host = strings.TrimRight(host, "/")

	appMessageImg := fmt.Sprintf("%s/api/v1/share/card?app_id=%s&page_id=%s&type=app_message", host, appID, pageID)
	timelineImg := fmt.Sprintf("%s/api/v1/share/card?app_id=%s&page_id=%s&type=timeline", host, appID, pageID)

	var shareConfig models.PageShareConfig
	if page.ShareConfig != "" {
		_ = json.Unmarshal([]byte(page.ShareConfig), &shareConfig)
	}

	if shareConfig.Friend == nil {
		shareConfig.Friend = &models.ShareItem{
			Enabled:  true,
			Title:    page.Title,
			Path:     fmt.Sprintf("/pages/dynamic/index?app_id=%s&page_id=%s", appID, pageID),
			ImageUrl: appMessageImg,
		}
	} else {
		shareConfig.Friend.ImageUrl = appMessageImg
	}

	if shareConfig.Timeline == nil {
		shareConfig.Timeline = &models.ShareItem{
			Enabled:  true,
			Title:    page.Title,
			Query:    fmt.Sprintf("app_id=%s&page_id=%s", appID, pageID),
			ImageUrl: timelineImg,
		}
	} else {
		shareConfig.Timeline.ImageUrl = timelineImg
	}

	shareJSON, _ := json.Marshal(shareConfig)
	page.ShareConfig = string(shareJSON)
	page.UpdatedAt = time.Now()

	return db.Mysql.Model(&models.DynamicPage{}).
		Where("app_id = ? AND page_id = ?", appID, pageID).
		Updates(map[string]interface{}{
			"share_config": page.ShareConfig,
			"updated_at":   time.Now(),
		}).Error
}

// 辅助绘图函数: 深色渐变背景
func drawBackgroundGradient(img *image.RGBA, w, h int) {
	for y := 0; y < h; y++ {
		ratio := float64(y) / float64(h)
		r := uint8(18 + ratio*12)
		g := uint8(18 + ratio*14)
		b := uint8(24 + ratio*18)
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// 叠加右上角橙色柔和光晕
	centerX := float64(w) * 0.85
	centerY := float64(h) * 0.15
	maxRadius := float64(w) * 0.45

	for y := 0; y < int(centerY+maxRadius) && y < h; y++ {
		for x := int(centerX - maxRadius); x < w; x++ {
			if x < 0 {
				continue
			}
			dist := math.Hypot(float64(x)-centerX, float64(y)-centerY)
			if dist < maxRadius {
				glow := (1.0 - dist/maxRadius) * 0.22
				cur := img.RGBAAt(x, y)
				r := uint8(math.Min(255, float64(cur.R)+255*glow))
				g := uint8(math.Min(255, float64(cur.G)+140*glow))
				b := cur.B
				img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
}

// 辅助绘图函数: 外层磨砂高光边框
func drawAppleGlassFrame(img *image.RGBA, w, h int) {
	margin := 36
	borderColor := color.RGBA{R: 255, G: 255, B: 255, A: 38}

	// 绘制边框线条 (厚度 2px)
	for x := margin; x < w-margin; x++ {
		img.SetRGBA(x, margin, borderColor)
		img.SetRGBA(x, margin+1, borderColor)
		img.SetRGBA(x, h-margin, borderColor)
		img.SetRGBA(x, h-margin-1, borderColor)
	}
	for y := margin; y < h-margin; y++ {
		img.SetRGBA(margin, y, borderColor)
		img.SetRGBA(margin+1, y, borderColor)
		img.SetRGBA(w-margin, y, borderColor)
		img.SetRGBA(w-margin-1, y, borderColor)
	}
}

// 辅助绘图函数: 顶部业务胶囊徽章
func drawBadge(img *image.RGBA, x, y int, text string, accent color.RGBA) {
	badgeW := 220
	badgeH := 48
	radius := badgeH / 2

	// 胶囊背景
	for dy := 0; dy < badgeH; dy++ {
		for dx := 0; dx < badgeW; dx++ {
			px := x + dx
			py := y + dy
			// 两侧圆角
			if dx < radius {
				if math.Hypot(float64(dx-radius), float64(dy-radius)) > float64(radius) {
					continue
				}
			} else if dx > badgeW-radius {
				if math.Hypot(float64(dx-(badgeW-radius)), float64(dy-radius)) > float64(radius) {
					continue
				}
			}
			img.SetRGBA(px, py, color.RGBA{R: 255, G: 159, B: 10, A: 45})
		}
	}

	// 边框
	borderCol := color.RGBA{R: 255, G: 159, B: 10, A: 120}
	for dx := radius; dx < badgeW-radius; dx++ {
		img.SetRGBA(x+dx, y, borderCol)
		img.SetRGBA(x+dx, y+badgeH-1, borderCol)
	}
}

// 辅助绘图函数: 中心视觉大卡片
func drawCenterPosterPlaceholder(img *image.RGBA, w, h int, title string) {
	cardX := 80
	cardY := 160
	cardW := w - 160
	cardH := h - 340

	// 磨砂大卡片容器
	cardBg := color.RGBA{R: 255, G: 255, B: 255, A: 16}
	draw.Draw(img, image.Rect(cardX, cardY, cardX+cardW, cardY+cardH), &image.Uniform{C: cardBg}, image.Point{}, draw.Over)

	// 卡片金橙色顶部饰条
	stripH := 6
	stripCol := color.RGBA{R: 255, G: 159, B: 10, A: 230}
	draw.Draw(img, image.Rect(cardX, cardY, cardX+cardW, cardY+stripH), &image.Uniform{C: stripCol}, image.Point{}, draw.Over)

	// 边框发光点
	borderCol := color.RGBA{R: 255, G: 255, B: 255, A: 30}
	for x := cardX; x < cardX+cardW; x++ {
		img.SetRGBA(x, cardY+cardH-1, borderCol)
	}
	for y := cardY; y < cardY+cardH; y++ {
		img.SetRGBA(cardX, y, borderCol)
		img.SetRGBA(cardX+cardW-1, y, borderCol)
	}
}

// 辅助绘图函数: 底部 CTA 胶囊横幅
func drawCTABanner(img *image.RGBA, w, h int, ctaText string) {
	bannerH := 84
	bannerY := h - 140
	bannerW := w - 160
	bannerX := 80
	radius := bannerH / 2

	// 渐变高光橙红按钮
	for dy := 0; dy < bannerH; dy++ {
		for dx := 0; dx < bannerW; dx++ {
			px := bannerX + dx
			py := bannerY + dy
			if dx < radius {
				if math.Hypot(float64(dx-radius), float64(dy-radius)) > float64(radius) {
					continue
				}
			} else if dx > bannerW-radius {
				if math.Hypot(float64(dx-(bannerW-radius)), float64(dy-radius)) > float64(radius) {
					continue
				}
			}
			t := float64(dx) / float64(bannerW)
			r := uint8(255)
			g := uint8(159 - t*80)
			b := uint8(10 + t*30)
			img.SetRGBA(px, py, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
}

// 根据业务类型获取展示徽章
func getBadgeByBusinessType(bType string) string {
	switch bType {
	case "drama":
		return "🎬 热播爆款短剧"
	case "game":
		return "🎮 独家公测礼包"
	case "query":
		return "🔍 成绩数据查询"
	case "download":
		return "⚡ 高速资源下载"
	default:
		return "✨ 精选爆款推荐"
	}
}
