// index.tsx
import { useState, useEffect, useCallback, useMemo } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline } from '@tarojs/taro'
import { PageResponseEnvelope, BlockItem } from '../../types/sdui'
import { request } from '../../utils/request'
import { dispatchAction } from '../../utils/action'
import { ensureSession } from '../../utils/auth'
import { AppleNavbar } from '../../components/AppleNavbar'
import { BlockRenderer } from '../../components/SDUI/BlockRenderer'
import './index.scss'

/**
 * SDUI 万能动态承载页容器 (Universal Dynamic Page Container)
 * 纯粹由后端下发的 JSON 协议完全驱动排版、积木组件与交互动作
 */
export default function DynamicPageIndex() {
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

  // 状态原子更新方法 (支持 actions.save_as 触发 React 重新渲染)
  const updateState = useCallback((key: string, value: any) => {
    setPageState((prev) => ({
      ...prev,
      [key]: value
    }))
  }, [])

  // 获取 URL Query 传参
  const routerParams = Taro.getCurrentInstance().router?.params || {}
  const pageId = routerParams.page_id || 'home'

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
    if (friendConfig && friendConfig.enabled) {
      return {
        title: friendConfig.title || envelope?.page?.title || '精彩短剧推荐',
        path: friendConfig.path || `/pages/dynamic/index?page_id=${pageId}`,
        imageUrl: friendConfig.image_url || envelope?.page?.share_config?.default_image_url
      }
    }
    return {
      title: envelope?.page?.title || '猴王下山 - 精选剧场',
      path: `/pages/dynamic/index?page_id=${pageId}`
    }
  })

  // 微信朋友圈分享配置 (符合微信客户端规范，使用 query)
  useShareTimeline(() => {
    const timelineConfig = envelope?.page?.share_config?.timeline
    if (timelineConfig && timelineConfig.enabled) {
      return {
        title: timelineConfig.title || envelope?.page?.title || '精彩短剧推荐',
        query: timelineConfig.query || `page_id=${pageId}&from=timeline`,
        imageUrl: timelineConfig.image_url || envelope?.page?.share_config?.default_image_url
      }
    }
    return {
      title: envelope?.page?.title || '猴王下山 - 精选剧场',
      query: `page_id=${pageId}&from=timeline`
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
  const pageTitle = envelope?.page?.title || '精选剧场'

  // 计算最终渲染的积木列表：优先消费服务端下发的同构 Layout IR 节点，确保与服务端截图 100% 像素级一致
  const effectiveBlocks = useMemo<BlockItem[]>(() => {
    if (envelope?.layout_ir?.nodes && envelope.layout_ir.nodes.length > 0) {
      const nodeToBlock = (node: any): BlockItem => ({
        id: node.id,
        type: node.type,
        props: {
          ...(node.props || {}),
          ...(node.children?.length ? { children: node.children.filter((child: any) => child.visible !== false).map(nodeToBlock) } : {}),
          _layout_height: node.bounding_box?.height
        },
        action: node.action,
        events: node.events,
        loading: node.loading,
        empty: node.empty,
        error: node.error,
        fallback: node.fallback,
        style: {
          margin_y: `${node.margin_y || 0}px`,
          border_radius: `${node.border_radius || 0}px`,
          padding: `${node.padding || 0}px`,
          glass_blur: node.glass_blur,
          accent_color: node.accent_color || undefined
        }
      })
      return envelope.layout_ir.nodes
        .filter((node) => node.visible !== false)
        .map(nodeToBlock)
    }
    return envelope?.page?.blocks || []
  }, [envelope])

  return (
    <View className={themeClass}>
      {/* 苹果原生毛玻璃胶囊对齐导航栏 */}
      <AppleNavbar
        title={pageTitle}
        subtitle={envelope?.page?.business_type ? `模式: ${envelope.page.business_type}` : undefined}
      />

      {/* 页面内容滚动区域 */}
      <ScrollView scrollY className="page-scroll-body">
        {/* 1. 加载中骨架占位 */}
        {loading && (
          <View className="sdui-loading-skeleton">
            <View className="skeleton-hero" />
            <View className="skeleton-card" />
            <View className="skeleton-btn" />
          </View>
        )}

        {/* 2. 页面要求登录受保护态 */}
        {!loading && authRequired && (
          <View className="sdui-error-panel auth-panel">
            <Text className="error-emoji">🔒</Text>
            <Text className="error-title">该内容需微信授权后查看</Text>
            <Text className="error-desc">请点击下方按钮完成快速微信授权登录</Text>
            <View className="retry-btn" onClick={handleLoginRetry}>
              <Text>微信一键快捷授权</Text>
            </View>
          </View>
        )}

        {/* 3. 网络错误提示与重试 */}
        {!loading && !authRequired && errorMsg && (
          <View className="sdui-error-panel">
            <Text className="error-emoji">⚠️</Text>
            <Text className="error-title">页面加载失败</Text>
            <Text className="error-desc">{errorMsg}</Text>
            <View className="retry-btn" onClick={fetchPageProtocol}>
              <Text>重新加载</Text>
            </View>
          </View>
        )}

        {/* 4. 积木列表渲染 (优先消费同构 Layout IR 节点，实现两端 100% 一致) */}
        {!loading && !authRequired && !errorMsg && effectiveBlocks.length > 0 && (
          <View className="blocks-container">
            {effectiveBlocks.map((block) => (
              <BlockRenderer
                key={block.id}
                block={block}
                onAction={handleBlockAction}
                context={{
                  entity: envelope?.data,
                  page: envelope?.page,
                  query: routerParams,
                  state: pageState,
                  updateState,
                  blockStates,
                  setBlockState: (blockId: string, state: string) => setBlockStates((prev) => ({ ...prev, [blockId]: state }))
                }}
              />
            ))}
          </View>
        )}

        {/* 5. 暂无积木空态 */}
        {!loading && !authRequired && !errorMsg && effectiveBlocks.length === 0 && (
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
