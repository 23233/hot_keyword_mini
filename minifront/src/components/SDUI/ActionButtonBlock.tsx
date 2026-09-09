// ActionButtonBlock.tsx
import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

interface ActionButtonBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 苹果风通栏大胶囊主按钮积木 (ActionButtonBlock)
 */
export const ActionButtonBlock: React.FC<ActionButtonBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const text = String(props.text || '立即前往')
  const badge = props.badge ? String(props.badge) : ''
  const variant = String(props.variant || 'primary')

  const targetAction = block.action || props.action || props.btn_action
  const handleClick = () => {
    if (!onAction || (!targetAction && !block.events?.tap && !props.events?.tap)) return
    onAction(targetAction, { item: props, actionPayload: props, btn_text: text, badge })
  }

  return (
    <View className={blockClassName(`sdui-action-btn-block variant-${variant}`, block.style)}>
      <View
        className="capsule-btn"
        onClick={handleClick}
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
