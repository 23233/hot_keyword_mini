// action.ts
import Taro from '@tarojs/taro'
import { BlockAction } from '../types/sdui'
import { ensureSession } from './auth'
import { request } from './request'
import { evaluateCondition } from './condition'

/**
 * 统一万能交互动作执行上下文
 */
export interface ActionContext {
  // 页面刷新回调句柄
  refresh?: () => void
  // 当前动态绑定的实体数据
  entity?: any
  // URL Query 参数
  query?: Record<string, any>
  // 列表循环项数据
  item?: any
  // 页面内部状态
  state?: Record<string, any>
  // 响应式状态更新回调 (触发 React 页面重新渲染)
  updateState?: (key: string, value: any) => void
  // 请求动作关联 block 的状态切换
  blockStates?: Record<string, string>
  setBlockState?: (blockId: string, state: string) => void
  // 上一步执行结果
  result?: any
  // 页面与租户公开上下文
  page?: any
  session?: any
  tenant?: any
}

/**
 * 安全深度读取对象属性
 */
function getByPath(obj: any, path: string): any {
  if (!obj || !path) return undefined
  const parts = path.split('.')
  let curr = obj
  for (const p of parts) {
    if (curr == null) return undefined
    curr = curr[p]
  }
  return curr
}

/**
 * 受控绑定路径求值辅助函数
 * 支持解析 $entity.*, $query.*, $item.*, $state.*, $result.*
 */
export function resolveBindingValue(val: any, context?: ActionContext): any {
  if (val == null) return val

  // 1. 处理显式对象路径形式: { path: "$entity.title" }
  if (typeof val === 'object' && val.path && typeof val.path === 'string') {
    return resolvePathString(val.path, context)
  }

  // 2. 处理直接字符串路径形式: "$entity.title" 或 "{{entity.title}}"
  if (typeof val === 'string') {
    if (val.startsWith('$')) {
      return resolvePathString(val, context)
    }
    if (val.startsWith('{{') && val.endsWith('}}')) {
      const path = val.slice(2, -2).trim()
      return resolvePathString(path.startsWith('$') ? path : `$${path}`, context)
    }
  }

  return val
}

function resolvePathString(path: string, context?: ActionContext): any {
  if (path.startsWith('$entity.') && context?.entity) {
    return getByPath(context.entity, path.slice('$entity.'.length))
  }
  if (path.startsWith('$query.') && context?.query) {
    return getByPath(context.query, path.slice('$query.'.length))
  }
  if (path.startsWith('$item.') && context?.item) {
    return getByPath(context.item, path.slice('$item.'.length))
  }
  if (path.startsWith('$state.') && context?.state) {
    return getByPath(context.state, path.slice('$state.'.length))
  }
  if (path.startsWith('$result.') && (context as any)?.result) {
    return getByPath((context as any).result, path.slice('$result.'.length))
  }
  if (path.startsWith('$page.') && context?.page) {
    return getByPath(context.page, path.slice('$page.'.length))
  }
  if (path.startsWith('$session.') && context?.session) {
    return getByPath(context.session, path.slice('$session.'.length))
  }
  if (path.startsWith('$tenant.') && context?.tenant) {
    return getByPath(context.tenant, path.slice('$tenant.'.length))
  }
  return path
}

/**
 * 递归解析对象/数组中的所有绑定值
 */
export function resolveObjectBindings(target: any, context?: ActionContext): any {
  if (target == null) return target

  // 若自身是绑定描述
  if (typeof target === 'object' && target.path && typeof target.path === 'string') {
    return resolveBindingValue(target, context)
  }

  if (typeof target === 'string') {
    return resolveBindingValue(target, context)
  }

  if (Array.isArray(target)) {
    return target.map((item) => resolveObjectBindings(item, context))
  }

  if (typeof target === 'object') {
    const result: Record<string, any> = {}
    for (const k of Object.keys(target)) {
      result[k] = resolveObjectBindings(target[k], context)
    }
    return result
  }

  return target
}

/**
 * 解析 block 属性但保留嵌套子 block，避免父容器提前消费子项的 $item/$state 绑定。
 */
export function resolveBlockPropsBindings(props: Record<string, any>, context?: ActionContext): Record<string, any> {
  const resolve = (value: any): any => {
    if (value == null) return value
    if (Array.isArray(value)) return value.map(resolve)
    if (typeof value === 'object') {
      if (typeof value.type === 'string' && typeof value.id === 'string') return value
      const result: Record<string, any> = {}
      Object.keys(value).forEach((key) => {
        result[key] = resolve(value[key])
      })
      return result
    }
    return resolveObjectBindings(value, context)
  }
  return resolve(props) || {}
}

/**
 * 批量分发积木事件动作列表 (支持 events.tap 动作序列按序执行)
 */
export async function dispatchEvents(events?: Record<string, BlockAction[] | BlockAction>, eventName = 'tap', context?: ActionContext): Promise<void> {
  if (!events) return
  const target = events[eventName]
  if (!target) return

  if (Array.isArray(target)) {
    for (const action of target) {
      await dispatchAction(action, context)
    }
  } else {
    await dispatchAction(target, context)
  }
}

/**
 * 万能原子交互动作分发器 (Action Dispatcher)
 * 支持登录拦截、跨小程序矩阵互跳、微信视频号原生拉起、剪贴板震动与多页面路由流转
 */
export async function dispatchAction(action?: BlockAction, context?: ActionContext): Promise<void> {
  if (!action || !action.type) return

  // 1. 动作执行前置受控条件求值 (若配置了 condition 且条件不满足则中断)
  if (action.condition) {
    const isMet = evaluateCondition(action.condition, {
      entity: context?.entity,
      query: context?.query,
      state: context?.state,
      item: context?.item,
      result: context?.result
    })
    if (!isMet) {
      return
    }
  }

  // 2. 交互前置二次确认弹窗 (confirm)
  if (action.confirm) {
    const title = action.confirm.title || '操作提示'
    const content = action.confirm.message || action.confirm.content || '确认执行此操作？'
    const confirmText = action.confirm.confirm_text || '确定'
    const cancelText = action.confirm.cancel_text || '取消'
    const modalRes = await Taro.showModal({
      title,
      content,
      confirmText,
      cancelText
    })
    if (!modalRes.confirm) {
      return
    }
  }

  // 3. 拦截动作级登录鉴权 (未登录时先走免密登录闭环)
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

  // 4. 动作数据埋点上报 (track)
  if (action.track) {
    console.log('[SDUI Action Track]', action.track.event_name || action.track.event_id, action.track.params)
  }

  const payload = action.payload || {}
  let actionSuccess = true
  let actionResult: any = undefined

  try {
    // 5. 根据标准动作类型执行对应业务逻辑
    switch (action.type) {
      // 强制触发登录授权
      case 'require_auth': {
        await ensureSession()
        break
      }

    // 复制内容至系统剪贴板 (带震动反馈与 Toast 引导)
    case 'copy_text': {
      let textToCopy = payload.text || payload.content || payload.path || ''
      textToCopy = resolveBindingValue(textToCopy, context)

      // 智能兜底: 若未显式提取出文本，但上下文中存在执行结果兑换码，则自动提取该兑换码
      if (!textToCopy && context?.result) {
        if (typeof context.result === 'object' && context.result.code) {
          textToCopy = context.result.code
        } else if (typeof context.result === 'string') {
          textToCopy = context.result
        }
      }

      if (!textToCopy) {
        Taro.showToast({ title: '暂无可复制内容', icon: 'none' })
        return
      }

      Taro.setClipboardData({
        data: String(textToCopy),
        success: () => {
          Taro.vibrateShort({ type: 'medium' })
          const toastText = payload.toast || `已成功复制: ${textToCopy}`
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

    // 页面内轻提示 (Toast)
    case 'toast': {
      const msg = resolveBindingValue(payload.text || payload.message || '操作已执行', context)
      Taro.showToast({
        title: String(msg),
        icon: payload.icon || 'none',
        duration: payload.duration || 2000
      })
      break
    }

    // 刷新当前页面协议与数据
    case 'refresh': {
      if (context?.refresh) {
        context.refresh()
      }
      break
    }

    // 微信小程序多页面路由流转 (万能动态承载页)
    case 'navigate_page': {
      const targetPageId = payload.page_id || 'home'
      const queryParts: string[] = [`page_id=${encodeURIComponent(targetPageId)}`]

      if (payload.id) {
        queryParts.push(`id=${encodeURIComponent(String(resolveBindingValue(payload.id, context)))}`)
      }
      if (payload.query && typeof payload.query === 'object') {
        Object.entries(payload.query).forEach(([k, v]) => {
          if (v !== undefined && v !== null) {
            queryParts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(resolveBindingValue(v, context)))}`)
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

      const isWeapp = Taro.getEnv() === Taro.ENV_TYPE.WEAPP
      if (isWeapp) {
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
      } else {
        Taro.showToast({
          title: `[模拟跳转] AppID: ${targetAppId}`,
          icon: 'none'
        })
      }
      break
    }

    // 网页 WebView H5 直达打开
    case 'open_webview': {
      let targetUrl = payload.url || payload.web_url || ''
      if (!targetUrl) {
        Taro.showToast({ title: '链接地址为空', icon: 'none' })
        return
      }

      // 需要登录的 WebView 不直接暴露用户标识，先换取一次性短期票据地址。
      if (action.require_auth) {
        const ticket = await request<{ url: string }>({
          url: '/api/v1/webview/ticket',
          method: 'POST',
          data: { url: targetUrl }
        })
        if (!ticket?.url) {
          throw new Error('未能获取一次性 WebView 地址')
        }
        targetUrl = ticket.url
      }

      Taro.navigateTo({
        url: `/pages/webview/index?url=${encodeURIComponent(targetUrl)}&title=${encodeURIComponent(payload.title || '')}`
      })
      break
    }

    // 全屏大图预览
    case 'preview_image': {
      const current = payload.current || payload.url || ''
      const urls: string[] = Array.isArray(payload.urls) ? payload.urls : (current ? [current] : [])
      if (urls.length === 0) return

      Taro.previewImage({
        current,
        urls
      })
      break
    }

    // 异步受控业务数据请求与事务触发 (支持 path_params / query / body / response / timeout / on_error 完整规范)
    case 'request_data': {
      const endpoint = payload.endpoint || ''
      let targetUrl = payload.url || '/api/v1/action/execute'
      const method = (payload.method || 'POST').toUpperCase()
      const idempotencyKey = payload.idempotency_key || `idem_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
      const timeoutMs = Number(payload.timeout_ms || payload.timeout || 15000)
      const stateTarget = typeof payload.target === 'string' ? payload.target : ''
      if (stateTarget && context?.setBlockState) context.setBlockState(stateTarget, 'loading')

      // 1. 严格端点安全白名单门禁: endpoint 优先；若为自定义 url 必须为同源相对路径
      if (endpoint) {
        targetUrl = '/api/v1/action/execute'
      } else {
        const isRelative = targetUrl.startsWith('/') && !targetUrl.startsWith('//')
        if (!isRelative) {
          console.error(`[安全拦截] request_data 拒绝外部非同源地址: ${targetUrl}`)
          Taro.showToast({ title: '非法请求端点被拦截', icon: 'none' })
          return
        }
      }

      // 2. 递归求值解析 path_params、query 与 body
      const resolvedPathParams = resolveObjectBindings(payload.path_params || {}, context)
      const resolvedQuery = resolveObjectBindings(payload.query || {}, context)
      const resolvedBody = resolveObjectBindings(payload.body || payload.payload || {}, context)

      // 3. 处理 URL 路径参数替换 (如 /api/v1/actions/game/{game_id}/redeem)
      if (resolvedPathParams && typeof resolvedPathParams === 'object') {
        for (const [k, v] of Object.entries(resolvedPathParams)) {
          targetUrl = targetUrl.replace(new RegExp(`\\{${k}\\}`, 'g'), encodeURIComponent(String(v)))
        }
      }

      // 4. 拼接 Query 参数
      if (resolvedQuery && Object.keys(resolvedQuery).length > 0) {
        const queryParts = Object.entries(resolvedQuery).map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
        targetUrl += (targetUrl.includes('?') ? '&' : '?') + queryParts.join('&')
      }

      // 5. 敏感或需鉴权端点前置登录门禁拦截
      if (endpoint === 'game.redeem' || action.require_auth) {
        const session = await ensureSession()
        if (!session) {
          Taro.showToast({ title: '请先完成微信授权登录', icon: 'none' })
          return
        }
      }

      Taro.showLoading({ title: '正在处理...', mask: true })
      try {
        let requestData: any = resolvedBody
        if (targetUrl.startsWith('/api/v1/action/execute')) {
          requestData = {
            endpoint: endpoint || 'game.redeem',
            payload: resolvedBody,
            idempotency_key: idempotencyKey
          }
        }

        const res = await request<any>({
          url: targetUrl,
          method: method as any,
          data: requestData,
          timeout: timeoutMs
        })
        Taro.hideLoading()
        if (stateTarget && context?.setBlockState) context.setBlockState(stateTarget, 'normal')

        // 契约对齐: request 层解包返回 data，直接消费 res；若为原始信封则消费 res.data
        const resultData = (res && typeof res === 'object' && res.data !== undefined) ? res.data : res

        // 6. response 状态持久化映射 (save_as 与 data_path) 并触发 React 响应式渲染
        let extractedData = resultData
        if (payload.response) {
          if (payload.response.data_path) {
            extractedData = getByPath(resultData, payload.response.data_path) ?? resultData
          }
          if (payload.response.save_as) {
            const saveKey = payload.response.save_as
            if (context) {
              if (!context.state) context.state = {}
              context.state[saveKey] = extractedData
              if (context.updateState) {
                context.updateState(saveKey, extractedData)
              }
            }
          }
        }

        // 7. 执行级联成功动作链 (on_success)
        const nextContext: ActionContext = {
          ...context,
          result: extractedData
        }

        if (Array.isArray(payload.on_success) && payload.on_success.length > 0) {
          for (const subAction of payload.on_success) {
            await dispatchAction(subAction, nextContext)
          }
        } else {
          // 默认成功反馈
          if (extractedData && extractedData.code) {
            Taro.setClipboardData({
              data: String(extractedData.code),
              success: () => {
                Taro.vibrateShort({ type: 'medium' })
                Taro.showToast({
                  title: `✅ 兑换码 ${extractedData.code} 已复制！`,
                  icon: 'none',
                  duration: 3000
                })
              }
            })
          } else {
            Taro.showToast({
              title: extractedData?.msg || '操作成功',
              icon: 'success'
            })
          }
        }
      } catch (err: any) {
        Taro.hideLoading()
        if (stateTarget && context?.setBlockState) context.setBlockState(stateTarget, 'error')
        // 8. 异常动作链调度 (on_error)
        const errorContext: ActionContext = {
          ...context,
          error: err
        } as any
        if (Array.isArray(payload.on_error) && payload.on_error.length > 0) {
          for (const errAction of payload.on_error) {
            await dispatchAction(errAction, errorContext)
          }
        } else {
          Taro.showToast({
            title: err.message || '操作未完成，请重试',
            icon: 'none'
          })
        }
      }
      break
    }

    // 创建后台商品订单并调起微信小程序支付。
    case 'request_payment': {
      if (!(await ensureSession())) {
        Taro.showToast({ title: '请先完成微信授权登录', icon: 'none' })
        return
      }
      const sku = String(payload.sku || payload.product_sku || '')
      if (!sku) {
        Taro.showToast({ title: '商品 SKU 不能为空', icon: 'none' })
        return
      }
      const idem = String(payload.idempotency_key || `pay_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`)
      const result = await request<any>({ url: '/api/v1/payment/orders', method: 'POST', data: { sku, idempotency_key: idem } })
      const payment = result?.payment || result
      await Taro.requestPayment({
        timeStamp: String(payment.timeStamp),
        nonceStr: String(payment.nonceStr),
        package: String(payment.package),
        signType: payment.signType || 'RSA',
        paySign: String(payment.paySign)
      } as any)
      const outTradeNo = String(result?.order?.out_trade_no || '')
      let orderStatus = 'pending'
      if (outTradeNo) {
        // 支付回调是异步的，短暂轮询后端状态，避免只相信客户端回调。
        for (let attempt = 0; attempt < 4; attempt += 1) {
          if (attempt > 0) await new Promise(resolve => setTimeout(resolve, 500))
          try {
            const order = await request<any>({ url: `/api/v1/payment/orders/${encodeURIComponent(outTradeNo)}` })
            orderStatus = String(order?.status || 'pending')
            if (orderStatus === 'paid') break
          } catch (_) {
            // 支付成功后查询失败不应覆盖微信支付结果，下一次尝试继续确认。
          }
        }
      }
      Taro.showToast({ title: orderStatus === 'paid' ? '支付成功' : '支付已提交', icon: orderStatus === 'paid' ? 'success' : 'none' })
      break
    }

    // 触发微信官方原生分享菜单
    case 'share': {
      if (Taro.getEnv() === Taro.ENV_TYPE.WEAPP) {
        try {
          Taro.showShareMenu({
            showShareItems: ['wechatFriends', 'wechatMoment']
          })
          Taro.showToast({
            title: payload.toast || '点击右上角【···】即可快速分享',
            icon: 'none',
            duration: 2500
          })
        } catch (e) {
          Taro.showToast({ title: payload.toast || '请点击右上角分享', icon: 'none' })
        }
      } else {
        Taro.showToast({
          title: `[模拟分享] ${payload.title || '精彩内容分享'}`,
          icon: 'none'
        })
      }
      break
    }

    // 微信小程序消息订阅授权
    case 'subscribe_message': {
      const tmplIds: string[] = Array.isArray(payload.tmpl_ids)
        ? payload.tmpl_ids
        : Array.isArray(payload.template_ids)
          ? payload.template_ids
          : typeof payload.template_id === 'string'
            ? [payload.template_id]
            : typeof payload.tmpl_id === 'string'
              ? [payload.tmpl_id]
              : typeof payload.tmplId === 'string'
                ? [payload.tmplId]
                : []

      if (tmplIds.length === 0) {
        console.warn('subscribe_message 缺少模板 ID (template_id / tmpl_ids)')
        break
      }

      if (Taro.getEnv() === Taro.ENV_TYPE.WEAPP) {
        try {
          const res = await Taro.requestSubscribeMessage({
            tmplIds: tmplIds
          } as any)
          actionResult = res
          const subContext: ActionContext = {
            ...context,
            result: res
          }
          if (Array.isArray(payload.on_success) && payload.on_success.length > 0) {
            for (const nextAction of payload.on_success) {
              await dispatchAction(nextAction, subContext)
            }
          } else {
            Taro.showToast({
              title: payload.toast || '订阅完成',
              icon: 'success'
            })
          }
        } catch (err: any) {
          const errContext: ActionContext = {
            ...context,
            error: err
          } as any
          if (Array.isArray(payload.on_error) && payload.on_error.length > 0) {
            for (const failAction of payload.on_error) {
              await dispatchAction(failAction, errContext)
            }
          } else {
            console.error('订阅消息失败:', err)
            Taro.showToast({
              title: '已取消订阅或订阅未成功',
              icon: 'none'
            })
          }
        }
      } else {
        Taro.showToast({
          title: `[模拟订阅] 模板: ${tmplIds.join(',')}`,
          icon: 'none'
        })
      }
      break
    }

    default:
      console.warn(`未识别或暂不支持的 SDUI 动作类型: ${action.type}`)
      break
    }
  } catch (err: any) {
    actionSuccess = false
    console.error(`动作 ${action.type} 执行异常:`, err)
    if (Array.isArray(action.on_error) && action.on_error.length > 0) {
      for (const errAct of action.on_error) {
        await dispatchAction(errAct, { ...context, result: err })
      }
    } else {
      Taro.showToast({ title: err?.message || '操作执行失败', icon: 'none' })
    }
  }

  // 6. 执行通用 on_success 动作链
  if (actionSuccess && Array.isArray(action.on_success) && action.on_success.length > 0) {
    const successContext: ActionContext = {
      ...context,
      result: actionResult !== undefined ? actionResult : context?.result
    }
    for (const succAct of action.on_success) {
      await dispatchAction(succAct, successContext)
    }
  }
}
