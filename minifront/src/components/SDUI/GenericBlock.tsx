// GenericBlock.tsx
import React from 'react'
import { View, Image, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface GenericBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/** 通用自由编排卡片与未预设积木的协议级通用渲染器，支持图片、徽标、标题、正文与主操作按钮。 */
export const GenericBlock: React.FC<GenericBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const imageUrl = props.image_url || props.cover_url || props.src || props.poster
  const title = props.title || props.name || props.text || (block.type !== 'custom' && block.type !== 'custom_block' ? block.type : '自由卡片')
  const subtitle = props.subtitle
  const content = props.content || props.desc || props.description || (props.redeem_code ? `兑换码: ${props.redeem_code}` : (props.code ? `口令: ${props.code}` : ''))
  const badge = props.badge || props.tag || props.category || (props.score !== undefined ? `分值: ${props.score}` : '')
  const btnText = props.btn_text || props.button_text || (props.redeem_code || props.code ? '复制口令' : '')
  const btnAction = props.btn_action || props.button_action

  const targetCardAction = block.action || props.card_action || props.action
  const handleCardClick = (e: any) => {
    if (targetCardAction || block.events?.tap || props.events?.tap) {
      e.stopPropagation?.()
      onAction?.(targetCardAction, { item: props, actionPayload: props, title })
    }
  }

  const handleBtnClick = (e: any) => {
    e.stopPropagation?.()
    const ctx = { item: props, actionPayload: props, title }
    if (btnAction) {
      onAction?.(btnAction, ctx)
    } else if (props.redeem_code || props.code) {
      const codeVal = String(props.redeem_code || props.code)
      onAction?.({
        type: 'copy_text',
        payload: { text: codeVal, toast: `✅ 口令 ${codeVal} 已复制！` }
      }, { ...ctx, code: codeVal })
    } else if (targetCardAction || block.events?.tap || props.events?.tap) {
      onAction?.(targetCardAction, ctx)
    }
  }

  return (
    <View className="sdui-generic-block" onClick={handleCardClick}>
      {imageUrl && <Image className="sdui-generic-image" src={String(imageUrl)} mode="aspectFill" />}
      <View className="sdui-generic-header">
        <Text className="sdui-generic-title">{String(title)}</Text>
        {badge && <Text className="sdui-generic-badge">{String(badge)}</Text>}
      </View>
      {subtitle && <Text style={{ fontSize: '24rpx', color: 'rgba(255,255,255,0.5)', marginTop: '4rpx', marginBottom: '8rpx', display: 'block' }}>{String(subtitle)}</Text>}
      {content && <Text className="sdui-generic-content">{String(content)}</Text>}
      {btnText && (
        <View className="sdui-generic-action-row">
          <View className="sdui-generic-btn" onClick={handleBtnClick}>
            <Text className="btn-text">{String(btnText)}</Text>
          </View>
        </View>
      )}
    </View>
  )
}
