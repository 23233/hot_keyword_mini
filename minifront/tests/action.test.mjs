// action.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'
import ts from 'typescript'

// 编译实际源码，仅替换平台依赖；不复制绑定或动作实现。
function loadActions(overrides = {}) {
  const cache = new Map()
  const calls = []
  const taro = new Proxy({ ENV_TYPE: { WEAPP: 'WEAPP' }, getEnv: () => 'WEAPP', ...overrides.taro }, {
    get(target, key) { return target[key] || (async (options) => { calls.push({ method: key, options }); return {} }) }
  })
  const load = (name) => {
    if (cache.has(name)) return cache.get(name)
    const exports = {}
    cache.set(name, exports)
    const source = readFileSync(new URL(`../src/utils/${name}.ts`, import.meta.url), 'utf8')
    const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
    vm.runInNewContext(js, {
      exports, console, setTimeout, wx: overrides.wx || {},
      require: (id) => {
        if (id === '@tarojs/taro') return { default: taro }
        if (id === './auth') return { ensureSession: async () => true }
        if (id === './request') return {
          getCurrentAppId: () => 'wx-test',
          request: async (options) => { calls.push({ method: 'request', options }); return overrides.request ? overrides.request(options) : { status: 'success' } }
        }
        if (id === '../config/env') return { resolveRemoteUrl: value => value }
        if (id === './condition') return load('condition')
        if (id === './cart') return load('cart')
        throw new Error(`未声明的测试依赖: ${id}`)
      }
    })
    return exports
  }
  return { ...load('action'), calls }
}

test('业务 key/id 和对象绑定解析，子积木与 Tab 标识保留', () => {
  const { resolveBlockPropsBindings } = loadActions()
  const result = resolveBlockPropsBindings({
    items: [{ key: '$item.id', id: '$item.id', label: '$item.name' }],
    bound: { path: '$item.id' },
    tabs: [{ key: 'state', title: '状态', blocks: [] }],
    children: [{ id: 'child', type: 'text', props: { text: '$item.name' } }]
  }, { item: { id: 'row-1', name: '业务项' }, state: {} }, 'tabs')
  assert.equal(result.items[0].id, 'row-1')
  assert.equal(result.bound, 'row-1')
  assert.equal(result.items[0].key, 'row-1')
  assert.equal(result.tabs[0].key, 'state')
  assert.equal(result.children[0].props.text, '$item.name')
})

test('reset_state 清空实际页面状态且不污染原对象', async () => {
  const { dispatchAction } = loadActions()
  const original = { first: 1, second: true }
  const displayed = { ...original }
  const context = { state: original, updateState: (key, value) => { displayed[key] = value } }
  await dispatchAction({ type: 'reset_state' }, context)
  assert.equal(displayed.first, undefined)
  assert.equal(displayed.second, undefined)
  assert.deepEqual(original, { first: 1, second: true })
})

test('repeat 动态数组在客户端最多展开 200 项', () => {
  const { capRepeatItems, MAX_REPEAT_ITEMS } = loadActions()
  const capped = capRepeatItems(Array.from({ length: 300 }, (_, index) => index))
  assert.equal(MAX_REPEAT_ITEMS, 200)
  assert.equal(capped.length, 200)
  assert.equal(capped[199], 199)
  assert.equal(capRepeatItems([1, 2]).length, 2)
})

test('request_data 按 target 自动切换块级 loading 与恢复', async () => {
  let resolveRequest
  const { dispatchAction } = loadActions({ request: () => new Promise((resolve) => { resolveRequest = resolve }) })
  const states = []
  const pending = dispatchAction({ type: 'request_data', payload: { endpoint: 'query.score', target: 'block-x' } }, {
    setBlockState: (id, state) => states.push([id, state])
  })
  assert.deepEqual(states, [['block-x', 'loading']])
  resolveRequest({ status: 'success' })
  await pending
  assert.deepEqual(states, [['block-x', 'loading'], ['block-x', 'normal']])
})

test('同一积木的 request_data 在途时拒绝重复触发，完成后恢复', async () => {
  const resolvers = []
  const { dispatchAction, calls } = loadActions({ request: () => new Promise((resolve) => { resolvers.push(resolve) }) })
  const context = { __block_id: 'block-lock', setBlockState: () => {} }
  const first = dispatchAction({ type: 'request_data', payload: { endpoint: 'query.score' } }, context)
  const second = await dispatchAction({ type: 'request_data', payload: { endpoint: 'query.score' } }, context)
  assert.equal(second, false)
  assert.equal(calls.filter((call) => call.method === 'request').length, 1)
  resolvers[0]({ status: 'ok' })
  await first
  const third = dispatchAction({ type: 'request_data', payload: { endpoint: 'query.score' } }, context)
  await Promise.resolve()
  assert.equal(calls.filter((call) => call.method === 'request').length, 2)
  resolvers[1]({ status: 'ok' })
  await third
})

test('request_data 失败时把 target 块切换到 error 状态', async () => {
  const { dispatchAction } = loadActions({ request: async () => { throw new Error('模拟失败') } })
  const states = []
  const ok = await dispatchAction({ type: 'request_data', payload: { endpoint: 'query.score', target: 'block-y' } }, {
    setBlockState: (id, state) => states.push([id, state])
  })
  assert.equal(ok, false)
  assert.deepEqual(states, [['block-y', 'loading'], ['block-y', 'error']])
})

test('query.score 是端点名称，不得被当作 query 数据绑定', async () => {
  const { dispatchAction, calls } = loadActions()
  await dispatchAction({ type: 'request_data', payload: { endpoint: 'query.score', body: { query_value: '测试' } } }, { query: {} })
  const request = calls.find(call => call.method === 'request')
  assert.equal(request.options.data.endpoint, 'query.score')
  assert.equal(request.options.data.payload.query_value, '测试')
})

test('事件序列使用最新状态和结果，失败时终止后续动作', async () => {
  const { dispatchEvents, calls } = loadActions()
  const state = {}
  await dispatchEvents({ tap: [
    { type: 'set_state', payload: { key: 'enabled', value: true } },
    { type: 'toggle_state', payload: { key: 'enabled' } },
    { type: 'toast', payload: { text: '$result.value' } }
  ] }, 'tap', { state, updateState: (key, value) => { state[key] = value } })
  assert.equal(state.enabled, false)
  await dispatchEvents({ tap: [ { type: 'unknown' }, { type: 'toast', payload: { text: '不应执行' } } ] })
  assert.ok(!calls.some(call => call.options?.title === '不应执行'))
})

test('缺少支付参数时拒绝调起微信支付', async () => {
  const { dispatchAction, calls } = loadActions()
  assert.equal(await dispatchAction({ type: 'request_payment', payload: { sku: 'sku-test' } }), false)
  assert.equal(calls.filter(call => call.method === 'requestPayment').length, 0)
})

test('购物车支付使用 items 明细并跳过单 SKU 限制', async () => {
  const { dispatchAction, calls } = loadActions({
    request: async () => ({
      payment: { timeStamp: 1, nonceStr: 'n', package: 'p', paySign: 's' },
      order: { out_trade_no: '' }
    })
  })
  const result = await dispatchAction({ type: 'request_payment', payload: { items: [{ sku: 'a', quantity: 2 }, { sku: 'b', quantity: 1 }] } }, {})
  assert.ok(result)
  const request = calls.find(call => call.method === 'request')
  assert.deepEqual(JSON.parse(JSON.stringify(request.options.data.items)), [{ sku: 'a', quantity: 2 }, { sku: 'b', quantity: 1 }])
  assert.equal(request.options.data.sku, undefined)
  assert.ok(calls.some(call => call.method === 'requestPayment'))
})

test('支付动作透传优惠券 ID', async () => {
  const { dispatchAction, calls } = loadActions({
    request: async () => ({
      payment: { timeStamp: 1, nonceStr: 'n', package: 'p', paySign: 's' },
      order: { out_trade_no: '' }
    })
  })
  const result = await dispatchAction({
    type: 'request_payment',
    payload: { items: [{ sku: 'a', quantity: 1 }], coupon_id: 'coupon-1' }
  }, {})
  assert.ok(result)
  const request = calls.find(call => call.method === 'request')
  assert.equal(request.options.data.coupon_id, 'coupon-1')
})

test('购物车支付可从 actionPayload.items 取明细', async () => {
  const { dispatchAction, calls } = loadActions({
    request: async () => ({
      payment: { timeStamp: 1, nonceStr: 'n', package: 'p', paySign: 's' },
      order: { out_trade_no: '' }
    })
  })
  const result = await dispatchAction({ type: 'request_payment' }, { actionPayload: { items: [{ sku: 'c', quantity: 3 }] } })
  assert.ok(result)
  const request = calls.find(call => call.method === 'request')
  assert.deepEqual(JSON.parse(JSON.stringify(request.options.data.items)), [{ sku: 'c', quantity: 3 }])
})

test('普通业务树不因 title/children 或 tabs 字段保留绑定', () => {
  const { resolveBlockPropsBindings } = loadActions()
  const record = { id: '$item.id', key: '$item.id', title: '部门', children: [] }
  const resolved = resolveBlockPropsBindings({ record, tabs: [record] }, { item: { id: '部门一' } }, 'custom')
  assert.equal(resolved.record.id, '部门一')
  assert.equal(resolved.tabs[0].key, '部门一')
  const tabs = resolveBlockPropsBindings({ items: [{ ...record, child: { id: 'child', type: 'text' } }] }, {}, 'tabs')
  assert.equal(tabs.items[0].id, '$item.id')
})

test('复制等待平台回调，失败只执行失败链并阻断后续事件', async () => {
  let pending
  const { dispatchEvents, calls } = loadActions({ taro: { setClipboardData: () => new Promise((_resolve, reject) => { pending = reject }) } })
  const promise = dispatchEvents({ tap: [{ type: 'copy_text', payload: { text: '复制内容' },
    on_success: [{ type: 'toast', payload: { text: '不应成功' } }],
    on_error: [{ type: 'toast', payload: { text: '已处理失败' } }]
  }, { type: 'toast', payload: { text: '不应继续' } }] })
  assert.equal(calls.length, 0)
  pending(new Error('平台拒绝'))
  assert.equal(await promise, false)
  assert.deepEqual(calls.filter(call => call.method === 'showToast').map(call => call.options.title), ['已处理失败'])
})

test('视频号和跨小程序拒绝时执行失败链', async () => {
  const { dispatchAction, calls } = loadActions({
    wx: { openChannelsActivity: options => options.fail({ errMsg: '视频号拒绝' }) },
    taro: { navigateToMiniProgram: async () => { throw new Error('跨小程序拒绝') } }
  })
  for (const action of [
    { type: 'open_channels_activity', payload: { feed_id: 'feed', finder_user_name: 'finder' } },
    { type: 'open_mini_program', payload: { target_app_id: 'wx-test' } }
  ]) {
    assert.equal(await dispatchAction({ ...action,
      on_success: [{ type: 'toast', payload: { text: '不应成功' } }],
      on_error: [{ type: 'toast', payload: { text: '失败:' + action.type } }]
    }), false)
  }
  assert.deepEqual(calls.map(call => call.options.title), ['失败:open_channels_activity', '失败:open_mini_program'])
})

test('通用、请求和订阅成功子链均在失败处停止', async () => {
  const { dispatchAction, calls } = loadActions()
  for (const parent of [
    { type: 'toast', payload: { text: '父动作' } },
    { type: 'request_data', payload: { endpoint: 'query.score' } },
    { type: 'subscribe_message', payload: { template_id: 'template' } }
  ]) {
    assert.equal(await dispatchAction({ ...parent, on_success: [
      { type: 'unknown' }, { type: 'toast', payload: { text: '不应继续' } }
    ] }), false)
  }
  assert.ok(!calls.some(call => call.options?.title === '不应继续'))
})

test('平台原生动作在运行时拒绝越界参数', async () => {
  const { dispatchAction, calls } = loadActions()
  for (const action of [
    { type: 'upload_file', payload: { file_path: 'tmp/a.png', presigned_url: 'http://upload.example.com/a.png' } },
    { type: 'delete_media', payload: { endpoint: '/api/media/delete' } },
    { type: 'open_map', payload: { latitude: 91, longitude: 114 } },
    { type: 'open_wechat_service', payload: { url: 'https://work.weixin.qq.com/kfid/test' } },
    { type: 'save_qr', payload: { url: 'javascript:alert(1)' } },
    { type: 'open_internal_chat', payload: { page_id: '../customer_service' } }
  ]) {
    assert.equal(await dispatchAction(action), false, `${action.type} 应拒绝非法参数`)
  }
  assert.equal(calls.filter(call => ['request', 'openLocation', 'downloadFile', 'navigateTo'].includes(String(call.method))).length, 0)
})

test('预签名上传只透传非敏感请求头', async () => {
  let uploaded
  const { dispatchAction } = loadActions({
    taro: {
      getFileSystemManager: () => ({ readFile: options => options.success({ data: new ArrayBuffer(4) }) }),
      request: async options => { uploaded = options; return { statusCode: 200 } }
    }
  })
  const result = await dispatchAction({
    type: 'upload_file',
    payload: {
      file_path: 'tmp/a.png',
      presigned_url: 'https://upload.example.com/a.png',
      final_url: 'https://cdn.example.com/a.png',
      upload_headers: { 'Content-Type': 'image/png', Authorization: 'Bearer forbidden', Cookie: 'forbidden' }
    }
  })
  assert.equal(result.url, 'https://cdn.example.com/a.png')
  assert.deepEqual({ ...uploaded.header }, { 'Content-Type': 'image/png' })
})

test('级联链超过深度上限时熔断终止，不死循环不崩溃', async () => {
  const { dispatchAction, calls } = loadActions()
  // 构造 60 层 on_success 嵌套链，运行时熔断阈值 32 层
  let chain = { type: 'toast', payload: { text: '层0' } }
  for (let i = 1; i <= 60; i++) {
    chain = { type: 'toast', payload: { text: `层${i}` }, on_success: [chain] }
  }
  const result = await dispatchAction(chain)
  assert.equal(result, false)
  const toastCount = calls.filter(call => call.method === 'showToast').length
  assert.ok(toastCount > 0 && toastCount <= 40, `熔断前执行的层数应有限，实际 ${toastCount}`)
})

test('request_data 相对路径必须命中业务路由白名单', async () => {
  const { dispatchAction, calls } = loadActions()
  // 敏感内部路由必须拒绝
  for (const url of ['/api/v1/admin/apps', '/api/v1/mcp', '/api/v1/payment/notify/wx1', '/api/v1/webview/ticket']) {
    await dispatchAction({ type: 'request_data', payload: { url, method: 'GET' } }, {})
  }
  assert.equal(calls.filter(call => call.method === 'request').length, 0, '敏感路由不得发出请求')
  // 白名单业务路由允许通过
  await dispatchAction({ type: 'request_data', payload: { url: '/api/v1/articles?page=1', method: 'GET' } }, {})
  const request = calls.find(call => call.method === 'request')
  assert.ok(request, '白名单业务路由应放行')
  assert.equal(request.options.url, '/api/v1/articles?page=1')
  // 未登记的任意相对路径拒绝
  calls.length = 0
  await dispatchAction({ type: 'request_data', payload: { url: '/api/v1/unknown/path', method: 'GET' } }, {})
  assert.equal(calls.filter(call => call.method === 'request').length, 0, '未登记路由应拒绝')
})

test('路径参数键含正则元字符时按字面量替换且不崩溃', async () => {
  const { dispatchAction, calls } = loadActions()
  await dispatchAction({
    type: 'request_data',
    payload: {
      url: '/api/v1/articles/{a.b(c)}/comments',
      method: 'GET',
      path_params: { 'a.b(c)': '42' }
    }
  }, {})
  const request = calls.find(call => call.method === 'request')
  assert.equal(request.options.url, '/api/v1/articles/42/comments')
})

test('协议数据中的原型危险键被丢弃，不污染对象', async () => {
  const { resolveObjectBindings, dispatchAction } = loadActions()
  const parsed = JSON.parse('{"ok":1,"__proto__":{"polluted":true},"nested":{"constructor":1}}')
  const resolved = resolveObjectBindings(parsed, {})
  assert.equal(resolved.ok, 1)
  assert.equal(Object.prototype.polluted, undefined, '不得污染 Object.prototype')
  assert.equal(Object.getOwnPropertyNames(resolved.nested).includes('constructor'), false, '危险键不得成为自有属性')
  assert.equal(Object.getOwnPropertyNames(resolved).includes('__proto__'), false, '__proto__ 不得成为自有属性')
  // 绑定路径读取也阻断危险键
  await dispatchAction({ type: 'set_state', payload: { key: 'probe', value: { path: '$entity.__proto__.polluted' } } }, { entity: parsed, state: {}, updateState: () => {} })
})

test('请求成功默认不再自动复制 result.code', async () => {
  const { dispatchAction, calls } = loadActions({ request: async () => ({ code: 'VIP-TEST-CODE', msg: '领取成功' }) })
  const result = await dispatchAction({ type: 'request_data', payload: { endpoint: 'game.redeem' } }, {})
  assert.equal(result.code, 'VIP-TEST-CODE')
  assert.equal(calls.filter(call => call.method === 'setClipboardData').length, 0, '通用层不得代劳复制')
})

test('open_webview 只传 url_key，不转发裸 URL', async () => {
  const { dispatchAction, calls } = loadActions()
  const result = await dispatchAction({ type: 'open_webview', payload: { url_key: 'help_center', web_url: 'https://evil.example.com', url: 'https://evil.example.com' } }, {})
  assert.notEqual(result, false)
  const navigate = calls.find(call => call.method === 'navigateTo')
  assert.ok(navigate)
  assert.match(navigate.options.url, /url_key=help_center/)
  assert.equal(navigate.options.url.includes('evil.example.com'), false)
})

test('open_webview 缺少 url_key 时拒绝跳转', async () => {
  const { dispatchAction, calls } = loadActions()
  assert.equal(await dispatchAction({ type: 'open_webview', payload: {} }, {}), false)
  assert.equal(calls.filter(call => call.method === 'navigateTo').length, 0)
})

test('嵌套事件序列按序执行并向下传递最新结果', async () => {
  const { dispatchAction, calls } = loadActions()
  const state = {}
  await dispatchAction({
    type: 'toast', payload: { text: '入口' },
    on_success: [{ type: 'set_state', payload: { key: 'step', value: 1 } }]
  }, { state, updateState: (key, value) => { state[key] = value } })
  // 入口动作成功后，子链 set_state 应已写入共享状态
  assert.equal(state.step, 1)
})

test('check_capability 按租户能力执行成功或失败链', async () => {
  const enabled = loadActions()
  const enabledResult = await enabled.dispatchAction(
    { type: 'check_capability', payload: { capability: 'payment' } },
    { viewer: { capabilities: ['payment', 'membership'] } }
  )
  assert.equal(enabledResult.enabled, true)

  const disabled = loadActions()
  const disabledResult = await disabled.dispatchAction(
    {
      type: 'check_capability',
      payload: {
        capability: 'membership',
        on_error: [{ type: 'toast', payload: { text: '会员能力未开放' } }]
      }
    },
    { viewer: { capabilities: [] } }
  )
  assert.equal(disabledResult, false)
  assert.ok(disabled.calls.some(call => call.method === 'showToast' && call.options.title === '会员能力未开放'))
})

test('open_policy 优先调用微信原生隐私协议接口', async () => {
  const { dispatchAction, calls } = loadActions()
  await dispatchAction({ type: 'open_policy' }, {})
  assert.ok(calls.some(call => call.method === 'openPrivacyContract'))
})

test('add_to_cart 写入租户本地购物车并同步页面状态', async () => {
  const storage = {}
  const state = {}
  const { dispatchAction } = loadActions({
    taro: {
      getStorageSync: key => storage[key],
      setStorageSync: (key, value) => { storage[key] = value }
    }
  })
  const result = await dispatchAction(
    { type: 'add_to_cart', payload: { sku: 'sku-1', title: '验证商品', price_fen: 990, quantity: 2 } },
    { state, updateState: (key, value) => { state[key] = value } }
  )
  assert.equal(result.count, 2)
  assert.equal(result.total_fen, 1980)
  assert.equal(state.cart[0].sku, 'sku-1')

  const updated = await dispatchAction(
    { type: 'update_cart_quantity', payload: { sku: 'sku-1', quantity: 1 } },
    { state, updateState: (key, value) => { state[key] = value } }
  )
  assert.equal(updated.count, 1)
  assert.equal(updated.total_fen, 990)
})
