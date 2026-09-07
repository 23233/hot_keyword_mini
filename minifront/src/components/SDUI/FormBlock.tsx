// FormBlock.tsx
import React, { useState } from 'react'
import { View, Text, Input } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { BlockItem, BlockAction } from '../../types/sdui'

interface FormBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

/**
 * 考分/数据查询与输入表单积木 (FormBlock)
 */
export const FormBlock: React.FC<FormBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '信息查询入口'
  const inputLabel = props.input_label
  const placeholder = props.placeholder || '请输入查询关键词'
  const btnText = props.btn_text || '立即查询'
  const initial = String(props.value ?? props.initial_value ?? props.default_value ?? '')
  const [inputVal, setInputVal] = useState<string>(initial)

  React.useEffect(() => {
    if (props.value !== undefined) {
      setInputVal(String(props.value ?? ''))
    }
  }, [props.value])

  const handleSubmit = (e?: any) => {
    e?.stopPropagation?.()
    if (!onAction) return

    const trimmed = inputVal.trim()
    if (!trimmed && props.allow_empty !== true) {
      Taro.showToast({
        title: props.empty_toast || placeholder || '请输入查询内容',
        icon: 'none'
      })
      return
    }
    const extraContext = {
      item: { query_value: inputVal, code: inputVal },
      actionPayload: { query_value: inputVal, code: inputVal },
      query_value: inputVal,
      code: inputVal,
      state: { query_value: inputVal, code: inputVal, form: { code: inputVal, query_value: inputVal } }
    }
    const targetAction = block.action || props.action
    if (block.events?.submit || props.events?.submit) {
      onAction(undefined, { ...extraContext, __event: 'submit' })
    } else if (block.events?.tap || props.events?.tap) {
      onAction(undefined, { ...extraContext, __event: 'tap' })
    } else if (targetAction) {
      onAction({
        ...targetAction,
        payload: {
          ...(targetAction.payload || {}),
          query_value: inputVal,
          code: inputVal
        }
      }, extraContext)
    }
  }

  return (
    <View
      className="sdui-form-block"
      style={{
        borderRadius: block.style?.border_radius || '28rpx'
      }}
    >
      <Text className="form-title">{title}</Text>
      {inputLabel && (
        <Text style={{ fontSize: '24rpx', color: 'rgba(255,255,255,0.7)', marginBottom: '12rpx', display: 'block' }}>
          {inputLabel}
        </Text>
      )}
      <Input
        className="form-input-field"
        placeholder={placeholder}
        placeholderStyle="color: rgba(255,255,255,0.3);"
        value={inputVal}
        onInput={(e) => setInputVal(e.detail.value)}
        onConfirm={handleSubmit}
        confirmType="search"
      />
      <View className="form-submit-btn" onClick={handleSubmit}>
        <Text>{btnText}</Text>
      </View>
    </View>
  )
}
