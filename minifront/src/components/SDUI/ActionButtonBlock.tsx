// ActionButtonBlock.tsx
import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface ActionButtonBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 苹果风通栏大胶囊主按钮积木 (ActionButtonBlock)
 */
export const ActionButtonBlock: React.FC<ActionButtonBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const text = props.text || '立即前往'
  const badge = props.badge || ''

  const targetAction = block.action || props.action || props.btn_action
  const handleClick = (e: any) => {
    e?.stopPropagation?.()
    if ((targetAction || block.events?.tap || props.events?.tap) && onAction) {
      onAction(targetAction, { item: props, actionPayload: props, btn_text: text, badge })
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
