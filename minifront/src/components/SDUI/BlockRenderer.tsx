import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { MediaHeroBlock } from './MediaHeroBlock'
import { ResourceCardBlock } from './ResourceCardBlock'
import { ActionButtonBlock } from './ActionButtonBlock'
import { NoticeBlock } from './NoticeBlock'
import { GameCardBlock } from './GameCardBlock'
import { FormBlock } from './FormBlock'
import { evaluateCondition } from '../../utils/condition'
import './sdui.scss'

interface BlockRendererProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
  context?: Record<string, any>
}

/**
 * 动态原子积木调度渲染器 (BlockRenderer)
 * 具备受控条件求值、类型调度与未知组件自动优雅降级保护
 */
export const BlockRenderer: React.FC<BlockRendererProps> = ({ block, onAction, context }) => {
  if (!block || !block.type) return null

  // 1. 检查受控条件可见性 (visible_when 多运算符受控求值)
  if (block.visible_when !== undefined) {
    const isVisible = evaluateCondition(block.visible_when, context)
    if (!isVisible) {
      return null
    }
  }

  // 2. 解析外层容器样式
  const style = block.style || {}
  const wrapperClass = `sdui-block-wrapper ${style.glass_blur !== false ? 'is-glass' : ''}`
  const wrapperStyle: React.CSSProperties = {
    marginTop: style.margin_y || undefined,
    marginBottom: style.margin_y || '24rpx',
    padding: style.padding || undefined
  }

  // 3. 按照注册表匹配对应积木组件
  const renderInner = () => {
    switch (block.type) {
      case 'media_hero':
        return <MediaHeroBlock block={block} onAction={onAction} />
      case 'resource_card':
        return <ResourceCardBlock block={block} onAction={onAction} />
      case 'action_button':
        return <ActionButtonBlock block={block} onAction={onAction} />
      case 'notice':
        return <NoticeBlock block={block} onAction={onAction} />
      case 'game_card':
        return <GameCardBlock block={block} onAction={onAction} />
      case 'form':
        return <FormBlock block={block} onAction={onAction} />
      default:
        // 4. 未知积木组件降级策略 (优先 fallback，无则优雅占位，杜绝整页白屏崩溃)
        if (block.fallback) {
          return <BlockRenderer block={block.fallback} onAction={onAction} />
        }
        return (
          <View className="sdui-fallback-block">
            <Text className="fallback-hint">暂未支持的组件类型: {block.type}</Text>
          </View>
        )
    }
  }

  return (
    <View className={wrapperClass} style={wrapperStyle}>
      {renderInner()}
    </View>
  )
}
