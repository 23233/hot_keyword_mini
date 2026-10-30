// SDUIPageContainer.tsx
import { useState, useEffect, useCallback, useMemo } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline, usePullDownRefresh, useReachBottom } from '@tarojs/taro'
import { PageResponseEnvelope } from '../../types/sdui'
import { request } from '../../utils/request'
import { dispatchAction } from '../../utils/action'
import { ensureSession } from '../../utils/auth'
import { setQueryStringParams, setUrlQueryParams } from '../../utils/url'
import { AppleNavbar } from '../AppleNavbar'
import { SafeBlockRenderer } from './BlockRenderer'
import './SDUIPageContainer.scss'

/**
 * SDUI 万能动态承载页容器 (Universal Dynamic Page Container)
 * 纯粹由后端下发的 JSON 协议完全驱动排版、积木组件与交互动作
 */
interface SDUIPageContainerProps {
  // 覆盖路由中的 page_id；首页固定为 home。
  pageId?: string
  // 页面协议未下发标题时的兜底标题。
  defaultTitle?: string
  // 分享路径前缀；默认当前动态页。
  shareBasePath?: string
  // 朋友圈默认 query；默认携带 page_id。
  defaultTimelineQuery?: string
}

export default function SDUIPageContainer({
  pageId: pageIdOverride,
  defaultTitle = '精选推荐',
  shareBasePath,
  defaultTimelineQuery
}: SDUIPageContainerProps) {
  // 当前动态承载页统一响应信封
  const [envelope, setEnvelope] = useState<PageResponseEnvelope | null>(null)
  // 加载中状态
  const [loading, setLoading] = useState<boolean>(true)
  // 错误提示文案
  const [errorMsg, setErrorMsg] = useState<string>('')
  // 是否需要登录授权阻断
  const [authRequired, setAuthRequired] = useState<boolean>(false)
  // 页面响应式状态空间 ($state.* 真实闭环)
  const [pageState, setPageState] = useState<Record<string, any>>({})
  const [blockStates, setBlockStates] = useState<Record<string, string>>({})
  // 任意弹层打开时锁定页面滚动，避免遮罩下内容跟随滑动
  const [overlayOpen, setOverlayOpen] = useState<boolean>(false)

  // 状态原子更新方法 (支持 actions.save_as 触发 React 重新渲染)
  const updateState = useCallback((key: string, value: any) => {
    setPageState((prev) => ({
      ...prev,
      [key]: value
    }))
  }, [])

  // 获取 URL Query 传参
  const routerParams = Taro.getCurrentInstance().router?.params || {}
  const pageId = pageIdOverride || routerParams.page_id || 'home'

  // 拉取服务端动态页面协议
  const fetchPageProtocol = useCallback(async () => {
    try {
      setLoading(true)
      setErrorMsg('')
      setAuthRequired(false)

      const queryEntries = Object.entries(routerParams)
        .filter(([k]) => k !== 'page_id')
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
      const queryStr = queryEntries.length > 0 ? `?${queryEntries.join('&')}` : ''

      const res = await request<PageResponseEnvelope>({
        url: `/api/v1/page/${pageId}${queryStr}`,
        method: 'GET'
      })

      // 若页面要求登录且尚未完成认证，先触发会话校验
      if (res?.page?.require_auth) {
        const session = await ensureSession()
        if (!session) {
          setAuthRequired(true)
          setEnvelope(res)
          return
        }
      }

      setEnvelope(res)

      // 动态设置小程序导航栏标题
      if (res?.page?.title) {
        Taro.setNavigationBarTitle({
          title: res.page.title
        })
      }
    } catch (err: any) {
      console.error(`获取 SDUI 动态页面 ${pageId} 失败:`, err)
      if (err.message && err.message.includes('401')) {
        setAuthRequired(true)
      } else {
        setErrorMsg(err.message || '网络连接异常，未能加载内容')
      }
    } finally {
      setLoading(false)
    }
  }, [pageId])

  useEffect(() => {
    fetchPageProtocol()
  }, [fetchPageProtocol])

  // 微信好友分享配置 (从页面下发协议中动态读取)
  useShareAppMessage(() => {
    const friendConfig = envelope?.page?.share_config?.friend
    const accessMode = envelope?.page?.share_config?.access?.mode
    const shareUserId = (envelope?.viewer as any)?.user_id
    const extras: Record<string, string> = { from: 'share' }
    if (accessMode === 'owner' && shareUserId) extras.share_from = String(shareUserId)
    const target = friendConfig?.path || shareBasePath || `/pages/dynamic/index?page_id=${pageId}`
    return {
      title: friendConfig?.title || envelope?.page?.title || '精选推荐',
      path: setUrlQueryParams(target, extras),
      imageUrl: friendConfig?.image_url || envelope?.page?.share_config?.default_image_url
    }
  })

  // 微信朋友圈分享配置 (符合微信客户端规范，使用 query)
  useShareTimeline(() => {
    const timelineConfig = envelope?.page?.share_config?.timeline
    const accessMode = envelope?.page?.share_config?.access?.mode
    const shareUserId = (envelope?.viewer as any)?.user_id
    const extras: Record<string, string> = { from: 'timeline' }
    if (accessMode === 'owner' && shareUserId) extras.share_from = String(shareUserId)
    const target = timelineConfig?.query || defaultTimelineQuery || `page_id=${pageId}`
    return {
      title: timelineConfig?.title || envelope?.page?.title || '精选推荐',
      query: setQueryStringParams(target, extras),
      imageUrl: timelineConfig?.image_url || envelope?.page?.share_config?.default_image_url
    }
  })

  // 支持微信小程序原生下拉刷新
  usePullDownRefresh(async () => {
    try {
      await fetchPageProtocol()
    } finally {
      Taro.stopPullDownRefresh()
    }
  })

  // 支持微信小程序原生触底加载
  useReachBottom(() => {
    const pageEvents = (envelope?.page as any)?.events
    if (pageEvents?.reach_bottom) {
      handleBlockAction(pageEvents.reach_bottom, { __event: 'reach_bottom' })
    } else if (pageEvents?.load_more) {
      handleBlockAction(pageEvents.load_more, { __event: 'load_more' })
    }
  })

  // 处理积木交互点击 (注入响应式 pageState 与 updateState 回调，透传 item 等局部上下文)
  const handleBlockAction = (action?: any, extraContext?: Record<string, any>) => {
    dispatchAction(action, {
      refresh: fetchPageProtocol,
      entity: envelope?.data,
      page: envelope?.page,
      query: routerParams,
      state: pageState,
      updateState,
      blockStates,
      capabilities: envelope?.capabilities_required || [],
      viewer: envelope?.viewer || {},
      setBlockState: (blockId: string, state: string) => setBlockStates((prev) => ({ ...prev, [blockId]: state })),
      ...extraContext
    })
  }

  // 触发登录重试
  const handleLoginRetry = async () => {
    const session = await ensureSession()
    if (session) {
      setAuthRequired(false)
      fetchPageProtocol()
    }
  }

  const themeClass = `dynamic-page-container theme-${envelope?.page?.theme || 'dark_glass'}`
  const pageTitle = envelope?.page?.title || defaultTitle

  // 微信字体大小设置（默认 16）不会自动缩放部分 WebView 文本，这里只对交互按钮做受控放大。
  const fontScale = useMemo(() => {
    try {
      const setting = Number(Taro.getSystemInfoSync()?.fontSizeSetting || 16)
      if (!Number.isFinite(setting)) return 1
      return Math.min(1.35, Math.max(1, setting / 16))
    } catch {
      return 1
    }
  }, [])

  // page.blocks 是小程序唯一的运行时渲染输入；layout_ir 仅供服务端验收和截图比对。
  const effectiveBlocks = envelope?.page?.blocks || []
  const shareGuard = (envelope?.viewer as any)?.share_guard as
    | { mode?: string; title?: string; message?: string; action?: string; action_text?: string; membership_page_id?: string }
    | undefined

  // 分享落地访问拦截：按服务端策略展示提示，不渲染原页面内容。
  const handleShareGuardAction = useCallback(() => {
    if (!shareGuard) return
    if (shareGuard.action === 'login') {
      handleLoginRetry()
      return
    }
    if (shareGuard.action === 'membership') {
      const membershipPageID = shareGuard.membership_page_id || 'membership'
      Taro.navigateTo({ url: `/pages/dynamic/index?page_id=${encodeURIComponent(membershipPageID)}` })
      return
    }
    Taro.reLaunch({ url: '/pages/index/index' })
  }, [shareGuard, handleLoginRetry])

  return (
    <View className={themeClass} style={{ ['--sdui-font-scale' as any]: String(fontScale) }}>
      {/* 苹果原生毛玻璃胶囊对齐导航栏 */}
      <AppleNavbar
        title={pageTitle}
        theme={envelope?.page?.theme || 'dark_glass'}
        fontScale={fontScale}
      />

      {/* 页面内容滚动区域 */}
      <ScrollView scrollY={!overlayOpen} className={`page-scroll-body ${overlayOpen ? 'is-overlay-locked' : ''}`.trim()}>
        {/* 1. 加载中骨架占位 */}
        {loading && (
          <View className="sdui-loading-skeleton">
            <View className="skeleton-hero" />
            <View className="skeleton-card" />
            <View className="skeleton-btn" />
          </View>
        )}

        {/* 2. 分享落地访问拦截提示 */}
        {!loading && !errorMsg && shareGuard && (
          <View className="sdui-share-guard">
            <Text className="share-guard-emoji">🔐</Text>
            <Text className="share-guard-title">{String(shareGuard.title || '该分享暂不可查看')}</Text>
            <Text className="share-guard-desc">{String(shareGuard.message || '分享内容受访问权限保护')}</Text>
            <View className="share-guard-action" onClick={handleShareGuardAction}>
              <Text>{String(shareGuard.action_text || '返回首页')}</Text>
            </View>
          </View>
        )}

        {/* 3. 页面要求登录受保护态 */}
        {!loading && !shareGuard && authRequired && (
          <View className="sdui-error-panel auth-panel">
            <Text className="error-emoji">🔒</Text>
            <Text className="error-title">该内容需微信授权后查看</Text>
            <Text className="error-desc">请点击下方按钮完成快速微信授权登录</Text>
            <View className="retry-btn" onClick={handleLoginRetry}>
              <Text>微信一键快捷授权</Text>
            </View>
          </View>
        )}

        {/* 4. 网络错误提示与重试 */}
        {!loading && !shareGuard && !authRequired && errorMsg && (
          <View className="sdui-error-panel">
            <Text className="error-emoji">⚠️</Text>
            <Text className="error-title">页面加载失败</Text>
            <Text className="error-desc">{errorMsg}</Text>
            <View className="retry-btn" onClick={fetchPageProtocol}>
              <Text>重新加载</Text>
            </View>
          </View>
        )}

        {/* 5. 积木列表渲染 */}
        {!loading && !shareGuard && !authRequired && !errorMsg && effectiveBlocks.length > 0 && (
          <View className="blocks-container">
            {effectiveBlocks.map((block) => (
              <SafeBlockRenderer
                key={block.id}
                block={block}
                onAction={handleBlockAction}
                context={{
                  entity: envelope?.data,
                  data: envelope?.data,
                  page: envelope?.page,
                  query: routerParams,
                  state: pageState,
                  updateState,
                  blockStates,
                  setOverlayOpen,
                  fontScale,
                  setBlockState: (blockId: string, state: string) => setBlockStates((prev) => ({ ...prev, [blockId]: state }))
                }}
              />
            ))}
          </View>
        )}

        {/* 6. 暂无积木空态 */}
        {!loading && !shareGuard && !authRequired && !errorMsg && effectiveBlocks.length === 0 && (
          <View className="sdui-empty-panel">
            <Text className="empty-emoji">📭</Text>
            <Text className="empty-title">页面暂未配置内容</Text>
            <Text className="empty-desc">该页面暂无积木组件，请在管理后台进行配置</Text>
          </View>
        )}
      </ScrollView>
    </View>
  )
}
