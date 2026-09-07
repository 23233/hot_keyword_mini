// minifront/src/app.config.ts
const appConfig = {
  pages: [
    'pages/index/index',
    'pages/dynamic/index',
    'pages/webview/index',
    'pages/motion/index'
  ],
  window: {
    backgroundTextStyle: 'dark',
    navigationBarBackgroundColor: '#000000',
    navigationBarTitleText: '热点精选',
    navigationBarTextStyle: 'white',
    navigationStyle: 'custom'
  },
  // 微信开发者工具 Agent 原子测试所需的本地能力描述。
  agent: {
    skills: [
      {
        name: 'sdui-acceptance',
        path: 'agent'
      }
    ]
  }
} as any

export default defineAppConfig(appConfig)
