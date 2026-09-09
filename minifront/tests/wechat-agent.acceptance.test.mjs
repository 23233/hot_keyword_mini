// wechat-agent.acceptance.test.mjs
import assert from 'node:assert/strict'
import { execFileSync, execSync } from 'node:child_process'
import { existsSync, readFileSync, statSync } from 'node:fs'
import test from 'node:test'
import path from 'node:path'
import ts from 'typescript'

const projectRoot = path.resolve(import.meta.dirname, '..')
const distRoot = path.join(projectRoot, 'dist')
const wechatProjectRoot = projectRoot
const cliPath = process.env.WECHAT_DEVTOOLS_CLI || 'C:\\Program Files (x86)\\Tencent\\微信web开发者工具\\cli.bat'
const ideCliPath = process.env.WECHAT_IDE_CLI || 'C:\\Program Files (x86)\\Tencent\\微信web开发者工具\\wechatide.cmd'
const required = process.env.WECHAT_AGENT_REQUIRED === '1'
const ideRequired = process.env.WECHAT_IDE_REQUIRED === '1'

function runAgent(args) {
  return execFileSync(process.env.ComSpec || 'cmd.exe', ['/d', '/c', cliPath, ...args], {
    cwd: projectRoot,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

function runWechatIDE(args) {
  const quote = (value) => {
    const text = String(value)
    return /[\s"&|<>]/.test(text) ? `"${text.replaceAll('"', '\\"')}"` : text
  }
  const commandLine = ['wechatide.cmd', '-c', process.env.WECHAT_IDE_CLIENT || 'codex', ...args.map(quote)].join(' ')
  return execSync(commandLine, {
    cwd: path.dirname(ideCliPath),
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
}

function runWechatIDERead(args) {
  const commandArgs = [...args]
  const selectorIndex = commandArgs.indexOf('--selector')
  const isElementRead = commandArgs.includes('automation_element_action') && commandArgs.includes('--action')
    && commandArgs[commandArgs.indexOf('--action') + 1] === 'outerWxml'
  if (isElementRead && selectorIndex >= 0 && !commandArgs.includes('--wait-for-selector')) {
    commandArgs.push('--wait-for-selector', commandArgs[selectorIndex + 1], '--wait', '2')
  }
  let lastOutput = ''
  for (let attempt = 0; attempt < 4; attempt += 1) {
    try {
      lastOutput = runWechatIDE(commandArgs)
    } catch (error) {
      lastOutput = `${error.stdout || ''}\n${error.stderr || ''}`
      if (!/timeout waiting for automator response|no such element/i.test(lastOutput)) {
        throw error
      }
    }
    if (!/timeout waiting for automator response|no such element/i.test(lastOutput)) {
      return lastOutput
    }
  }
  return lastOutput
}

function parseWechatIDE(output, tool) {
  assert.match(output, new RegExp(`\\"tool\\": \\"${tool}\\"`), `${tool} 未返回工具结果`)
  const starts = [...output.matchAll(/\{\s*"ok"\s*:/gs)].map((match) => match.index ?? -1)
  for (const jsonStart of starts.reverse()) {
    let depth = 0
    let inString = false
    let escaped = false
    let jsonEnd = -1
    for (let index = jsonStart; index < output.length; index += 1) {
      const char = output[index]
      if (inString) {
        if (escaped) escaped = false
        else if (char === '\\') escaped = true
        else if (char === '"') inString = false
        continue
      }
      if (char === '"') inString = true
      else if (char === '{') depth += 1
      else if (char === '}' && --depth === 0) {
        jsonEnd = index + 1
        break
      }
    }
    if (jsonEnd < 0) continue
    try {
      const parsed = JSON.parse(output.slice(jsonStart, jsonEnd))
      if (parsed.tool === tool) {
        assert.equal(parsed.ok, true, `${tool} MCP 调用失败`)
        return parsed
      }
    } catch {
      // CLI 日志可能包含多个 JSON 片段，继续尝试下一个完整结果。
    }
  }
  assert.fail(`${tool} 未返回可解析的 MCP JSON 结果: ${output.slice(0, 1000)}`)
}

function toolResult(output, tool) {
  return parseWechatIDE(output, tool).result
}

function readWechatNetwork(command) {
  let last = ''
  for (let attempt = 0; attempt < 4; attempt += 1) {
    const result = toolResult(runWechatIDE(['get_simulator_network', '--project', wechatProjectRoot, '--command', command]), 'get_simulator_network')
    last = typeof result === 'string' ? result : JSON.stringify(result)
    if (/HTTP_RESPONSE|\"status\":\s*200/.test(last)) return last
  }
  return last
}

function removeWechatIDEInternalErrors(value) {
  return String(value || '').split('\n--\n').filter((entry) => (
    !entry.includes('inspectee MPPage.getCurrent error')
    && !entry.includes('routeDone with a webviewId')
    && !entry.includes('appLaunch with non-empty page stack')
  )).join('\n')
}

function currentPageEventually(predicate) {
  let page
  for (let attempt = 0; attempt < 3; attempt += 1) {
    page = toolResult(runWechatIDERead(['automation_runtime_info', '--project', wechatProjectRoot, '--action', 'currentPage']), 'automation_runtime_info').currentPage
    if (predicate(page)) return page
  }
  return page
}

function evaluateRuntime(fn) {
  const source = ts.transpileModule(`const fn = ${fn.toString()}`, { compilerOptions: { target: ts.ScriptTarget.ES2017 } }).outputText
    .replace(/^const fn = /, '').replace(/;\s*$/, '').replace(/\r?\n/g, ' ')
  const output = toolResult(runWechatIDE([
    'automation_evaluate', '--project', wechatProjectRoot, '--fn-source', source
  ]), 'automation_evaluate')
  assert.equal(output.success, true)
  return output.result.result
}

function tapAction(type) {
  assert.equal(toolResult(runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap',
    '--selector', `#lab_action_${type} .capsule-btn`, '--wait', '1'
  ]), 'automation_element_action').success, true)
}

function labDom() {
  return String(toolResult(runWechatIDERead([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-container-block'
  ]), 'automation_element_action'))
}

test('微信 MCP 独立动作与失败分支验收', { skip: !ideRequired }, async () => {
  runWechatIDE(['open_project_window', '--project', wechatProjectRoot, '--window-mode', 'liteMode'])
  const pageId = process.env.WECHAT_AGENT_PAGE_ID || 'component_lab_acceptance_20260907'
  const backend = await fetch(`http://127.0.0.1:8080/api/v1/page/${pageId}`, { headers: { 'X-WX-AppID': 'wx516563cfe994bbc6' } }).then(response => response.json())
  assert.ok(backend.page, '后端未返回验收页面')
  runWechatIDE(['simulator_open_page', '--project', wechatProjectRoot, '--page', 'pages/dynamic/index', '--query', `page_id=${pageId}`])
  assert.match(labDom(), /lab_action_request_payment/)
  // 仅替换平台边界。SDUI 页面、真实点击、状态和 query.score 请求仍走生产实现。
  evaluateRuntime(function () {
    const app = getApp()
    app.sduiAcceptance = { originals: {}, calls: [] }
    for (const method of ['setClipboardData', 'showToast', 'showShareMenu', 'previewImage', 'navigateToMiniProgram', 'openChannelsActivity', 'requestSubscribeMessage']) {
      app.sduiAcceptance.originals[method] = wx[method]
      wx[method] = function (options) {
        app.sduiAcceptance.calls.push({ method: method, options: JSON.parse(JSON.stringify(options)) })
        const denied = method === 'requestSubscribeMessage'
        const result = { errMsg: method + (denied ? ':fail 验收受控拒绝' : ':ok') }
        if (denied && options.fail) options.fail(result)
        if (!denied && options.success) options.success(result)
        if (options.complete) options.complete(result)
      }
    }
    app.sduiAcceptance.originals.request = wx.request
    wx.request = function (options) {
      if (options.url.indexOf('/api/v1/payment/orders') >= 0) {
        app.sduiAcceptance.calls.push({ method: 'paymentOrder', options: JSON.parse(JSON.stringify(options)) })
        options.success({ statusCode: 400, data: { code: 400, msg: '验收拒绝创建支付订单' } })
        return { abort: function () {} }
      }
      return app.sduiAcceptance.originals.request.call(wx, options)
    }
    return true
  })
  try {
    for (const type of ['copy_text', 'toast', 'set_state', 'toggle_state', 'set_state', 'reset_state', 'show_loading_state', 'show_empty_state', 'show_error_state', 'reset_block_state', 'show_error_state', 'refresh', 'request', 'request_data', 'require_auth', 'share', 'preview_image', 'open_mini_program', 'open_channels_activity', 'subscribe_message', 'request_payment']) {
      tapAction(type)
      const dom = labDom()
      const denied = ['subscribe_message', 'request_payment'].includes(type)
      assert.ok(dom.includes(`${denied ? '失败' : '通过'}:${type}`), `${type} 未完成对应成功/失败分支`)
      if (type === 'set_state') assert.match(dom, /show_extended 已开启/)
      if (type === 'toggle_state' || type === 'reset_state') assert.doesNotMatch(dom, /show_extended 已开启/)
      if (type === 'show_empty_state') assert.match(dom, /暂无实验数据/)
      if (type === 'show_error_state') assert.match(dom, /状态请求失败/)
      if (type === 'reset_block_state' || type === 'refresh') assert.match(dom, /异步状态容器/)
      if (type === 'request' || type === 'request_data') assert.match(dom, /请求 success/)
    }
    const calls = evaluateRuntime(function () { return getApp().sduiAcceptance.calls })
    assert.equal(calls.find(call => call.method === 'setClipboardData').options.data, 'SDUI-COPY')
    assert.equal(calls.find(call => call.method === 'navigateToMiniProgram').options.appId, 'wx0000000000000000')
    assert.equal(calls.find(call => call.method === 'openChannelsActivity').options.feedId, 'export/component-lab')
    assert.deepEqual(calls.find(call => call.method === 'requestSubscribeMessage').options.tmplIds, ['lab_notice_template'])
    assert.equal(calls.find(call => call.method === 'paymentOrder').options.data.sku, 'sdui-lab-sku')
    assert.ok(calls.some(call => call.method === 'showShareMenu'))
    assert.match(calls.find(call => call.method === 'previewImage').options.urls[0], /assets\/sdui-component-lab.png$/)
    const expected = await fetch('http://127.0.0.1:8080/api/v1/action/execute', {
      method: 'POST', headers: { 'X-WX-AppID': 'wx516563cfe994bbc6', 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: 'query.score', payload: { query_value: 'SDUI-QUERY' } })
    }).then(response => response.json())
    assert.equal(expected.data.status, 'success')
  } finally {
    evaluateRuntime(function () {
      const app = getApp()
      for (const method of Object.keys(app.sduiAcceptance.originals)) wx[method] = app.sduiAcceptance.originals[method]
      delete app.sduiAcceptance
      return true
    })
  }
  tapAction('navigate_page')
  assert.equal(currentPageEventually(page => page.path === 'pages/index/index').path, 'pages/index/index')
  runWechatIDE(['simulator_open_page', '--project', wechatProjectRoot, '--page', 'pages/dynamic/index', '--query', `page_id=${pageId}`])
  assert.match(labDom(), /lab_action_open_webview/)
  tapAction('open_webview')
  const webview = currentPageEventually(page => page.path === 'pages/webview/index')
  assert.equal(webview.path, 'pages/webview/index')
  assert.equal(decodeURIComponent(webview.query.url), 'https://example.com')
})

test('微信开发者工具 MCP 模拟器复杂 SDUI 渲染验收', { skip: !ideRequired }, () => {
  assert.ok(existsSync(cliPath), `未找到微信开发者工具 CLI: ${cliPath}`)
  assert.ok(existsSync(ideCliPath), `未找到 wechatide CLI: ${ideCliPath}`)
  assert.ok(existsSync(path.join(distRoot, 'app.json')), '请先执行 pnpm run build:weapp')
  assert.match(readFileSync(path.join(distRoot, 'common.js'), 'utf8'), /http:\/\/127\.0\.0\.1:8080/, '构建产物未切换到本地 API')

  const open = runWechatIDE(['open_project_window', '--project', wechatProjectRoot, '--window-mode', 'liteMode'])
  assert.equal(toolResult(open, 'open_project_window').success, true)

  const page = runWechatIDE([
    'simulator_open_page', '--project', wechatProjectRoot, '--page', 'pages/dynamic/index',
    '--query', `page_id=${process.env.WECHAT_AGENT_PAGE_ID || 'component_lab_acceptance_20260907'}`
  ])
  assert.equal(toolResult(page, 'simulator_open_page').success, true)

  const screenshotPath = path.join(process.env.TEMP || process.cwd(), 'wechat-mcp-sdui-acceptance.png')
  const screenshot = runWechatIDE([
    'simulator_screenshot', '--project', wechatProjectRoot, '--optimize', 'false', '--path', screenshotPath, '--wait', '2'
  ])
  assert.equal(toolResult(screenshot, 'simulator_screenshot').success, true)
  assert.ok(existsSync(screenshotPath), '微信模拟器截图未生成')
  assert.ok(statSync(screenshotPath).size > 10_000, '微信模拟器截图为空或仍停留在错误页')

  const runtime = runWechatIDE(['automation_runtime_info', '--project', wechatProjectRoot, '--action', 'currentPage'])
  const runtimeResult = toolResult(runtime, 'automation_runtime_info')
  assert.equal(runtimeResult.success, true)
  assert.equal(runtimeResult.currentPage.path, 'pages/dynamic/index')

  const dom = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-container-block'
  ])
  const domResult = String(toolResult(dom, 'automation_element_action'))
  for (const expected of ['执行状态与事件链', '循环项 A', 'rich-text', 'sdui-img-inner', 'sdui-grid-layout-block', 'sdui-tabs-block', 'sdui-carousel-block']) {
    assert.match(domResult, new RegExp(expected), `真实 WXML 未渲染关键结构: ${expected}`)
  }
  // component_lab 页面覆盖协议注册的全部 Block；真实 WXML 必须逐类出现，避免仅通过 Go 校验而前端漏渲染。
  const registeredBlocks = [
    'stack', 'container', 'grid', 'tabs', 'carousel', 'list', 'spacer', 'text', 'rich_text', 'image', 'video',
    'notice', 'timeline', 'empty', 'skeleton', 'media_hero', 'resource_card', 'action_button', 'game_card', 'form',
    'episode_list', 'item_grid', 'score_panel', 'coupon_card', 'countdown', 'result_table', 'contact_card', 'map_card',
    'game_header', 'redeem_code_card', 'server_status', 'product_card', 'download_card', 'event_card', 'poll', 'feed_list',
    'category_nav', 'article_feed', 'article_detail', 'membership_plan_list', 'comment_thread', 'collection_nav',
    'content_feed', 'content_detail', 'offer_list', 'discussion_thread', 'custom', 'custom_block'
  ]
  // empty/skeleton 属于状态分支，custom_block 属于条件分支，分别在后续真实交互中验收。
  const normalBlockTypes = registeredBlocks.filter((type) => !['empty', 'skeleton', 'custom_block'].includes(type))
  const missingNormalBlocks = normalBlockTypes.filter((blockType) => !new RegExp(`type-${blockType}|sdui-${blockType.replaceAll('_', '-')}`).test(domResult))
  assert.deepEqual(missingNormalBlocks, [], `普通态真实 WXML 缺少 Block: ${missingNormalBlocks.join(', ')}`)
  assert.match(domResult, /http:\/\/127\.0\.0\.1:8080\/assets\/sdui-component-lab\.png/, '图片组件未复用本地 HTTP 静态资源')
  assert.doesNotMatch(domResult, /暂无图片内容/, '图片组件加载失败并降级为占位态')

  const networkResult = readWechatNetwork('grep component_lab_acceptance_20260907')
  assert.match(networkResult, /http:\/\/127\.0\.0\.1:8080\/api\/v1\/page\/component_lab_acceptance_20260907/, '模拟器未请求本地 SDUI 接口')
  assert.match(networkResult, /"X-WX-AppID":"wx516563cfe994bbc6"/, '模拟器请求缺少目标 AppID')
  assert.match(networkResult, /"X-Client-Capabilities":"[^"]*grid[^"]*tabs[^"]*carousel[^"]*"/, '模拟器请求缺少复杂布局能力声明')
  assert.match(networkResult, /"status":200/, '本地 SDUI 接口未返回 200')

  const consoleOutput = runWechatIDE(['get_simulator_console', '--project', wechatProjectRoot, '--command', 'grep error'])
  const consoleResult = toolResult(consoleOutput, 'get_simulator_console')
  // 微信开发者工具在页面切换时偶发输出 inspectee 自身诊断异常，不代表业务运行时错误。
  const appConsole = removeWechatIDEInternalErrors(consoleResult)
  assert.doesNotMatch(appConsole, /error|exception|failed/i, '模拟器 console 出现运行时错误')

  const stateTab = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap',
    '--selector', '.tab-index-1', '--wait', '1'
  ])
  assert.equal(toolResult(stateTab, 'automation_element_action').success, true)
  const stateTabDom = String(toolResult(runWechatIDERead([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.tabs-content-body'
  ]), 'automation_element_action'))
  assert.match(stateTabDom, /type-empty/, '状态 Tab 未渲染 empty Block')
  assert.match(stateTabDom, /type-skeleton/, '状态 Tab 未渲染 skeleton Block')

  const layoutTab = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap',
    '--selector', '.tab-index-0', '--wait', '1'
  ])
  assert.equal(toolResult(layoutTab, 'automation_element_action').success, true)

  const click = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap', '--selector', '#lab_state_actions .capsule-btn', '--wait', '2'
  ])
  assert.equal(toolResult(click, 'automation_element_action').success, true)

  const stateShot = runWechatIDE([
    'automation_viewport_action', '--project', wechatProjectRoot, '--action', 'pageScrollTo', '--scroll-top', '520'
  ])
  assert.equal(toolResult(stateShot, 'automation_viewport_action').success, true)

  const stateDom = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-container-block'
  ])
  const stateDomResult = String(toolResult(stateDom, 'automation_element_action'))
  assert.match(stateDomResult, /show_extended 已开启/, '点击后条件渲染状态未生效')
  assert.match(stateDomResult, /type-custom_block/, '点击后未渲染 custom_block 条件分支')

  const stateScreenshotPath = path.join(process.env.TEMP || process.cwd(), 'wechat-mcp-sdui-after-click.png')
  const stateScreenshot = runWechatIDE([
    'simulator_screenshot', '--project', wechatProjectRoot, '--optimize', 'false', '--path', stateScreenshotPath, '--wait', '1'
  ])
  assert.equal(toolResult(stateScreenshot, 'simulator_screenshot').success, true)
  assert.ok(statSync(stateScreenshotPath).size > 10_000, '点击后微信模拟器截图为空')
})

test('微信开发者工具 MCP AI 破甲首页与文章权限验收', { skip: !ideRequired }, () => {
  const home = runWechatIDE([
    'simulator_open_page', '--project', wechatProjectRoot, '--page', 'pages/index/index',
    '--query', `mcp_run=${Date.now()}`
  ])
  assert.equal(toolResult(home, 'simulator_open_page').success, true)

  const homeDom = String(toolResult(runWechatIDERead([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-home-blocks-container'
  ]), 'automation_element_action'))
  for (const expected of ['精选', '最新', '深度', '工具', '会员', 'AI 破甲：从热点信息到可验证结论', '会员专享：AI 产品拆解周报', '单篇解锁：AI 产品深度拆解', '单篇 ¥19.90']) {
    assert.match(homeDom, new RegExp(expected), `AI 破甲首页缺少真实内容: ${expected}`)
  }
  assert.match(homeDom, /http:\/\/127\.0\.0\.1:8080\/assets\/ai-editorial-cover\.png/, '首页未加载新的本地资讯封面')
  assert.doesNotMatch(homeDom, /今天发生了什么|可验证资讯导航|business_type|ai_breakthrough|模式:/, '首页泄露了开发提示词或内部协议字段')

  const homeNetwork = readWechatNetwork('grep /api/v1/page/home')
  assert.match(homeNetwork, /http:\/\/127\.0\.0\.1:8080\/api\/v1\/page\/home/, '首页未请求本地 SDUI 接口')
  assert.match(homeNetwork, /"X-WX-AppID":"wx516563cfe994bbc6"/, '首页请求缺少目标 AppID')
  assert.match(homeNetwork, /"status":200/, '首页 SDUI 接口未返回 200')

  const homeScreenshotPath = path.join(process.env.TEMP || process.cwd(), 'wechat-mcp-ai-home.png')
  assert.equal(toolResult(runWechatIDE([
    'simulator_screenshot', '--project', wechatProjectRoot, '--optimize', 'false', '--path', homeScreenshotPath, '--wait', '1'
  ]), 'simulator_screenshot').success, true)
  assert.ok(statSync(homeScreenshotPath).size > 10_000, 'AI 破甲首页截图为空')

  const paidClick = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap', '--selector', '.sdui-content-row', '--wait', '2'
  ])
  assert.equal(toolResult(paidClick, 'automation_element_action').success, true)
  const paidPage = currentPageEventually((page) => page?.query?.page_id === 'article_detail')
  assert.equal(paidPage.path, 'pages/dynamic/index')
  assert.equal(paidPage.query.page_id, 'article_detail')
  assert.match(String(paidPage.query.id), /^\d+$/)
  const paidDom = String(toolResult(runWechatIDERead([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-content-detail'
  ]), 'automation_element_action'))
  assert.match(paidDom, /免费试读/)
  assert.match(paidDom, /购买后阅读全文/)
  assert.match(paidDom, /购买全文/)

  assert.equal(toolResult(runWechatIDE(['simulator_open_page', '--project', wechatProjectRoot, '--page', 'pages/index/index']), 'simulator_open_page').success, true)
  assert.equal(currentPageEventually((page) => page?.path === 'pages/index/index').path, 'pages/index/index')
  const featuredClick = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap', '--selector', '.sdui-content-feature', '--wait', '2'
  ])
  assert.equal(toolResult(featuredClick, 'automation_element_action').success, true)
  const articlePage = currentPageEventually((page) => page?.query?.page_id === 'article_detail' && String(page?.query?.id) !== String(paidPage.query.id))
  assert.equal(articlePage.path, 'pages/dynamic/index')
  assert.equal(articlePage.query.page_id, 'article_detail')
  const articleDom = String(toolResult(runWechatIDERead([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-content-detail'
  ]), 'automation_element_action'))
  assert.match(articleDom, /AI 破甲编辑部 · 公开阅读/)
  assert.match(articleDom, /欢迎来到 AI 破甲/)
  assert.doesNotMatch(articleDom, /购买后阅读全文|会员专享内容/)

  const articleNetwork = readWechatNetwork('grep /api/v1/articles/')
  assert.match(articleNetwork, /"status":200/, '文章详情或评论接口未返回 200')
  const appConsole = removeWechatIDEInternalErrors(toolResult(runWechatIDE([
    'get_simulator_console', '--project', wechatProjectRoot, '--command', 'grep error'
  ]), 'get_simulator_console'))
  assert.doesNotMatch(appConsole, /error|exception|failed/i, 'AI 破甲页面 console 出现运行时错误')

  const articleScreenshotPath = path.join(process.env.TEMP || process.cwd(), 'wechat-mcp-ai-article.png')
  assert.equal(toolResult(runWechatIDE([
    'simulator_screenshot', '--project', wechatProjectRoot, '--optimize', 'false', '--path', articleScreenshotPath, '--wait', '1'
  ]), 'simulator_screenshot').success, true)
  assert.ok(statSync(articleScreenshotPath).size > 10_000, 'AI 破甲文章详情截图为空')
})

test('微信开发者工具 Agent 复杂 SDUI 渲染验收', { skip: !required }, () => {
  assert.ok(existsSync(cliPath), `未找到微信开发者工具 CLI: ${cliPath}`)
  assert.ok(existsSync(path.join(distRoot, 'app.json')), '请先执行 pnpm run build:weapp')

  const appConfig = JSON.parse(readFileSync(path.join(distRoot, 'app.json'), 'utf8'))
  assert.ok(appConfig.agent?.skills?.length, '构建产物缺少 agent.skills')

  const projectConfig = JSON.parse(readFileSync(path.join(distRoot, 'project.config.json'), 'utf8'))
  assert.equal(projectConfig.appid, process.env.WECHAT_AGENT_APPID || 'wx516563cfe994bbc6')

  runAgent(['agent', 'start', '--project', distRoot, '--trust-project', 'true'])
  const render = runAgent([
    'agent', 'render', '--project', distRoot, '--name', 'render',
    '--output', path.join(process.env.TEMP || process.cwd(), 'wechat-agent-sdui.png'),
    '--timeout', '60000', '--trust-project', 'true'
  ])
  assert.match(render, /"status":\s*"ok"/, '微信 Agent render 未返回成功')

  const dom = runAgent([
    'agent', 'get-dom', '--project', distRoot, '--target', 'card',
    '--full', '--timeout', '60000', '--trust-project', 'true'
  ])
  assert.match(dom, /"status":\s*"ok"/, '微信 Agent get-dom 未返回成功')

  const click = runAgent([
    'agent', 'card-click', '--project', distRoot,
    '--text', process.env.WECHAT_AGENT_CLICK_TEXT || '复制',
    '--wait-response', 'true', '--timeout', '60000', '--trust-project', 'true'
  ])
  assert.match(click, /"status":\s*"ok"/, '微信 Agent card-click 未返回成功')
})
