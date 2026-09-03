import React, { useState } from 'react'
import { View, Text, Input } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface FormBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
}

/**
 * 考分/数据查询与输入表单积木 (FormBlock)
 */
export const FormBlock: React.FC<FormBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '信息查询入口'
  const placeholder = props.placeholder || '请输入查询关键词'
  const btnText = props.btn_text || '立即查询'

  const [inputVal, setInputVal] = useState<string>('')

  const handleSubmit = () => {
    if (block.action && onAction) {
      onAction({
        ...block.action,
        payload: {
          ...block.action.payload,
          query_value: inputVal
        }
      })
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
      <Input
        className="form-input-field"
        placeholder={placeholder}
        placeholderStyle="color: rgba(255,255,255,0.3);"
        value={inputVal}
        onInput={(e) => setInputVal(e.detail.value)}
      />
      <View className="form-submit-btn" onClick={handleSubmit}>
        <Text>{btnText}</Text>
      </View>
    </View>
  )
}
