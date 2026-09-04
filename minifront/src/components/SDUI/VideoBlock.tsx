// VideoBlock.tsx
import React from 'react'
import { View, Video } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface VideoBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
  * 通用原生视频播放积木组件 (VideoBlock)
  */
export const VideoBlock: React.FC<VideoBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const videoUrl = props.video_url || props.src || ''
  const poster = props.poster || props.cover_url || ''
  const autoplay = props.autoplay === true
  const controls = props.controls !== false
  const loop = props.loop === true

  const containerStyle: React.CSSProperties = {
    borderRadius: block.style?.border_radius || '20rpx',
    overflow: 'hidden',
    position: 'relative',
    width: '100%',
    aspectRatio: props.aspect_ratio ? props.aspect_ratio.replace(':', '/') : '16/9'
  }

  const handleEnded = () => {
    if (props.on_ended_action && onAction) {
      onAction(props.on_ended_action)
    }
  }

  return (
    <View className="sdui-video-block" style={containerStyle}>
      <Video
        src={videoUrl}
        poster={poster}
        autoplay={autoplay}
        controls={controls}
        loop={loop}
        onEnded={handleEnded}
        className="sdui-video-inner"
        style={{ width: '100%', height: '100%' }}
      />
    </View>
  )
}
