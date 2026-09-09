// minifront/src/app.ts
// 安全注入 process 垫片，防止微信小程序运行时因无 process 对象崩溃
if (typeof (globalThis as any).process === 'undefined') {
  ;(globalThis as any).process = { env: {} }
}

import { PropsWithChildren } from 'react'
import { useLaunch } from '@tarojs/taro'
import { ensureSession } from './utils/auth'
import './app.scss'

function App({ children }: PropsWithChildren) {
  useLaunch(() => {
    console.log('App launched.')
    void ensureSession()
  })

  return children
}

export default App
