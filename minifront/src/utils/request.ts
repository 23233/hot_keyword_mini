import Taro from '@tarojs/taro'
import { ApiResponse } from '../types/drama'
import { getAppId, getStoredSession, refreshSession, loginWithWechat, isAccessTokenExpiringSoon } from './auth'

// 后端 API 基础地址（默认本地后端端口 8080）
export const BASE_URL = 'http://127.0.0.1:8080'

// 请求参数配置接口
export interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  data?: any
  header?: Record<string, string>
  // 内部标记：是否为 401 重试请求 (严格限制单次重试防死循环)
  _isRetry?: boolean
}

/**
 * 封装企业级网络请求层
 * 支持多租户请求头自动注入、双 Token 无感预刷新、401 并发刷新合并与单次自动重发
 */
export async function request<T>(options: RequestOptions): Promise<T> {
  const { url, method = 'GET', data, header = {}, _isRetry = false } = options
  const appId = getAppId()

  // 1. 无感预刷新检查 (非 auth 接口且 Access Token 即将过期时提前静默换新)
  if (!url.startsWith('/api/v1/auth/')) {
    const session = getStoredSession()
    if (session && isAccessTokenExpiringSoon(session)) {
      await refreshSession()
    }
  }

  // 2. 组装多租户与鉴权标准请求头
  const session = getStoredSession()
  const requestHeaders: Record<string, string> = {
    'content-type': 'application/json',
    'X-App-Id': appId,
    'X-SDUI-Version': '1.1',
    ...header
  }

  if (session?.access_token && !requestHeaders['Authorization']) {
    requestHeaders['Authorization'] = `Bearer ${session.access_token}`
  }

  try {
    const res = await Taro.request<any>({
      url: `${BASE_URL}${url}`,
      method,
      data,
      header: requestHeaders
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
