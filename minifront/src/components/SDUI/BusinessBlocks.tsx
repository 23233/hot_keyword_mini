// minifront/src/components/SDUI/BusinessBlocks.tsx
import React, { useState, useEffect } from 'react'
import { View, Text, Image } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface BusinessBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 战力/积分/评分面板积木 (ScorePanelBlock)
 * 遵循 Apple 健身与游戏中心成就面板设计规范，呈现大字号高光分值、排名徽章与多维属性明细
 */
export const ScorePanelBlock: React.FC<BusinessBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const score = props.score ?? props.total_score ?? '--'
  const title = props.title || '当前战力评分'
  const subtitle = props.subtitle || (props.rank ? `全区排名: ${props.rank}` : '')
  const level = props.level ? `Lv.${props.level}` : (props.badge || '')
  const avatarUrl = props.avatar_url || props.icon_url
  const items = Array.isArray(props.items) ? props.items : []

  const handleClick = (e: any) => {
    e?.stopPropagation?.()
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, score, level })
    }
  }

  return (
    <View className="sdui-score-panel" onClick={handleClick}>
      <View className="score-panel-header">
        <View className="score-panel-meta">
          {avatarUrl && <Image className="score-avatar" src={String(avatarUrl)} mode="aspectFill" />}
          <View>
            <Text className="score-title">{String(title)}</Text>
            {subtitle && <Text className="score-subtitle">{String(subtitle)}</Text>}
          </View>
        </View>
        {level && <Text className="score-level-badge">{String(level)}</Text>}
      </View>

      <View className="score-main-value">
        <Text className="score-number">{String(score)}</Text>
        {props.unit && <Text className="score-unit">{String(props.unit)}</Text>}
      </View>

      {items.length > 0 && (
        <View className="score-stats-grid">
          {items.map((stat: any, idx: number) => (
            <View key={idx} className="score-stat-cell">
              <Text className="stat-label">{String(stat.label || stat.name || `指标${idx + 1}`)}</Text>
              <Text className="stat-val">{String(stat.value ?? stat.val ?? '--')}</Text>
            </View>
          ))}
        </View>
      )}
    </View>
  )
}

/**
 * 卡券与兑换码积木 (CouponCardBlock)
 * 遵循 Apple 钱包票券设计规范，带有虚线/打孔凹槽质感，支持面额展示与一键快捷复制/领取
 */
export const CouponCardBlock: React.FC<BusinessBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '专属优惠特权'
  const desc = props.desc || props.description || '全场通用 · 即领即用'
  const discount = props.discount || props.amount || (props.redeem_code || props.code ? '兑换码' : '特惠')
  const code = props.redeem_code || props.code || ''
  const validUntil = props.valid_until || props.expire_time || props.expires || ''
  const btnText = props.btn_text || (code ? '一键复制' : '立即领取')
  const isUsed = props.status === 'used' || props.status === 'expired'

  const handleCardClick = (e: any) => {
    e?.stopPropagation?.()
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, code, discount })
    }
  }

  const handleBtnClick = (e: any) => {
    e?.stopPropagation?.()
    if (props.btn_action && onAction) {
      onAction(props.btn_action, { item: props, actionPayload: props, code, discount })
    } else if (code && onAction) {
      const codeStr = String(code)
      onAction({
        type: 'copy_text',
        payload: { text: codeStr, toast: `✅ 兑换码 ${codeStr} 已复制！` }
      }, { item: props, code: codeStr })
    } else if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, code, discount })
    }
  }

  return (
    <View className={`sdui-coupon-card ${isUsed ? 'is-disabled' : ''}`} onClick={handleCardClick}>
      <View className="coupon-left">
        <View className="coupon-discount-wrap">
          {props.amount && !props.discount && <Text className="coupon-currency">¥</Text>}
          <Text className="coupon-discount">{String(discount)}</Text>
        </View>
        {props.min_spend && <Text className="coupon-min-spend">{String(props.min_spend)}</Text>}
      </View>

      <View className="coupon-notch-divider">
        <View className="notch-circle notch-top" />
        <View className="notch-line" />
        <View className="notch-circle notch-bottom" />
      </View>

      <View className="coupon-right">
        <View className="coupon-info">
          <Text className="coupon-title">{String(title)}</Text>
          <Text className="coupon-desc">{String(desc)}</Text>
          {validUntil && <Text className="coupon-expire">有效期至: {String(validUntil)}</Text>}
        </View>
        <View className="coupon-action-btn" onClick={handleBtnClick}>
          <Text className="action-text">{String(btnText)}</Text>
        </View>
      </View>
    </View>
  )
}

/**
 * 实时倒计时积木 (CountdownBlock)
 * 采用 Apple 计时器微发光数字胶囊设计，动态计算剩余天、时、分、秒
 */
export const CountdownBlock: React.FC<BusinessBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '活动倒计时'
  const subtitle = props.subtitle || ''
  const expiredText = props.expired_text || '活动已截止'
  const target = props.target_time || props.end_time || props.target

  const [timeLeft, setTimeLeft] = useState<{ d: string; h: string; m: string; s: string; isEnded: boolean }>({
    d: '00',
    h: '00',
    m: '00',
    s: '00',
    isEnded: false
  })

  useEffect(() => {
    if (!target) return

    const calculate = () => {
      let targetMs = 0
      if (typeof target === 'number') {
        targetMs = target < 10000000000 ? target * 1000 : target
      } else {
        targetMs = new Date(String(target)).getTime()
      }

      const diff = targetMs - Date.now()
      if (isNaN(diff) || diff <= 0) {
        setTimeLeft({ d: '00', h: '00', m: '00', s: '00', isEnded: true })
        return
      }

      const d = Math.floor(diff / (1000 * 60 * 60 * 24))
      const h = Math.floor((diff / (1000 * 60 * 60)) % 24)
      const m = Math.floor((diff / (1000 * 60)) % 60)
      const s = Math.floor((diff / 1000) % 60)

      setTimeLeft({
        d: d > 0 ? String(d).padStart(2, '0') : '',
        h: String(h).padStart(2, '0'),
        m: String(m).padStart(2, '0'),
        s: String(s).padStart(2, '0'),
        isEnded: false
      })
    }

    calculate()
    const timer = setInterval(calculate, 1000)
    return () => clearInterval(timer)
  }, [target])

  const handleClick = (e: any) => {
    e?.stopPropagation?.()
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, title, target })
    }
  }

  return (
    <View className="sdui-countdown-block" onClick={handleClick}>
      <View className="countdown-header">
        <Text className="countdown-title">⏱️ {String(title)}</Text>
        {subtitle && <Text className="countdown-subtitle">{String(subtitle)}</Text>}
      </View>

      {timeLeft.isEnded ? (
        <View className="countdown-ended">
          <Text className="ended-text">{String(expiredText)}</Text>
        </View>
      ) : (
        <View className="countdown-digits-row">
          {timeLeft.d && (
            <>
              <View className="digit-box">
                <Text className="digit-num">{timeLeft.d}</Text>
                <Text className="digit-unit">天</Text>
              </View>
              <Text className="digit-sep">:</Text>
            </>
          )}
          <View className="digit-box">
            <Text className="digit-num">{timeLeft.h}</Text>
            <Text className="digit-unit">时</Text>
          </View>
          <Text className="digit-sep">:</Text>
          <View className="digit-box">
            <Text className="digit-num">{timeLeft.m}</Text>
            <Text className="digit-unit">分</Text>
          </View>
          <Text className="digit-sep">:</Text>
          <View className="digit-box">
            <Text className="digit-num">{timeLeft.s}</Text>
            <Text className="digit-unit">秒</Text>
          </View>
        </View>
      )}
    </View>
  )
}

/**
 * 服务节点与状态监控积木 (ServerStatusBlock)
 * 实时展示服务节点健康度、网络延迟与当前服务负载状态 (流畅 / 拥挤 / 爆满 / 维护)
 */
export const ServerStatusBlock: React.FC<BusinessBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const serverName = props.server_name || props.name || props.title || '亚太核心主节点'
  const region = props.region || '亚太华南'
  const latency = props.latency !== undefined ? `${props.latency}ms` : '18ms'
  const status = String(props.status || 'normal').toLowerCase()
  const notice = props.notice || ''
  const btnText = props.btn_text || '切换节点'

  let statusColor = '#30d158'
  let statusText = '运行流畅'
  if (status === 'busy' || status === 'crowded') {
    statusColor = '#ff9f0a'
    statusText = '服务繁忙'
  } else if (status === 'full') {
    statusColor = '#ff453a'
    statusText = '已爆满'
  } else if (status === 'maintenance' || status === 'offline') {
    statusColor = '#8e8e93'
    statusText = '停服维护'
  }

  const handleClick = (e: any) => {
    e?.stopPropagation?.()
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, serverName, status, latency })
    }
  }

  return (
    <View className="sdui-server-status-block" onClick={handleClick}>
      <View className="server-main-info">
        <View className="server-title-row">
          <View className="status-breath-dot" style={{ background: statusColor, boxShadow: `0 0 10rpx ${statusColor}` }} />
          <Text className="server-name">{String(serverName)}</Text>
          <Text className="server-region-tag">{String(region)}</Text>
        </View>
        <View className="server-meta-row">
          <Text className="server-status-label" style={{ color: statusColor }}>{statusText}</Text>
          <Text className="server-meta-divider">·</Text>
          <Text className="server-latency">延迟 {String(latency)}</Text>
        </View>
        {notice && <Text className="server-notice">{String(notice)}</Text>}
      </View>

      <View className="server-action-pill" onClick={handleClick}>
        <Text className="pill-text">{String(btnText)}</Text>
      </View>
    </View>
  )
}

/**
 * 苹果风商品与周边卡片积木 (ProductCardBlock)
 * 采用 Apple Store 零售卡片极简美学，突出产品图像、大字号金额与购买转化按钮
 */
export const ProductCardBlock: React.FC<BusinessBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || props.name || '精品推荐'
  const subtitle = props.subtitle || props.desc || ''
  const imageUrl = props.image_url || props.cover_url || props.src
  const price = props.price !== undefined ? String(props.price) : '9.9'
  const originalPrice = props.original_price ? String(props.original_price) : ''
  const tag = props.tag || props.badge || ''
  const sales = props.sales ? `已售 ${props.sales}` : ''
  const btnText = props.btn_text || '立即购买'

  const sku = props.sku || props.product_sku || props.id || ''

  const handleCardClick = (e: any) => {
    e?.stopPropagation?.()
    const targetAction = block.action || props.card_action
    if ((targetAction || block.events?.tap) && onAction) {
      onAction(targetAction, { item: props, actionPayload: props, title, price, sku })
    }
  }

  const handleBtnClick = (e: any) => {
    e?.stopPropagation?.()
    const btnAct = props.btn_action || props.action
    if (btnAct && onAction) {
      onAction(btnAct, { item: props, actionPayload: props, title, price, sku })
    } else if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, title, price, sku })
    } else if (sku && onAction) {
      onAction({ type: 'request_payment', payload: { sku } }, { item: props, sku })
    }
  }

  return (
    <View className="sdui-product-card" onClick={handleCardClick}>
      {imageUrl && (
        <View className="product-image-wrap">
          <Image className="product-img" src={String(imageUrl)} mode="aspectFill" />
          {tag && <Text className="product-tag">{String(tag)}</Text>}
        </View>
      )}

      <View className="product-content">
        <Text className="product-title">{String(title)}</Text>
        {subtitle && <Text className="product-subtitle">{String(subtitle)}</Text>}

        <View className="product-footer">
          <View className="product-pricing">
            <Text className="price-currency">¥</Text>
            <Text className="price-num">{price}</Text>
            {originalPrice && <Text className="price-original">¥{originalPrice}</Text>}
            {sales && <Text className="product-sales">{sales}</Text>}
          </View>
          <View className="product-buy-btn" onClick={handleBtnClick}>
            <Text className="buy-btn-text">{String(btnText)}</Text>
          </View>
        </View>
      </View>
    </View>
  )
}

/**
 * 经典应用与资源下载卡片积木 (DownloadCardBlock)
 * 采用 App Store 标志性圆角图标与胶囊“获取/下载”按钮，展示版本、包体大小与平台标签
 */
export const DownloadCardBlock: React.FC<BusinessBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const name = props.name || props.title || '官方客户端'
  const iconUrl = props.icon_url || props.image_url || props.logo
  const version = props.version ? `v${props.version}` : ''
  const size = props.size || ''
  const platform = props.platform || '全平台通用'
  const desc = props.desc || props.description || ''
  const btnText = props.btn_text || '获取'

  const handleCardClick = (e: any) => {
    e?.stopPropagation?.()
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, name, version })
    }
  }

  const handleBtnClick = (e: any) => {
    e?.stopPropagation?.()
    if (props.btn_action && onAction) {
      onAction(props.btn_action, { item: props, actionPayload: props, name, version })
    } else if (props.download_url && onAction) {
      onAction({
        type: 'copy_text',
        payload: { text: String(props.download_url), toast: '✅ 下载链接已复制到剪贴板！' }
      }, { item: props, name })
    } else if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, name, version })
    }
  }

  return (
    <View className="sdui-download-card" onClick={handleCardClick}>
      <View className="download-header-row">
        {iconUrl && <Image className="download-app-icon" src={String(iconUrl)} mode="aspectFill" />}
        <View className="download-meta-info">
          <Text className="app-name">{String(name)}</Text>
          <View className="app-sub-specs">
            {version && <Text className="spec-item">{version}</Text>}
            {size && <Text className="spec-item">{String(size)}</Text>}
            <Text className="spec-platform-badge">{String(platform)}</Text>
          </View>
        </View>
        <View className="download-action-pill" onClick={handleBtnClick}>
          <Text className="pill-text">{String(btnText)}</Text>
        </View>
      </View>

      {desc && <Text className="download-desc">{String(desc)}</Text>}
    </View>
  )
}
