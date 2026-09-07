// GameCardBlock.tsx
import React from 'react'
import { View, Text, Image } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface GameCardBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 游戏礼包与兑换码积木组件 (GameCardBlock)
 * 支持卡片主体交互跳转（如启动游戏/查看详情）与独家兑换码一键复制解耦
 */
export const GameCardBlock: React.FC<GameCardBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '热门游戏'
  const subtitle = props.subtitle || ''
  const coverUrl = props.cover_url || ''
  const version = props.version || '最新公测'
  const redeemCode = props.redeem_code || 'VIP888'

  // 卡片主体动作 (如跳转游戏详情或跨小程序启动游戏)
  const cardAction = props.card_action || block.action
  const handleCardClick = (e: any) => {
    e?.stopPropagation?.()
    const ctx = { item: props, actionPayload: props, title, version, redeem_code: redeemCode, code: redeemCode }
    if (cardAction && onAction) {
      onAction(cardAction, ctx)
    } else if (block.events?.tap && onAction) {
      onAction(undefined, ctx)
    }
  }

  // 兑换码一键复制动作
  const handleCopy = (e: any) => {
    e.stopPropagation?.()
    if (onAction) {
      const copyAction = props.copy_action || props.redeem_action
      if (copyAction) {
        onAction(copyAction, { redeem_code: redeemCode, title })
      } else {
        onAction({
          type: 'copy_text',
          payload: {
            text: redeemCode,
            toast: `✅ 兑换码 ${redeemCode} 已复制到剪贴板！`
          }
        }, { redeem_code: redeemCode, title })
      }
    }
  }

  return (
    <View
      className="sdui-game-card"
      onClick={handleCardClick}
      style={{
        borderRadius: block.style?.border_radius || '28rpx'
      }}
    >
      <View className="game-banner">
        {coverUrl ? <Image src={coverUrl} mode="aspectFill" className="banner-img" /> : null}
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
