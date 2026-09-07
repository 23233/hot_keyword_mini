// wechat-agent.acceptance.test.mjs
import assert from 'node:assert/strict'
import { execFileSync, execSync } from 'node:child_process'
import { existsSync, readFileSync, statSync } from 'node:fs'
import test from 'node:test'
import path from 'node:path'

const projectRoot = path.resolve(import.meta.dirname, '..')
const distRoot = path.join(projectRoot, 'dist')
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

test('微信开发者工具 MCP 模拟器复杂 SDUI 渲染验收', { skip: !ideRequired }, () => {
  assert.ok(existsSync(cliPath), `未找到微信开发者工具 CLI: ${cliPath}`)
  assert.ok(existsSync(ideCliPath), `未找到 wechatide CLI: ${ideCliPath}`)
  assert.ok(existsSync(path.join(distRoot, 'app.json')), '请先执行 pnpm run build:weapp')
  assert.match(readFileSync(path.join(distRoot, 'common.js'), 'utf8'), /http:\/\/127\.0\.0\.1:8080/, '构建产物未切换到本地 API')

  const open = runWechatIDE(['open_project_window', '--project', distRoot, '--window-mode', 'liteMode'])
  assert.equal(toolResult(open, 'open_project_window').success, true)

  const page = runWechatIDE([
    'simulator_open_page', '--project', distRoot, '--page', 'pages/dynamic/index',
    '--query', `page_id=${process.env.WECHAT_AGENT_PAGE_ID || 'component_lab_acceptance_20260907'}`
  ])
  assert.equal(toolResult(page, 'simulator_open_page').success, true)

  const screenshotPath = path.join(process.env.TEMP || process.cwd(), 'wechat-mcp-sdui-acceptance.png')
  const screenshot = runWechatIDE([
    'simulator_screenshot', '--project', distRoot, '--optimize', 'false', '--path', screenshotPath, '--wait', '2'
  ])
  assert.equal(toolResult(screenshot, 'simulator_screenshot').success, true)
  assert.ok(existsSync(screenshotPath), '微信模拟器截图未生成')
  assert.ok(statSync(screenshotPath).size > 10_000, '微信模拟器截图为空或仍停留在错误页')

  const runtime = runWechatIDE(['automation_runtime_info', '--project', distRoot, '--action', 'currentPage'])
  const runtimeResult = toolResult(runtime, 'automation_runtime_info')
  assert.equal(runtimeResult.success, true)
  assert.equal(runtimeResult.currentPage.path, 'pages/dynamic/index')

  const dom = runWechatIDE([
    'automation_element_action', '--project', distRoot, '--action', 'outerWxml', '--selector', '.sdui-container-block'
  ])
  const domResult = String(toolResult(dom, 'automation_element_action'))
  for (const expected of ['执行状态与事件链', '循环项 A', 'rich-text', 'sdui-img-inner', 'sdui-grid-layout-block', 'sdui-tabs-block', 'sdui-carousel-block']) {
    assert.match(domResult, new RegExp(expected), `真实 WXML 未渲染关键结构: ${expected}`)
  }
  assert.match(domResult, /http:\/\/127\.0\.0\.1:8080\/assets\/sdui-component-lab\.png/, '图片组件未复用本地 HTTP 静态资源')
  assert.doesNotMatch(domResult, /暂无图片内容/, '图片组件加载失败并降级为占位态')

  const networkOutput = runWechatIDE(['get_simulator_network', '--project', distRoot, '--command', 'grep component_lab_acceptance_20260907'])
  const networkResult = String(toolResult(networkOutput, 'get_simulator_network'))
  assert.match(networkResult, /http:\/\/127\.0\.0\.1:8080\/api\/v1\/page\/component_lab_acceptance_20260907/, '模拟器未请求本地 SDUI 接口')
  assert.match(networkResult, /"X-WX-AppID":"wx516563cfe994bbc6"/, '模拟器请求缺少目标 AppID')
  assert.match(networkResult, /"X-Client-Capabilities":"[^"]*grid[^"]*tabs[^"]*carousel[^"]*"/, '模拟器请求缺少复杂布局能力声明')
  assert.match(networkResult, /"status":200/, '本地 SDUI 接口未返回 200')

  const consoleOutput = runWechatIDE(['get_simulator_console', '--project', distRoot, '--command', 'grep error'])
  const consoleResult = toolResult(consoleOutput, 'get_simulator_console')
  assert.doesNotMatch(String(consoleResult || ''), /error|exception|failed/i, '模拟器 console 出现运行时错误')

  const click = runWechatIDE([
    'automation_element_action', '--project', distRoot, '--action', 'tap', '--selector', '.capsule-btn', '--wait', '2'
  ])
  assert.equal(toolResult(click, 'automation_element_action').success, true)

  const stateShot = runWechatIDE([
    'automation_viewport_action', '--project', distRoot, '--action', 'pageScrollTo', '--scroll-top', '520'
  ])
  assert.equal(toolResult(stateShot, 'automation_viewport_action').success, true)

  const stateDom = runWechatIDE([
    'automation_element_action', '--project', distRoot, '--action', 'outerWxml', '--selector', '.sdui-container-block'
  ])
  assert.match(String(toolResult(stateDom, 'automation_element_action')), /show_extended 已开启/, '点击后条件渲染状态未生效')

  const stateScreenshotPath = path.join(process.env.TEMP || process.cwd(), 'wechat-mcp-sdui-after-click.png')
  const stateScreenshot = runWechatIDE([
    'simulator_screenshot', '--project', distRoot, '--optimize', 'false', '--path', stateScreenshotPath, '--wait', '1'
  ])
  assert.equal(toolResult(stateScreenshot, 'simulator_screenshot').success, true)
  assert.ok(statSync(stateScreenshotPath).size > 10_000, '点击后微信模拟器截图为空')
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
