import Taro from '@tarojs/taro'
import { BlockAction } from '../types/sdui'
import { ensureSession } from './auth'
import { request } from './request'

/**
 * 统一万能交互动作执行上下文
 */
export interface ActionContext {
  // 页面刷新回调句柄
  refresh?: () => void
  // 当前动态绑定的实体数据
  entity?: any
}

/**
 * 万能原子交互动作分发器 (Action Dispatcher)
 * 支持登录拦截、跨小程序矩阵互跳、微信视频号原生拉起、剪贴板震动与多页面路由流转
 */
export async function dispatchAction(action?: BlockAction, context?: ActionContext): Promise<void> {
  if (!action || !action.type) return

  // 1. 拦截动作级登录鉴权 (未登录时先走免密登录闭环)
  if (action.require_auth) {
    const isAuthed = await ensureSession()
    if (!isAuthed) {
      Taro.showToast({
        title: '请先完成微信授权登录',
        icon: 'none'
      })
      return
    }
  }

  const payload = action.payload || {}

  // 2. 根据标准动作类型执行对应业务逻辑
  switch (action.type) {
    // 复制内容至系统剪贴板 (带震动反馈与 Toast 引导)
    case 'copy_text': {
      const textToCopy = payload.text || payload.content || ''
      if (!textToCopy) {
        Taro.showToast({ title: '暂无复制内容', icon: 'none' })
        return
      }

      Taro.setClipboardData({
        data: String(textToCopy),
        success: () => {
          // 触发轻柔触感震动反馈
          Taro.vibrateShort({ type: 'medium' })
          const toastText = payload.toast || '已成功复制到剪贴板'
          Taro.showToast({
            title: toastText,
            icon: 'none',
            duration: 2500
          })
        },
        fail: (err) => {
          console.error('复制文本失败:', err)
          Taro.showToast({ title: '复制失败，请重试', icon: 'none' })
        }
      })
      break
    }

    // 微信小程序多页面路由流转 (万能动态承载页)
    case 'navigate_page': {
      const targetPageId = payload.page_id || 'home'
      const queryParts: string[] = [`page_id=${encodeURIComponent(targetPageId)}`]

      // 携带附加业务 ID 或参数
      if (payload.id) {
        queryParts.push(`id=${encodeURIComponent(payload.id)}`)
      }
      if (payload.query && typeof payload.query === 'object') {
        Object.entries(payload.query).forEach(([k, v]) => {
          if (v !== undefined && v !== null) {
            queryParts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
          }
        })
      }

      Taro.navigateTo({
        url: `/pages/dynamic/index?${queryParts.join('&')}`,
        fail: (err) => {
          console.error('动态页面跳转失败:', err)
        }
      })
      break
    }

    // 微信视频号动态原生拉起 (调起微信原生剧场)
    case 'open_channels_activity': {
      const feedId = payload.feed_id || payload.feedId || ''
      const finderUserName = payload.finder_user_name || payload.finderUserName || ''

      if (!feedId || !finderUserName) {
        Taro.showToast({ title: '视频号参数未配置完整', icon: 'none' })
        return
      }

      const isWeapp = Taro.getEnv() === Taro.ENV_TYPE.WEAPP
      if (isWeapp && typeof (wx as any) !== 'undefined' && typeof (wx as any).openChannelsActivity === 'function') {
        ;(wx as any).openChannelsActivity({
          feedId,
          finderUserName,
          fail: (err: any) => {
            console.warn('拉起微信视频号失败:', err)
            Taro.showToast({ title: '拉起视频号失败，请稍后重试', icon: 'none' })
          }
        })
      } else {
        Taro.showToast({
          title: `[模拟器] 调起视频号: ${finderUserName}`,
          icon: 'none'
        })
      }
      break
    }

    // 跨小程序矩阵跳转 (流量互导与分流承接)
    case 'open_mini_program': {
      const targetAppId = payload.target_app_id || payload.app_id || payload.appId
      if (!targetAppId) {
        Taro.showToast({ title: '目标小程序 AppID 未指定', icon: 'none' })
        return
      }

      Taro.navigateToMiniProgram({
        appId: targetAppId,
        path: payload.target_path || payload.path || '',
        extraData: payload.extra_data || {},
        envVersion: payload.env_version || 'release',
        fail: (err) => {
          console.warn('跳转小程序失败:', err)
          Taro.showToast({ title: '跳转小程序失败', icon: 'none' })
        }
      })
      break
    }

    // 网页 WebView H5 直达打开
    case 'open_webview': {
      const url = payload.url || payload.web_url || ''
      if (!url) {
        Taro.showToast({ title: '链接地址为空', icon: 'none' })
        return
      }

      Taro.navigateTo({
        url: `/pages/webview/index?url=${encodeURIComponent(url)}&title=${encodeURIComponent(payload.title || '')}`
      })
      break
    }

    // 全屏大图预览
    case 'preview_image': {
      const current = payload.current || ''
      const urls: string[] = Array.isArray(payload.urls) ? payload.urls : (current ? [current] : [])
      if (urls.length === 0) return

      Taro.previewImage({
        current,
        urls
      })
      break
    }

    // 文字轻提示
    case 'toast': {
      Taro.showToast({
        title: payload.text || payload.toast || '提示',
        icon: (payload.icon as any) || 'none',
        duration: payload.duration || 2000
      })
      break
    }

    // 触发刷新
    case 'refresh': {
      if (typeof context?.refresh === 'function') {
        context.refresh()
      }
      break
    }

    // 异步受控业务数据请求与事务触发 (如领取游戏独家礼包兑换码)
    case 'request_data': {
      const endpoint = payload.endpoint || 'game.redeem'
      const reqPayload = payload.payload || {}
      const idempotencyKey = payload.idempotency_key || `idem_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`

      Taro.showLoading({ title: '正在领取...', mask: true })
      try {
        const res = await request<any>({
          url: '/api/v1/action/execute',
          method: 'POST',
          data: {
            endpoint,
            payload: reqPayload,
            idempotency_key: idempotencyKey
          }
        })
        Taro.hideLoading()

        if (res.code === 0 && res.data) {
          // 若分配了真实兑换码，直接写入系统剪贴板并触发触感震动反馈
          if (res.data.code) {
            Taro.setClipboardData({
              data: String(res.data.code),
              success: () => {
                Taro.vibrateShort({ type: 'medium' })
                Taro.showToast({
                  title: `✅ 兑换码 ${res.data.code} 已成功复制！`,
                  icon: 'none',
                  duration: 3000
                })
              }
            })
          } else {
            Taro.showToast({
              title: res.msg || '操作成功',
              icon: 'success'
            })
          }

          // 触发级联成功动作链 (如配置了 on_success)
          if (Array.isArray(payload.on_success)) {
            for (const subAction of payload.on_success) {
              await dispatchAction(subAction, context)
            }
          }
        } else {
          Taro.showToast({
            title: res.msg || '操作失败，请稍后重试',
            icon: 'none'
          })
        }
      } catch (err: any) {
        Taro.hideLoading()
        Taro.showToast({
          title: err.message || '网络异常，请重试',
          icon: 'none'
        })
      }
      break
    }

    default:
      console.warn(`未识别或暂不支持的 SDUI 动作类型: ${action.type}`)
      break
  }
}
