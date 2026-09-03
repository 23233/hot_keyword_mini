import React, { useState, useEffect, useCallback } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import Taro, { useShareAppMessage, useShareTimeline } from '@tarojs/taro'
import { PageResponseEnvelope } from '../../types/sdui'
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

  // 获取 URL Query 传参
  const routerParams = Taro.getCurrentInstance().router?.params || {}
  const pageId = routerParams.page_id || 'home'

  // 拉取服务端动态页面协议
  const fetchPageProtocol = useCallback(async () => {
    try {
      setLoading(true)
      setErrorMsg('')

      const queryEntries = Object.entries(routerParams)
        .filter(([k]) => k !== 'page_id')
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
      const queryStr = queryEntries.length > 0 ? `?${queryEntries.join('&')}` : ''

      const res = await request<PageResponseEnvelope>({
        url: `/api/v1/page/${pageId}${queryStr}`,
        method: 'GET'
      })

      setEnvelope(res)

      // 动态设置小程序导航栏标题
      if (res?.page?.title) {
        Taro.setNavigationBarTitle({
          title: res.page.title
        })
      }

      // 若页面声明全页受保强制鉴权，触发登录闭环
      if (res?.page?.require_auth) {
        await ensureSession()
      }
    } catch (err: any) {
      console.error(`获取 SDUI 动态页面 ${pageId} 失败:`, err)
      setErrorMsg(err.message || '网络连接异常，未能加载内容')
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

  // 处理积木交互点击
  const handleBlockAction = (action?: any) => {
    dispatchAction(action, {
      refresh: fetchPageProtocol,
      entity: envelope?.data
    })
  }

  const themeClass = `dynamic-page-container theme-${envelope?.page?.theme || 'dark_glass'}`
  const pageTitle = envelope?.page?.title || '精选剧场'

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

        {/* 2. 网络错误提示与重试 */}
        {!loading && errorMsg && (
          <View className="sdui-error-panel">
            <Text className="error-emoji">⚠️</Text>
            <Text className="error-title">页面加载失败</Text>
            <Text className="error-desc">{errorMsg}</Text>
            <View className="retry-btn" onClick={fetchPageProtocol}>
              <Text>重新加载</Text>
            </View>
          </View>
        )}

        {/* 3. 积木列表渲染 */}
        {!loading && !errorMsg && envelope?.page?.blocks && (
          <View className="blocks-container">
            {envelope.page.blocks.map((block) => (
              <BlockRenderer
                key={block.id}
                block={block}
                onAction={handleBlockAction}
                context={{ entity: envelope?.data, query: router.params }}
              />
            ))}
          </View>
        )}

        {/* 4. 暂无积木空态 */}
        {!loading && !errorMsg && (!envelope?.page?.blocks || envelope.page.blocks.length === 0) && (
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
