// minifront/src/utils/cart.ts
import Taro from '@tarojs/taro'
import { getCurrentAppId } from './request'

// 本地购物车条目；金额统一使用分，展示文本可由协议提供。
export interface CartItem {
  sku: string
  title?: string
  price_fen?: number
  price_text?: string
  image_url?: string
  spec?: string
  quantity: number
}

// 按租户隔离的本地购物车存储键。
const cartStorageKey = () => `sdui_cart_${getCurrentAppId() || 'default'}`

/** 读取当前租户本地购物车；存储异常时返回空数组。 */
export function loadCart(): CartItem[] {
  try {
    const raw = Taro.getStorageSync(cartStorageKey())
    if (!Array.isArray(raw)) return []
    return raw.filter((item) => item && typeof item === 'object' && item.sku)
  } catch (error) {
    return []
  }
}

/** 保存购物车并返回最新列表。 */
export function saveCart(items: CartItem[]): CartItem[] {
  try {
    Taro.setStorageSync(cartStorageKey(), items)
  } catch (error) {
    // 本地存储失败不阻断页面交互。
  }
  return items
}

/** 加入购物车；同一 SKU 累加数量。 */
export function addCartItem(item: CartItem): CartItem[] {
  const quantity = Math.max(1, Number(item.quantity || 1))
  const items = loadCart()
  const index = items.findIndex((current) => current.sku === item.sku)
  if (index >= 0) {
    items[index] = { ...items[index], ...item, quantity: Number(items[index].quantity || 0) + quantity }
  } else {
    items.push({ ...item, quantity })
  }
  return saveCart(items)
}

/** 更新指定 SKU 数量；数量为 0 时移除。 */
export function updateCartItem(sku: string, quantity: number): CartItem[] {
  const items = loadCart()
  if (quantity <= 0) {
    return saveCart(items.filter((item) => item.sku !== sku))
  }
  return saveCart(items.map((item) => item.sku === sku ? { ...item, quantity } : item))
}

/** 移除指定 SKU。 */
export function removeCartItem(sku: string): CartItem[] {
  return saveCart(loadCart().filter((item) => item.sku !== sku))
}

/** 清空当前租户购物车。 */
export function clearCart(): CartItem[] {
  return saveCart([])
}

/** 计算购物车件数与金额（分）。 */
export function cartTotals(items: CartItem[]): { count: number; totalFen: number } {
  return items.reduce((result, item) => {
    result.count += Number(item.quantity || 0)
    result.totalFen += Number(item.price_fen || 0) * Number(item.quantity || 0)
    return result
  }, { count: 0, totalFen: 0 })
}