// acceptance-evidence.mjs
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const output = path.join(root, 'doc', 'acceptance')
fs.mkdirSync(output, { recursive: true })
const results = []
function run(name, command, args, env = {}) {
  const result = spawnSync(command, args, { cwd: root, encoding: 'utf8', env: { ...process.env, ...env }, maxBuffer: 32 * 1024 * 1024 })
  const tests = []
  if (command === 'go' && args.includes('-json')) {
    for (const line of result.stdout.split('\n')) {
      try {
        const event = JSON.parse(line)
        if (event.Test && ['pass', 'fail', 'skip'].includes(event.Action)) tests.push({ name: event.Test, package: event.Package, status: event.Action })
      } catch { /* Go 初始化信息不属于测试事件。 */ }
    }
  }
  results.push({ name, command: [command, ...args].join(' '), exitCode: result.status, tests })
  console.log(`${name}: ${result.status === 0 ? '通过' : '失败'}`)
}
run('Go测试', 'go', ['test', './...', '-count=1', '-json'])
run('独立数据库验收', 'go', ['test', './services', '-run', '^TestLocalDatabaseAcceptance$', '-count=1', '-json'], { ACCEPTANCE_MYSQL: '1' })
run('Go构建', 'go', ['build', './...'])
const revision = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).stdout.trim()
const tracked = spawnSync('git', ['diff', '--name-only'], { cwd: root, encoding: 'utf8' }).stdout.trim().split('\n').filter(Boolean)
const sources = [...new Set([...tracked, 'services/acceptance_database_test.go', 'scripts/acceptance-evidence.mjs'])]
const sourceHashes = Object.fromEntries(sources.filter(file => fs.existsSync(path.join(root,file))).map(file => [file,createHash('sha256').update(fs.readFileSync(path.join(root,file))).digest('hex')]))
fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ generatedAt: new Date().toISOString(), baseRevision: revision, modifiedFiles: tracked, sourceHashes, results }, null, 2) + '\n')

const baseline = 'SDUI通用能力平台开发与验收规范.md'
const lines = fs.readFileSync(path.join(root, 'doc', baseline), 'utf8').split(/\r?\n/)
const rows = []
let heading = '', fenced = false
for (let i = 0; i < lines.length; i++) {
  const line = lines[i]
  if (line.startsWith('```')) { fenced = !fenced; continue }
  if (fenced) continue
  if (/^#{2,3} /.test(line)) heading = line.replace(/^#+ /, '')
  const requirement = /^- /.test(line) || /^\d+\. /.test(line) || (line.startsWith('| ') && !line.includes('---') && !/^\|\s*-/.test(lines[i+1] || ''))
  if (!requirement || ['文档控制', '版本记录', '变更规则', '规范术语'].includes(heading)) continue
  const knownGap = /^6\.[1234]/.test(heading) || heading.startsWith('7.') || heading.startsWith('9.2')
  let status = knownGap ? '未完整满足，见问题清单' : '部分证据；尚不能判定整条通过'
  let evidence = '无完整执行证据'
  if (/^[12458]\.|^五、|^四、/.test(heading)) evidence = 'results.json：TestLocalDatabaseAcceptance、TestMCPToolScopeContract、TestDisabledCapabilityCannotPublish；仅覆盖所列子场景'
  if (/^2\.|^二、/.test(heading)) evidence = 'results.json：TestEnvelopeStructure、TestValidation_AllStandardBlocksAndActions、TestLayoutIR_BlockStateVariants；不含全量视觉'
  if (/^6\./.test(heading)) evidence = 'results.json：TestPlatformDomainStateMachines、支付回调与退款权益；完整领域不满足，见 G01-G05'
  if (/^7\./.test(heading)) evidence = '缺少本轮全量截图/哈希；见 G07'
  if (/^9\./.test(heading)) evidence = 'results.json 的构建与测试结果；交付物仍存在缺项'
  if (line.includes('Go 项目通过 `go test ./...`')) {
    status = results[0].exitCode === 0 ? '命令通过，跳过项单列' : '失败'
    evidence = 'results.json：Go测试；持久化项另见独立数据库验收'
  }
  if (line.includes('Go 项目通过 `go build ./...`')) {
    status = results[2].exitCode === 0 ? '通过' : '失败'
    evidence = 'results.json：Go构建'
  }
  if (heading.startsWith('十、')) { status = '外部配置或业务决策边界'; evidence = '本次不连接生产服务，不把外部配置视为代码验收通过' }
  rows.push(`| R${String(rows.length+1).padStart(3,'0')} | ${heading}（原文 ${i+1} 行） | ${line.replace(/^[-\d.]+ /,'').replaceAll('|','\\|')} | ${status} | ${evidence} |`)
}
const report = ['<!-- 逐条验收矩阵.md -->', '# 逐条验收矩阵', '',
  '本文件保留权威规范的每条列表要求和表格记录，包括范围、约束和交付物。results.json 保存本轮真实测试结果；测试通过不自动代表整条规范通过。当前结论：未达到全量验收。', '',
  '已执行的新增集成检查：配置更新保留首页、模板持久化、草稿 CAS、会话重放、WebView 租户隔离、双 AppID 支付回调/退款权益、双 AppID MCP 草稿发布闭环。见 results.json 中 TestLocalDatabaseAcceptance 子项。', '',
  '已知差距及修复记录见 [问题清单](../验收问题与修复记录.md)。微信与视觉结果沿用既有历史结论，本次未重新生成全量截图，不能算本轮全量视觉通过。', '',
  '| 编号 | 原文位置 | 要求原文 | 验收状态 | 证据及缺口 |', '| --- | --- | --- | --- | --- |', ...rows, ''].join('\n')
fs.writeFileSync(path.join(output, '逐条验收矩阵.md'), report)
console.log(`已记录 ${rows.length} 条规范记录；未将缺失证据标记为通过。`)
if (results.some(result => result.exitCode !== 0)) process.exitCode = 1
