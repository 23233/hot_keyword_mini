// EpisodeListBlock.tsx
import { useEffect, useState } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface EpisodeListBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: any
}

/**
 * 苹果 HIG 规范短剧选集列表积木 (EpisodeListBlock)
 * 支持集数网格、当前播放高亮、锁定状态提示与点击换集
 */
export const EpisodeListBlock: React.FC<EpisodeListBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '选集列表'
  const configuredTotal = Number(props.total_episodes || props.total || 80)
  const total = Number.isFinite(configuredTotal) ? Math.min(1000, Math.max(0, Math.floor(configuredTotal))) : 80
  const currentEp = Number(props.current_episode || 1)
  const [selected, setSelected] = useState<number>(currentEp)

  useEffect(() => {
    const next = Number.isFinite(currentEp) ? Math.floor(currentEp) : 1
    setSelected(total > 0 ? Math.min(total, Math.max(1, next)) : 0)
  }, [currentEp, total])

  const episodes = Array.from({ length: total }, (_, i) => i + 1)

  const handleSelect = (num: number) => {
    if (total <= 0) return
    setSelected(num)
    if (!onAction) return
    if (block.action) {
      onAction({
        ...block.action,
        payload: {
          ...(block.action.payload || {}),
          episode_num: num
        }
      })
    } else if (block.events?.tap) {
      onAction(undefined, { actionPayload: { episode_num: num } })
    }
  }

  return (
    <View
      className="sdui-episode-list-block"
      style={{
        borderRadius: block.style?.border_radius || '28rpx',
        padding: '28rpx',
        background: 'rgba(255, 255, 255, 0.05)',
        backdropFilter: 'blur(20px)',
        border: '1px solid rgba(255, 255, 255, 0.08)'
      }}
    >
      <View style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20rpx' }}>
        <Text style={{ fontSize: '30rpx', fontWeight: 600, color: '#fff' }}>{title}</Text>
        <Text style={{ fontSize: '24rpx', color: 'rgba(255, 255, 255, 0.5)' }}>共 {total} 集全</Text>
      </View>

      <ScrollView scrollX style={{ whiteSpace: 'nowrap', width: '100%' }}>
        <View style={{ display: 'flex', gap: '16rpx', paddingBottom: '10rpx' }}>
          {episodes.map((num) => {
            const isCurrent = num === selected
            return (
              <View
                key={num}
                onClick={() => handleSelect(num)}
                style={{
                  minWidth: '88rpx',
                  height: '88rpx',
                  borderRadius: '16rpx',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: isCurrent ? 'linear-gradient(135deg, #FF9F0A, #FF375F)' : 'rgba(255, 255, 255, 0.08)',
                  border: isCurrent ? '1px solid rgba(255, 159, 10, 0.8)' : '1px solid rgba(255, 255, 255, 0.06)',
                  color: isCurrent ? '#fff' : 'rgba(255, 255, 255, 0.8)',
                  fontSize: '28rpx',
                  fontWeight: isCurrent ? 700 : 500,
                  flexShrink: 0
                }}
              >
                <Text>{num}</Text>
              </View>
            )
          })}
        </View>
      </ScrollView>
    </View>
  )
}
