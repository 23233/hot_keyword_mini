// CommerceBlocks.tsx
import React, { useEffect, useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { BlockAction, BlockItem } from '../../types/sdui'
import { cartTotals, loadCart } from '../../utils/cart'
import { request } from '../../utils/request'

interface CommerceBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: Record<string, any>
}

// 将协议中的动作对象安全转换为 BlockAction。
const asAction = (value: any): BlockAction | undefined => {
  if (value && typeof value === 'object' && typeof value.type === 'string') return value as BlockAction
  return undefined
}

// 列表字段兼容数组与空值。
const listOf = (value: any): any[] => Array.isArray(value) ? value : []

// 金额统一格式化为人民币展示文本。
const moneyText = (fen: unknown): string => {
  const value = Number(fen)
  if (!Number.isFinite(value)) return ''
  return `¥${(value / 100).toFixed(2)}`
}

// 触发动作并附带当前项上下文。
const emit = (
  onAction: CommerceBlockProps['onAction'],
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

/** SKU 选择器：规格、价格、库存、数量与提交动作全部由协议驱动。 */
export const SKUSelectorBlock: React.FC<CommerceBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const options = listOf(props.skus).length > 0 ? listOf(props.skus) : listOf(props.options)
  const optionKey = (option: any, index: number) => String(option.sku || option.id || option.value || option.label || index)
  const selectedSeed = props.selected ?? props.default
  const initialIndex = Math.max(0, options.findIndex((option: any, index: number) => optionKey(option, index) === String(selectedSeed)))
  const [selectedIndex, setSelectedIndex] = useState(initialIndex)
  const [quantity, setQuantity] = useState(Math.max(1, Number(props.quantity || props.default_quantity || 1)))
  const min = Math.max(1, Number(props.min || 1))
  const max = Math.max(min, Number(props.max || 99))
  const selected = options[selectedIndex] || {}
  const stock = Number(selected.stock)
  const soldOut = Number.isFinite(stock) && stock <= 0
  const priceText = selected.price_text
    || (selected.price_fen !== undefined ? moneyText(selected.price_fen) : String(selected.price ?? ''))

  const submit = () => {
    if (soldOut) {
      Taro.showToast({ title: '该规格暂时缺货', icon: 'none' })
      return
    }
    const action = asAction(block.action) || asAction(props.submit_action)
    emit(onAction, action, {
      sku: selected.sku || selected.id || selected.value || '',
      quantity,
      selected,
      price_fen: selected.price_fen,
      price_text: priceText
    }, { ...props, selected, quantity })
  }

  return (
    <View className='sdui-sku-selector'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      {options.length === 0 && <Text className='sdui-commerce-empty'>{String(props.empty_text || '暂无可选规格')}</Text>}
      <View className='sdui-sku-options'>
        {options.map((option: any, index: number) => {
          const disabled = Number.isFinite(Number(option.stock)) && Number(option.stock) <= 0
          return (
            <View
              key={optionKey(option, index)}
              className={`sdui-sku-option ${index === selectedIndex ? 'is-active' : ''} ${disabled ? 'is-disabled' : ''}`}
              onClick={() => { if (!disabled) setSelectedIndex(index) }}
            >
              <Text className='sdui-sku-option-label'>{String(option.label || option.title || option.sku || '')}</Text>
              {(option.price_text || option.price_fen || option.price) && (
                <Text className='sdui-sku-option-price'>{String(option.price_text || moneyText(option.price_fen ?? option.price))}</Text>
              )}
              {disabled && <Text className='sdui-sku-option-stock'>缺货</Text>}
            </View>
          )
        })}
      </View>
      <View className='sdui-sku-footer'>
        <View className='sdui-quantity-inline'>
          <Text className='sdui-quantity-label'>{String(props.quantity_label || '数量')}</Text>
          <View className={`sdui-quantity-control ${soldOut ? 'is-disabled' : ''}`}>
            <View className='sdui-quantity-button' onClick={() => { if (!soldOut) setQuantity(Math.max(min, quantity - 1)) }}>−</View>
            <Text className='sdui-quantity-value'>{quantity}</Text>
            <View className='sdui-quantity-button' onClick={() => { if (!soldOut) setQuantity(Math.min(max, quantity + 1)) }}>+</View>
          </View>
        </View>
        {priceText && <Text className='sdui-commerce-price'>{priceText}</Text>}
      </View>
      <View className={`sdui-commerce-submit ${soldOut ? 'is-disabled' : ''}`} onClick={submit}>
        <Text>{String(props.submit_text || '立即购买')}</Text>
      </View>
    </View>
  )
}

/** 数量加减器：变更后携带 value / quantity 触发动作。 */
export const QuantityStepperBlock: React.FC<CommerceBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const min = Number(props.min ?? 1)
  const max = Number(props.max ?? 99)
  const step = Math.max(1, Number(props.step || 1))
  const [value, setValue] = useState<number>(Number(props.value ?? props.default ?? min))

  useEffect(() => {
    if (props.value !== undefined) setValue(Number(props.value))
  }, [props.value])

  const change = (delta: number) => {
    const next = Math.min(max, Math.max(min, value + delta))
    if (next === value) return
    setValue(next)
    const action = asAction(props.change_action) || asAction(block.action)
    emit(onAction, action, { value: next, quantity: next, delta }, { ...props, value: next })
  }

  return (
    <View className='sdui-quantity-stepper'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      <View className='sdui-quantity-control is-large'>
        <View className={`sdui-quantity-button ${value <= min ? 'is-disabled' : ''}`} onClick={() => change(-step)}>−</View>
        <Text className='sdui-quantity-value'>{value}</Text>
        <View className={`sdui-quantity-button ${value >= max ? 'is-disabled' : ''}`} onClick={() => change(step)}>+</View>
      </View>
    </View>
  )
}

/** 购物车商品列表：支持协议数据与本地购物车两种来源，数量增减、删除和单商品动作均可配置。 */
export const CartItemListBlock: React.FC<CommerceBlockProps> = ({ block, onAction, context }) => {
  const props = block.props || {}
  const propItems = listOf(props.items).length > 0 ? listOf(props.items) : listOf(props.cart_items)
  const stateCart = Array.isArray(context?.state?.cart) ? context.state.cart : null
  const [storedItems, setStoredItems] = useState<any[]>(() => (props.source === 'local' ? loadCart() : []))
  useEffect(() => {
    if (props.source === 'local') setStoredItems(loadCart())
  }, [props.source])
  const items = props.source === 'local' ? (stateCart || storedItems) : propItems
  // 同一页面的 cart_summary 开启 server_quote 后会把服务端报价写入 cart_quote，这里优先展示服务端标题与价格。
  const quoteMap = new Map<string, any>()
  if (Array.isArray(context?.state?.cart_quote?.items)) {
    context.state.cart_quote.items.forEach((quoted: any) => {
      if (quoted?.sku) quoteMap.set(String(quoted.sku), quoted)
    })
  }
  const quotedOf = (item: any) => quoteMap.get(String(item?.sku || ''))
  const itemKey = (item: any, index: number) => String(item.sku || item.id || index)

  const changeQuantity = (item: any, delta: number, index: number) => {
    const next = Math.max(0, Number(item.quantity || 0) + delta)
    if (next === 0) {
      emit(onAction, asAction(props.remove_action) || asAction(block.action) || { type: 'remove_from_cart', payload: { sku: item.sku || item.id } }, { sku: item.sku || item.id, quantity: 0, item, index }, item)
      return
    }
    const action = asAction(delta > 0 ? props.increment_action : props.decrement_action)
      || asAction(block.action)
      || { type: 'update_cart_quantity', payload: { sku: item.sku || item.id, quantity: next } }
    emit(onAction, action, {
      sku: item.sku || item.id,
      quantity: next,
      delta,
      item,
      index
    }, item)
  }

  return (
    <View className='sdui-cart-list'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      {items.length === 0 && <Text className='sdui-commerce-empty'>{String(props.empty_text || '购物车为空')}</Text>}
      {items.map((item: any, index: number) => (
        <View className='sdui-cart-row' key={itemKey(item, index)}>
          {item.image_url && <Image className='sdui-cart-image' src={String(item.image_url)} mode='aspectFill' />}
          <View className='sdui-cart-copy' onClick={() => emit(onAction, asAction(props.item_action) || asAction(block.action), { sku: item.sku || item.id, item, index }, item)}>
            <Text className='sdui-cart-title'>{String(quotedOf(item)?.title || item.title || item.name || '')}</Text>
            {item.spec && <Text className='sdui-cart-spec'>{String(item.spec)}</Text>}
            <Text className='sdui-cart-price'>{String(quotedOf(item)?.price_text || item.price_text || ((quotedOf(item)?.price_fen ?? item.price_fen) !== undefined ? moneyText(quotedOf(item)?.price_fen ?? item.price_fen) : item.price ?? ''))}</Text>
          </View>
          <View className='sdui-cart-actions'>
            <View className='sdui-quantity-control is-small'>
              <View className='sdui-quantity-button' onClick={() => changeQuantity(item, -1, index)}>−</View>
              <Text className='sdui-quantity-value'>{Number(item.quantity || 0)}</Text>
              <View className='sdui-quantity-button' onClick={() => changeQuantity(item, 1, index)}>+</View>
            </View>
            <Text
              className='sdui-cart-remove'
              onClick={() => emit(onAction, asAction(props.remove_action) || asAction(block.action) || { type: 'remove_from_cart', payload: { sku: item.sku || item.id } }, { sku: item.sku || item.id, quantity: 0, item, index }, item)}
            >删除</Text>
          </View>
        </View>
      ))}
      {props.clear_action && (
        <View className='sdui-cart-clear' onClick={() => emit(onAction, asAction(props.clear_action), {}, props)}>
          <Text>{String(props.clear_text || '清空购物车')}</Text>
        </View>
      )}
    </View>
  )
}

/** 购物车汇总：支持协议数据与本地购物车，金额可由协议传入或按 price_fen * quantity 计算。 */
export const CartSummaryBlock: React.FC<CommerceBlockProps> = ({ block, onAction, context }) => {
  const props = block.props || {}
  const stateCart = Array.isArray(context?.state?.cart) ? context.state.cart : null
  const [storedItems] = useState<any[]>(() => (props.source === 'local' ? loadCart() : []))
  const items = props.source === 'local' ? (stateCart || storedItems) : listOf(props.items)
  const totals = cartTotals(items)
  const quoteEnabled = props.server_quote === true
  const [quote, setQuote] = useState<Record<string, any> | null>(null)
  const [quoteError, setQuoteError] = useState<string>('')
  const itemsKey = items.map((item: any) => `${item?.sku || ''}:${Number(item?.quantity || 0)}`).join('|')

  // server_quote: 交由 cart.quote 按服务端商品配置重算金额，客户端价格仅作展示兜底。
  useEffect(() => {
    if (!quoteEnabled) return
    const payloadItems = items
      .filter((item: any) => item && item.sku && Number(item.quantity) > 0)
      .map((item: any) => ({ sku: String(item.sku), quantity: Number(item.quantity) }))
    if (payloadItems.length === 0) {
      setQuote(null)
      setQuoteError('')
      context?.updateState?.('cart_quote', null)
      return
    }
    let cancelled = false
    request<any>({
      url: '/api/v1/action/execute',
      method: 'POST',
      data: { endpoint: 'cart.quote', payload: { items: payloadItems } }
    })
      .then((result) => {
        if (cancelled) return
        const next = result && typeof result === 'object' ? result : null
        setQuote(next)
        setQuoteError('')
        context?.updateState?.('cart_quote', next)
      })
      .catch((error) => {
        if (cancelled) return
        setQuote(null)
        setQuoteError(error?.message || '价格校验失败，请重试')
        context?.updateState?.('cart_quote', null)
      })
    return () => { cancelled = true }
  }, [quoteEnabled, itemsKey])

  const count = quote?.count !== undefined ? Number(quote.count) : (props.count !== undefined ? Number(props.count) : totals.count)
  const computedFen = props.source === 'local' ? totals.totalFen : items.reduce((sum: number, item: any) => sum + Number(item.price_fen || 0) * Number(item.quantity || 0), 0)
  const totalFen = quote?.total_fen !== undefined ? Number(quote.total_fen) : computedFen
  const totalText = quote?.total_text || props.total_text || props.total || (totalFen > 0 ? moneyText(totalFen) : '¥0.00')
  const checkoutItems = Array.isArray(quote?.items) && quote.items.length > 0 ? quote.items : items

  return (
    <View className='sdui-cart-summary'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      <View className='sdui-cart-summary-row'>
        <Text className='sdui-cart-summary-label'>共 {count} 件</Text>
        <Text className='sdui-commerce-price'>{String(totalText)}</Text>
      </View>
      {props.note && <Text className='sdui-commerce-hint'>{String(props.note)}</Text>}
      {quoteError && <Text className='sdui-commerce-hint is-error'>{quoteError}</Text>}
      <View className='sdui-commerce-submit' onClick={() => emit(onAction, asAction(block.action) || asAction(props.action), { count, total_fen: totalFen, total_text: totalText, items: checkoutItems }, props)}>
        <Text>{String(props.action_text || '去结算')}</Text>
      </View>
    </View>
  )
}

/** 优惠券列表：展示面额、门槛、有效期与状态，支持选中动作。 */
export const CouponListBlock: React.FC<CommerceBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const autoLoad = props.auto_load === true || props.source === 'user'
  const [remoteItems, setRemoteItems] = useState<any[] | null>(null)
  const [loading, setLoading] = useState<boolean>(false)
  const [loadError, setLoadError] = useState<string>('')

  // auto_load: 读取当前用户已领取优惠券；协议仍可用静态 items 直接渲染。
  useEffect(() => {
    if (!autoLoad) return
    let cancelled = false
    setLoading(true)
    setLoadError('')
    request<any>({
      url: '/api/v1/action/execute',
      method: 'POST',
      data: { endpoint: 'coupon.list', payload: {} }
    })
      .then((result) => {
        if (cancelled) return
        const records = Array.isArray(result?.items) ? result.items : []
        setRemoteItems(records.map((record: any) => {
          const data = record?.data && typeof record.data === 'object' ? record.data : record
          const minSpend = Number(data?.min_spend_fen || 0)
          return {
            id: record?.id,
            title: data?.title,
            amount_fen: Number(data?.amount_fen || 0),
            condition: minSpend > 0 ? `满 ${moneyText(minSpend)} 可用` : '无门槛',
            valid_until: String(data?.valid_until || '').slice(0, 10),
            status: record?.status || 'available'
          }
        }))
      })
      .catch((error) => { if (!cancelled) setLoadError(error?.message || '优惠券加载失败') })
      .then(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [autoLoad])

  const items = remoteItems || listOf(props.items)

  return (
    <View className='sdui-coupon-list'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      {loading && <Text className='sdui-commerce-hint'>优惠券加载中…</Text>}
      {loadError && <Text className='sdui-commerce-hint is-error'>{loadError}</Text>}
      {items.length === 0 && !loading && <Text className='sdui-commerce-empty'>{String(props.empty_text || '暂无优惠券')}</Text>}
      {items.map((item: any, index: number) => {
        const status = String(item.status || 'available')
        const disabled = status !== 'available' && status !== 'active'
        return (
          <View
            key={item.id || item.key || index}
            className={`sdui-coupon-row ${item.selected ? 'is-selected' : ''} ${disabled ? 'is-disabled' : ''}`}
            onClick={() => {
              if (disabled) return
              if (props.selectable === false) return
              emit(onAction, asAction(item.action) || asAction(props.select_action) || asAction(block.action), { id: item.id, coupon_id: item.id, item, index }, item)
            }}
          >
            <View className='sdui-coupon-amount'>
              <Text>{String(item.amount_text || (item.amount_fen !== undefined ? moneyText(item.amount_fen) : item.amount ?? ''))}</Text>
            </View>
            <View className='sdui-coupon-copy'>
              <Text className='sdui-coupon-title'>{String(item.title || '')}</Text>
              {item.condition && <Text className='sdui-coupon-condition'>{String(item.condition)}</Text>}
              {item.valid_until && <Text className='sdui-coupon-valid'>{String(item.valid_until)}</Text>}
            </View>
            {status !== 'available' && <Text className='sdui-coupon-status'>{status === 'used' ? '已使用' : status === 'expired' ? '已过期' : status === 'locked' || status === 'reserved' ? '锁定中' : status}</Text>}
          </View>
        )
      })}
    </View>
  )
}

/** 发票信息表单：个人/企业抬头、税号与邮箱。 */
export const InvoiceFormBlock: React.FC<CommerceBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const [type, setType] = useState<'personal' | 'company'>(props.type === 'company' ? 'company' : 'personal')
  const [title, setTitle] = useState(String(props.title_value || ''))
  const [taxNo, setTaxNo] = useState(String(props.tax_no || ''))
  const [email, setEmail] = useState(String(props.email || ''))

  const submit = () => {
    if (!title.trim()) {
      Taro.showToast({ title: '请填写发票抬头', icon: 'none' })
      return
    }
    if (type === 'company' && !taxNo.trim()) {
      Taro.showToast({ title: '请填写企业税号', icon: 'none' })
      return
    }
    if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      Taro.showToast({ title: '邮箱格式不正确', icon: 'none' })
      return
    }
    const payload = { type, title, tax_no: type === 'company' ? taxNo : '', email }
    const action: BlockAction | undefined = asAction(block.action)
      || asAction(props.submit_action)
      || { type: 'request_data', require_auth: true, payload: { endpoint: 'invoice.save', body: payload } }
    emit(onAction, action, payload, { ...props, ...payload })
  }

  return (
    <View className='sdui-invoice-form'>
      {props.title && <Text className='sdui-commerce-title'>{String(props.title)}</Text>}
      <View className='sdui-form-option-list'>
        {[{ value: 'personal', label: '个人' }, { value: 'company', label: '企业' }].map((option) => (
          <View key={option.value} className={`sdui-form-option ${type === option.value ? 'is-active' : ''}`} onClick={() => setType(option.value as 'personal' | 'company')}>
            <Text>{option.label}</Text>
          </View>
        ))}
      </View>
      <View className='sdui-form-field'>
        <Text className='sdui-form-field-label'>发票抬头 *</Text>
        <Input className='sdui-form-input' cursorSpacing={24} adjustPosition value={title} onInput={(event) => setTitle(event.detail.value)} placeholder='请输入发票抬头' />
      </View>
      {type === 'company' && (
        <View className='sdui-form-field'>
          <Text className='sdui-form-field-label'>企业税号 *</Text>
          <Input className='sdui-form-input' cursorSpacing={24} adjustPosition value={taxNo} onInput={(event) => setTaxNo(event.detail.value)} placeholder='请输入企业税号' />
        </View>
      )}
      <View className='sdui-form-field'>
        <Text className='sdui-form-field-label'>接收邮箱</Text>
        <Input className='sdui-form-input' cursorSpacing={24} adjustPosition type='text' value={email} onInput={(event) => setEmail(event.detail.value)} placeholder='用于接收电子发票' />
      </View>
      <View className='sdui-commerce-submit' onClick={submit}>
        <Text>{String(props.submit_text || '保存发票信息')}</Text>
      </View>
    </View>
  )
}