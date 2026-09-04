// StateBlocks.tsx
import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface StateBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
  * 通用空态/异常态积木 (EmptyBlock)
  * 支持覆盖：库存不足(out_of_stock)、资源过期(expired)、网络离线(offline)、能力不足(capability_missing)等异常态
  */
export const EmptyBlock: React.FC<StateBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const state = props.state || 'empty'
  let defaultIcon = '📭'
  let defaultTitle = '暂无内容'
  let defaultDesc = '当前暂无可用数据'

  switch (state) {
    case 'out_of_stock':
      defaultIcon = '📦'
      defaultTitle = '已被领完'
      defaultDesc = '礼包或资源已被全部领取，请关注后续补仓'
      break
    case 'expired':
      defaultIcon = '⌛'
      defaultTitle = '内容已过期'
      defaultDesc = '该活动或资源已超过有效时限，已自动下架'
      break
    case 'offline':
      defaultIcon = '📡'
      defaultTitle = '网络连接异常'
      defaultDesc = '未能连接到网络服务，请检查网络后重试'
      break
    case 'capability_missing':
      defaultIcon = '⚙️'
      defaultTitle = '暂不支持该功能'
      defaultDesc = '当前微信客户端版本过低，缺少所需基础库能力'
      break
  }

  const icon = props.icon || defaultIcon
  const title = props.title || defaultTitle
  const desc = props.desc || props.message || defaultDesc
  const btnText = props.btn_text || props.action_text || ''

  const handleClick = () => {
    if (block.action || block.events?.tap) {
      onAction?.(block.action)
    }
  }

  return (
    <View className="sdui-empty-card-block">
      <Text className="state-icon">{icon}</Text>
      <Text className="state-title">{title}</Text>
      <Text className="state-desc">{desc}</Text>
      {btnText && (
        <View className="state-action-btn" onClick={handleClick}>
          <Text className="action-btn-text">{btnText}</Text>
        </View>
      )}
    </View>
  )
}

/**
  * 通用骨架屏占位积木 (SkeletonBlock)
  */
export const SkeletonBlock: React.FC<{ block: BlockItem }> = ({ block }) => {
  const props = block.props || {}
  const configuredRows = Number(props.rows)
  const rowCount = Number.isFinite(configuredRows) ? Math.min(20, Math.max(0, Math.floor(configuredRows))) : 3
  const showHero = props.hero !== false
  const rows = Array.from({ length: rowCount })

  return (
    <View className="sdui-skeleton-block">
      {showHero && <View className="skeleton-line is-hero" />}
      {rows.map((_, idx) => (
        <View
          key={idx}
          className="skeleton-line"
          style={{ width: `${85 - idx * 15}%` }}
        />
      ))}
    </View>
  )
}
