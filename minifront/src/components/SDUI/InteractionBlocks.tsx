// InteractionBlocks.tsx
import React, { useEffect, useState } from 'react'
import { Image, Input, RootPortal, Switch, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { BlockAction, BlockItem } from '../../types/sdui'
import { request } from '../../utils/request'

interface InteractionBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: Record<string, any>
}

const asAction = (value: any): BlockAction | undefined => {
  if (value && typeof value === 'object' && typeof value.type === 'string') return value as BlockAction
  return undefined
}

const listOf = (value: any): any[] => Array.isArray(value) ? value : []

const emit = (
  onAction: InteractionBlockProps['onAction'],
  action: BlockAction | undefined,
  payload: Record<string, any>,
  item: Record<string, any>
) => {
  if (!action) {
    Taro.showToast({ title: '该操作暂未配置', icon: 'none' })
    return
  }
  onAction?.(action, { item, actionPayload: payload })
}

/** 地址簿：选择、编辑、新增和删除动作均由协议配置。 */
export const AddressBookBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const items = listOf(props.items)
  const selectedID = String(props.selected_id || props.selected || '')

  return (
    <View className='sdui-address-book'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      {items.length === 0 && <Text className='sdui-commerce-empty'>{String(props.empty_text || '暂无收货地址')}</Text>}
      {items.map((item: any, index: number) => {
        const id = String(item.id || item.address_id || index)
        return (
          <View
            key={id}
            className={`sdui-address-row ${selectedID === id || item.is_default ? 'is-selected' : ''}`}
            onClick={() => emit(onAction, asAction(props.select_action) || asAction(block.action), { id, address_id: id, item, index }, item)}
          >
            <View className='sdui-address-copy'>
              <View className='sdui-address-head'>
                <Text className='sdui-address-name'>{String(item.name || '')}</Text>
                <Text className='sdui-address-phone'>{String(item.phone || '')}</Text>
                {item.is_default && <Text className='sdui-address-default'>默认</Text>}
                {item.tag && <Text className='sdui-address-tag'>{String(item.tag)}</Text>}
              </View>
              <Text className='sdui-address-detail'>{[item.region, item.detail].filter(Boolean).join(' ')}</Text>
            </View>
            {props.edit_action && (
              <Text className='sdui-address-op' onClick={(event) => { event?.stopPropagation?.(); emit(onAction, asAction(props.edit_action), { id, address_id: id, item, index }, item) }}>编辑</Text>
            )}
            {props.delete_action && (
              <Text className='sdui-address-op is-danger' onClick={(event) => { event?.stopPropagation?.(); emit(onAction, asAction(props.delete_action), { id, address_id: id, item, index }, item) }}>删除</Text>
            )}
          </View>
        )
      })}
      {props.add_action && (
        <View className='sdui-address-add' onClick={() => emit(onAction, asAction(props.add_action), {}, props)}>
          <Text>{String(props.add_text || '+ 新增地址')}</Text>
        </View>
      )}
    </View>
  )
}

/** 地址新增/编辑表单：提交 name/phone/region/detail/is_default。 */
export const AddressFormBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const seed = props.values && typeof props.values === 'object' ? props.values : {}
  const [name, setName] = useState(String(seed.name || props.name || ''))
  const [phone, setPhone] = useState(String(seed.phone || props.phone || ''))
  const [region, setRegion] = useState(String(seed.region || props.region || ''))
  const [detail, setDetail] = useState(String(seed.detail || props.detail || ''))
  const [isDefault, setIsDefault] = useState(seed.is_default === true || props.is_default === true)

  useEffect(() => {
    if (!props.values || typeof props.values !== 'object') return
    setName(String(props.values.name || ''))
    setPhone(String(props.values.phone || ''))
    setRegion(String(props.values.region || ''))
    setDetail(String(props.values.detail || ''))
    setIsDefault(props.values.is_default === true)
  }, [props.values])

  const submit = () => {
    if (!name.trim()) {
      Taro.showToast({ title: '请填写收货人', icon: 'none' })
      return
    }
    if (!/^1[3-9]\d{9}$/.test(phone.trim()) && !/^0\d{2,3}-?\d{7,8}$/.test(phone.trim())) {
      Taro.showToast({ title: '请填写有效联系电话', icon: 'none' })
      return
    }
    if (!region.trim() || !detail.trim()) {
      Taro.showToast({ title: '请填写完整地址', icon: 'none' })
      return
    }
    const payload = { name, phone, region, detail, is_default: isDefault }
    const action: BlockAction | undefined = asAction(block.action)
      || asAction(props.submit_action)
      || { type: 'request_data', require_auth: true, payload: { endpoint: 'address.save', body: payload } }
    emit(onAction, action, payload, { ...props, ...payload })
  }

  return (
    <View className='sdui-address-form'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      <View className='sdui-form-field'>
        <Text className='sdui-form-field-label'>收货人 *</Text>
        <Input className='sdui-form-input' cursorSpacing={24} adjustPosition value={name} onInput={(event) => setName(event.detail.value)} placeholder='请输入收货人姓名' />
      </View>
      <View className='sdui-form-field'>
        <Text className='sdui-form-field-label'>联系电话 *</Text>
        <Input className='sdui-form-input' cursorSpacing={24} adjustPosition type='number' value={phone} onInput={(event) => setPhone(event.detail.value)} placeholder='请输入手机号或座机' />
      </View>
      <View className='sdui-form-field'>
        <Text className='sdui-form-field-label'>所在地区 *</Text>
        <Input className='sdui-form-input' cursorSpacing={24} adjustPosition value={region} onInput={(event) => setRegion(event.detail.value)} placeholder='省 / 市 / 区' />
      </View>
      <View className='sdui-form-field'>
        <Text className='sdui-form-field-label'>详细地址 *</Text>
        <Input className='sdui-form-input' cursorSpacing={24} adjustPosition value={detail} onInput={(event) => setDetail(event.detail.value)} placeholder='街道、门牌号' />
      </View>
      {props.show_default !== false && (
        <View className='sdui-form-switch-row'>
          <Text className='sdui-form-switch-label'>设为默认地址</Text>
          <Switch checked={isDefault} onChange={(event) => setIsDefault(!!event.detail.value)} />
        </View>
      )}
      <View className='sdui-commerce-submit' onClick={submit}>
        <Text>{String(props.submit_text || '保存地址')}</Text>
      </View>
    </View>
  )
}

/** 横向筛选栏：点击选项后携带 key/value/active 触发动作。 */
export const FilterBarBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const filters = listOf(props.filters).length > 0 ? listOf(props.filters) : listOf(props.groups)
  const [active, setActive] = useState<Record<string, any>>(
    props.active && typeof props.active === 'object' ? { ...props.active } : {}
  )

  useEffect(() => {
    if (props.active && typeof props.active === 'object') setActive({ ...props.active })
  }, [props.active])

  const choose = (filterKey: string, value: any) => {
    const next = { ...active }
    if (next[filterKey] === value) {
      delete next[filterKey]
    } else {
      next[filterKey] = value
    }
    setActive(next)
    emit(onAction, asAction(props.change_action) || asAction(block.action), { key: filterKey, value, active: next, filters: next }, { ...props, active: next })
  }

  return (
    <View className='sdui-filter-bar'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      {filters.map((filter: any, index: number) => {
        const filterKey = String(filter.key || index)
        return (
          <View className='sdui-filter-group' key={filterKey}>
            {filter.label && <Text className='sdui-filter-label'>{String(filter.label)}</Text>}
            <View className='sdui-filter-options'>
              <View
                className={`sdui-filter-chip ${active[filterKey] === undefined ? 'is-active' : ''}`}
                onClick={() => choose(filterKey, undefined)}
              >
                <Text>{String(props.all_label || '全部')}</Text>
              </View>
              {listOf(filter.options).map((option: any, optionIndex: number) => (
                <View
                  key={`${filterKey}_${optionIndex}`}
                  className={`sdui-filter-chip ${String(active[filterKey]) === String(option.value) ? 'is-active' : ''}`}
                  onClick={() => choose(filterKey, option.value)}
                >
                  <Text>{String(option.label ?? option.value ?? '')}</Text>
                </View>
              ))}
            </View>
          </View>
        )
      })}
    </View>
  )
}

/** 底部筛选面板：多组条件、确认与重置。 */
export const FilterSheetBlock: React.FC<InteractionBlockProps> = ({ block, onAction, context }) => {
  const props = block.props || {}
  const groups = listOf(props.groups).length > 0 ? listOf(props.groups) : listOf(props.filters)
  const [open, setOpen] = useState(props.open === true)
  // 协议显式下发 open 时同步弹出状态，未声明时由触发按钮和确定/重置按钮控制
  useEffect(() => {
    if (typeof props.open === 'boolean') setOpen(props.open)
  }, [props.open])
  // 筛选面板打开期间同步锁定底层滚动
  useEffect(() => {
    context?.setOverlayOpen?.(open)
    return () => context?.setOverlayOpen?.(false)
  }, [open, context?.setOverlayOpen])
  const [selected, setSelected] = useState<Record<string, any>>(
    props.selected && typeof props.selected === 'object' ? { ...props.selected } : {}
  )

  const toggle = (key: string, value: any) => {
    setSelected((prev) => {
      const next = { ...prev }
      if (next[key] === value) delete next[key]
      else next[key] = value
      return next
    })
  }

  const confirm = () => {
    emit(onAction, asAction(props.confirm_action) || asAction(block.action), { selected, filters: selected }, { ...props, selected })
    setOpen(false)
  }

  const reset = () => {
    setSelected({})
    emit(onAction, asAction(props.reset_action), { selected: {}, filters: {} }, props)
  }

  if (!open) {
    return (
      <View id={`${block.id}_trigger`} className='sdui-filter-sheet-trigger' onClick={() => setOpen(true)}>
        <Text>{String(props.trigger_text || props.title || '筛选')}</Text>
      </View>
    )
  }

  return (
    <View className='sdui-overlay-root'>
      <RootPortal enable>
      <View className='sdui-overlay-mask is-bottom' style={{ ['--sdui-font-scale' as any]: String(context?.fontScale || 1) }} onClick={() => setOpen(false)}>
        <View className='sdui-overlay-panel is-bottom_sheet' onClick={(event) => event?.stopPropagation?.()}>
          <View className='sdui-overlay-header'>
            <Text className='sdui-overlay-title'>{String(props.title || '筛选条件')}</Text>
          </View>
          {groups.map((group: any, index: number) => {
            const groupKey = String(group.key || index)
            return (
              <View className='sdui-filter-group' key={groupKey}>
                {group.label && <Text className='sdui-filter-label'>{String(group.label)}</Text>}
                <View className='sdui-filter-options'>
                  {listOf(group.options).map((option: any, optionIndex: number) => (
                    <View
                      key={`${groupKey}_${optionIndex}`}
                      className={`sdui-filter-chip ${String(selected[groupKey]) === String(option.value) ? 'is-active' : ''}`}
                      onClick={() => toggle(groupKey, option.value)}
                    >
                      <Text>{String(option.label ?? option.value ?? '')}</Text>
                    </View>
                  ))}
                </View>
              </View>
            )
          })}
          <View className='sdui-overlay-footer'>
            {props.show_reset !== false && (
              <View id={`${block.id}_reset`} className='sdui-overlay-button is-secondary' onClick={reset}>
                <Text>{String(props.reset_text || '重置')}</Text>
              </View>
            )}
            <View id={`${block.id}_confirm`} className='sdui-overlay-button is-primary' onClick={confirm}>
              <Text>{String(props.confirm_text || '确定')}</Text>
            </View>
          </View>
        </View>
      </View>
      </RootPortal>
    </View>
  )
}

/** 星级评分：只读展示或点击评分并触发动作。 */
export const RatingBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const max = Math.max(1, Number(props.max || 5))
  const readonly = props.readonly === true
  const [value, setValue] = useState(Number(props.value || 0))

  useEffect(() => {
    if (props.value !== undefined) setValue(Number(props.value))
  }, [props.value])

  const choose = (next: number) => {
    if (readonly) return
    setValue(next)
    emit(onAction, asAction(props.change_action) || asAction(block.action), { value: next, rating: next }, { ...props, value: next })
  }

  return (
    <View className='sdui-rating'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      <View className='sdui-rating-stars'>
        {Array.from({ length: max }).map((_, index) => (
          <Text
            key={index}
            className={`sdui-rating-star ${index < value ? 'is-active' : ''}`}
            onClick={() => choose(index + 1)}
          >{index < value ? '★' : '☆'}</Text>
        ))}
        {props.show_value !== false && <Text className='sdui-rating-value'>{value.toFixed(1)}</Text>}
      </View>
      {value > 0 && props.labels && Array.isArray(props.labels) && props.labels[Math.max(0, value - 1)] && (
        <Text className='sdui-rating-label'>{String(props.labels[Math.max(0, value - 1)])}</Text>
      )}
    </View>
  )
}

/** 评价汇总：均分、星级分布和评价列表。 */
export const ReviewSummaryBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const productID = Number(props.product_id || props.productId || 0)
  const sku = String(props.sku || '').trim()
  const autoLoad = props.auto_load !== false && (productID > 0 || sku !== '')
  const [remote, setRemote] = useState<Record<string, any> | null>(null)
  const [loading, setLoading] = useState<boolean>(false)
  const [loadError, setLoadError] = useState<string>('')

  // 商品评价由 review.list 公开端点加载，协议仍可用静态 items 直接渲染。
  useEffect(() => {
    if (!autoLoad) return
    let cancelled = false
    setLoading(true)
    setLoadError('')
    request<any>({
      url: '/api/v1/action/execute',
      method: 'POST',
      data: {
        endpoint: 'review.list',
        payload: {
          ...(productID > 0 ? { product_id: productID } : {}),
          ...(sku !== '' ? { sku } : {}),
          limit: Math.max(1, Math.min(50, Number(props.limit || 20)))
        }
      }
    })
      .then((result) => { if (!cancelled) setRemote(result && typeof result === 'object' ? result : null) })
      .catch((error) => { if (!cancelled) setLoadError(error?.message || '评价加载失败') })
      .then(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [autoLoad, productID, sku, props.limit])

  const data = remote || props
  const items = listOf(data.items)
  const distribution = Array.isArray(data.distribution)
    ? data.distribution
    : Object.entries(data.distribution && typeof data.distribution === 'object' ? data.distribution : {})
      .map(([stars, count]) => ({ stars: Number(stars), count: Number(count) }))
  const total = Number(data.total || items.length || 0)

  return (
    <View className='sdui-review-summary'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      {loading && !remote && <Text className='sdui-review-state'>评价加载中…</Text>}
      {!loading && loadError && <Text className='sdui-review-state is-error'>{loadError}</Text>}
      <View className='sdui-review-head'>
        <View className='sdui-review-score'>
          <Text className='sdui-review-average'>{Number(data.average || 0).toFixed(1)}</Text>
          <Text className='sdui-review-total'>共 {total} 条评价</Text>
        </View>
        <View className='sdui-review-bars'>
          {distribution.map((entry: any, index: number) => {
            const percent = total > 0 ? Math.min(100, Math.round((Number(entry.count || 0) / total) * 100)) : 0
            return (
              <View className='sdui-review-bar-row' key={index}>
                <Text className='sdui-review-bar-label'>{Number(entry.stars || entry[0] || 0)} 星</Text>
                <View className='sdui-review-bar'><View className='sdui-review-bar-fill' style={{ width: `${percent}%` }} /></View>
                <Text className='sdui-review-bar-count'>{Number(entry.count || entry[1] || 0)}</Text>
              </View>
            )
          })}
        </View>
      </View>
      {items.length === 0 && !loading && <Text className='sdui-commerce-empty'>{String(data.empty_text || props.empty_text || '暂无评价')}</Text>}
      {items.map((item: any, index: number) => (
        <View className='sdui-review-item' key={item.id || index}>
          <View className='sdui-review-item-head'>
            {item.avatar_url && <Image className='sdui-review-avatar' src={String(item.avatar_url)} mode='aspectFill' />}
            <Text className='sdui-review-author'>{String(item.author || item.name || '匿名用户')}</Text>
            <Text className='sdui-review-stars'>{'★'.repeat(Math.max(0, Math.min(5, Number(item.rating || 0))))}</Text>
            {item.time && <Text className='sdui-review-time'>{formatReviewTime(item.time)}</Text>}
          </View>
          {item.content && <Text className='sdui-review-content'>{String(item.content)}</Text>}
        </View>
      ))}
      {props.action_text && (
        <View className='sdui-review-action' onClick={() => emit(onAction, asAction(block.action) || asAction(props.action), {}, props)}>
          <Text>{String(props.action_text)}</Text>
        </View>
      )}
    </View>
  )
}

/** 将服务端 RFC3339 时间格式化为本地可读时间。 */
const formatReviewTime = (value: any): string => {
  const text = String(value || '')
  const date = new Date(text)
  if (!text || Number.isNaN(date.getTime())) return text
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** 通用资料卡：头像、名称、副标题、统计项和徽章。 */
export const ProfileCardBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const stats = listOf(props.stats)
  const badges = listOf(props.badges)
  return (
    <View className='sdui-profile-card' onClick={() => emit(onAction, asAction(block.action), {}, props)}>
      <View className='sdui-profile-head'>
        {props.avatar_url && <Image className='sdui-profile-avatar' src={String(props.avatar_url)} mode='aspectFill' />}
        <View className='sdui-profile-copy'>
          <Text className='sdui-profile-name'>{String(props.title || props.name || '')}</Text>
          {(props.subtitle || props.description) && <Text className='sdui-profile-subtitle'>{String(props.subtitle || props.description)}</Text>}
        </View>
      </View>
      {badges.length > 0 && (
        <View className='sdui-profile-badges'>
          {badges.map((badge: any, index: number) => <Text className='sdui-profile-badge' key={index}>{String(badge.label || badge)}</Text>)}
        </View>
      )}
      {stats.length > 0 && (
        <View className='sdui-profile-stats'>
          {stats.map((stat: any, index: number) => (
            <View className='sdui-profile-stat' key={index}>
              <Text className='sdui-profile-stat-value'>{String(stat.value ?? '')}</Text>
              <Text className='sdui-profile-stat-label'>{String(stat.label ?? '')}</Text>
            </View>
          ))}
        </View>
      )}
      {props.action_text && <Text className='sdui-profile-action'>{String(props.action_text)}</Text>}
    </View>
  )
}

/** 用户主页头部：背景、头像、名称、元信息和徽章。 */
export const UserHeaderBlock: React.FC<InteractionBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const badges = listOf(props.badges)
  return (
    <View className='sdui-user-header' onClick={() => emit(onAction, asAction(block.action), {}, props)}>
      {props.background_url && <Image className='sdui-user-header-bg' src={String(props.background_url)} mode='aspectFill' />}
      <View className='sdui-user-header-body'>
        {props.avatar_url && <Image className='sdui-user-header-avatar' src={String(props.avatar_url)} mode='aspectFill' />}
        <Text className='sdui-user-header-name'>{String(props.name || props.title || '')}</Text>
        {props.meta && <Text className='sdui-user-header-meta'>{String(props.meta)}</Text>}
        {badges.length > 0 && (
          <View className='sdui-user-header-badges'>
            {badges.map((badge: any, index: number) => <Text className='sdui-user-header-badge' key={index}>{String(badge.label || badge)}</Text>)}
          </View>
        )}
      </View>
    </View>
  )
}