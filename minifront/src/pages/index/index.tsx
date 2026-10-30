// minifront/src/pages/index/index.tsx
import { useEffect } from 'react'
import Taro from '@tarojs/taro'
import SDUIPageContainer from '../../components/SDUI/SDUIPageContainer'

/**
 * 小程序首页入口：复用统一 SDUI 容器，仅固定 page_id=home 与首页分享兜底。
 * 动效实验室已独立到 pages/motion/index，不再在首页容器内维护第二套渲染逻辑。
 */
export default function Index() {
  useEffect(() => {
    const params = Taro.getCurrentInstance().router?.params || {}
    if (params.mode === 'motion' || params.debug === 'motion') {
      Taro.redirectTo({ url: '/pages/motion/index' })
    }
  }, [])

  return (
    <SDUIPageContainer
      pageId='home'
      defaultTitle='首页精选'
      shareBasePath='/pages/index/index'
      defaultTimelineQuery='from=timeline'
    />
  )
}
