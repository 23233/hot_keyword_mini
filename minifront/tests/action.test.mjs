// action.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'
import ts from 'typescript'

// 编译实际源码，仅替换平台依赖；不复制绑定或动作实现。
function loadActions() {
  const cache = new Map()
  const calls = []
  const taro = new Proxy({ ENV_TYPE: { WEAPP: 'WEAPP' }, getEnv: () => 'WEAPP' }, {
    get(target, key) { return target[key] || (async (options) => { calls.push({ method: key, options }); return {} }) }
  })
  const load = (name) => {
    if (cache.has(name)) return cache.get(name)
    const exports = {}
    cache.set(name, exports)
    const source = readFileSync(new URL(`../src/utils/${name}.ts`, import.meta.url), 'utf8')
    const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
    vm.runInNewContext(js, {
      exports, console, setTimeout, wx: {},
      require: (id) => {
        if (id === '@tarojs/taro') return { default: taro }
        if (id === './auth') return { ensureSession: async () => true }
        if (id === './request') return { request: async (options) => { calls.push({ method: 'request', options }); return { status: 'success' } } }
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
  }, { item: { id: 'row-1', name: '业务项' }, state: {} })
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
