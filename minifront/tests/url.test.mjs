// url.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'
import ts from 'typescript'

// 编译 URL/WebView 工具，仅替换模块加载，不复制实现。
function loadUtils() {
  const cache = new Map()
  const load = (name) => {
    if (cache.has(name)) return cache.get(name)
    const exports = {}
    cache.set(name, exports)
    const source = readFileSync(new URL(`../src/utils/${name}.ts`, import.meta.url), 'utf8')
    const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
    vm.runInNewContext(js, {
      exports,
      require: (id) => {
        if (id === './url') return load('url')
        throw new Error(`未声明的测试依赖: ${id}`)
      }
    })
    return exports
  }
  return { ...load('url'), ...load('webview') }
}

test('分享 URL 保留原参数并覆盖 from', () => {
  const { setUrlQueryParams } = loadUtils()
  assert.equal(setUrlQueryParams('/pages/dynamic/index?page_id=home', { from: 'share' }), '/pages/dynamic/index?page_id=home&from=share')
  assert.equal(setUrlQueryParams('/pages/dynamic/index?page_id=home&from=old#x', { from: 'timeline' }), '/pages/dynamic/index?page_id=home&from=timeline#x')
  assert.equal(setUrlQueryParams('/pages/webview/index', { url_key: 'a b', from: 'share' }), '/pages/webview/index?url_key=a%20b&from=share')
})

test('朋友圈 query 不产生多余问号', () => {
  const { setQueryStringParams } = loadUtils()
  assert.equal(setQueryStringParams('page_id=home&from=timeline', { from: 'share' }), 'page_id=home&from=share')
  assert.equal(setQueryStringParams('', { page_id: 'home' }), 'page_id=home')
})

test('WebView 字体与主题参数追加且保留票据', () => {
  const { buildWebViewDisplayURL } = loadUtils()
  const url = buildWebViewDisplayURL('https://wx.example.com/page?webview_ticket=abc', { fontSizeSetting: 21, theme: 'light_clean' })
  assert.match(url, /webview_ticket=abc/)
  assert.match(url, /sdui_font_size=21/)
  assert.match(url, /sdui_font_scale=1\.31/)
  assert.match(url, /sdui_theme=light_clean/)
  const capped = buildWebViewDisplayURL('https://wx.example.com/page', { fontSizeSetting: 40 })
  assert.match(capped, /sdui_font_scale=1\.35/)
})
