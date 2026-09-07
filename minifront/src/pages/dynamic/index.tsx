// minifront/src/pages/dynamic/index.tsx
import { useState, useEffect, useCallback, useMemo } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline, usePullDownRefresh, useReachBottom } from '@tarojs/taro'
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
        title: friendConfig.title || envelope?.page?.title || '精选推荐',
        path: friendConfig.path || `/pages/dynamic/index?page_id=${pageId}`,
        imageUrl: friendConfig.image_url || envelope?.page?.share_config?.default_image_url
      }
    }
    return {
      title: envelope?.page?.title || '精选推荐',
      path: `/pages/dynamic/index?page_id=${pageId}`
    }
  })

  // 微信朋友圈分享配置 (符合微信客户端规范，使用 query)
  useShareTimeline(() => {
    const timelineConfig = envelope?.page?.share_config?.timeline
    if (timelineConfig && timelineConfig.enabled) {
      return {
        title: timelineConfig.title || envelope?.page?.title || '精选推荐',
        query: timelineConfig.query || `page_id=${pageId}&from=timeline`,
        imageUrl: timelineConfig.image_url || envelope?.page?.share_config?.default_image_url
      }
    }
    return {
      title: envelope?.page?.title || '精选推荐',
      query: `page_id=${pageId}&from=timeline`
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
  const pageTitle = envelope?.page?.title || '精选推荐'

  // 计算最终渲染的积木列表：优先消费服务端下发的同构 Layout IR 节点，确保与服务端截图 100% 像素级一致
  const effectiveBlocks = useMemo<BlockItem[]>(() => {
    const rawBlocks = envelope?.page?.blocks || []
    if (envelope?.layout_ir?.nodes && envelope.layout_ir.nodes.length > 0) {
      const blockMap = new Map<string, BlockItem>()
      const registerBlock = (b: BlockItem) => {
        if (!b || !b.id) return
        blockMap.set(b.id, b)
        const children = (b.props?.children || b.props?.items || b.props?.blocks) as BlockItem[]
        if (Array.isArray(children)) {
          children.forEach(registerBlock)
        }
        if (Array.isArray(b.props?.tabs)) {
          b.props.tabs.forEach((tab: any) => {
            const tabChildren = (tab.blocks || tab.children || (tab.child ? [tab.child] : [])) as BlockItem[]
            if (Array.isArray(tabChildren)) {
              tabChildren.forEach(registerBlock)
            }
          })
        }
      }
      rawBlocks.forEach(registerBlock)

      const nodeToBlock = (node: any, isChild = false): BlockItem => {
        const orig = blockMap.get(node.id) || blockMap.get(node.id.replace(/_\d+$/, ''))
        const childrenList = node.children?.length ? node.children.map((c: any) => nodeToBlock(c, true)) : undefined
        const mergedProps: Record<string, any> = {
          ...(node.props || {}),
          // 保留原始绑定表达式，避免初始 IR 的已解析值覆盖 $state/$result 后续响应式更新。
          ...(orig?.props || {}),
          _layout_height: node.bounding_box?.height
        }
        if (node.type === 'tabs') {
          // 关键防护：保留 tabs 结构配置，防止被扁平 children 替换导致标签丢失
          if (orig?.props?.tabs) {
            mergedProps.tabs = orig.props.tabs
          }
        } else if (childrenList && childrenList.length > 0) {
          mergedProps.children = childrenList
        }

        return {
          id: node.id,
          type: node.type,
          props: mergedProps,
          visible_when: node.visible_when !== undefined ? node.visible_when : (orig?.visible_when !== undefined ? orig.visible_when : (node.visible === false ? false : undefined)),
          repeat: node.repeat,
          action: node.action || orig?.action,
          events: node.events || orig?.events,
          loading: node.loading || orig?.loading,
          empty: node.empty || orig?.empty,
          error: node.error || orig?.error,
          fallback: node.fallback || orig?.fallback,
          style: {
            ...(orig?.style || {}),
            margin_y: isChild
              ? orig?.style?.margin_y
              : (node.margin_y ? `${node.margin_y}px` : (orig?.style?.margin_y || '24rpx')),
            border_radius: node.border_radius ? `${node.border_radius}px` : orig?.style?.border_radius,
            padding: node.padding ? `${node.padding}px` : orig?.style?.padding,
            glass_blur: node.glass_blur !== undefined ? node.glass_blur : orig?.style?.glass_blur,
            accent_color: node.accent_color || orig?.style?.accent_color
          }
        }
      }
      return envelope.layout_ir.nodes.map((n: any) => nodeToBlock(n, false))
    }
    return rawBlocks
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
