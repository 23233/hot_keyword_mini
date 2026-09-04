// LayoutBlocks.tsx
import React, { useEffect, useState } from 'react'
import { View, Text, Swiper, SwiperItem } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface LayoutBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: Record<string, any>
  renderBlock?: (child: BlockItem, childContext?: Record<string, any>) => React.ReactNode
}

/**
  * 通用弹性容器/堆叠积木 (Stack / Container)
  */
export const ContainerBlock: React.FC<LayoutBlockProps> = ({ block, context, renderBlock }) => {
  const props = block.props || {}
  const direction = props.direction || props.flex_direction || 'column'
  const gap = props.gap || '16rpx'
  const align = props.align || props.align_items || 'stretch'
  const justify = props.justify || props.justify_content || 'flex-start'
  const children = (props.children || props.items || props.blocks) as BlockItem[]

  const flexStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: direction,
    gap: gap,
    alignItems: align,
    justifyContent: justify,
    width: '100%'
  }

  return (
    <View className="sdui-container-block" style={flexStyle}>
      {Array.isArray(children) &&
        children.map((child, idx) => (
          <View key={child.id || `child_${idx}`} className="sdui-container-item" style={{ flex: props.item_flex || undefined }}>
            {renderBlock ? renderBlock(child, context) : null}
          </View>
        ))}
    </View>
  )
}

/**
  * 通用网格布局积木 (GridBlock)
  */
export const GridBlock: React.FC<LayoutBlockProps> = ({ block, context, renderBlock }) => {
  const props = block.props || {}
  const configuredColumns = Number(props.columns)
  const columns = Number.isFinite(configuredColumns) ? Math.min(4, Math.max(1, Math.floor(configuredColumns))) : 2
  const gap = props.gap || '16rpx'
  const children = (props.children || props.items || props.blocks) as BlockItem[]

  const gridStyle: React.CSSProperties = {
    display: 'grid',
    gridTemplateColumns: `repeat(${columns}, 1fr)`,
    gap: gap,
    width: '100%'
  }

  return (
    <View className="sdui-grid-layout-block" style={gridStyle}>
      {Array.isArray(children) &&
        children.map((child, idx) => (
          <View key={child.id || `grid_item_${idx}`} className="sdui-grid-cell">
            {renderBlock ? renderBlock(child, context) : null}
          </View>
        ))}
    </View>
  )
}

/**
  * 通用选项卡切换积木 (TabsBlock)
  */
export const TabsBlock: React.FC<LayoutBlockProps> = ({ block, context, renderBlock }) => {
  const props = block.props || {}
  const tabs = (props.tabs || props.items) as Array<{ key: string; title: string; blocks?: BlockItem[]; child?: BlockItem }>
  const defaultKey = props.default_active_key != null
    ? String(props.default_active_key)
    : (Array.isArray(tabs) && tabs[0]?.key != null ? String(tabs[0].key) : '0')
  const tabSignature = Array.isArray(tabs) ? tabs.map((tab, idx) => String(tab.key || idx)).join('|') : ''
  const [activeKey, setActiveKey] = useState<string>(defaultKey)

  useEffect(() => {
    const availableKeys = Array.isArray(tabs) ? tabs.map((tab, idx) => String(tab.key || idx)) : []
    setActiveKey(availableKeys.includes(defaultKey) ? defaultKey : (availableKeys[0] || '0'))
  }, [defaultKey, tabSignature])

  if (!Array.isArray(tabs) || tabs.length === 0) {
    return null
  }

  const activeTab = tabs.find((t, idx) => String(t.key || idx) === activeKey) || tabs[0]
  const activeChildren = activeTab?.blocks || (activeTab?.child ? [activeTab.child] : [])

  return (
    <View className="sdui-tabs-block">
      {/* 顶部苹果磨砂分段胶囊选择器 */}
      <View className="tabs-header-bar">
        {tabs.map((tab, idx) => {
          const key = String(tab.key || idx)
          const isActive = key === activeKey
          return (
            <View
              key={key}
              className={`tab-pill-item ${isActive ? 'is-active' : ''}`}
              onClick={() => setActiveKey(key)}
            >
              <Text className="tab-pill-text">{tab.title}</Text>
            </View>
          )
        })}
      </View>

      {/* 选中的内容块区域 */}
      <View className="tabs-content-body">
        {Array.isArray(activeChildren) &&
          activeChildren.map((child, cIdx) => (
            <React.Fragment key={child.id || `tab_child_${cIdx}`}>
              {renderBlock ? renderBlock(child, context) : null}
            </React.Fragment>
          ))}
      </View>
    </View>
  )
}

/**
  * 通用走马灯轮播积木 (CarouselBlock)
  */
export const CarouselBlock: React.FC<LayoutBlockProps> = ({ block, context, renderBlock }) => {
  const props = block.props || {}
  const items = (props.items || props.children || props.blocks) as BlockItem[]
  const autoplay = props.autoplay !== false
  const configuredInterval = Number(props.interval)
  const configuredDuration = Number(props.duration)
  const interval = Number.isFinite(configuredInterval) ? Math.max(100, configuredInterval) : 3500
  const duration = Number.isFinite(configuredDuration) ? Math.max(0, configuredDuration) : 500
  const indicatorDots = props.indicator !== false
  const height = props.height || '360rpx'

  if (!Array.isArray(items) || items.length === 0) {
    return null
  }

  return (
    <View className="sdui-carousel-block" style={{ height }}>
      <Swiper
        className="carousel-swiper-inner"
        indicatorDots={indicatorDots}
        indicatorColor="rgba(255, 255, 255, 0.3)"
        indicatorActiveColor="#FF9F0A"
        autoplay={autoplay}
        interval={interval}
        duration={duration}
        circular
        style={{ width: '100%', height: '100%' }}
      >
        {items.map((child, idx) => (
          <SwiperItem key={child.id || `carousel_item_${idx}`} className="carousel-swiper-item">
            {renderBlock ? renderBlock(child, context) : null}
          </SwiperItem>
        ))}
      </Swiper>
    </View>
  )
}

/**
  * 通用占位间隙积木 (SpacerBlock)
  */
export const SpacerBlock: React.FC<{ block: BlockItem }> = ({ block }) => {
  const props = block.props || {}
  const height = props.height || '24rpx'
  const width = props.width || '100%'

  return <View className="sdui-spacer-block" style={{ height, width }} />
}
