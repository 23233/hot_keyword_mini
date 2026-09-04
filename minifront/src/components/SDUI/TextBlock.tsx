// TextBlock.tsx
import React from 'react'
import { View, Text, RichText } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface TextBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
  * 通用文本与富文本积木组件 (TextBlock)
  * 支持纯文本、RichText 富文本、字号、颜色、对齐方式与多行截断
  */
export const TextBlock: React.FC<TextBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const text = props.text !== undefined ? String(props.text) : props.content !== undefined ? String(props.content) : ''
  const richNodes = props.rich_text || props.nodes

  const handleClick = () => {
    if (block.action || block.events?.tap) {
      onAction?.(block.action)
    }
  }

  const textStyle: React.CSSProperties = {
    fontSize: props.font_size ? (typeof props.font_size === 'number' ? `${props.font_size}px` : props.font_size) : undefined,
    fontWeight: props.font_weight || undefined,
    color: props.color || undefined,
    textAlign: props.align || props.text_align || 'left',
    lineHeight: props.line_height || 1.6
  }

  const maxLines = Number(props.max_lines) || 0
  const clampClass = maxLines > 0 ? `sdui-text-clamp-${maxLines}` : ''

  return (
    <View className={`sdui-text-block ${clampClass}`} style={textStyle} onClick={handleClick}>
      {richNodes ? (
        <RichText nodes={richNodes} />
      ) : (
        <Text className="sdui-text-content">{text}</Text>
      )}
    </View>
  )
}
