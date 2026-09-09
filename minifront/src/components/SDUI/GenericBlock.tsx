// GenericBlock.tsx
import React from 'react'
import { View, Image, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

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
  const rows = Array.isArray(props.rows) ? props.rows : []
  const items = Array.isArray(props.items) ? props.items : []
  const options = Array.isArray(props.options) ? props.options : []

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
    <View className={blockClassName('sdui-generic-block', block.style)} onClick={handleCardClick}>
      {imageUrl && <Image className="sdui-generic-image" src={String(imageUrl)} mode="aspectFill" />}
      <View className="sdui-generic-header">
        <Text className="sdui-generic-title">{String(title)}</Text>
        {badge && <Text className="sdui-generic-badge">{String(badge)}</Text>}
      </View>
      {subtitle && <Text className="sdui-generic-subtitle">{String(subtitle)}</Text>}
      {content && <Text className="sdui-generic-content">{String(content)}</Text>}
      {rows.length > 0 && (
        <View className="sdui-generic-list">
          {rows.map((row: any, index: number) => <View className="sdui-generic-list-row" key={row.id || index}><Text>{String(row.label || row.name || row.key || '')}</Text><Text>{String(row.value ?? row.val ?? row.content ?? '')}</Text></View>)}
        </View>
      )}
      {items.length > 0 && (
        <View className="sdui-generic-list">
          {items.map((item: any, index: number) => <View className="sdui-generic-list-row" key={item.id || index}><Text>{String(item.title || item.name || item.label || '')}</Text><Text>{String(item.summary || item.value || item.status || '')}</Text></View>)}
        </View>
      )}
      {options.length > 0 && (
        <View className="sdui-generic-list">
          {options.map((option: any, index: number) => <View className="sdui-generic-option" key={option.id || index} onClick={(event) => { event.stopPropagation?.(); onAction?.(option.action || block.action, { item: option, actionPayload: option }) }}><Text>{String(option.label || option.title || option.name || option)}</Text></View>)}
        </View>
      )}
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
