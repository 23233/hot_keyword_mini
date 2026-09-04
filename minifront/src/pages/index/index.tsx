// minifront/src/pages/index/index.tsx
import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { View, Text } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline } from '@tarojs/taro'
import { DramaHomeData, DramaInfo, EpisodeItem, ActionChannel } from '../../types/drama'
import { PageResponseEnvelope, BlockItem } from '../../types/sdui'
import { request } from '../../utils/request'
import { dispatchAction } from '../../utils/action'
import { AppleNavbar } from '../../components/AppleNavbar'
import { ActionModal } from '../../components/ActionModal'
import { ImmersiveVideoView } from '../../components/ImmersiveVideoView'
import { EpisodeGridView } from '../../components/EpisodeGridView'
import { DirectPortalView } from '../../components/DirectPortalView'
import { GalleryMatrixView } from '../../components/GalleryMatrixView'
import { DetailPlayerModal } from '../../components/DetailPlayerModal'
import { MotionLab } from '../../components/MotionLab'
import { BlockRenderer } from '../../components/SDUI/BlockRenderer'
import './index.scss'

export default function Index() {
  // SDUI 动态主页优先信封
  const [sduiEnvelope, setSduiEnvelope] = useState<PageResponseEnvelope | null>(null)
  // 首页全量驱动短剧数据 (降级模式)
  const [pageData, setPageData] = useState<DramaHomeData | null>(null)
  // 加载中状态
  const [loading, setLoading] = useState<boolean>(true)
  // 错误信息
  const [errorMsg, setErrorMsg] = useState<string>('')
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
          title: next ? '🛠️ 动效调试模式' : '🎬 恢复短剧模式',
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

  // 看后续核心弹窗
  const [actionModalVisible, setActionModalVisible] = useState<boolean>(false)
  const [targetEpisodeNum, setTargetEpisodeNum] = useState<number | undefined>(undefined)


  // 短剧播放详情弹窗/抽屉 (画廊模式点击进入)
  const [detailModalVisible, setDetailModalVisible] = useState<boolean>(false)
  const [detailDrama, setDetailDrama] = useState<DramaInfo | null>(null)
  const [detailEpisodes, setDetailEpisodes] = useState<EpisodeItem[]>([])

  // 从后端接口拉取首页数据 (优先连接 SDUI 线上激活主页，无则优雅降级为短剧模式)
  const fetchHomeData = useCallback(async () => {
    try {
      setLoading(true)
      setErrorMsg('')

      // 1. 尝试拉取当前租户设为“激活主页”的 SDUI 动态页面协议
      try {
        const sduiRes = await request<PageResponseEnvelope>({
          url: '/api/v1/page/home',
          method: 'GET'
        })

        if (sduiRes?.page?.blocks && sduiRes.page.blocks.length > 0) {
          setSduiEnvelope(sduiRes)
          if (sduiRes.page.title) {
            Taro.setNavigationBarTitle({ title: sduiRes.page.title })
          }
          setLoading(false)
          return
        }
      } catch (sduiErr) {
        // SDUI 页面不存在或处于草稿未发布时，静默走经典短剧接口降级
        console.log('未配置或未发布 SDUI 首页，进入经典短剧主页模式')
      }

      // 2. 降级为经典短剧业务主页
      setSduiEnvelope(null)
      const data = await request<DramaHomeData>({
        url: '/api/v1/drama/home',
        method: 'GET'
      })

      setPageData(data)

      // 动态设置小程序标题 (完全由接口驱动)
      if (data?.page_title) {
        Taro.setNavigationBarTitle({
          title: data.page_title
        })
      }

      // 若当前模式为 webview，且配置了有效目标 url，自动重定向至 webview 页面
      if (data?.display_mode === 'webview') {
        const targetUrl = data.webview_url || data.drama?.web_url
        if (targetUrl) {
          Taro.redirectTo({
            url: `/pages/webview/index?url=${encodeURIComponent(targetUrl)}&title=${encodeURIComponent(data.page_title || '')}`
          })
          return
        }
      }
    } catch (err: any) {
      console.error('获取首页数据失败:', err)
      setErrorMsg(err.message || '网络连接异常，未能加载内容')
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

  // 处理 SDUI 积木交互点击 (注入响应式 pageState 与 updateState 回调，透传 query 等上下文)
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

  // 首页与动态页统一消费服务端 Layout IR，避免同一页面走两套 block 解释路径。
  const effectiveSduiBlocks = useMemo<BlockItem[]>(() => {
    const nodes = sduiEnvelope?.layout_ir?.nodes
    if (!nodes || nodes.length === 0) return sduiEnvelope?.page?.blocks || []
    const toBlock = (node: any): BlockItem => ({
      id: node.id,
      type: node.type,
      props: {
        ...(node.props || {}),
        ...(node.children?.length ? { children: node.children.filter((child: any) => child.visible !== false).map(toBlock) } : {}),
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
    return nodes.filter((node) => node.visible !== false).map(toBlock)
  }, [sduiEnvelope])

  // 微信转发分享 (优先消费 SDUI 激活页面配置，降级使用旧版 pageData)
  useShareAppMessage(() => {
    if (sduiEnvelope?.page?.share_config?.friend?.enabled) {
      const friendConfig = sduiEnvelope.page.share_config.friend
      return {
        title: friendConfig.title || sduiEnvelope.page.title || '精选推荐',
        path: friendConfig.path || '/pages/index/index',
        imageUrl: friendConfig.image_url || sduiEnvelope.page.share_config.default_image_url || ''
      }
    }
    if (sduiEnvelope?.page) {
      return {
        title: sduiEnvelope.page.title || '精选推荐',
        path: '/pages/index/index',
        imageUrl: sduiEnvelope.page.share_config?.default_image_url || ''
      }
    }
    return {
      title: pageData?.share_title || pageData?.page_title || '',
      path: '/pages/index/index',
      imageUrl: pageData?.share_cover || pageData?.drama?.cover_url || ''
    }
  })

  // 微信朋友圈分享 (优先消费 SDUI 激活页面配置，降级使用旧版 pageData)
  useShareTimeline(() => {
    if (sduiEnvelope?.page?.share_config?.timeline?.enabled) {
      const timelineConfig = sduiEnvelope.page.share_config.timeline
      return {
        title: timelineConfig.title || sduiEnvelope.page.title || '精选推荐',
        query: timelineConfig.query || '',
        imageUrl: timelineConfig.image_url || sduiEnvelope.page.share_config.default_image_url || ''
      }
    }
    if (sduiEnvelope?.page) {
      return {
        title: sduiEnvelope.page.title || '精选推荐',
        query: '',
        imageUrl: sduiEnvelope.page.share_config?.default_image_url || ''
      }
    }
    return {
      title: pageData?.share_title || pageData?.page_title || '',
      query: '',
      imageUrl: pageData?.share_cover || pageData?.drama?.cover_url || ''
    }
  })

  // 打开看后续弹窗
  const handleOpenActionModal = (epNum?: number) => {
    setTargetEpisodeNum(epNum)
    setActionModalVisible(true)
  }

  // 画廊模式下点击某部短剧：调起播放详情抽屉
  const handleSelectGalleryDrama = async (drama: DramaInfo) => {
    setDetailDrama(drama)
    try {
      Taro.showLoading({ title: '加载中...' })
      const res = await request<{ drama: DramaInfo; episodes: EpisodeItem[] }>({
        url: `/api/v1/drama/detail?id=${drama.id}`,
        method: 'GET'
      })
      setDetailEpisodes(res.episodes || [])
      setDetailModalVisible(true)
    } catch {
      setDetailEpisodes([])
      setDetailModalVisible(true)
    } finally {
      Taro.hideLoading()
    }
  }

  // 提取网盘通用渠道 (如有网盘数据就显示网盘)
  const panChannel: ActionChannel | undefined = pageData?.action_channels?.find(
    (c) => c.type === 'pan'
  )

  // 处理动态浮动按钮点击事件
  const handleFloatingAction = () => {
    if (!pageData?.floating_button) return
    const fb = pageData.floating_button
    if (fb.action_type === 'open_modal') {
      handleOpenActionModal()
    } else if (fb.action_type === 'copy_pan' && panChannel) {
      let copyText = panChannel.content
      if (panChannel.fetch_code) {
        copyText = `${panChannel.content} 提取码: ${panChannel.fetch_code}`
      }
      Taro.setClipboardData({
        data: copyText,
        success: () => {
          Taro.showToast({ title: '网盘链接已复制', icon: 'success' })
        }
      })
    } else {
      handleOpenActionModal()
    }
  }

  return (
    <View className='index-page-container'>
      {/* 苹果风毛玻璃导航栏 (长按 1.2 秒触发动效实验室调试彩蛋) */}
      <View onTouchStart={handleNavTouchStart} onTouchEnd={handleNavTouchEnd}>
        <AppleNavbar
          title={
            debugMotionMode
              ? '🛠️ 动效调试模式 (长按恢复)'
              : pageData?.page_title || pageData?.drama?.title || (errorMsg ? '灵动视界 · 动效实验室' : '猴王下山')
          }
          subtitle={
            debugMotionMode
              ? 'Native Physics & Shader Lab'
              : pageData?.page_subtitle || pageData?.drama?.subtitle || (errorMsg ? '动效艺术工坊' : '精选短剧')
          }
        />
      </View>

      <View className='page-body'>
        {/* 开发者调试模式提示横幅 */}
        {debugMotionMode && (
          <View className='announcement-banner' style={{ background: 'rgba(10, 132, 255, 0.15)', borderColor: 'rgba(10, 132, 255, 0.3)' }}>
            <Text className='announcement-text' style={{ color: '#0a84ff' }}>
              🛠️ 开发者动效调试模式已激活 · 正在调试物理动效与 Shader 渲染引擎（长按标题或点击退出）
            </Text>
          </View>
        )}

        {/* 顶部通告横幅 (接口有且非调试模式时显示) */}
        {!debugMotionMode && pageData?.announcement && (
          <View className='announcement-banner'>
            <Text className='announcement-text'>{pageData.announcement}</Text>
          </View>
        )}

        {/* 开发者强制调试模式 / 接口未连接异常：直接呈现动效与 Shader 视觉实验室 */}
        {(debugMotionMode || (!loading && errorMsg)) && (
          <MotionLab
            onRetry={() => {
              setDebugMotionMode(false)
              fetchHomeData()
            }}
          />
        )}

        {/* 1. 加载中：苹果风骨架屏 */}
        {loading && !debugMotionMode && (
          <View className='skeleton-container'>
            <View className='skeleton-block skeleton-hero' />
            <View className='skeleton-block skeleton-title' />
            <View className='skeleton-block skeleton-tags' />
            <View className='skeleton-block skeleton-cards' />
          </View>
        )}

        {/* 2. SDUI 动态激活主页渲染 (当后台将任一页面设为激活主页时生效) */}
        {!loading && !debugMotionMode && effectiveSduiBlocks.length > 0 && (
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

        {/* 3. 常规短剧模式渲染 (未配置 SDUI 时的优雅降级底座) */}
        {!loading && !debugMotionMode && !sduiEnvelope && pageData && (
          <View className='mode-render-wrapper'>
            {/* 风格 1：沉浸影音模式 */}
            {pageData.display_mode === 'immersive_video' && (
              <ImmersiveVideoView
                drama={pageData.drama}
                episodes={pageData.episodes}
                panChannel={panChannel}
                onOpenActionModal={handleOpenActionModal}
              />
            )}


            {/* 风格 2：剧集矩阵模式 */}
            {pageData.display_mode === 'episode_grid' && (
              <EpisodeGridView
                drama={pageData.drama}
                episodes={pageData.episodes}
                recommendations={pageData.recommendations}
                onOpenActionModal={handleOpenActionModal}
              />
            )}

            {/* 风格 3：极速直达模式 */}
            {pageData.display_mode === 'direct_portal' && (
              <DirectPortalView
                drama={pageData.drama}
                channels={pageData.action_channels}
              />
            )}

            {/* 风格 4：短剧画廊矩阵模式 */}
            {pageData.display_mode === 'gallery_matrix' && (
              <GalleryMatrixView
                dramaList={pageData.gallery_list || []}
                onSelectDrama={handleSelectGalleryDrama}
                onOpenActionModal={() => handleOpenActionModal()}
              />
            )}

            {/* 风格 5：Webview 独立网页直达模式 */}
            {pageData.display_mode === 'webview' && (
              <View className='webview-portal-card apple-card'>
                <Text className='portal-icon'>🌐</Text>
                <Text className='portal-title'>{pageData.page_title || '正版网页专区'}</Text>
                <Text className='portal-sub'>{pageData.page_subtitle || '即将前往正版高清播放页面'}</Text>
                <View
                  className='portal-btn apple-pill-btn'
                  onClick={() => {
                    const targetUrl = pageData.webview_url || pageData.drama?.web_url
                    if (targetUrl) {
                      Taro.navigateTo({
                        url: `/pages/webview/index?url=${encodeURIComponent(targetUrl)}&title=${encodeURIComponent(pageData.page_title || '')}`
                      })
                    } else {
                      Taro.showToast({ title: '未配置目标网页地址', icon: 'none' })
                    }
                  }}
                >
                  <Text className='btn-text'>点击立即进入网页</Text>
                </View>
              </View>
            )}
          </View>
        )}
      </View>

      {/* 动态悬浮按钮 (有浮动的按钮就显示浮动的，接口未配置则不显示) */}
      {pageData?.floating_button && pageData.floating_button.is_visible && (
        <View className='page-floating-action apple-press-feedback' onClick={handleFloatingAction}>
          {pageData.floating_button.badge && (
            <View className='fab-badge'><Text>{pageData.floating_button.badge}</Text></View>
          )}
          {pageData.floating_button.icon && (
            <Text className='fab-icon'>{pageData.floating_button.icon}</Text>
          )}
          <Text className='fab-text'>{pageData.floating_button.text}</Text>
        </View>
      )}

      {/* 看后续全集核心交互弹窗 (全动态文本) */}
      <ActionModal
        visible={actionModalVisible}
        onClose={() => setActionModalVisible(false)}
        channels={pageData?.action_channels || []}
        targetEpisodeNum={targetEpisodeNum}
        dramaTitle={pageData?.drama?.title}
        totalEpisodes={pageData?.drama?.total_episodes}
      />

      {/* 剧集播放详情弹窗 (通用播放底座与选集网格) */}
      <DetailPlayerModal
        visible={detailModalVisible}
        onClose={() => setDetailModalVisible(false)}
        drama={detailDrama}
        episodes={detailEpisodes}
        panChannel={panChannel}
        onOpenActionModal={handleOpenActionModal}
      />
    </View>
  )
}
