import Taro from '@tarojs/taro'
import { UserSessionState } from '../types/sdui'
import { getBaseUrl } from '../config/env'

// 本地存储会话键名
const SESSION_STORAGE_KEY = 'hot_mini_user_session'
// 并发刷新请求锁与 Promise 共享句柄
let refreshPromise: Promise<UserSessionState | null> | null = null
// 并发微信登录请求锁与 Promise 共享句柄
let loginPromise: Promise<UserSessionState | null> | null = null

/**
 * 获取本地缓存的登录会话信息
 */
export function getStoredSession(): UserSessionState | null {
  try {
    const data = Taro.getStorageSync(SESSION_STORAGE_KEY)
    if (data && typeof data === 'object' && data.access_token) {
      return data as UserSessionState
    }
  } catch (err) {
    console.warn('读取本地会话异常:', err)
  }
  return null
}

/**
 * 持久化保存登录会话信息至小程序安全存储
 */
export function setStoredSession(session: UserSessionState): void {
  try {
    Taro.setStorageSync(SESSION_STORAGE_KEY, session)
  } catch (err) {
    console.error('保存会话至本地存储失败:', err)
  }
}

/**
 * 清除本地登录会话 (登出或凭证失效时调用)
 */
export function clearSession(): void {
  try {
    Taro.removeStorageSync(SESSION_STORAGE_KEY)
  } catch (err) {
    console.warn('清理本地会话失败:', err)
  }
}

/**
 * 判断 Access Token 是否即将过期 (提前 90 秒判定，避免临界点请求 401)
 */
export function isAccessTokenExpiringSoon(session: UserSessionState): boolean {
  if (!session.access_expires_at) return true
  const expireTime = new Date(session.access_expires_at).getTime()
  const now = Date.now()
  // 提前 90 秒刷新
  return now >= expireTime - 90 * 1000
}

/**
 * 判断 Refresh Token 是否已经过期
 */
export function isRefreshTokenExpired(session: UserSessionState): boolean {
  if (!session.refresh_expires_at) return true
  const expireTime = new Date(session.refresh_expires_at).getTime()
  return Date.now() >= expireTime
}

/**
 * 使用微信原生 Code 执行免密登录换取双 Token (支持并发共享)
 */
export async function loginWithWechat(): Promise<UserSessionState | null> {
  if (loginPromise) {
    return loginPromise
  }

  loginPromise = (async () => {
    try {
      const loginRes = await Taro.login()
      if (!loginRes.code) {
        throw new Error('获取微信登录凭证 code 失败')
      }

      const res = await Taro.request<{ code: number; msg?: string; data: UserSessionState }>({
        url: `${getBaseUrl()}/api/v1/auth/wechat-login`,
        method: 'POST',

        data: {
          code: loginRes.code
        },
        header: {
          'content-type': 'application/json',
          'X-SDUI-Version': '1.1'
        }
      })

      if (res.statusCode === 200 && res.data && res.data.code === 0 && res.data.data) {
        const session = res.data.data
        setStoredSession(session)
        return session
      } else {
        throw new Error(res.data?.msg || `微信登录接口响应异常: ${res.statusCode}`)
      }
    } catch (err: any) {
      console.error('执行微信登录闭环失败:', err)
      return null
    } finally {
      loginPromise = null
    }
  })()

  return loginPromise
}

/**
 * 使用长期 Refresh Token 刷新会话换取全新双 Token (支持并发刷新合并与防重放保护)
 */
export async function refreshSession(): Promise<UserSessionState | null> {
  if (refreshPromise) {
    return refreshPromise
  }

  refreshPromise = (async () => {
    try {
      const session = getStoredSession()
      if (!session || !session.refresh_token) {
        return null
      }

      if (isRefreshTokenExpired(session)) {
        console.warn('本地 Refresh Token 已过期，放弃刷新')
        clearSession()
        return null
      }

      const res = await Taro.request<{ code: number; msg?: string; data: UserSessionState }>({
        url: `${getBaseUrl()}/api/v1/auth/refresh`,
        method: 'POST',

        data: {
          refresh_token: session.refresh_token
        },
        header: {
          'content-type': 'application/json',
          'X-SDUI-Version': '1.1'
        }
      })

      if (res.statusCode === 200 && res.data && res.data.code === 0 && res.data.data) {
        const newSession = res.data.data
        setStoredSession(newSession)
        return newSession
      } else {
        console.warn('刷新令牌失效或被拦截:', res.data?.msg)
        clearSession()
        return null
      }
    } catch (err: any) {
      console.error('刷新会话请求异常:', err)
      clearSession()
      return null
    } finally {
      refreshPromise = null
    }
  })()

  return refreshPromise
}

/**
 * 确保当前处于有效登录态 (预刷新 -> 401 刷新 -> 微信登录回退)
 */
export async function ensureSession(): Promise<boolean> {
  const current = getStoredSession()

  // 1. 无本地会话或 Refresh Token 已过期 -> 触发微信免密登录
  if (!current || isRefreshTokenExpired(current)) {
    const session = await loginWithWechat()
    return !!session
  }

  // 2. Access Token 即将过期 -> 无感提前刷新
  if (isAccessTokenExpiringSoon(current)) {
    const refreshed = await refreshSession()
    if (refreshed) {
      return true
    }
    // 刷新失败，兜底重新微信登录
    const session = await loginWithWechat()
    return !!session
  }

  return true
}

/**
 * 安全获取短期有效 Access Token
 */
export async function getValidAccessToken(): Promise<string | null> {
  const ready = await ensureSession()
  if (!ready) return null
  const session = getStoredSession()
  return session?.access_token || null
}

/**
 * 主动退出登录并注销服务端会话
 */
export async function logout(): Promise<void> {
  const session = getStoredSession()
  if (session?.session_id && session?.access_token) {
    try {
      await Taro.request({
        url: `${getBaseUrl()}/api/v1/auth/logout`,
        method: 'POST',
        data: { session_id: session.session_id },
        header: {
          'Authorization': `Bearer ${session.access_token}`
        }
      })
    } catch (e) {
      // ignore
    }
  }
  clearSession()
}
