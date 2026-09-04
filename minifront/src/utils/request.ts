// request.ts
import Taro from '@tarojs/taro'
import { getStoredSession, refreshSession, loginWithWechat, isAccessTokenExpiringSoon } from './auth'
import { getBaseUrl } from '../config/env'

export { getBaseUrl }

// 请求参数配置接口
export interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  data?: any
  header?: Record<string, string>
  // 请求超时时间(毫秒)
  timeout?: number
  // 内部标记：是否为 401 重试请求 (严格限制单次重试防死循环)
  _isRetry?: boolean
}

/**
 * 封装企业级网络请求层
 * 支持多租户请求头自动注入、双 Token 无感预刷新、401 并发刷新合并与单次自动重发
 */
export async function request<T>(options: RequestOptions): Promise<T> {
  const { url, method = 'GET', data, header = {}, timeout, _isRetry = false } = options
  const baseUrl = getBaseUrl()

  // 判断是否为同源受信任地址 (相对路径或与当前 baseUrl 同源)
  const isRelative = url.startsWith('/') && !url.startsWith('//')
  const isSameOrigin = url.startsWith(baseUrl)

  // 1. 无感预刷新检查 (仅对同源受信任且非 auth 接口生效)
  if ((isRelative || isSameOrigin) && !url.startsWith('/api/v1/auth/')) {
    const session = getStoredSession()
    if (session && isAccessTokenExpiringSoon(session)) {
      await refreshSession()
    }
  }

  // 2. 组装请求头：严格执行凭证防泄露边界拦截
  const session = getStoredSession()
  const requestHeaders: Record<string, string> = {
    'content-type': 'application/json',
    'X-SDUI-Version': '1.1',
    'X-Client-Capabilities': 'media_hero,resource_card,action_button,notice,game_card,form,episode_list,item_grid,timeline,clipboard,video,request_payment',
    ...header
  }

  // 严密安全门禁: 仅向同源或受信任白名单后端地址发送多租户与用户 Authorization Token，杜绝凭证泄露
  if (isRelative || isSameOrigin) {
    if (session?.access_token) {
      requestHeaders['Authorization'] = `Bearer ${session.access_token}`
    }
  } else {
    // 跨域或非同源第三方外部请求，清除敏感认证 Header
    delete requestHeaders['X-App-Id']
    delete requestHeaders['Authorization']
  }

  try {
    const fullUrl = url.startsWith('http://') || url.startsWith('https://') ? url : `${baseUrl}${url}`
    const res = await Taro.request<any>({
      url: fullUrl,
      method,
      data,
      header: requestHeaders,
      timeout: timeout || 15000
    })

    // 3. 处理 401 Unauthorized 身份失效拦截
    if (res.statusCode === 401 && !_isRetry && !url.startsWith('/api/v1/auth/')) {
      console.warn(`请求 ${url} 触发 401 拦截，尝试无感刷新令牌并单次重试...`)
      const refreshed = await refreshSession()
      if (refreshed) {
        return request<T>({
          ...options,
          _isRetry: true
        })
      }

      // 刷新失败，回退尝试一次静默微信登录
      const reLogin = await loginWithWechat()
      if (reLogin) {
        return request<T>({
          ...options,
          _isRetry: true
        })
      }

      throw new Error('用户登录态已失效，请重新授权')
    }

    // 4. 处理 2xx 正常响应 (兼容 SDUI 统一响应信封与旧版 ApiResponse)
    if (res.statusCode >= 200 && res.statusCode < 300) {
      // 若为 SDUI 统一响应信封结构 (包含 protocol_version 与 page)
      if (res.data && res.data.protocol_version && res.data.page) {
        return res.data as T
      }

      // 若为标准业务信封结构 { code: 0, data: ... }
      if (res.data && typeof res.data.code === 'number') {
        if (res.data.code === 0) {
          return res.data.data as T
        }
        throw new Error(res.data.msg || '业务请求返回异常')
      }

      // 其他直接返回 JSON
      return res.data as T
    }

    // 5. 非 2xx 异常
    throw new Error(res.data?.msg || `网络响应异常: ${res.statusCode}`)
  } catch (err: any) {
    console.error(`Request Error [${method} ${url}]:`, err)
    throw err
  }
}
