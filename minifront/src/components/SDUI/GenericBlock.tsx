// GenericBlock.tsx
import React from 'react'
import { View, Image, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface GenericBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/** 未注册业务块的协议级通用渲染器，优先展示图片、标题和正文并保留动作。 */
export const GenericBlock: React.FC<GenericBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const imageUrl = props.image_url || props.cover_url || props.src
  const title = props.title || props.name || props.text || block.type
  const content = props.content || props.desc || props.description || props.subtitle
  return (
    <View className="sdui-generic-block" onClick={() => onAction?.(block.action)}>
      {imageUrl && <Image className="sdui-generic-image" src={String(imageUrl)} mode="aspectFill" />}
      <Text className="sdui-generic-title">{String(title)}</Text>
      {content && <Text className="sdui-generic-content">{String(content)}</Text>}
    </View>
  )
}
