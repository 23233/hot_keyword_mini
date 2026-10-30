// capture-sdui-visual.mjs
// 通过微信开发者工具 wechatide Skill 逐块滚动并截图，生成 SDUI 真机视觉证据与哈希清单。
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import path from 'node:path'

const projectRoot = path.resolve(import.meta.dirname, '..')
const now = new Date()
const visualDate = process.env.VISUAL_DATE || `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
const outputRoot = process.env.VISUAL_OUTPUT_DIR || path.resolve(projectRoot, `../doc/acceptance/visual/${visualDate}`)
const ideCli = process.env.WECHAT_IDE_CLI || 'C:\\Program Files (x86)\\Tencent\\微信web开发者工具\\wechatide.cmd'
const client = process.env.WECHAT_IDE_CLIENT || 'codex'
const appID = process.env.WECHAT_AGENT_APPID || 'wx516563cfe994bbc6'
const pageID = process.env.WECHAT_AGENT_PAGE_ID || 'component_lab_acceptance_20260907'

const targets = [
  { id: 'lab_form_v2', name: 'form-v2' },
  { id: 'lab_dialog', name: 'dialog', open: '#lab_dialog_trigger', close: '#lab_dialog_cancel' },
  { id: 'lab_action_sheet', name: 'action-sheet', open: '#lab_action_sheet_trigger', close: '#lab_action_sheet_cancel' },
  { id: 'lab_bottom_sheet', name: 'bottom-sheet', open: '#lab_bottom_sheet_trigger', close: '#lab_bottom_sheet_cancel' },
  { id: 'lab_sku_selector', name: 'sku-selector' },
  { id: 'lab_quantity_stepper', name: 'quantity-stepper' },
  { id: 'lab_cart_item_list', name: 'cart-item-list' },
  { id: 'lab_cart_summary', name: 'cart-summary' },
  { id: 'lab_address_book', name: 'address-book' },
  { id: 'lab_address_form', name: 'address-form' },
  { id: 'lab_coupon_list', name: 'coupon-list' },
  { id: 'lab_invoice_form', name: 'invoice-form' },
  { id: 'lab_filter_bar', name: 'filter-bar' },
  { id: 'lab_filter_sheet', name: 'filter-sheet', open: '#lab_filter_sheet_trigger', close: '#lab_filter_sheet_confirm' },
  { id: 'lab_rating', name: 'rating' },
  { id: 'lab_review_summary', name: 'review-summary' },
  { id: 'lab_profile_card', name: 'profile-card' },
  { id: 'lab_user_header', name: 'user-header' },
  // 状态页签：空态、缺货、加载与错误兜底，切换页签后统一截图
  { id: 'lab_tab_empty', name: 'state-empty', tab: '.tab-index-1' },
  { id: 'lab_tab_skeleton', name: 'state-skeleton', tab: '.tab-index-1' },
  { id: 'lab_state_cart_empty', name: 'state-cart-empty', tab: '.tab-index-1' },
  { id: 'lab_state_address_empty', name: 'state-address-empty', tab: '.tab-index-1' },
  { id: 'lab_state_coupon_empty', name: 'state-coupon-empty', tab: '.tab-index-1' },
  { id: 'lab_state_review_empty', name: 'state-review-empty', tab: '.tab-index-1' },
  { id: 'lab_state_sku_disabled', name: 'state-sku-sold-out', tab: '.tab-index-1' },
  { id: 'lab_state_form_loading_fallback', name: 'state-form-loading', tab: '.tab-index-1' },
  { id: 'lab_state_form_error_fallback', name: 'state-form-error', tab: '.tab-index-1' },
  { id: 'lab_state_long_text', name: 'state-long-text', tab: '.tab-index-1' },
  { id: 'lab_state_long_list', name: 'state-long-list', tab: '.tab-index-1' },
  { id: 'lab_state_review_live', name: 'state-review-live', tab: '.tab-index-1' },
  { id: 'lab_state_cart_quote', name: 'state-cart-quote', tab: '.tab-index-1' },
  { id: 'lab_state_coupon_live', name: 'state-coupon-live', tab: '.tab-index-1' },
  { id: 'lab_state_cart_quote_items', name: 'state-cart-quote-items', tab: '.tab-index-1' },
  { id: 'lab_state_async_error', name: 'state-async-error', tab: '.tab-index-1', open: '#lab_state_async_error_submit', wait: 2, mask: false, reload: true, after_id: 'lab_state_async_error_fallback' }
]

function run(tool, args) {
  const output = execFileSync(process.env.ComSpec || 'cmd.exe', ['/d', '/c', ideCli, '-c', client, tool, ...args], {
    cwd: path.dirname(ideCli),
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  })
  for (let index = output.length - 1; index >= 0; index -= 1) {
    if (output[index] !== '{') continue
    try {
      const parsed = JSON.parse(output.slice(index))
      if (parsed.tool === tool) {
        if (parsed.ok !== true) throw new Error(`${tool} 返回失败: ${output.slice(index)}`)
        return parsed.result
      }
    } catch (error) {
      if (error instanceof SyntaxError) continue
      throw error
    }
  }
  throw new Error(`${tool} 未返回可解析结果: ${output.slice(0, 800)}`)
}

function scrollToElement(selector) {
  // offset 依赖当前滚动位置；先回顶再测量，保证得到页面绝对位置。
  run('automation_viewport_action', ['--project', projectRoot, '--action', 'pageScrollTo', '--scroll-top', '0', '--wait', '1'])
  const offset = run('automation_element_action', ['--project', projectRoot, '--action', 'offset', '--selector', selector])
  const top = Number(offset?.top || 0)
  run('automation_viewport_action', ['--project', projectRoot, '--action', 'pageScrollTo', '--scroll-top', String(Math.max(0, Math.floor(top - 120))), '--wait', '1'])
}

function screenshot(name, waitSeconds = 1) {
  const file = path.join(outputRoot, `${name}.png`)
  const result = run('simulator_screenshot', ['--project', projectRoot, '--optimize', 'false', '--wait', String(waitSeconds), '--path', file])
  if (!existsSync(file) || statSync(file).size <= 0) throw new Error(`截图未生成: ${file}`)
  const hash = createHash('sha256').update(readFileSync(file)).digest('hex')
  return { file, width: result.imageWidth, height: result.imageHeight, sha256: hash }
}

mkdirSync(outputRoot, { recursive: true })
run('open_project_window', ['--project', projectRoot, '--window-mode', 'liteMode'])
run('simulator_refresh', ['--project', projectRoot])
run('simulator_open_page', ['--project', projectRoot, '--page', 'pages/dynamic/index', '--query', `page_id=${pageID}`])

const manifest = []
let activeTab = ''
for (const target of targets) {
  try {
    // 需要干净状态的动作用例先重载页面，避免上一次点击留下的块级状态污染截图
    if (target.reload) {
      run('automation_navigate', ['--project', projectRoot, '--action', 'reLaunch', '--url', `/pages/dynamic/index?page_id=${pageID}`, '--wait', '3'])
      activeTab = ''
    }
    // 状态页签等非默认视图需先切换页签，切换后同页签内继续截图
    if (target.tab && target.tab !== activeTab) {
      run('automation_element_action', ['--project', projectRoot, '--action', 'tap', '--selector', target.tab, '--wait', target.reload ? '3' : '1'])
      activeTab = target.tab
    }
    scrollToElement(`#${target.id}`)
    if (target.open) {
      run('automation_element_action', ['--project', projectRoot, '--action', 'tap', '--selector', target.open, '--wait', '1'])
      if (target.mask === false) {
        // 非弹层用例（如异步错误兜底）保持原位置，重新对齐目标块后再截图
        scrollToElement(`#${target.after_id || target.id}`)
      } else {
        // 开发者工具在页面滚动后会把 fixed 弹层按页面坐标定位；回顶后再截图以取得完整面板。
        run('automation_viewport_action', ['--project', projectRoot, '--action', 'pageScrollTo', '--scroll-top', '0', '--wait', '1'])
      }
      const opened = screenshot(`${target.name}-open`, target.wait || 1)
      manifest.push({ id: target.id, state: 'open', app_id: appID, page_id: pageID, ...opened })
      if (target.close) {
        try {
          run('automation_element_action', ['--project', projectRoot, '--action', 'tap', '--selector', target.close, '--wait', '1'])
        } catch (error) {
          console.warn(`[visual] 关闭 ${target.id} 失败: ${error.message}`)
        }
      }
    } else {
      const normal = screenshot(target.name)
      manifest.push({ id: target.id, state: target.tab ? 'variant' : 'normal', app_id: appID, page_id: pageID, ...normal })
    }
  } catch (error) {
    manifest.push({ id: target.id, state: 'error', error: error.message })
    console.error(`[visual] ${target.id} 截图失败:`, error.message)
  }
}

writeFileSync(path.join(outputRoot, 'manifest.json'), JSON.stringify({
  generatedAt: new Date().toISOString(),
  appID,
  pageID,
  device: process.env.VISUAL_DEVICE || '',
  items: manifest
}, null, 2))
console.log(`已生成 ${manifest.filter((item) => item.file).length} 张截图，清单: ${path.join(outputRoot, 'manifest.json')}`)