// index.tsx
import React, { useMemo } from 'react'
import { View, Text } from '@tarojs/components'
import Taro from '@tarojs/taro'
import './index.scss'

interface AppleNavbarProps {
  title?: string
  subtitle?: string
}

export const AppleNavbar: React.FC<AppleNavbarProps> = ({
  title = '猴王下山',
  subtitle = '全网热播短剧'
}) => {
  // 精确获取微信官方胶囊按钮和系统状态栏信息
  const { navBarHeight, statusBarHeight } = useMemo(() => {
    try {
      const menu = Taro.getMenuButtonBoundingClientRect()
      const sys = Taro.getSystemInfoSync()
      const statusHeight = Number(sys.statusBarHeight) > 0 ? Number(sys.statusBarHeight) : 20

      // 微信官方推荐导航栏总高度公式: (胶囊top - 状态栏height)*2 + 胶囊height + 状态栏height
      let navHeight = 88
      if (menu && Number(menu.top) >= statusHeight && Number(menu.height) > 0) {
        navHeight = (Number(menu.top) - statusHeight) * 2 + Number(menu.height) + statusHeight
      } else {
        navHeight = statusHeight + 44
      }

      return {
        navBarHeight: Math.max(navHeight, 64),
        statusBarHeight: statusHeight
      }
    } catch {
      return {
        navBarHeight: 64,
        statusBarHeight: 20
      }
    }
  }, [])

  return (
    <>
      {/* 固定在顶部的导航栏 */}
      <View className='apple-navbar-container' style={{ height: `${navBarHeight}px` }}>
        <View
          className='apple-navbar-content'
          style={{
            paddingTop: `${statusBarHeight}px`,
            height: `${navBarHeight - statusBarHeight}px`
          }}
        >
          <View className='nav-title-group'>
            <Text className='nav-title'>{title}</Text>
            {subtitle && <Text className='nav-subtitle'>{subtitle}</Text>}
          </View>
        </View>
      </View>
      {/* 真实文档流占位块，确保下方任何内容绝不被胶囊和导航栏遮挡 */}
      <View className='apple-navbar-placeholder' style={{ height: `${navBarHeight}px` }} />
    </>
  )
}
