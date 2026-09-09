// wechat-agent.acceptance.test.mjs
import assert from 'node:assert/strict'
import { execFileSync, execSync } from 'node:child_process'
import { existsSync, readFileSync, statSync } from 'node:fs'
import test from 'node:test'
import path from 'node:path'

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

  const click = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'tap', '--selector', '.capsule-btn', '--wait', '2'
  ])
  assert.equal(toolResult(click, 'automation_element_action').success, true)

  const stateShot = runWechatIDE([
    'automation_viewport_action', '--project', wechatProjectRoot, '--action', 'pageScrollTo', '--scroll-top', '520'
  ])
  assert.equal(toolResult(stateShot, 'automation_viewport_action').success, true)

  const stateDom = runWechatIDE([
    'automation_element_action', '--project', wechatProjectRoot, '--action', 'outerWxml', '--selector', '.sdui-container-block'
  ])
  assert.match(String(toolResult(stateDom, 'automation_element_action')), /show_extended 已开启/, '点击后条件渲染状态未生效')

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
