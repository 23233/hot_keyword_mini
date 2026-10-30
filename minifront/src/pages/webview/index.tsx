// minifront/src/pages/webview/index.tsx
import { useEffect, useMemo, useState } from 'react'
import { View, Text, WebView } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline } from '@tarojs/taro'
import { request } from '../../utils/request'
import { ensureSession, getStoredSession } from '../../utils/auth'
import { setQueryStringParams, setUrlQueryParams } from '../../utils/url'
import { buildWebViewDisplayURL } from '../../utils/webview'
import './index.scss'

// 安全解码路由参数；重复解码导致的异常直接回退原值。
const safeDecode = (value?: string): string => {
  if (!value) return ''
  try {
    return decodeURIComponent(value)
  } catch (error) {
    return value
  }
}

// 设置导航标题；标题为空时忽略。
const setTitle = (title?: string) => {
  if (!title) return
  Taro.setNavigationBarTitle({ title })
}

// 分享访问拦截提示。
interface WebViewGuard {
  title: string
  message: string
  action: 'login' | 'home'
  actionText: string
}

/**
 * 受控 WebView 页面：
 * - url_key：服务端按租户登记表解析地址；需要登录时换取一次性票据；
 * - drama_id：服务端解析短剧/剧集已配置的 HTTPS 播放地址；
 * - 不接受裸 url 参数，避免外部路径注入任意网页；
 * - 把字体大小/缩放/主题作为 sdui_* 参数传给目标网页，并支持好友/朋友圈分享。
 */
export default function WebViewPage() {
  const routerParams = Taro.getCurrentInstance().router?.params || {}
  const urlKey = safeDecode(routerParams.url_key)
  const dramaID = Number(routerParams.drama_id || 0)
  const episodeID = Number(routerParams.episode_id || 0)
  const requiresAuth = routerParams.auth === '1'
  const theme = safeDecode(routerParams.theme)
  const shared = routerParams.from === 'share' || routerParams.from === 'timeline'
  const shareTitle = safeDecode(routerParams.title) || '精选推荐'

  const [targetUrl, setTargetUrl] = useState<string>('')
  const [errorMsg, setErrorMsg] = useState<string>('')
  const [guard, setGuard] = useState<WebViewGuard | null>(null)
  const [reloadToken, setReloadToken] = useState<number>(0)

  // 微信字体大小（默认 16）与主题一起传给目标网页，由网页按需适配。
  const fontSizeSetting = useMemo(() => {
    try {
      const setting = Number(Taro.getSystemInfoSync()?.fontSizeSetting || 16)
      return Number.isFinite(setting) ? setting : 16
    } catch {
      return 16
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    setTargetUrl('')
    setErrorMsg('')
    setGuard(null)
    setTitle(safeDecode(routerParams.title))

    const displayURL = (url: string) => buildWebViewDisplayURL(url, { fontSizeSetting, theme })
    const load = async () => {
      if (urlKey) {
        if (requiresAuth && shared && !getStoredSession()) {
          setGuard({
            title: '该网页分享需要登录',
            message: '为保护分享内容，请先完成微信授权后查看。',
            action: 'login',
            actionText: '微信授权登录'
          })
          return
        }
        if (requiresAuth) {
          const ticket = await request<{ url: string }>({
            url: '/api/v1/webview/ticket',
            method: 'POST',
            data: { url_key: urlKey }
          })
          if (!ticket?.url) throw new Error('未能获取一次性 WebView 地址')
          if (!cancelled) setTargetUrl(displayURL(ticket.url))
          return
        }
        const resolved = await request<{ url: string; title?: string }>({
          url: `/api/v1/webview/resolve?url_key=${encodeURIComponent(urlKey)}`
        })
        if (!resolved?.url) throw new Error('WebView 入口不可用')
        if (!cancelled) {
          setTargetUrl(displayURL(resolved.url))
          if (!routerParams.title) setTitle(resolved.title)
        }
        return
      }

      if (dramaID > 0) {
        const resolved = await request<{ url: string; title?: string }>({
          url: `/api/v1/drama/web-url?drama_id=${encodeURIComponent(String(dramaID))}&episode_id=${encodeURIComponent(String(episodeID))}`
        })
        if (!resolved?.url) throw new Error('该剧集未配置网页播放地址')
        if (!cancelled) {
          setTargetUrl(displayURL(resolved.url))
          if (!routerParams.title) setTitle(resolved.title)
        }
        return
      }

      throw new Error('请通过已登记的 WebView 入口打开网页')
    }

    load().catch((error: any) => {
      if (cancelled) return
      if (shared) {
        setGuard({
          title: '分享的网页暂时无法打开',
          message: error?.message || '链接可能已失效，请稍后重试。',
          action: requiresAuth ? 'login' : 'home',
          actionText: requiresAuth ? '微信授权登录' : '返回首页'
        })
        return
      }
      setErrorMsg(error?.message || '网页加载失败')
    })
    return () => {
      cancelled = true
    }
  }, [reloadToken, fontSizeSetting, theme, urlKey, dramaID, episodeID, requiresAuth, shared])

  // 分享路径只携带受控 url_key / 剧集标识，不暴露一次性票据或裸 URL。
  const shareParams = (from: 'share' | 'timeline'): Record<string, string> | null => {
    if (urlKey) {
      return {
        url_key: urlKey,
        auth: requiresAuth ? '1' : '0',
        title: shareTitle,
        theme,
        from
      }
    }
    if (dramaID > 0) {
      return { drama_id: String(dramaID), episode_id: String(episodeID), title: shareTitle, from }
    }
    return null
  }

  useShareAppMessage(() => {
    const params = shareParams('share')
    if (!params) return { title: shareTitle, path: '/pages/index/index' }
    return { title: shareTitle, path: setUrlQueryParams('/pages/webview/index', params) }
  })

  useShareTimeline(() => {
    const params = shareParams('timeline')
    if (!params) return { title: shareTitle }
    return { title: shareTitle, query: setQueryStringParams('', params) }
  })

  // 返回首页
  const handleBackHome = () => {
    Taro.reLaunch({
      url: '/pages/index/index'
    })
  }

  // 登录后重试当前 WebView；其他情况返回首页。
  const handleGuardAction = async () => {
    if (guard?.action === 'login') {
      const session = await ensureSession()
      if (session) {
        setGuard(null)
        setReloadToken((value) => value + 1)
      }
      return
    }
    handleBackHome()
  }

  // 分享权限提示优先于 WebView 渲染。
  if (guard) {
    return (
      <View className='webview-error-container'>
        <View className='error-card apple-card'>
          <Text className='error-icon'>🔐</Text>
          <Text className='error-title'>{guard.title}</Text>
          <Text className='error-desc'>{guard.message}</Text>
          <View className='back-btn apple-pill-btn' onClick={handleGuardAction}>
            <Text className='btn-text'>{guard.actionText}</Text>
          </View>
        </View>
      </View>
    )
  }

  // 正常加载 WebView
  if (targetUrl) {
    return <WebView className='webview-component' src={targetUrl} />
  }

  // 异常或空链接提示
  return (
    <View className='webview-error-container'>
      <View className='error-card apple-card'>
        <Text className='error-icon'>🌐</Text>
        <Text className='error-title'>网页链接无效</Text>
        <Text className='error-desc'>{errorMsg || '正在准备载入网页...'}</Text>
        <View className='back-btn apple-pill-btn' onClick={handleBackHome}>
          <Text className='btn-text'>返回小程序首页</Text>
        </View>
      </View>
    </View>
  )
}
