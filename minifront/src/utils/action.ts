// minifront/src/utils/action.ts
import Taro from '@tarojs/taro'
import { BlockAction } from '../types/sdui'
import { ensureSession } from './auth'
import { request } from './request'
import { evaluateCondition, isKnownScopedPath } from './condition'
import { resolveRemoteUrl } from '../config/env'

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
  props?: Record<string, any>
  page?: any
  session?: any
  tenant?: any
}

/**
 * 安全深度读取对象属性
 */
function getByPath(obj: any, path: string): any {
  if (obj === undefined || obj === null) return undefined
  if (!path) return obj
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
 * 支持解析 $entity.*, entity.*, $query.*, $item.*, $state.*, $result.*
 */
export function resolveBindingValue(val: any, context?: ActionContext): any {
  if (val == null) return val

  // 1. 处理显式对象路径形式: { path: "$entity.title" } 或 { path: "entity.title" }
  if (typeof val === 'object' && val.path && typeof val.path === 'string') {
    return resolvePathString(val.path, context)
  }

  // 2. 处理直接字符串路径形式: "$entity.title", "entity.title", "{{entity.title}}" 或内嵌插值文本 "前缀 {{path}} 后缀"
  if (typeof val === 'string') {
    if (val.startsWith('$')) {
      return resolvePathString(val, context)
    }
    if (val.startsWith('{{') && val.endsWith('}}') && !val.slice(2, -2).includes('{{')) {
      const path = val.slice(2, -2).trim()
      return resolvePathString(path.startsWith('$') ? path : `$${path}`, context)
    }
    if (isKnownScopedPath(val)) {
      return resolvePathString(val, context)
    }
    if (val.includes('{{') && val.includes('}}')) {
      return val.replace(/\{\{\s*(\$?[a-zA-Z0-9_.]+)\s*\}\}/g, (_match, path) => {
        const resolved = resolvePathString(path.startsWith('$') ? path : `$${path}`, context)
        return resolved !== undefined && resolved !== null ? String(resolved) : _match
      })
    }
  }

  return val
}

function resolvePathString(path: string, context?: ActionContext): any {
  if (!path) return undefined
  const normalizedPath = path.startsWith('$') ? path : `$${path}`
  const segments = normalizedPath.split('.')
  const root = segments[0]
  const rawRoot = root.startsWith('$') ? root.slice(1) : root
  const ctx = context as Record<string, any> | undefined
  if (!ctx) return undefined

  // 双向兼容：支持 context 中无论以 $xxx 还是以无 $ 存储均能精准读取
  const scopeVal = ctx[root] ?? ctx[rawRoot] ?? ctx[`$${rawRoot}`]
  if (scopeVal === undefined || scopeVal === null) return undefined
  return getByPath(scopeVal, normalizedPath.slice(root.length + 1))
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

/** 解析当前动作载荷，保留级联动作到实际执行时再按最新结果求值。 */
function resolveActionPayload(payload: Record<string, any>, context?: ActionContext): Record<string, any> {
  const resolved: Record<string, any> = {}
  Object.keys(payload || {}).forEach((key) => {
    resolved[key] = key === 'on_success' || key === 'on_error' || key === 'endpoint'
      ? payload[key]
      : resolveObjectBindings(payload[key], context)
  })
  return resolved
}

/**
 * 解析 block 属性但保留嵌套子 block，避免父容器提前消费子项的 $item/$state 绑定。
 */
export function resolveBlockPropsBindings(props: Record<string, any>, context: ActionContext | undefined, blockType: string): Record<string, any> {
  const tabField = Array.isArray(props.tabs) ? 'tabs' : 'items'
  const resolve = (value: any, preserveIdentity = false): any => {
    if (value == null) return value
    if (Array.isArray(value)) return value.map(item => resolve(item, preserveIdentity))
    if (typeof value === 'object') {
      if (typeof value.type === 'string') return value
      if (Object.keys(value).length === 1 && typeof value.path === 'string' && isKnownScopedPath(value.path)) {
        return resolveBindingValue(value.path, context)
      }
      const result: Record<string, any> = {}
      Object.keys(value).forEach((key) => {
        // 只有 tabs 块直接选项列表的 key/id 属于结构字段。
        result[key] = preserveIdentity && (key === 'key' || key === 'id')
          ? value[key]
          : resolve(value[key], value === props && blockType === 'tabs' && key === tabField)
      })
      return result
    }
    return resolveObjectBindings(value, context)
  }
  return resolve(props) || {}
}

/**
 * 批量分发积木事件动作列表 (支持 events.tap 动作序列按序执行、前置失败安全阻断与上下文流转管道)
 */
export async function dispatchEvents(events?: Record<string, BlockAction[] | BlockAction>, eventName = 'tap', context?: ActionContext): Promise<any> {
  if (!events) return undefined
  const target = events[eventName]
  if (!target) return undefined

  let currentContext: ActionContext = { ...context }
  let lastResult: any = currentContext.result

  if (Array.isArray(target)) {
    for (const action of target) {
      const actResult = await dispatchAction(action, currentContext)
      // 若关键动作执行失败或用户取消，阻断后续强依赖的动作序列
      if (actResult === false) {
        return false
      }
      if (actResult !== undefined) {
        lastResult = actResult
        currentContext = {
          ...currentContext,
          result: actResult
        }
      }
    }
  } else {
    lastResult = await dispatchAction(target, currentContext)
  }
  return lastResult
}

/**
 * 万能原子交互动作分发器 (Action Dispatcher)
 * 支持登录拦截、跨小程序矩阵互跳、微信视频号原生拉起、剪贴板震动与多页面路由流转
 */
export async function dispatchAction(action?: BlockAction | BlockAction[], context?: ActionContext): Promise<any> {
  if (!action) return undefined
  if (Array.isArray(action)) {
    return dispatchEvents({ tap: action }, 'tap', context)
  }
  if (!action.type) return undefined

  // 1. 动作执行前置受控条件求值 (若配置了 condition 且条件不满足则中断)
  if (action.condition) {
    const isMet = evaluateCondition(action.condition, context || {})
    if (!isMet) {
      return undefined
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
      return false
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
      return false
    }
  }

  // 4. 动作数据埋点上报 (track)
  if (action.track) {
    console.log('[SDUI Action Track]', action.track.event_name || action.track.event_id, action.track.params)
  }

  const payload = resolveActionPayload(action.payload || {}, context)
  let actionSuccess = true
  let successChainHandled = false
  let actionResult: any = undefined

  try {
    // 5. 根据标准动作类型执行对应业务逻辑
    switch (action.type) {
      // 强制触发登录授权
      case 'require_auth': {
        if (!(await ensureSession())) throw new Error('微信登录授权未完成')
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
        actionSuccess = false
        break
      }

      actionResult = textToCopy
      await Taro.setClipboardData({ data: String(textToCopy) })
      Taro.vibrateShort({ type: 'medium' })
      const toastText = payload.toast || `已成功复制: ${textToCopy}`
      Taro.showToast({ title: toastText, icon: 'none', duration: 2500 })
      break
    }

    // 页面内轻提示 (Toast)
    case 'toast': {
      const msg = resolveBindingValue(payload.text || payload.message || '操作已执行', context)
      actionResult = msg
      Taro.showToast({
        title: String(msg),
        icon: payload.icon || 'none',
        duration: payload.duration || 2000
      })
      break
    }

    // 刷新当前页面协议与数据 (支持文档 3.8 节按 target 局部重置积木状态，无 target 时触发全页刷新)
    case 'refresh': {
      const target = payload.target
      if (target && context?.setBlockState) {
        context.setBlockState(target, 'normal')
        actionResult = { target, state: 'normal' }
      } else if (context?.refresh) {
        context.refresh()
      }
      break
    }

    // 页面响应式状态设置，原地同步消除时序竞争
    case 'set_state': {
      const stateKey = payload.key || payload.name || payload.target
      const stateVal = resolveBindingValue(payload.value !== undefined ? payload.value : payload.val, context)
      if (stateKey) {
        if (context) {
          context.state = { ...(context.state || {}), [stateKey]: stateVal }
        }
        if (context?.updateState) {
          context.updateState(stateKey, stateVal)
        }
        actionResult = { key: stateKey, value: stateVal }
      }
      break
    }

    // 页面响应式状态反转/折叠切换
    case 'toggle_state': {
      const stateKey = payload.key || payload.name || payload.target
      if (stateKey) {
        const curr = context?.state?.[stateKey]
        const toggled = !curr
        if (context) {
          context.state = { ...(context.state || {}), [stateKey]: toggled }
        }
        if (context?.updateState) {
          context.updateState(stateKey, toggled)
        }
        actionResult = { key: stateKey, value: toggled }
      }
      break
    }

    // 页面响应式状态重置
    case 'reset_state': {
      const stateKey = payload.key || payload.name || payload.target
      const resetKeys = stateKey ? [stateKey] : Object.keys(context?.state || {})
      if (context) {
        if (stateKey) {
          context.state = { ...(context.state || {}), [stateKey]: undefined }
        } else {
          context.state = {}
        }
      }
      if (context?.updateState) {
        resetKeys.forEach((key) => context.updateState!(key, undefined))
      }
      actionResult = { reset: true }
      break
    }

    // 块级局部状态切换 (支持文档 3.8 节 show_error_state / show_empty_state / show_loading_state / reset_block_state)
    case 'show_error_state': {
      const target = payload.target
      if (target && context?.setBlockState) {
        context.setBlockState(target, 'error')
        actionResult = { target, state: 'error' }
      }
      break
    }
    case 'show_empty_state': {
      const target = payload.target
      if (target && context?.setBlockState) {
        context.setBlockState(target, 'empty')
        actionResult = { target, state: 'empty' }
      }
      break
    }
    case 'show_loading_state': {
      const target = payload.target
      if (target && context?.setBlockState) {
        context.setBlockState(target, 'loading')
        actionResult = { target, state: 'loading' }
      }
      break
    }
    case 'reset_block_state': {
      const target = payload.target
      if (target && context?.setBlockState) {
        context.setBlockState(target, 'normal')
        actionResult = { target, state: 'normal' }
      }
      break
    }

    // 微信小程序多页面路由流转 (万能动态承载页)
    case 'navigate_page': {
      const openType = payload.open_type || payload.type
      if (openType === 'back' || payload.delta) {
        Taro.navigateBack({ delta: Number(payload.delta) || 1 })
        break
      }

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

      const targetUrl = targetPageId === 'home' && queryParts.length === 1
        ? '/pages/index/index'
        : `/pages/dynamic/index?${queryParts.join('&')}`

      if (openType === 'reLaunch') {
        Taro.reLaunch({ url: targetUrl })
      } else if (openType === 'redirect') {
        Taro.redirectTo({ url: targetUrl })
      } else {
        Taro.navigateTo({
          url: targetUrl,
          fail: (err) => {
            console.warn('动态页面 navigateTo 失败，尝试以 redirectTo 打开:', err)
            Taro.redirectTo({
              url: targetUrl,
              fail: (reErr) => {
                console.error('动态页面 redirectTo 亦失败:', reErr)
              }
            })
          }
        })
      }
      break
    }

    // 微信视频号动态原生拉起 (调起微信原生剧场)
    case 'open_channels_activity': {
      const feedId = String(resolveBindingValue(payload.feed_id || payload.feedId, context) || '')
      const finderUserName = String(resolveBindingValue(payload.finder_user_name || payload.finderUserName, context) || '')

      if (!feedId || !finderUserName) {
        Taro.showToast({ title: '视频号参数未配置完整', icon: 'none' })
        actionSuccess = false
        break
      }

      const isWeapp = Taro.getEnv() === Taro.ENV_TYPE.WEAPP
      if (isWeapp && typeof (wx as any) !== 'undefined' && typeof (wx as any).openChannelsActivity === 'function') {
        await new Promise<void>((resolve, reject) => {
          ;(wx as any).openChannelsActivity({
            feedId, finderUserName,
            success: () => resolve(),
            fail: (err: any) => reject(new Error(err?.errMsg || '拉起视频号失败'))
          })
        })
      } else {
        throw new Error('当前环境不支持视频号动态')
      }
      break
    }

    // 跨小程序矩阵跳转 (流量互导与分流承接)
    case 'open_mini_program': {
      const targetAppId = String(resolveBindingValue(payload.target_app_id || payload.app_id || payload.appId, context) || '')
      if (!targetAppId) {
        Taro.showToast({ title: '目标小程序 AppID 未指定', icon: 'none' })
        actionSuccess = false
        break
      }

      const isWeapp = Taro.getEnv() === Taro.ENV_TYPE.WEAPP
      if (isWeapp) {
        await Taro.navigateToMiniProgram({
          appId: targetAppId,
          path: payload.target_path || payload.path || '',
          extraData: payload.extra_data || {},
          envVersion: payload.env_version || 'release'
        })
      } else {
        throw new Error('当前环境不支持跨小程序跳转')
      }
      break
    }

    // 网页 WebView H5 直达打开
    case 'open_webview': {
      let targetUrl = payload.url || payload.web_url || ''
      if (!targetUrl) {
        Taro.showToast({ title: '链接地址为空', icon: 'none' })
        actionSuccess = false
        break
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
      const current = resolveRemoteUrl(payload.current || payload.url)
      const urls: string[] = (Array.isArray(payload.urls) ? payload.urls : (current ? [current] : [])).map(resolveRemoteUrl)
      if (urls.length === 0) {
        actionSuccess = false
        break
      }

      Taro.previewImage({
        current,
        urls
      })
      break
    }

    // 异步受控业务数据请求与事务触发 (支持 path_params / query / body / response / timeout / on_error 完整规范)
    case 'request':
    case 'request_data': {
      const endpoint = payload.endpoint || action.endpoint || (action as any).endpoint || ''
      let targetUrl = payload.url || action.url || (action as any).url || '/api/v1/action/execute'
      const method = endpoint ? 'POST' : (payload.method || 'POST').toUpperCase()
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
          if (stateTarget && context?.setBlockState) context.setBlockState(stateTarget, 'error')
          actionSuccess = false
          break
        }
      }

      // 2. 递归求值解析 path_params、query 与 body
      const resolvedPathParams = resolveObjectBindings(payload.path_params || {}, context)
      const resolvedQuery = resolveObjectBindings(payload.query || {}, context)

      // 智能提取请求体: 优先读取 payload.body 或 payload.payload; 若未显式包裹，则提取 payload 顶层业务参数
      let rawBody = payload.body ?? payload.payload
      if (!rawBody || (typeof rawBody === 'object' && Object.keys(rawBody).length === 0)) {
        const reservedKeys = new Set([
          'endpoint', 'url', 'method', 'idempotency_key', 'timeout_ms', 'timeout',
          'target', 'response', 'on_success', 'on_error', 'path_params', 'query',
          'body', 'payload', 'require_auth'
        ])
        const extracted: Record<string, any> = {}
        for (const [k, v] of Object.entries(payload)) {
          if (!reservedKeys.has(k)) {
            extracted[k] = v
          }
        }
        if (Object.keys(extracted).length > 0) {
          rawBody = extracted
        }
      }

      let resolvedBody = resolveObjectBindings(rawBody || {}, context)
      if (typeof resolvedBody !== 'object' || resolvedBody === null) {
        resolvedBody = {}
      }

      // 智能业务端点上下文兜底注入 (覆盖表单查询、礼包套餐等关键链路)
      if (endpoint === 'query.score' && !(resolvedBody as any).query_value && !(resolvedBody as any).code) {
        const fallbackVal = (context as any)?.query_value ?? (context?.item as any)?.query_value ?? (context?.state as any)?.query_value ?? (context?.state as any)?.form?.query_value
        if (fallbackVal) {
          resolvedBody = { ...(resolvedBody as any), query_value: fallbackVal }
        }
      }
      if (endpoint === 'game.redeem' && !(resolvedBody as any).package_id) {
        const fallbackPkg = (context as any)?.package_id ?? (context?.item as any)?.package_id
        if (fallbackPkg) {
          resolvedBody = { ...(resolvedBody as any), package_id: fallbackPkg }
        }
      }

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
          if (stateTarget && context?.setBlockState) context.setBlockState(stateTarget, 'error')
          actionSuccess = false
          break
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

        // 契约对齐: request 层已解包返回 data，直接消费 res；若为未解包原始业务信封 ({ code: 0, data: ... }) 则消费 res.data
        const resultData = (res && typeof res === 'object' && typeof res.code === 'number' && res.data !== undefined) ? res.data : res

        // 6. response 状态持久化映射 (save_as 与 data_path) 并触发 React 响应式渲染
        let extractedData = resultData
        if (payload.response) {
          if (payload.response.data_path) {
            extractedData = getByPath(resultData, payload.response.data_path) ?? resultData
          }
          if (payload.response.save_as) {
            const saveKey = payload.response.save_as
            if (context) {
              context.state = { ...(context.state || {}), [saveKey]: extractedData }
              if (context.updateState) {
                context.updateState(saveKey, extractedData)
              }
            }
          }
        }
        actionResult = extractedData

        // 7. 执行级联成功动作链 (on_success)
        const nextContext: ActionContext = {
          ...context,
          result: extractedData
        }

        const successActions = Array.isArray(action.on_success) && action.on_success.length > 0
          ? action.on_success
          : payload.on_success
        if (Array.isArray(successActions) && successActions.length > 0) {
          successChainHandled = true
          if ((await dispatchEvents({ tap: successActions }, 'tap', nextContext)) === false) actionSuccess = false
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
        actionSuccess = false
        Taro.hideLoading()
        if (stateTarget && context?.setBlockState) context.setBlockState(stateTarget, 'error')
        // 8. 异常动作链调度 (on_error)
        const errorContext: ActionContext = {
          ...context,
          error: err
        } as any
        const errorActions = Array.isArray(action.on_error) && action.on_error.length > 0 ? action.on_error : payload.on_error
        if (Array.isArray(errorActions) && errorActions.length > 0) {
          await dispatchEvents({ tap: errorActions }, 'tap', errorContext)
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
        actionSuccess = false
        break
      }
      let sku = String(resolveBindingValue(payload.sku || payload.product_sku, context) || '')
      if (!sku) {
        // 智能从上下文 item 或 actionPayload 中兜底提取 sku
        const fallbackSku = (context as any)?.sku ?? (context?.item as any)?.sku ?? (context?.item as any)?.product_sku ?? (context?.item as any)?.id ?? (context as any)?.actionPayload?.sku ?? (context as any)?.actionPayload?.id
        if (fallbackSku) {
          sku = String(fallbackSku)
        }
      }
      if (!sku) {
        Taro.showToast({ title: '商品 SKU 不能为空', icon: 'none' })
        actionSuccess = false
        break
      }
      const idem = String(payload.idempotency_key || `pay_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`)
      const result = await request<any>({ url: '/api/v1/payment/orders', method: 'POST', data: { sku, idempotency_key: idem } })
      actionResult = result
      const payment = result?.payment || result
      if (!payment || !['timeStamp', 'nonceStr', 'package', 'paySign'].every(key =>
        (typeof payment[key] === 'string' || (key === 'timeStamp' && typeof payment[key] === 'number')) && String(payment[key]).trim() !== ''
      )) {
        throw new Error('服务端返回的支付参数不完整')
      }
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
        actionSuccess = false
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
          const successActions = Array.isArray(action.on_success) && action.on_success.length > 0
            ? action.on_success
            : payload.on_success
          if (Array.isArray(successActions) && successActions.length > 0) {
            successChainHandled = true
            if ((await dispatchEvents({ tap: successActions }, 'tap', subContext)) === false) actionSuccess = false
          } else {
            Taro.showToast({
              title: payload.toast || '订阅完成',
              icon: 'success'
            })
          }
        } catch (err: any) {
          actionSuccess = false
          const errContext: ActionContext = {
            ...context,
            error: err
          } as any
          const errorActions = Array.isArray(action.on_error) && action.on_error.length > 0 ? action.on_error : payload.on_error
          if (Array.isArray(errorActions) && errorActions.length > 0) {
            await dispatchEvents({ tap: errorActions }, 'tap', errContext)
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
      actionSuccess = false
      console.warn(`未识别或暂不支持的 SDUI 动作类型: ${action.type}`)
      break
    }
  } catch (err: any) {
    actionSuccess = false
    console.error(`动作 ${action.type} 执行异常:`, err)
    if (Array.isArray(action.on_error) && action.on_error.length > 0) {
      await dispatchEvents({ tap: action.on_error }, 'tap', { ...context, result: err })
    } else {
      Taro.showToast({ title: err?.message || '操作执行失败', icon: 'none' })
    }
  }

  // 6. 执行通用 on_success 动作链 (兼容 action.on_success 与 payload.on_success)
  const successActions = (Array.isArray(action.on_success) && action.on_success.length > 0)
    ? action.on_success
    : (Array.isArray(payload?.on_success) && payload.on_success.length > 0 && action.type !== 'request_data' && action.type !== 'request' && action.type !== 'subscribe_message'
      ? payload.on_success
      : null)

  if (actionSuccess && !successChainHandled && successActions && successActions.length > 0) {
    const successContext: ActionContext = {
      ...context,
      result: actionResult !== undefined ? actionResult : context?.result
    }
    if ((await dispatchEvents({ tap: successActions }, 'tap', successContext)) === false) return false
  }

  return actionSuccess ? (actionResult !== undefined ? actionResult : context?.result) : false
}
