// OverlayBlock.tsx
import React, { useEffect, useState } from 'react'
import { RootPortal, Text, View } from '@tarojs/components'
import { BlockAction, BlockItem } from '../../types/sdui'

interface OverlayBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: Record<string, any>
}

/**
 * 通用弹层积木：
 * - dialog：居中确认弹窗；
 * - action_sheet：底部动作菜单；
 * - bottom_sheet：底部内容面板。
 * 可通过 trigger_text 自带入口，也可由 visible_when / props.open 由页面状态控制。
 */
export const OverlayBlock: React.FC<OverlayBlockProps> = ({ block, onAction, context }) => {
  const props = block.props || {}
  const [open, setOpen] = useState(props.open === true)

  useEffect(() => {
    // 协议显式下发 open 时按外部状态同步，未声明时保持组件内部开关
    if (typeof props.open === 'boolean') setOpen(props.open)
  }, [props.open])

  // 弹层打开期间通知页面锁定底层滚动，关闭或卸载时恢复
  useEffect(() => {
    context?.setOverlayOpen?.(open)
    return () => context?.setOverlayOpen?.(false)
  }, [open, context?.setOverlayOpen])

  const presentation = block.type === 'dialog'
    ? 'dialog'
    : block.type === 'action_sheet'
      ? 'action_sheet'
      : 'bottom_sheet'
  const items = Array.isArray(props.items) ? props.items : []
  const maskClosable = props.mask_closable !== false

  /** 触发动作并关闭弹层。 */
  const emit = (action: any, payload?: Record<string, any>) => {
    if (action && typeof action === 'object' && action.type) {
      onAction?.(action, {
        item: props,
        actionPayload: payload || {}
      })
    }
    setOpen(false)
  }

  const close = () => setOpen(false)
  const trigger = !open && props.trigger_text ? (
    <View id={`${block.id}_trigger`} className='sdui-overlay-trigger' onClick={() => setOpen(true)}>
      <Text>{String(props.trigger_text)}</Text>
    </View>
  ) : null

  if (!open) {
    return <View className='sdui-overlay-closed'>{trigger}</View>
  }

  return (
    <View className='sdui-overlay-root'>
      {trigger}
      <RootPortal enable>
      <View
        className={`sdui-overlay-mask ${presentation === 'dialog' ? 'is-center' : 'is-bottom'}`}
        style={{ ['--sdui-font-scale' as any]: String(context?.fontScale || 1) }}
        onClick={() => { if (maskClosable) close() }}
      >
        <View
          className={`sdui-overlay-panel is-${presentation}`}
          onClick={(event) => event?.stopPropagation?.()}
        >
          {presentation !== 'action_sheet' && (
            <View className='sdui-overlay-header'>
              <Text className='sdui-overlay-title'>{String(props.title || '提示')}</Text>
              {props.description && <Text className='sdui-overlay-desc'>{String(props.description)}</Text>}
            </View>
          )}
          {presentation !== 'action_sheet' && props.content && (
            <Text className='sdui-overlay-content'>{String(props.content)}</Text>
          )}

          {items.length > 0 && (
            <View className='sdui-overlay-actions'>
              {items.map((item: any, index: number) => (
                <View
                  key={item.key || item.label || index}
                  className={`sdui-overlay-action ${item.danger ? 'is-danger' : ''} ${item.disabled ? 'is-disabled' : ''}`}
                  onClick={() => {
                    if (item.disabled) return
                    emit(item.action || block.action, { ...(item.payload || {}), item, index })
                  }}
                >
                  <Text className='sdui-overlay-action-label'>{String(item.label || item.title || '')}</Text>
                  {item.description && <Text className='sdui-overlay-action-desc'>{String(item.description)}</Text>}
                </View>
              ))}
            </View>
          )}

          {presentation !== 'action_sheet' && (
            <View className='sdui-overlay-footer'>
              <View id={`${block.id}_cancel`} className='sdui-overlay-button is-secondary' onClick={() => emit(props.cancel_action, {})}>
                <Text>{String(props.cancel_text || '取消')}</Text>
              </View>
              <View id={`${block.id}_confirm`} className='sdui-overlay-button is-primary' onClick={() => emit(props.confirm_action || block.action, {})}>
                <Text>{String(props.confirm_text || '确认')}</Text>
              </View>
            </View>
          )}

          {presentation === 'action_sheet' && (
            <View id={`${block.id}_cancel`} className='sdui-overlay-sheet-cancel' onClick={() => emit(props.cancel_action, {})}>
              <Text>{String(props.cancel_text || '取消')}</Text>
            </View>
          )}
        </View>
      </View>
      </RootPortal>
    </View>
  )
}