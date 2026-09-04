// GameCardBlock.tsx
import React from 'react'
import { View, Text, Image } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface GameCardBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
}

/**
 * 游戏礼包与兑换码积木组件 (GameCardBlock)
 */
export const GameCardBlock: React.FC<GameCardBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '热门游戏'
  const subtitle = props.subtitle || ''
  const coverUrl = props.cover_url || 'https://images.unsplash.com/photo-1542751371-adc38448a05e?w=800&q=80'
  const version = props.version || '最新公测'
  const redeemCode = props.redeem_code || 'VIP888'

  const handleCopy = () => {
    if ((block.action || block.events?.tap) && onAction) {
      onAction(block.action)
    }
  }

  return (
    <View
      className="sdui-game-card"
      style={{
        borderRadius: block.style?.border_radius || '28rpx'
      }}
    >
      <View className="game-banner">
        <Image src={coverUrl} mode="aspectFill" className="banner-img" />
        <View className="game-badge">
          <Text>🎮 官方公测</Text>
        </View>
      </View>

      <View className="game-body">
        <View className="game-header-row">
          <View>
            <Text className="game-title">{title}</Text>
            {subtitle && <Text style={{ fontSize: '22rpx', color: 'rgba(255,255,255,0.5)', display: 'block', marginTop: '4rpx' }}>{subtitle}</Text>}
          </View>
          <Text className="game-version">{version}</Text>
        </View>

        <View className="redeem-box">
          <View className="code-meta">
            <Text className="code-tip">🎁 独家礼包兑换码</Text>
            <Text className="code-str">{redeemCode}</Text>
          </View>
          <View className="btn-copy-code" onClick={handleCopy}>
            <Text>一键复制</Text>
          </View>
        </View>
      </View>
    </View>
  )
}
