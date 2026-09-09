// EpisodeListBlock.tsx
import { useEffect, useState } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

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

  const handleSelect = (num: number, e?: any) => {
    e?.stopPropagation?.()
    if (total <= 0) return
    setSelected(num)
    if (!onAction) return
    const episodeContext = {
      item: { episode_num: num },
      actionPayload: { episode_num: num },
      episode_num: num
    }
    if (block.action) {
      onAction({
        ...block.action,
        payload: {
          ...(block.action.payload || {}),
          episode_num: num
        }
      }, episodeContext)
    } else if (block.events?.tap) {
      onAction(undefined, episodeContext)
    }
  }

  return (
    <View
      className={blockClassName('sdui-episode-list-block', block.style)}
      style={block.style?.border_radius ? { borderRadius: block.style.border_radius } : undefined}
    >
      <View className="sdui-episode-heading">
        <Text className="sdui-episode-title">{title}</Text>
        <Text className="sdui-episode-count">共 {total} 集全</Text>
      </View>

      <ScrollView scrollX className="sdui-episode-scroll">
        <View className="sdui-episode-items">
          {episodes.map((num) => {
            const isCurrent = num === selected
            return (
              <View
                key={num}
                onClick={(e) => handleSelect(num, e)}
                className={`sdui-episode-item ${isCurrent ? 'is-current' : ''}`}
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
