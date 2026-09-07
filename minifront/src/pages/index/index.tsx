// minifront/src/pages/index/index.tsx
import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline, usePullDownRefresh, useReachBottom } from '@tarojs/taro'
import { PageResponseEnvelope, BlockItem } from '../../types/sdui'
import { request } from '../../utils/request'
import { dispatchAction } from '../../utils/action'
import { ensureSession } from '../../utils/auth'
import { AppleNavbar } from '../../components/AppleNavbar'
import { MotionLab } from '../../components/MotionLab'
import { BlockRenderer } from '../../components/SDUI/BlockRenderer'
import './index.scss'

/**
 * SDUI 万能动态主页承载容器 (Universal SDUI Home Container)
 * 遵循“一切皆通用 Block、一切皆自由编排”原则，纯粹由服务端下发的 JSON 协议驱动
 */
export default function Index() {
  // SDUI 动态主页统一响应信封
  const [sduiEnvelope, setSduiEnvelope] = useState<PageResponseEnvelope | null>(null)
  // 加载中状态
  const [loading, setLoading] = useState<boolean>(true)
  // 错误信息
  const [errorMsg, setErrorMsg] = useState<string>('')
  // 是否需要登录授权阻断
  const [authRequired, setAuthRequired] = useState<boolean>(false)
  // 页面响应式状态空间 ($state.* 闭环)
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

  // 开发者动效调试模式 (可通过 ?mode=motion 或长按导航栏标题 1.2 秒触发)
  const [debugMotionMode, setDebugMotionMode] = useState<boolean>(false)
  const longPressTimerRef = useRef<any>(null)

  // 长按导航栏标题快速切换动效实验室彩蛋
  const handleNavTouchStart = () => {
    longPressTimerRef.current = setTimeout(() => {
      Taro.vibrateShort({ type: 'medium' })
      setDebugMotionMode((prev) => {
        const next = !prev
        Taro.showToast({
          title: next ? '🛠️ 动效调试模式' : '🎬 恢复通用主页模式',
          icon: 'none'
        })
        return next
      })
    }, 1200)
  }

  const handleNavTouchEnd = () => {
    if (longPressTimerRef.current) {
      clearTimeout(longPressTimerRef.current)
    }
  }

  // 从后端接口拉取首页 SDUI 动态协议
  const fetchHomeData = useCallback(async () => {
    try {
      setLoading(true)
      setErrorMsg('')
      setAuthRequired(false)

      const queryEntries = Object.entries(routerParams)
        .filter(([k]) => k !== 'page_id')
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
      const queryStr = queryEntries.length > 0 ? `?${queryEntries.join('&')}` : ''

      const res = await request<PageResponseEnvelope>({
        url: `/api/v1/page/home${queryStr}`,
        method: 'GET'
      })

      // 若主页要求登录且尚未完成认证，先触发会话校验
      if (res?.page?.require_auth) {
        const session = await ensureSession()
        if (!session) {
          setAuthRequired(true)
          setSduiEnvelope(res)
          return
        }
      }

      setSduiEnvelope(res)

      // 动态设置小程序标题 (完全由协议驱动)
      if (res?.page?.title) {
        Taro.setNavigationBarTitle({
          title: res.page.title
        })
      }
    } catch (err: any) {
      console.error('获取 SDUI 主页数据失败:', err)
      if (err.message && err.message.includes('401')) {
        setAuthRequired(true)
      } else {
        setErrorMsg(err.message || '网络连接异常，未能加载内容')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const params = Taro.getCurrentInstance().router?.params || {}
    if (params.mode === 'motion' || params.debug === 'motion') {
      setDebugMotionMode(true)
    }
    fetchHomeData()
  }, [fetchHomeData])

  // 处理 SDUI 积木交互点击
  const handleBlockAction = (action?: any, extraContext?: Record<string, any>) => {
    dispatchAction(action, {
      refresh: fetchHomeData,
      entity: sduiEnvelope?.data,
      page: sduiEnvelope?.page,
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
      fetchHomeData()
    }
  }

  // 首页优先消费服务端同构 Layout IR，实现与服务端 100% 像素级对齐
  const effectiveSduiBlocks = useMemo<BlockItem[]>(() => {
    const nodes = sduiEnvelope?.layout_ir?.nodes
    const rawBlocks = sduiEnvelope?.page?.blocks || []
    if (!nodes || nodes.length === 0) return rawBlocks

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

    const toBlock = (node: any, isChild = false): BlockItem => {
      const orig = blockMap.get(node.id) || blockMap.get(node.id.replace(/_\d+$/, ''))
      const childrenList = node.children?.length ? node.children.map((c: any) => toBlock(c, true)) : undefined
      const mergedProps: Record<string, any> = {
        ...(orig?.props || {}),
        ...(node.props || {}),
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
    return nodes.map((n: any) => toBlock(n, false))
  }, [sduiEnvelope])

  // 微信好友分享 (完全由页面下发协议驱动)
  useShareAppMessage(() => {
    const friendConfig = sduiEnvelope?.page?.share_config?.friend
    if (friendConfig && friendConfig.enabled) {
      return {
        title: friendConfig.title || sduiEnvelope?.page?.title || '精选推荐',
        path: friendConfig.path || '/pages/index/index',
        imageUrl: friendConfig.image_url || sduiEnvelope?.page?.share_config?.default_image_url || ''
      }
    }
    return {
      title: sduiEnvelope?.page?.title || '精选主页',
      path: '/pages/index/index',
      imageUrl: sduiEnvelope?.page?.share_config?.default_image_url || ''
    }
  })

  // 微信朋友圈分享 (符合微信规范，采用 query)
  useShareTimeline(() => {
    const timelineConfig = sduiEnvelope?.page?.share_config?.timeline
    if (timelineConfig && timelineConfig.enabled) {
      return {
        title: timelineConfig.title || sduiEnvelope?.page?.title || '精选推荐',
        query: timelineConfig.query || 'from=timeline',
        imageUrl: timelineConfig.image_url || sduiEnvelope?.page?.share_config?.default_image_url || ''
      }
    }
    return {
      title: sduiEnvelope?.page?.title || '精选主页',
      query: 'from=timeline',
      imageUrl: sduiEnvelope?.page?.share_config?.default_image_url || ''
    }
  })

  // 支持微信小程序原生下拉刷新
  usePullDownRefresh(async () => {
    try {
      await fetchHomeData()
    } finally {
      Taro.stopPullDownRefresh()
    }
  })

  // 支持微信小程序原生触底加载
  useReachBottom(() => {
    const pageEvents = (sduiEnvelope?.page as any)?.events
    if (pageEvents?.reach_bottom) {
      handleBlockAction(pageEvents.reach_bottom, { __event: 'reach_bottom' })
    } else if (pageEvents?.load_more) {
      handleBlockAction(pageEvents.load_more, { __event: 'load_more' })
    }
  })

  const pageTitle = sduiEnvelope?.page?.title || '首页精选'
  const themeClass = `index-page-container theme-${sduiEnvelope?.page?.theme || 'dark_glass'}`

  return (
    <View className={themeClass}>
      {/* 苹果风毛玻璃胶囊对齐导航栏 (长按 1.2 秒触发动效实验室调试彩蛋) */}
      <View onTouchStart={handleNavTouchStart} onTouchEnd={handleNavTouchEnd}>
        <AppleNavbar
          title={debugMotionMode ? '🛠️ 动效调试模式 (长按恢复)' : pageTitle}
          subtitle={
            debugMotionMode
              ? 'Native Physics & Shader Lab'
              : sduiEnvelope?.page?.business_type ? `分类: ${sduiEnvelope.page.business_type}` : undefined
          }
        />
      </View>

      {/* 页面主滚动体 */}
      <ScrollView scrollY className='page-scroll-body'>
        {/* 开发者调试模式横幅 */}
        {debugMotionMode && (
          <View className='announcement-banner' style={{ background: 'rgba(10, 132, 255, 0.15)', borderColor: 'rgba(10, 132, 255, 0.3)' }}>
            <Text className='announcement-text' style={{ color: '#0a84ff' }}>
              🛠️ 开发者动效调试模式已激活 · 正在调试物理动效与 Shader 渲染引擎（长按标题或点击退出）
            </Text>
          </View>
        )}

        {/* 动效实验室彩蛋视图 */}
        {debugMotionMode && (
          <MotionLab
            onRetry={() => {
              setDebugMotionMode(false)
              fetchHomeData()
            }}
          />
        )}

        {/* 1. 加载中：苹果 HIG 磨砂骨架屏 */}
        {loading && !debugMotionMode && (
          <View className='skeleton-container'>
            <View className='skeleton-block skeleton-hero' />
            <View className='skeleton-block skeleton-title' />
            <View className='skeleton-block skeleton-tags' />
            <View className='skeleton-block skeleton-cards' />
          </View>
        )}

        {/* 2. 页面强制登录受保态 */}
        {!loading && !debugMotionMode && authRequired && (
          <View className='sdui-error-panel auth-panel'>
            <Text className='error-emoji'>🔒</Text>
            <Text className='error-title'>该主页需微信授权后查看</Text>
            <Text className='error-desc'>请点击下方按钮完成快速微信授权登录</Text>
            <View className='retry-btn' onClick={handleLoginRetry}>
              <Text>微信一键快捷授权</Text>
            </View>
          </View>
        )}

        {/* 3. 网络异常与重试面板 */}
        {!loading && !debugMotionMode && !authRequired && errorMsg && (
          <View className='sdui-error-panel'>
            <Text className='error-emoji'>⚠️</Text>
            <Text className='error-title'>主页加载失败</Text>
            <Text className='error-desc'>{errorMsg}</Text>
            <View className='retry-btn' onClick={fetchHomeData}>
              <Text>重新加载</Text>
            </View>
          </View>
        )}

        {/* 4. SDUI 通用原子积木树渲染 (消费 Layout IR，100% 同构) */}
        {!loading && !debugMotionMode && !authRequired && !errorMsg && effectiveSduiBlocks.length > 0 && (
          <View className='sdui-home-blocks-container' style={{ padding: '24rpx' }}>
            {effectiveSduiBlocks.map((block) => (
              <BlockRenderer
                key={block.id}
                block={block}
                onAction={handleBlockAction}
                context={{
                  entity: sduiEnvelope?.data,
                  page: sduiEnvelope?.page,
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

        {/* 5. 空态占位 */}
        {!loading && !debugMotionMode && !authRequired && !errorMsg && effectiveSduiBlocks.length === 0 && (
          <View className='sdui-empty-panel'>
            <Text className='empty-emoji'>📭</Text>
            <Text className='empty-title'>主页暂未配置内容</Text>
            <Text className='empty-desc'>请在管理后台为当前小程序配置并发布主页积木</Text>
          </View>
        )}
      </ScrollView>
    </View>
  )
}
