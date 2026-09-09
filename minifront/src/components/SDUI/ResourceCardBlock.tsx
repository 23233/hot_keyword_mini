// ResourceCardBlock.tsx
import React, { useState } from 'react'
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

interface PanChannel {
  name: string
  url?: string
  fetch_code?: string
  extract_code?: string
  desc?: string
}

interface ResourceCardBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 网盘与核心资源提取承接卡片积木 (ResourceCardBlock)
 * 支持多网盘渠道（夸克、百度网盘等）标签切换与一键复制提取码/链接
 */
export const ResourceCardBlock: React.FC<ResourceCardBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '极速全集资源'
  const desc = props.desc || '高清未删减版 免费自取'
  const btnText = props.btn_text || '一键复制网盘'

  // 多网盘渠道列表
  const rawChannels = props.channels || props.pan_list
  const channels: PanChannel[] = Array.isArray(rawChannels) && rawChannels.length > 0
    ? rawChannels
    : [
        {
          name: props.pan_name || '网盘直达',
          url: props.pan_url || props.url || '',
          fetch_code: props.fetch_code || props.extract_code || '',
          desc: props.desc
        }
      ]

  const [selectedChannelIdx, setSelectedChannelIdx] = useState<number>(0)
  const currentChannel = channels[selectedChannelIdx] || channels[0]

  const panName = currentChannel?.name || '网盘直达'
  const panUrl = currentChannel?.url || ''
  const fetchCode = currentChannel?.fetch_code || currentChannel?.extract_code || ''

  const handleAction = (e?: any) => {
    e?.stopPropagation?.()
    if (onAction) {
      const channelAction = (currentChannel as any)?.action
      const actCtx = {
        item: currentChannel,
        actionPayload: currentChannel,
        channel: currentChannel,
        pan_name: panName,
        pan_url: panUrl,
        fetch_code: fetchCode
      }
      if (channelAction) {
        onAction(channelAction, actCtx)
      } else if (block.action || block.events?.tap) {
        onAction(block.action, actCtx)
      } else {
        const copyContent = panUrl
          ? (fetchCode ? `链接: ${panUrl} 提取码: ${fetchCode}` : panUrl)
          : (fetchCode ? `提取码: ${fetchCode}` : title)
        onAction({
          type: 'copy_text',
          payload: {
            text: copyContent,
            toast: `✅ ${panName} 信息已复制到剪贴板！`
          }
        }, actCtx)
      }
    }
  }

  return (
    <View
      className={blockClassName('sdui-resource-card', block.style)}
      style={block.style?.border_radius ? { borderRadius: block.style.border_radius } : undefined}
    >
      <View className="resource-header">
        <View className="pan-icon-chip">
          <Text>{panName}</Text>
        </View>
        <Text className="card-title">{title}</Text>
      </View>

      {/* 多网盘渠道快捷切换胶囊栏 */}
      {channels.length > 1 && (
        <View className="sdui-resource-channels">
          {channels.map((ch, idx) => {
            const isSelected = idx === selectedChannelIdx
            return (
              <View
                key={ch.name || idx}
                onClick={(e) => {
                  e?.stopPropagation?.()
                  setSelectedChannelIdx(idx)
                }}
                className={`sdui-resource-channel ${isSelected ? 'is-selected' : ''}`}
              >
                <Text>{ch.name}</Text>
              </View>
            )
          })}
        </View>
      )}

      <Text className="resource-desc">{currentChannel?.desc || desc}</Text>

      <View className="resource-action-row">
        <View className="code-box">
          <Text className="code-label">提取码 / 口令</Text>
          <Text className="code-val">{fetchCode || (panUrl ? '点击直接访问' : '直接访问无提取码')}</Text>
        </View>

        <View className="copy-btn" onClick={handleAction}>
          <Text>{btnText}</Text>
        </View>
      </View>
    </View>
  )
}
