// MediaHeroBlock.tsx
import React from 'react'
import { View, Text, Image, Video } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

interface MediaHeroBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 媒体大焦点海报/视频试看积木组件 (MediaHeroBlock)
 */
export const MediaHeroBlock: React.FC<MediaHeroBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '精彩内容'
  const subtitle = props.subtitle || ''
  const coverUrl = props.cover_url || ''
  const videoUrl = props.video_url || ''
  const rating = props.rating || 9.8
  const badge = props.badge || '🎬 精选热播'

  const handleClick = (e: any) => {
    e?.stopPropagation?.()
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action, { item: props, actionPayload: props, title, cover_url: coverUrl, video_url: videoUrl })
    }
  }

  const handleVideoEnded = () => {
    if (props.on_ended_action && onAction) {
      onAction(props.on_ended_action, { title, video_url: videoUrl })
    } else if (block.events?.ended && onAction) {
      onAction(undefined, { __event: 'ended', title, video_url: videoUrl })
    }
  }

  return (
    <View
      className={blockClassName('sdui-media-hero', block.style)}
      onClick={handleClick}
    >
      <View
        className="media-player-box"
        onClick={(e) => {
          // 若正在展示原生视频播放器，阻止播放控件交互向外冒泡触发卡片跳转
          if (videoUrl) {
            e.stopPropagation?.()
          }
        }}
      >
        {videoUrl ? (
          <Video
            src={videoUrl}
            poster={coverUrl}
            className="hero-video"
            controls
            showFullscreenBtn
            showPlayBtn
            autoplay={false}
            onEnded={handleVideoEnded}
          />
        ) : (
          coverUrl ? <Image src={coverUrl} mode="aspectFill" className="hero-cover" /> : null
        )}
        <View className="play-badge-overlay">
          <Text className="badge-text">{badge}</Text>
        </View>
      </View>

      <View className="media-info-bar">
        <View className="info-left">
          <Text className="hero-title">{title}</Text>
          {subtitle && <Text className="hero-subtitle">{subtitle}</Text>}
        </View>
        <View className="info-right">
          <Text className="rating-tag">{rating}</Text>
          <Text className="rating-label">全网评分</Text>
        </View>
      </View>
    </View>
  )
}
