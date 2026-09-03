import React from 'react'
import { View, Text } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { AppleNavbar } from '../../components/AppleNavbar'
import { MotionLab } from '../../components/MotionLab'
import './index.scss'

export default function MotionDebugPage() {
  // 返回短剧首页
  const handleBackHome = () => {
    Taro.reLaunch({
      url: '/pages/index/index'
    })
  }

  return (
    <View className='motion-page-container'>
      {/* 顶部苹果风导航栏 */}
      <AppleNavbar
        title='灵动视界 · 动效实验室'
        subtitle='Native Physics & Shader Lab (调试工作台)'
      />

      <View className='motion-page-body'>
        {/* 调试模式醒目指示条 */}
        <View className='debug-info-banner'>
          <View className='badge'><Text>🛠️ DEBUG 模式</Text></View>
          <Text className='desc'>当前处于物理动效与 Shader 独立调试视窗，代码保存即时热重载</Text>
          <View className='back-btn apple-press-feedback' onClick={handleBackHome}>
            <Text>返回短剧首页 ›</Text>
          </View>
        </View>

        {/* 动效核心实验室 */}
        <MotionLab onRetry={handleBackHome} />
      </View>
    </View>
  )
}
