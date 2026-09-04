// ResourceCardBlock.tsx
import React from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface ResourceCardBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
}

/**
 * 网盘与核心资源提取承接卡片积木 (ResourceCardBlock)
 */
export const ResourceCardBlock: React.FC<ResourceCardBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '极速全集资源'
  const desc = props.desc || '高清未删减版 免费自取'
  const btnText = props.btn_text || '一键复制网盘'
  const panName = props.pan_name || '网盘直达'
  const fetchCode = props.fetch_code || ''

  const handleAction = () => {
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action)
    }
  }

  return (
    <View
      className="sdui-resource-card"
      style={{
        borderRadius: block.style?.border_radius || '28rpx'
      }}
    >
      <View className="resource-header">
        <View className="pan-icon-chip">
          <Text>{panName}</Text>
        </View>
        <Text className="card-title">{title}</Text>
      </View>

      <Text className="resource-desc">{desc}</Text>

      <View className="resource-action-row">
        <View className="code-box">
          <Text className="code-label">提取码 / 口令</Text>
          <Text className="code-val">{fetchCode || '直接访问无提取码'}</Text>
        </View>

        <View className="copy-btn" onClick={handleAction}>
          <Text>{btnText}</Text>
        </View>
      </View>
    </View>
  )
}
