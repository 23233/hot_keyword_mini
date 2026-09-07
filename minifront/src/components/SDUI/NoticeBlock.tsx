// NoticeBlock.tsx
import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface NoticeBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 跑马灯通告栏积木组件 (NoticeBlock)
 */
export const NoticeBlock: React.FC<NoticeBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const text = props.text || props.content || ''
  const icon = props.icon || '📢'

  const handleClick = (e: any) => {
    if ((block.action || block.events?.tap) && onAction) {
      e.stopPropagation?.()
      onAction(block.action, { item: props, actionPayload: props, text })
    }
  }

  if (!text) return null

  return (
    <View className="sdui-notice-bar" onClick={handleClick}>
      <Text className="notice-icon">{icon}</Text>
      <Text className="notice-text">{text}</Text>
    </View>
  )
}
