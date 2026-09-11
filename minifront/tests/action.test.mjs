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
        if (id === './request') return { request: async (options) => { calls.push({ method: 'request', options }); return overrides.request ? overrides.request(options) : { status: 'success' } } }
        if (id === '../config/env') return { resolveRemoteUrl: value => value }
        if (id === './condition') return load('condition')
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
