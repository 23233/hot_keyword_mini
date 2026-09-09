// VideoBlock.tsx
import React from 'react'
import { View, Video } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { resolveRemoteUrl } from '../../config/env'
import { blockClassName } from './style'

interface VideoBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
  * 通用原生视频播放积木组件 (VideoBlock)
  */
export const VideoBlock: React.FC<VideoBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const videoUrl = resolveRemoteUrl(props.video_url || props.src)
  const poster = resolveRemoteUrl(props.poster || props.cover_url)
  const autoplay = props.autoplay === true
  const controls = props.controls !== false
  const loop = props.loop === true

  const containerStyle: React.CSSProperties = {
    ...(block.style?.border_radius ? { borderRadius: block.style.border_radius } : {}),
    overflow: 'hidden',
    position: 'relative',
    width: '100%',
    aspectRatio: props.aspect_ratio ? props.aspect_ratio.replace(':', '/') : '16/9'
  }

  const handleEnded = () => {
    if (props.on_ended_action && onAction) {
      onAction(props.on_ended_action)
    } else if (block.events?.ended && onAction) {
      onAction(undefined, { __event: 'ended' })
    }
  }

  return (
    <View className={blockClassName('sdui-video-block', block.style)} style={containerStyle} onClick={(e) => e.stopPropagation?.()}>
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
