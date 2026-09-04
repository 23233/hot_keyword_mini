// Package services share_card.go
package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	"os"
	"strconv"
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
// 严格检查页面的发布状态、下架与过期时效，杜绝继续暴露失效或私密内容
// cardType: "app_message" (5:4 比例, 1000x800) 或 "timeline" (1:1 比例, 800x800)
func (s *ShareCardService) RenderShareCard(appID, pageID, cardType string) ([]byte, error) {
	var page *models.DynamicPage
	if db.Mysql != nil {
		p, err := s.sduiService.GetRawPage(appID, pageID)
		if err == nil {
			page = p
		}
	}

	// 1. 安全合规门禁检查: 若页面处于非发布状态(草稿/归档/下架)或已过有效期，渲染安全失效兜底卡片
	if page != nil {
		isExpired := page.ExpiresAt != nil && time.Now().After(*page.ExpiresAt)
		isUnpublished := page.Status != "published"
		if isExpired || isUnpublished {
			reason := "内容已下线"
			if isExpired {
				reason = "活动已结束"
			}
			fallbackPage := &models.DynamicPage{
				AppID:        appID,
				PageID:       pageID,
				Title:        fmt.Sprintf("抱歉，当前%s", reason),
				BusinessType: "custom",
				Keyword:      reason,
				Theme:        "dark_glass",
			}
			return s.RenderShareCardFromPage(fallbackPage, cardType)
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

// RenderDraftShareCard 根据草稿实体动态渲染图片 (供 AI MCP 与管理后台草稿预览专用)
func (s *ShareCardService) RenderDraftShareCard(appID, pageID, cardType string) ([]byte, error) {
	draft, err := s.sduiService.GetRawDraft(appID, pageID)
	if err != nil {
		return nil, err
	}

	tempPage := &models.DynamicPage{
		AppID:        draft.AppID,
		PageID:       draft.PageID,
		Title:        draft.Title,
		BusinessType: draft.BusinessType,
		Keyword:      draft.Keyword,
		Theme:        draft.Theme,
		AccentColor:  draft.AccentColor,
		Blocks:       draft.Blocks,
		ShareConfig:  draft.ShareConfig,
	}

	return s.RenderShareCardFromPage(tempPage, cardType)
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
	drawCenterPosterPlaceholder(img, width, height, page)

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

// 辅助绘图函数: 中心视觉大卡片 (消费真实 Blocks 数据绘制组件图层)
func drawCenterPosterPlaceholder(img *image.RGBA, w, h int, page *models.DynamicPage) {
	cardX := 80
	cardY := 160
	cardW := w - 160
	cardH := h - 340

	// 1. 磨砂大卡片容器底板
	cardBg := color.RGBA{R: 255, G: 255, B: 255, A: 16}
	draw.Draw(img, image.Rect(cardX, cardY, cardX+cardW, cardY+cardH), &image.Uniform{C: cardBg}, image.Point{}, draw.Over)

	// 2. 卡片金橙色顶部高光饰条
	stripH := 6
	stripCol := color.RGBA{R: 255, G: 159, B: 10, A: 230}
	draw.Draw(img, image.Rect(cardX, cardY, cardX+cardW, cardY+stripH), &image.Uniform{C: stripCol}, image.Point{}, draw.Over)

	// 3. 边框微光
	borderCol := color.RGBA{R: 255, G: 255, B: 255, A: 30}
	for x := cardX; x < cardX+cardW; x++ {
		img.SetRGBA(x, cardY+cardH-1, borderCol)
	}
	for y := cardY; y < cardY+cardH; y++ {
		img.SetRGBA(cardX, y, borderCol)
		img.SetRGBA(cardX+cardW-1, y, borderCol)
	}

	// 4. 解析真实 Blocks
	var blocks []models.BlockItem
	if page.Blocks != "" {
		_ = json.Unmarshal([]byte(page.Blocks), &blocks)
	}

	// 5. 绘制卡片内标题文字
	titleText := page.Title
	if titleText == "" {
		titleText = "SDUI Dynamic Engine"
	}
	drawBitmapString(img, cardX+32, cardY+36, titleText, color.RGBA{R: 255, G: 255, B: 255, A: 240}, 3)

	// 6. 消费 Blocks 绘制同构积木视觉切片
	blockOffsetY := cardY + 90
	for i, b := range blocks {
		if i >= 4 || blockOffsetY+50 > cardY+cardH {
			break
		}

		// 绘制积木胶囊行
		sliceH := 46
		sliceBg := color.RGBA{R: 255, G: 255, B: 255, A: 22}
		draw.Draw(img, image.Rect(cardX+32, blockOffsetY, cardX+cardW-32, blockOffsetY+sliceH), &image.Uniform{C: sliceBg}, image.Point{}, draw.Over)

		// 绘制积木类型标识色块
		typeCol := color.RGBA{R: 255, G: 159, B: 10, A: 200}
		if b.Type == "media_hero" {
			typeCol = color.RGBA{R: 10, G: 132, B: 255, A: 220}
		} else if b.Type == "game_card" {
			typeCol = color.RGBA{R: 48, G: 209, B: 88, A: 220}
		} else if b.Type == "action_button" {
			typeCol = color.RGBA{R: 255, G: 55, B: 95, A: 220}
		}
		draw.Draw(img, image.Rect(cardX+32, blockOffsetY, cardX+40, blockOffsetY+sliceH), &image.Uniform{C: typeCol}, image.Point{}, draw.Over)

		// 绘制积木类型和摘要文字
		desc := fmt.Sprintf("[%s] %s", b.Type, b.ID)
		if t, ok := b.Props["title"].(string); ok && t != "" {
			desc = fmt.Sprintf("[%s] %s", b.Type, t)
		}
		drawBitmapString(img, cardX+52, blockOffsetY+14, desc, color.RGBA{R: 255, G: 255, B: 255, A: 200}, 2)

		blockOffsetY += sliceH + 14
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

	// 绘制 CTA 文本
	textW := len(ctaText) * 6 * 3
	startX := bannerX + (bannerW-textW)/2
	if startX < bannerX+20 {
		startX = bannerX + 20
	}
	drawBitmapString(img, startX, bannerY+28, ctaText, color.RGBA{R: 255, G: 255, B: 255, A: 255}, 3)
}

// 5x7 点阵字体简化字模 (包含数字、大写英文字母与常用标点)
var basic5x7 = map[byte][5]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00},
	'!': {0x00, 0x00, 0x5F, 0x00, 0x00},
	'-': {0x08, 0x08, 0x08, 0x08, 0x08},
	'_': {0x80, 0x80, 0x80, 0x80, 0x80},
	':': {0x00, 0x36, 0x36, 0x00, 0x00},
	'[': {0x00, 0x7F, 0x41, 0x41, 0x00},
	']': {0x00, 0x41, 0x41, 0x7F, 0x00},
	'0': {0x3E, 0x51, 0x49, 0x45, 0x3E},
	'1': {0x00, 0x42, 0x7F, 0x40, 0x00},
	'2': {0x42, 0x61, 0x51, 0x49, 0x46},
	'3': {0x21, 0x41, 0x45, 0x4B, 0x31},
	'4': {0x18, 0x14, 0x12, 0x7F, 0x10},
	'5': {0x27, 0x45, 0x45, 0x45, 0x39},
	'6': {0x3C, 0x4A, 0x49, 0x49, 0x30},
	'7': {0x01, 0x71, 0x09, 0x05, 0x03},
	'8': {0x36, 0x49, 0x49, 0x49, 0x36},
	'9': {0x06, 0x49, 0x49, 0x29, 0x1E},
	'A': {0x7E, 0x11, 0x11, 0x11, 0x7E},
	'B': {0x7F, 0x49, 0x49, 0x49, 0x36},
	'C': {0x3E, 0x41, 0x41, 0x41, 0x22},
	'D': {0x7F, 0x41, 0x41, 0x22, 0x1C},
	'E': {0x7F, 0x49, 0x49, 0x49, 0x41},
	'F': {0x7F, 0x09, 0x09, 0x09, 0x01},
	'G': {0x3E, 0x41, 0x49, 0x49, 0x7A},
	'H': {0x7F, 0x08, 0x08, 0x08, 0x7F},
	'I': {0x00, 0x41, 0x7F, 0x41, 0x00},
	'J': {0x20, 0x40, 0x41, 0x3F, 0x01},
	'K': {0x7F, 0x08, 0x14, 0x22, 0x41},
	'L': {0x7F, 0x40, 0x40, 0x40, 0x40},
	'M': {0x7F, 0x02, 0x0C, 0x02, 0x7F},
	'N': {0x7F, 0x04, 0x08, 0x10, 0x7F},
	'O': {0x3E, 0x41, 0x41, 0x41, 0x3E},
	'P': {0x7F, 0x09, 0x09, 0x09, 0x06},
	'Q': {0x3E, 0x41, 0x51, 0x21, 0x5E},
	'R': {0x7F, 0x09, 0x19, 0x29, 0x46},
	'S': {0x46, 0x49, 0x49, 0x49, 0x31},
	'T': {0x01, 0x01, 0x7F, 0x01, 0x01},
	'U': {0x3F, 0x40, 0x40, 0x40, 0x3F},
	'V': {0x1F, 0x20, 0x40, 0x20, 0x1F},
	'W': {0x7F, 0x20, 0x18, 0x20, 0x7F},
	'X': {0x63, 0x14, 0x08, 0x14, 0x63},
	'Y': {0x07, 0x08, 0x70, 0x08, 0x07},
	'Z': {0x61, 0x51, 0x49, 0x45, 0x43},
}

// drawBitmapString 利用点阵字模与中文字符支持将字符串绘制到图像缓冲区
func drawBitmapString(img *image.RGBA, x, y int, text string, col color.RGBA, scale int) {
	currX := x
	runes := []rune(text)
	for _, r := range runes {
		if r <= 127 {
			ch := byte(r)
			if ch >= 'a' && ch <= 'z' {
				ch = ch - 'a' + 'A'
			}
			glyph, exists := basic5x7[ch]
			if !exists {
				glyph = basic5x7[' ']
			}

			for colIdx := 0; colIdx < 5; colIdx++ {
				bits := glyph[colIdx]
				for rowIdx := 0; rowIdx < 7; rowIdx++ {
					if (bits & (1 << rowIdx)) != 0 {
						for sx := 0; sx < scale; sx++ {
							for sy := 0; sy < scale; sy++ {
								px := currX + colIdx*scale + sx
								py := y + rowIdx*scale + sy
								if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
									img.SetRGBA(px, py, col)
								}
							}
						}
					}
				}
			}
			currX += 6 * scale
		} else {
			// 中文字符全角点阵渲染 (7x7 带字模特征的结构点阵，杜绝空白)
			hash := uint32(r)
			for colIdx := 0; colIdx < 7; colIdx++ {
				for rowIdx := 0; rowIdx < 7; rowIdx++ {
					isPixel := false
					if rowIdx == 0 || rowIdx == 6 || colIdx == 0 || colIdx == 6 {
						if !((rowIdx == 0 || rowIdx == 6) && (colIdx == 0 || colIdx == 6)) {
							isPixel = true
						}
					} else {
						patternBit := ((hash >> ((rowIdx*3 + colIdx) % 16)) & 1) == 1
						if rowIdx == 3 || colIdx == 3 || patternBit {
							isPixel = true
						}
					}

					if isPixel {
						for sx := 0; sx < scale; sx++ {
							for sy := 0; sy < scale; sy++ {
								px := currX + colIdx*scale + sx
								py := y + rowIdx*scale + sy
								if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
									img.SetRGBA(px, py, col)
								}
							}
						}
					}
				}
			}
			currX += 8 * scale
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

// RenderLayoutIRScreenshot 消费服务端同构中间表示 PageLayoutIR 渲染像素级同构视觉基线截图
func (s *ShareCardService) RenderLayoutIRScreenshot(ir *PageLayoutIR) ([]byte, error) {
	if ir == nil {
		return nil, errors.New("PageLayoutIR 不能为空")
	}

	w := ir.Device.Width
	if w <= 0 {
		w = 390
	}
	h := ir.TotalHeight
	if h < ir.Device.Height {
		h = ir.Device.Height
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// 1. 绘制深黑磨砂底色
	drawBackgroundGradient(img, w, h)

	// 2. 绘制顶部动态导航栏基线，与目标设备安全区保持一致
	navH := deviceTopInset(ir.Device)
	draw.Draw(img, image.Rect(0, 0, w, navH), &image.Uniform{C: color.RGBA{R: 28, G: 28, B: 30, A: 240}}, image.Point{}, draw.Over)
	drawBitmapString(img, 20, navH/2-8, "DYNAMIC SDUI PREVIEW", color.RGBA{R: 255, G: 255, B: 255, A: 200}, 2)

	// 3. 逐个渲染 Layout IR 节点（包含递归子节点）
	for _, node := range flattenLayoutNodes(ir.Nodes) {
		if !node.Visible {
			continue
		}

		bx := node.BoundingBox.X
		by := node.BoundingBox.Y
		bw := node.BoundingBox.Width
		bh := node.BoundingBox.Height

		// 针对不同积木类型应用同构苹果 HIG 视觉样式
		switch node.Type {
		case "action_button":
			// 苹果高光主按钮胶囊底色 (根据强调色填充)
			btnBg := color.RGBA{R: 255, G: 159, B: 10, A: 235}
			if strings.HasPrefix(node.AccentColor, "#") && len(node.AccentColor) == 7 {
				btnBg = parseHexColor(node.AccentColor, 235)
			}
			draw.Draw(img, image.Rect(bx, by, bx+bw, by+bh), &image.Uniform{C: btnBg}, image.Point{}, draw.Over)
			btnText := node.TextSummary
			if btnText == "" {
				btnText = "立即操作"
			}
			textX := bx + (bw-len(btnText)*12)/2
			if textX < bx+16 {
				textX = bx + 16
			}
			drawBitmapString(img, textX, by+(bh-16)/2, btnText, color.RGBA{R: 0, G: 0, B: 0, A: 255}, 2)

		case "image":
			// 通用图片块：绘制带微妙边框的深灰底板与图片标识
			draw.Draw(img, image.Rect(bx, by, bx+bw, by+bh), &image.Uniform{C: color.RGBA{R: 35, G: 35, B: 38, A: 220}}, image.Point{}, draw.Over)
			drawBitmapString(img, bx+12, by+10, "[IMAGE]", color.RGBA{R: 10, G: 132, B: 255, A: 240}, 1)
			imgText := node.TextSummary
			if imgText == "" || imgText == "image" {
				if imgUrl, ok := node.Props["image_url"].(string); ok {
					imgText = imgUrl
				}
			}
			if imgText != "" {
				drawBitmapString(img, bx+12, by+bh/2-8, imgText, color.RGBA{R: 200, G: 200, B: 205, A: 230}, 2)
			}

		case "media_hero":
			// 媒体大焦点：绘制海报底色与原生视频播放替身
			draw.Draw(img, image.Rect(bx, by, bx+bw, by+bh), &image.Uniform{C: color.RGBA{R: 21, G: 21, B: 24, A: 240}}, image.Point{}, draw.Over)
			// 顶部微标签
			drawBitmapString(img, bx+12, by+10, "[MEDIA HERO]", color.RGBA{R: 255, G: 159, B: 10, A: 240}, 1)
			if node.TextSummary != "" {
				drawBitmapString(img, bx+12, by+bh-44, node.TextSummary, color.RGBA{R: 255, G: 255, B: 255, A: 240}, 2)
			}
			if node.NativeStub != "" {
				drawBitmapString(img, bx+12, by+bh-20, "NATIVE: 视频号原生剧场", color.RGBA{R: 48, G: 209, B: 88, A: 220}, 1)
			}

		default:
			// 通用卡片底板 (毛玻璃磨砂效果)
			cardBg := color.RGBA{R: 255, G: 255, B: 255, A: 18}
			if node.GlassBlur {
				cardBg = color.RGBA{R: 255, G: 255, B: 255, A: 24}
			}
			draw.Draw(img, image.Rect(bx, by, bx+bw, by+bh), &image.Uniform{C: cardBg}, image.Point{}, draw.Over)

			// 边框描边
			borderCol := color.RGBA{R: 255, G: 255, B: 255, A: 32}
			for x := bx; x < bx+bw; x++ {
				img.SetRGBA(x, by, borderCol)
				img.SetRGBA(x, by+bh-1, borderCol)
			}
			for y := by; y < by+bh; y++ {
				img.SetRGBA(bx, y, borderCol)
				img.SetRGBA(bx+bw-1, y, borderCol)
			}

			// 顶部类型微标签
			tagText := fmt.Sprintf("[%s]", strings.ToUpper(node.Type))
			tagCol := color.RGBA{R: 255, G: 159, B: 10, A: 220}
			if node.Type == "notice" {
				tagCol = color.RGBA{R: 255, G: 214, B: 10, A: 220}
			}
			drawBitmapString(img, bx+12, by+10, tagText, tagCol, 1)

			// 文本摘要与原生替身提示
			if node.TextSummary != "" {
				drawBitmapString(img, bx+12, by+26, node.TextSummary, color.RGBA{R: 255, G: 255, B: 255, A: 230}, 2)
			}
			if node.NativeStub != "" {
				stubDesc := fmt.Sprintf("NATIVE STUB: %s", node.NativeStub)
				drawBitmapString(img, bx+12, by+bh-20, stubDesc, color.RGBA{R: 48, G: 209, B: 88, A: 200}, 1)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("PNG 编码失败: %w", err)
	}

	return buf.Bytes(), nil
}

// flattenLayoutNodes 将递归布局树按父子顺序展开，供截图渲染器复用统一节点语义。
func flattenLayoutNodes(nodes []BlockLayoutNode) []BlockLayoutNode {
	result := make([]BlockLayoutNode, 0, len(nodes))
	var visit func([]BlockLayoutNode)
	visit = func(current []BlockLayoutNode) {
		for _, node := range current {
			result = append(result, node)
			visit(node.Children)
		}
	}
	visit(nodes)
	return result
}

// GenerateScreenshotSignature 为草稿截图生成有时效的 HMAC-SHA256 安全签名
func GenerateScreenshotSignature(appID, pageID, hash string, expires int64) string {
	secret := os.Getenv("ADMIN_JWT_SECRET")
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		secret = "sdui_screenshot_signature_salt_2026"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	raw := fmt.Sprintf("%s:%s:%s:%d", appID, pageID, hash, expires)
	mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateScreenshotSignature 校验草稿截图签名的合法性与时效 (过期或签名篡改返回 false)
func ValidateScreenshotSignature(appID, pageID, hash string, expires int64, sign string) bool {
	if sign == "" || expires <= 0 {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	expected := GenerateScreenshotSignature(appID, pageID, hash, expires)
	return hmac.Equal([]byte(sign), []byte(expected))
}

// parseHexColor 解析十六进制颜色字符串 (如 "#FF9F0A")
func parseHexColor(hexStr string, alpha uint8) color.RGBA {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) == 6 {
		r, _ := strconv.ParseUint(hexStr[0:2], 16, 8)
		g, _ := strconv.ParseUint(hexStr[2:4], 16, 8)
		b, _ := strconv.ParseUint(hexStr[4:6], 16, 8)
		return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: alpha}
	}
	return color.RGBA{R: 255, G: 159, B: 10, A: alpha}
}
