// FormBlock.tsx
import React, { useEffect, useState } from 'react'
import { Input, Picker, Switch, Text, Textarea, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { BlockAction, BlockItem } from '../../types/sdui'
import { blockClassName } from './style'

interface FormBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

// 表单字段协议；未声明 fields 时自动回退到单字段查询模式。
interface FormField {
  // 字段键，提交时作为载荷键。
  key?: string
  // 字段标签。
  label?: string
  // 字段类型。
  type?: 'text' | 'textarea' | 'number' | 'switch' | 'select' | 'date'
  // 输入提示。
  placeholder?: string
  // 是否必填。
  required?: boolean
  // 默认值。
  default?: any
  // 当前值。
  value?: any
  // select 选项。
  options?: Array<{ value?: any; label?: string }>
  // 数值范围。
  min?: number
  max?: number
  step?: number
  // textarea 行数提示。
  rows?: number
  // 辅助说明。
  hint?: string
  // 是否禁用。
  disabled?: boolean
}

// 兼容 select 选项既可以是 {value,label}，也可以是纯字符串。
const optionValue = (option: any): any => typeof option === 'string' ? option : option?.value

// 兼容 select 选项既可以是 {value,label}，也可以是纯字符串。
const optionLabel = (option: any): string => typeof option === 'string'
  ? option
  : String(option?.label ?? option?.value ?? '')

/** 读取字段默认值；select 未显式指定时使用首个选项。 */
const initialFieldValue = (field: FormField): any => {
  if (field.value !== undefined) return field.value
  if (field.default !== undefined) return field.default
  if (field.type === 'select' && Array.isArray(field.options) && field.options.length > 0) {
    return optionValue(field.options[0])
  }
  if (field.type === 'switch') return false
  return ''
}

/** 将协议中的 fields 统一解析为数组，兼容 JSON 字符串。 */
const parseFields = (raw: any): FormField[] => {
  if (Array.isArray(raw)) return raw.filter((item) => item && typeof item === 'object')
  if (typeof raw === 'string' && raw.trim()) {
    try {
      const parsed = JSON.parse(raw)
      return Array.isArray(parsed) ? parsed.filter((item) => item && typeof item === 'object') : []
    } catch (error) {
      return []
    }
  }
  return []
}

/**
 * 通用表单积木：
 * - 声明 fields 时渲染多字段表单，支持文本、多行文本、数字、开关、选择和日期；
 * - 未声明 fields 时保持原有单字段查询行为，兼容既有页面协议。
 */
export const FormBlock: React.FC<FormBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const fields = parseFields(props.fields)
  const isMultiField = fields.length > 0
  const title = String(props.title || '信息查询入口')
  const btnText = String(props.btn_text || props.submit_text || (isMultiField ? '提交' : '立即查询'))

  // 单字段模式沿用既有状态。
  const legacyInitial = String(props.value ?? props.initial_value ?? props.default_value ?? '')
  const [legacyValue, setLegacyValue] = useState<string>(legacyInitial)
  // 多字段模式使用以 key 为索引的值对象。
  const [values, setValues] = useState<Record<string, any>>(() => {
    const initial: Record<string, any> = {}
    fields.forEach((field, index) => {
      const key = String(field.key || `field_${index + 1}`)
      initial[key] = initialFieldValue(field)
    })
    return initial
  })

  useEffect(() => {
    if (props.value !== undefined) {
      setLegacyValue(String(props.value ?? ''))
    }
  }, [props.value])

  // 协议切换字段集合时重置表单值，避免残留上一份表单的状态。
  const fieldsSignature = fields.map((field) => String(field.key || '')).join('|')
  useEffect(() => {
    if (!isMultiField) return
    const initial: Record<string, any> = {}
    fields.forEach((field, index) => {
      const key = String(field.key || `field_${index + 1}`)
      initial[key] = initialFieldValue(field)
    })
    setValues(initial)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fieldsSignature])

  const fieldKey = (field: FormField, index: number) => String(field.key || `field_${index + 1}`)

  const updateField = (key: string, value: any) => {
    setValues((prev) => ({ ...prev, [key]: value }))
  }

  /** 校验必填字段并返回首个错误提示。 */
  const validate = (): string => {
    for (let index = 0; index < fields.length; index += 1) {
      const field = fields[index]
      if (!field.required) continue
      const key = fieldKey(field, index)
      const value = values[key]
      if (field.type === 'switch') continue
      if (value === undefined || value === null || String(value).trim() === '') {
        return `${field.label || key}不能为空`
      }
    }
    return ''
  }

  const collectValues = (): Record<string, any> => {
    const result: Record<string, any> = {}
    fields.forEach((field, index) => {
      const key = fieldKey(field, index)
      result[key] = values[key]
    })
    return result
  }

  const submitMultiField = () => {
    const error = validate()
    if (error) {
      Taro.showToast({ title: error, icon: 'none' })
      return
    }
    const formValues = collectValues()
    const targetAction = block.action || props.submit_action || props.action
    const extraContext = {
      item: formValues,
      actionPayload: formValues,
      form_values: formValues,
      values: formValues,
      state: { ...(props.state || {}), form: formValues },
      __event: 'submit'
    }
    if (block.events?.submit || props.events?.submit) {
      onAction?.(undefined, extraContext)
      return
    }
    if (targetAction) {
      onAction?.({
        ...targetAction,
        payload: {
          ...(targetAction.payload || {}),
          ...formValues,
          values: formValues,
          form_values: formValues
        }
      }, extraContext)
      return
    }
    Taro.showToast({ title: '表单已提交', icon: 'success' })
  }

  const submitLegacy = (e?: any) => {
    e?.stopPropagation?.()
    if (!onAction) return

    const trimmed = legacyValue.trim()
    if (!trimmed && props.allow_empty !== true) {
      Taro.showToast({
        title: props.empty_toast || props.placeholder || '请输入查询内容',
        icon: 'none'
      })
      return
    }
    const extraContext = {
      item: { query_value: legacyValue, code: legacyValue },
      actionPayload: { query_value: legacyValue, code: legacyValue },
      query_value: legacyValue,
      code: legacyValue,
      state: { query_value: legacyValue, code: legacyValue, form: { code: legacyValue, query_value: legacyValue } }
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
          query_value: legacyValue,
          code: legacyValue
        }
      }, extraContext)
    }
  }

  /** 渲染单个字段控件。 */
  const renderField = (field: FormField, index: number) => {
    const key = fieldKey(field, index)
    const type = field.type || 'text'
    const value = values[key]
    if (type === 'switch') {
      return (
        <View className='sdui-form-switch-row' key={key}>
          <Text className='sdui-form-switch-label'>{field.label || key}</Text>
          <Switch
            checked={!!value}
            disabled={field.disabled === true}
            onChange={(event) => updateField(key, !!event.detail.value)}
          />
        </View>
      )
    }
    if (type === 'select') {
      const options = Array.isArray(field.options) ? field.options : []
      return (
        <View className='sdui-form-field' key={key}>
          <Text className='sdui-form-field-label'>{field.label || key}{field.required ? ' *' : ''}</Text>
          <View className='sdui-form-option-list'>
            {options.map((option, optionIndex) => (
              <View
                key={`${key}_${optionIndex}`}
                className={`sdui-form-option ${String(optionValue(option)) === String(value) ? 'is-active' : ''}`}
                onClick={() => updateField(key, optionValue(option))}
              >
                <Text>{optionLabel(option)}</Text>
              </View>
            ))}
          </View>
          {field.hint && <Text className='sdui-form-field-hint'>{field.hint}</Text>}
        </View>
      )
    }
    if (type === 'date') {
      return (
        <View className='sdui-form-field' key={key}>
          <Text className='sdui-form-field-label'>{field.label || key}{field.required ? ' *' : ''}</Text>
          <Picker
            mode='date'
            value={String(value || '')}
            onChange={(event) => updateField(key, String(event.detail.value || ''))}
          >
            <View className='sdui-form-date'>{String(value || field.placeholder || '请选择日期')}</View>
          </Picker>
          {field.hint && <Text className='sdui-form-field-hint'>{field.hint}</Text>}
        </View>
      )
    }
    if (type === 'textarea') {
      return (
        <View className='sdui-form-field' key={key}>
          <Text className='sdui-form-field-label'>{field.label || key}{field.required ? ' *' : ''}</Text>
          <Textarea
            className='sdui-form-textarea'
            value={String(value ?? '')}
            placeholder={field.placeholder || `请输入${field.label || ''}`}
            maxlength={-1}
            cursorSpacing={24}
            adjustPosition
            onInput={(event) => updateField(key, event.detail.value)}
          />
          {field.hint && <Text className='sdui-form-field-hint'>{field.hint}</Text>}
        </View>
      )
    }
    const numeric = type === 'number'
    return (
      <View className='sdui-form-field' key={key}>
        <Text className='sdui-form-field-label'>{field.label || key}{field.required ? ' *' : ''}</Text>
        <Input
          className='sdui-form-input'
          type={numeric ? 'digit' : 'text'}
          value={String(value ?? '')}
          disabled={field.disabled === true}
          placeholder={field.placeholder || `请输入${field.label || ''}`}
          cursorSpacing={24}
          adjustPosition
          onInput={(event) => updateField(key, numeric ? event.detail.value : event.detail.value)}
        />
        {field.hint && <Text className='sdui-form-field-hint'>{field.hint}</Text>}
      </View>
    )
  }

  if (isMultiField) {
    return (
      <View className={blockClassName('sdui-form-block sdui-form-multi', block.style)}>
        <Text className='form-title'>{title}</Text>
        {props.description && <Text className='sdui-form-description'>{String(props.description)}</Text>}
        {fields.map(renderField)}
        <View id={`${block.id}_submit`} className={`form-submit-btn ${props.disabled === true ? 'is-disabled' : ''}`} onClick={props.disabled === true ? undefined : submitMultiField}>
          <Text>{btnText}</Text>
        </View>
      </View>
    )
  }

  const inputLabel = props.input_label
  const placeholder = props.placeholder || '请输入查询关键词'
  return (
    <View
      className={blockClassName('sdui-form-block', block.style)}
    >
      <Text className="form-title">{title}</Text>
      {inputLabel && (
        <Text className="form-input-label">
          {inputLabel}
        </Text>
      )}
      <Input
        className="form-input-field"
        placeholder={placeholder}
        placeholderClass="form-input-placeholder"
        value={legacyValue}
        cursorSpacing={24}
        adjustPosition
        onInput={(e) => setLegacyValue(e.detail.value)}
        onConfirm={submitLegacy}
        confirmType="search"
      />
      <View id={`${block.id}_submit`} className="form-submit-btn" onClick={submitLegacy}>
        <Text>{btnText}</Text>
      </View>
    </View>
  )
}