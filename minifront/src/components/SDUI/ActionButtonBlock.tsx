import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface ActionButtonBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
}

/**
 * 苹果风通栏大胶囊主按钮积木 (ActionButtonBlock)
 */
export const ActionButtonBlock: React.FC<ActionButtonBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const text = props.text || '立即前往'
  const badge = props.badge || ''

  const handleClick = () => {
    if (block.action && onAction) {
      onAction(block.action)
    }
  }

  return (
    <View className="sdui-action-btn-block">
      <View
        className="capsule-btn"
        onClick={handleClick}
        style={{
          borderRadius: block.style?.border_radius || '999rpx',
          background: block.style?.background || undefined
        }}
      >
        <Text className="btn-text">{text}</Text>
        {badge && (
          <View className="btn-badge">
            <Text>{badge}</Text>
          </View>
        )}
      </View>
    </View>
  )
}
