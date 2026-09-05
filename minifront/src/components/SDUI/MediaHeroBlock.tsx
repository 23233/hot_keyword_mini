// MediaHeroBlock.tsx
import React from 'react'
import { View, Text, Image, Video } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface MediaHeroBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
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

  const handleClick = () => {
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action)
    }
  }

  return (
    <View
      className="sdui-media-hero"
      onClick={handleClick}
      style={{
        borderRadius: block.style?.border_radius || '28rpx',
        backgroundColor: block.style?.background || '#151518'
      }}
    >
      <View className="media-player-box">
        {videoUrl ? (
          <Video
            src={videoUrl}
            poster={coverUrl}
            className="hero-video"
            controls
            showFullscreenBtn
            showPlayBtn
            autoplay={false}
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
