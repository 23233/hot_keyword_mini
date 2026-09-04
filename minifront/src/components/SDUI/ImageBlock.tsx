// ImageBlock.tsx
import React, { useState } from 'react'
import { View, Image, Text } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { BlockItem, BlockAction } from '../../types/sdui'

interface ImageBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
  * 通用图片积木组件 (ImageBlock)
  * 支持自定义宽高、裁剪模式、宽高比、圆角、占位图、全屏预览与交互动作绑定
  */
export const ImageBlock: React.FC<ImageBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const imageUrl = props.image_url || props.src || props.url || ''
  const fallbackUrl = props.fallback_url || props.placeholder || ''
  const mode = (props.mode as any) || 'aspectFill'
  const aspectRatio = props.aspect_ratio || ''
  const enablePreview = props.preview === true

  const [hasError, setHasError] = useState(false)
  const [fallbackError, setFallbackError] = useState(false)

  // 处理点击事件
  const handleClick = () => {
    if (block.action || block.events?.tap) {
      onAction?.(block.action)
    } else if (enablePreview && imageUrl) {
      Taro.previewImage({
        current: imageUrl,
        urls: [imageUrl]
      })
    }
  }

  // 计算宽高比样式
  const containerStyle: React.CSSProperties = {
    borderRadius: block.style?.border_radius || props.border_radius || '16rpx',
    overflow: 'hidden',
    position: 'relative'
  }

  if (aspectRatio) {
    switch (aspectRatio) {
      case '16:9':
        containerStyle.aspectRatio = '16/9'
        break
      case '4:3':
        containerStyle.aspectRatio = '4/3'
        break
      case '1:1':
        containerStyle.aspectRatio = '1/1'
        break
      case '3:2':
        containerStyle.aspectRatio = '3/2'
        break
      default:
        containerStyle.aspectRatio = aspectRatio
    }
  } else if (props.height) {
    containerStyle.height = typeof props.height === 'number' ? `${props.height}px` : props.height
  } else {
    // 默认保持 16:9 优雅比例
    containerStyle.aspectRatio = '16/9'
  }

  if (props.width) {
    containerStyle.width = typeof props.width === 'number' ? `${props.width}px` : props.width
  } else {
    containerStyle.width = '100%'
  }

  const currentSrc = !hasError && imageUrl ? imageUrl : (!fallbackError ? fallbackUrl : '')

  return (
    <View className="sdui-image-block" style={containerStyle} onClick={handleClick}>
      {currentSrc ? (
        <Image
          src={currentSrc}
          mode={mode}
          className="sdui-img-inner"
          onError={() => {
            if (!hasError && imageUrl) setHasError(true)
            else setFallbackError(true)
          }}
          style={{ width: '100%', height: '100%', display: 'block' }}
        />
      ) : (
        <View className="sdui-img-placeholder">
          <Text className="placeholder-icon">🖼️</Text>
          <Text className="placeholder-hint">暂无图片内容</Text>
        </View>
      )}
    </View>
  )
}
