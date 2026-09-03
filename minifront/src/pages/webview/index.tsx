import React, { useEffect, useState } from 'react'
import { View, Text, WebView } from '@tarojs/components'
import Taro from '@tarojs/taro'
import './index.scss'

export default function WebViewPage() {
  const [targetUrl, setTargetUrl] = useState<string>('')
  const [errorMsg, setErrorMsg] = useState<string>('')

  useEffect(() => {
    // 从路由参数中读取 url 与 title
    const params = Taro.getCurrentInstance().router?.params || {}
    const rawUrl = params.url

    if (!rawUrl) {
      setErrorMsg('未提供有效的网页跳转链接')
      return
    }

    try {
      const decodedUrl = decodeURIComponent(rawUrl)
      // 验证 URL 合法性
      if (decodedUrl.startsWith('http://') || decodedUrl.startsWith('https://')) {
        setTargetUrl(decodedUrl)
      } else {
        setErrorMsg('无效的网页地址格式 (必须以 http:// 或 https:// 开头)')
      }

      // 如果传了自定义标题，动态更新导航栏
      if (params.title) {
        Taro.setNavigationBarTitle({
          title: decodeURIComponent(params.title)
        })
      }
    } catch (e: any) {
      setErrorMsg('网页链接解析失败: ' + e.message)
    }
  }, [])

  // 返回首页
  const handleBackHome = () => {
    Taro.reLaunch({
      url: '/pages/index/index'
    })
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
