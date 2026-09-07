// LayoutBlocks.tsx
import React, { useEffect, useState } from 'react'
import { View, Text, Swiper, SwiperItem } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { resolveBindingValue } from '../../utils/action'

interface LayoutBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: Record<string, any>
  renderBlock?: (child: BlockItem, childContext?: Record<string, any>) => React.ReactNode
}

/**
 * 将容器内声明了 repeat 的子积木扁平化展开为平级兄弟节点
 * 确保网格子项一一对应单元格、Flex 横向子项平铺且具备独立的 item 上下文
 */
function flattenContainerChildren(children?: BlockItem[], context?: Record<string, any>): BlockItem[] {
  if (!Array.isArray(children) || children.length === 0) return []
  const flattened: BlockItem[] = []

  for (let i = 0; i < children.length; i++) {
    const child = children[i]
    if (!child) continue

    if (child.repeat && typeof child.repeat === 'object') {
      let repeatList: any[] = []
      let hasExplicitSource = false

      if (Array.isArray(child.repeat.items)) {
        hasExplicitSource = true
        repeatList = child.repeat.items
      } else if (child.repeat.path) {
        hasExplicitSource = true
        const resolved = resolveBindingValue({ path: child.repeat.path }, context)
        if (Array.isArray(resolved)) {
          repeatList = resolved
        }
      }

      if (repeatList.length > 0) {
        repeatList.forEach((itemData, rIdx) => {
          flattened.push({
            ...child,
            id: `${child.id || 'child'}_${rIdx}`,
            repeat: undefined, // 避免重复进入 BlockRenderer 时再次循环展开
            props: {
              ...(child.props || {}),
              _repeat_item: itemData,
              _repeat_index: rIdx
            }
          })
        })
      } else if (hasExplicitSource && child.empty) {
        flattened.push(child.empty)
      }
    } else {
      flattened.push(child)
    }
  }

  return flattened
}

/**
  * 通用弹性容器/堆叠积木 (Stack / Container)
  * 支持堆叠、横排/竖排对齐、层叠重叠 (Overlap/ZStack)、换行与子积木嵌套组合
  */
export const ContainerBlock: React.FC<LayoutBlockProps> = ({ block, onAction, context, renderBlock }) => {
  const props = block.props || {}
  const isOverlap = props.mode === 'overlap' || props.mode === 'zstack' || props.layout === 'overlap'
  const direction = props.direction || props.flex_direction || 'column'
  const isWrap = props.wrap === 'wrap' || props.flex_wrap === 'wrap' || props.wrap === true
  const gap = props.gap || (isOverlap ? '0' : '16rpx')
  const align = props.align || props.align_items || 'stretch'
  const justify = props.justify || props.justify_content || 'flex-start'
  const rawChildren = (props.children || props.items || props.blocks) as BlockItem[]
  const children = flattenContainerChildren(rawChildren, context)

  // 区分普通弹性流与重叠层叠 (Overlap) 布局规范
  const containerStyle: React.CSSProperties = isOverlap
    ? {
        display: 'grid',
        gridTemplateAreas: "'overlap'",
        position: 'relative',
        width: '100%',
        alignItems: align,
        justifyItems: justify
      }
    : {
        display: 'flex',
        flexDirection: direction,
        flexWrap: isWrap ? 'wrap' : 'nowrap',
        gap: gap,
        alignItems: align,
        justifyContent: justify,
        width: '100%'
      }

  // 容器自身点击交互 (如卡片整体跳转或领取，兼容 props.action 与 props.card_action)
  const containerAction = block.action || props.action || props.card_action
  const handleContainerClick = (e: any) => {
    e?.stopPropagation?.()
    if ((containerAction || block.events?.tap || props.events?.tap) && onAction) {
      onAction(containerAction, context)
    }
  }

  return (
    <View className={`sdui-container-block ${isOverlap ? 'is-overlap' : ''}`} style={containerStyle} onClick={handleContainerClick}>
      {Array.isArray(children) &&
        children.map((child, idx) => {
          const childProps = child.props || {}
          const childStyle = child.style || {}
          const itemAlignSelf = childProps.align_self || (childStyle as any).align_self || undefined
          const itemJustifySelf = childProps.justify_self || (childStyle as any).justify_self || undefined
          const isFloatingItem = isOverlap && (itemAlignSelf || itemJustifySelf || childProps.width)
          const isFullBleed = isOverlap && !isFloatingItem

          // 综合检查子项自身或内部是否具备交互行为
          const hasChildInteraction = !!(
            child.action ||
            child.events ||
            childProps.action ||
            childProps.btn_action ||
            childProps.card_action ||
            childProps.click_action ||
            ['action_button', 'form', 'notice', 'episode_list', 'coupon_card', 'redeem_code_card', 'product_card', 'download_card', 'resource_card', 'game_card'].includes(child.type)
          )

          // 浮动修饰项若无交互行为，设置 pointer-events: none 避免阻挡底层点击
          const pointerEvents = isOverlap
            ? (hasChildInteraction ? 'auto' : 'none')
            : undefined

          const itemStyle: React.CSSProperties = isOverlap
            ? {
                gridArea: 'overlap',
                alignSelf: itemAlignSelf as any,
                justifySelf: itemJustifySelf as any,
                zIndex: (childStyle as any).z_index || idx + 1,
                width: childProps.width || (isFullBleed ? '100%' : undefined),
                height: childProps.height || (isFullBleed ? '100%' : undefined),
                pointerEvents
              }
            : {
                flex: props.item_flex || (childProps as any).item_flex || ((childStyle as any).flex ? String((childStyle as any).flex) : undefined)
              }

          const childCtx = childProps._repeat_item !== undefined
            ? { ...context, isNested: true, index: idx, item: childProps._repeat_item, $item: childProps._repeat_item }
            : { ...context, isNested: true, index: idx }

          return (
            <View
              key={child.id || `child_${idx}`}
              className={`sdui-container-item ${isFullBleed ? 'is-full-bleed' : ''}`}
              style={itemStyle}
              onClick={(e) => {
                // 若子组件具备交互行为，阻止事件向外冒泡至父容器，杜绝双重触发
                if (hasChildInteraction) {
                  e.stopPropagation?.()
                }
              }}
            >
              <View style={{
                width: isFullBleed ? '100%' : (childProps.width || undefined),
                height: isFullBleed ? '100%' : (childProps.height || undefined),
                pointerEvents: isOverlap ? 'auto' : undefined
              }}>
                {renderBlock ? renderBlock(child, childCtx) : null}
              </View>
            </View>
          )
        })}
    </View>
  )
}

/**
  * 通用网格布局积木 (GridBlock)
  * 支持自适应列数、网格单元格渲染与交互动作
  */
export const GridBlock: React.FC<LayoutBlockProps> = ({ block, onAction, context, renderBlock }) => {
  const props = block.props || {}
  const configuredColumns = Number(props.columns)
  const columns = Number.isFinite(configuredColumns) ? Math.min(4, Math.max(1, Math.floor(configuredColumns))) : 2
  const gap = props.gap || '16rpx'
  const align = props.align || props.align_items || undefined
  const justify = props.justify || props.justify_items || undefined
  const rawChildren = (props.children || props.items || props.blocks) as BlockItem[]
  const children = flattenContainerChildren(rawChildren, context)

  const gridStyle: React.CSSProperties = {
    display: 'grid',
    gridTemplateColumns: `repeat(${columns}, 1fr)`,
    gap: gap,
    width: '100%',
    alignItems: align,
    justifyItems: justify
  }

  const gridAction = block.action || props.action || props.card_action
  const handleGridClick = (e: any) => {
    e?.stopPropagation?.()
    if ((gridAction || block.events?.tap || props.events?.tap) && onAction) {
      onAction(gridAction, context)
    }
  }

  return (
    <View className="sdui-grid-layout-block" style={gridStyle} onClick={handleGridClick}>
      {Array.isArray(children) &&
        children.map((child, idx) => {
          const childProps = child.props || {}
          const hasChildInteraction = !!(
            child.action ||
            child.events ||
            childProps.action ||
            childProps.btn_action ||
            childProps.card_action ||
            childProps.click_action ||
            ['action_button', 'form', 'notice', 'episode_list', 'coupon_card', 'redeem_code_card', 'product_card', 'download_card', 'resource_card', 'game_card'].includes(child.type)
          )

          const childCtx = childProps._repeat_item !== undefined
            ? { ...context, isNested: true, index: idx, item: childProps._repeat_item, $item: childProps._repeat_item }
            : { ...context, isNested: true, index: idx }

          return (
            <View
              key={child.id || `grid_item_${idx}`}
              className="sdui-grid-cell"
              onClick={(e) => {
                if (hasChildInteraction) {
                  e.stopPropagation?.()
                }
              }}
            >
              {renderBlock ? renderBlock(child, childCtx) : null}
            </View>
          )
        })}
    </View>
  )
}

/**
  * 通用选项卡切换积木 (TabsBlock)
  * 支持苹果分段选择器、动态切换、IR 解包适配与切换动作事件流
  */
export const TabsBlock: React.FC<LayoutBlockProps> = ({ block, onAction, context, renderBlock }) => {
  const props = block.props || {}
  const rawTabs = props.tabs || props.items
  const tabs = Array.isArray(rawTabs)
    ? (rawTabs as Array<{ key?: string; id?: string; title: string; blocks?: BlockItem[]; child?: BlockItem; children?: BlockItem[] }>)
    : []

  const getTabKey = (tab: any, idx: number) => String(tab?.key != null ? tab.key : (tab?.id != null ? tab.id : idx))

  // 优先支持受控模式 ($state 驱动的 active_key 或 active_tab)
  const controlledKey = props.active_key != null
    ? String(props.active_key)
    : (props.active_tab != null ? String(props.active_tab) : undefined)

  const defaultKey = controlledKey != null
    ? controlledKey
    : (props.default_active_key != null
        ? String(props.default_active_key)
        : (props.default_index != null && tabs[Number(props.default_index)]
            ? getTabKey(tabs[Number(props.default_index)], Number(props.default_index))
            : (tabs[0] ? getTabKey(tabs[0], 0) : '0')))

  const tabSignature = tabs.map(getTabKey).join('|')
  const [activeKey, setActiveKey] = useState<string>(defaultKey)

  useEffect(() => {
    if (controlledKey != null) {
      setActiveKey(controlledKey)
      return
    }
    const availableKeys = tabs.map(getTabKey)
    setActiveKey(availableKeys.includes(defaultKey) ? defaultKey : (availableKeys[0] || '0'))
  }, [controlledKey, defaultKey, tabSignature])

  // 若无 tabs 声明，但存在 props.children（如 LayoutIR 扁平化映射），直接以通用容器方式呈现
  if (tabs.length === 0) {
    const fallbackChildren = (props.children || props.blocks) as BlockItem[]
    if (Array.isArray(fallbackChildren) && fallbackChildren.length > 0) {
      return (
        <View className="sdui-tabs-fallback-container">
          {fallbackChildren.map((child, idx) => (
            <React.Fragment key={child.id || `fallback_tab_child_${idx}`}>
              {renderBlock ? renderBlock(child, { ...context, isNested: true }) : null}
            </React.Fragment>
          ))}
        </View>
      )
    }
    return null
  }

  const activeTab = tabs.find((t, idx) => getTabKey(t, idx) === activeKey) || tabs[0]
  // 兼容读取 blocks, child, children
  const activeChildren = activeTab?.blocks || (activeTab?.children) || (activeTab?.child ? [activeTab.child] : [])

  const handleTabSelect = (key: string, tab: any) => {
    setActiveKey(key)
    const extra = { tab_key: key, tab_title: tab.title, item: tab, actionPayload: tab }
    if (tab.action && onAction) {
      onAction(tab.action, extra)
    } else if (props.on_tab_change_action && onAction) {
      onAction(props.on_tab_change_action, extra)
    } else if (block.events?.change && onAction) {
      onAction(undefined, { ...extra, __event: 'change' })
    }
  }

  return (
    <View className="sdui-tabs-block">
      {/* 顶部苹果磨砂分段胶囊选择器 */}
      <View className="tabs-header-bar">
        {tabs.map((tab, idx) => {
          const key = getTabKey(tab, idx)
          const isActive = key === activeKey
          return (
            <View
              key={key}
              className={`tab-pill-item ${isActive ? 'is-active' : ''}`}
              onClick={(e) => {
                e?.stopPropagation?.()
                handleTabSelect(key, tab)
              }}
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
              {renderBlock ? renderBlock(child, { ...context, isNested: true, active_tab_key: activeKey }) : null}
            </React.Fragment>
          ))}
      </View>
    </View>
  )
}

/**
  * 通用走马灯轮播积木 (CarouselBlock)
  */
export const CarouselBlock: React.FC<LayoutBlockProps> = ({ block, onAction, context, renderBlock }) => {
  const props = block.props || {}
  const rawItems = (props.items || props.children || props.blocks) as any[]
  const autoplay = props.autoplay !== false
  const configuredInterval = Number(props.interval)
  const configuredDuration = Number(props.duration)
  const interval = Number.isFinite(configuredInterval) ? Math.max(100, configuredInterval) : 3500
  const duration = Number.isFinite(configuredDuration) ? Math.max(0, configuredDuration) : 500
  const indicatorDots = props.indicator !== false
  const height = props.height || '360rpx'

  if (!Array.isArray(rawItems) || rawItems.length === 0) {
    return null
  }

  // 格式化每一项为符合 BlockItem 规范的结构
  const items: BlockItem[] = rawItems.map((child, idx) => {
    if (child && typeof child === 'object' && child.type) {
      return child as BlockItem
    }
    const imgUrl = (child && typeof child === 'object' && (child.image_url || child.image || child.cover_url || child.src)) || (typeof child === 'string' ? child : '')
    return {
      id: child?.id || `carousel_item_${idx}`,
      type: 'image',
      props: {
        image_url: String(imgUrl),
        aspect_ratio: '16:9',
        mode: 'aspectFill'
      },
      action: child?.action
    }
  })

  const handleSlideClick = (child: BlockItem, idx: number) => {
    const slideAction = child.action || (child.props as any)?.action
    const targetAction = slideAction || block.action
    if (targetAction && onAction) {
      onAction(targetAction, { ...context, isNested: true, index: idx, item: child, actionPayload: child })
    } else if (block.events?.tap && onAction) {
      onAction(undefined, { ...context, isNested: true, index: idx, item: child, actionPayload: child })
    }
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
          <SwiperItem
            key={child.id || `carousel_item_${idx}`}
            className="carousel-swiper-item"
            onClick={(e) => {
              e?.stopPropagation?.()
              handleSlideClick(child, idx)
            }}
          >
            {renderBlock ? renderBlock(child, { ...context, isNested: true, index: idx }) : null}
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
  const hasWidth = props.width !== undefined
  const hasHeight = props.height !== undefined

  const height = hasHeight ? props.height : (hasWidth ? undefined : '24rpx')
  const width = hasWidth ? props.width : '100%'

  return <View className="sdui-spacer-block" style={{ height, width, flexShrink: 0 }} />
}
