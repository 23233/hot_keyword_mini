// env.ts
import Taro from '@tarojs/taro'

/**
 * 生产环境合法 HTTPS 接口主域名
 * 支持通过环境变量 TARO_APP_API_BASE_URL 注入覆盖
 */
const envBaseUrl = (typeof process !== 'undefined' && process.env?.TARO_APP_API_BASE_URL)
  ? process.env.TARO_APP_API_BASE_URL.replace(/\/+$/, '')
  : ''

export const PRODUCTION_API_URL = envBaseUrl

/**
 * 体验版测试环境域名
 */
export const TRIAL_API_URL = envBaseUrl

/**
 * 本地开发默认端口
 */
export const LOCAL_DEV_URL = envBaseUrl || 'http://127.0.0.1:8080'

/**
 * 获取当前小程序运行时所处环境
 * develop: 微信开发者工具或真机开发版
 * trial: 体验版
 * release: 正式发布线上版
 */
export function getMiniProgramEnv(): 'develop' | 'trial' | 'release' {
  try {
    const accountInfo = Taro.getAccountInfoSync?.()
    if (accountInfo?.miniProgram?.envVersion) {
      return accountInfo.miniProgram.envVersion as 'develop' | 'trial' | 'release'
    }
  } catch (e) {
    // 降级兜底
  }
  return 'develop'
}

/**
 * 统一获取当前环境的最佳 API 基础请求地址
 * 支持 process.env.TARO_APP_API_BASE_URL 编译期注入与真机自适应
 */
export function getBaseUrl(): string {
  // 1. 优先读取编译期注入的环境变量
  if (envBaseUrl) {
    return envBaseUrl
  }

  const env = getMiniProgramEnv()

  // 2. 正式发布包只使用构建时注入的线上域名
  if (env === 'release') {
    if (!PRODUCTION_API_URL) {
      throw new Error('未配置生产 API 域名，请注入 TARO_APP_API_BASE_URL')
    }
    return PRODUCTION_API_URL
  }

  // 3. 体验版: 使用构建时注入的 HTTPS 域名
  if (env === 'trial') {
    if (!TRIAL_API_URL) {
      throw new Error('未配置体验版 API 域名，请注入 TARO_APP_API_BASE_URL')
    }
    return TRIAL_API_URL
  }

  // 4. 开发版: 允许通过 Storage 动态指定真机调试 IP (避免真机 127.0.0.1 指向手机自身)
  try {
    const customUrl = Taro.getStorageSync('custom_api_base_url')
    if (customUrl) {
      return String(customUrl).replace(/\/+$/, '')
    }
  } catch (e) {
    // ignore
  }

  return LOCAL_DEV_URL
}
